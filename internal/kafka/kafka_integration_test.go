//go:build integration

package kafka_test

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"

	cachemem "github.com/Gunvolt24/wb_l0/internal/cache/memory"
	ikafka "github.com/Gunvolt24/wb_l0/internal/kafka"
	"github.com/Gunvolt24/wb_l0/internal/ports"
	pgrepo "github.com/Gunvolt24/wb_l0/internal/repo/postgres"
	"github.com/Gunvolt24/wb_l0/internal/testutil"
	"github.com/Gunvolt24/wb_l0/internal/usecase"
	"github.com/Gunvolt24/wb_l0/pkg/logger"
	"github.com/Gunvolt24/wb_l0/pkg/validate"
)

var reUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safe(t *testing.T) string { return reUnsafe.ReplaceAllString(t.Name(), "-") }

// 1) Успешное сохранение в Kafka
func TestKafka_Valid_Saved_TC(t *testing.T) {
	// длинный контекст только на старт контейнеров
	ctxStart, cancelStart := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelStart()

	pg, stopPG, err := testutil.StartPostgresTC(ctxStart)
	require.NoError(t, err)
	t.Cleanup(func() { _ = stopPG(context.Background()) })

	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	kf, stopKF, err := testutil.StartKafkaTC(ctxStart, "orders-itc")
	require.NoError(t, err)
	t.Cleanup(func() { _ = stopKF(context.Background()) })

	// короткий контекст на сам тест
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// свой пул из DSN контейнера
	pool, err := pgxpool.New(ctx, pg.DSN)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// уникальные topic/group и явное создание топика
	topic, group := testutil.UniqueTopicAndGroup(kf.BaseTopic + "-" + safe(t))
	require.NoError(t, testutil.EnsureTopic(ctx, kf.Brokers[0], topic))

	// зависимости приложения
	logg, cleanup, err := logger.NewZapLogger(false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cleanup() })

	repo := pgrepo.NewOrderRepository(pool)
	svc := usecase.NewOrderService(repo, cachemem.NewLRUCacheTTL(100, time.Minute), logg, validate.NewOrderValidator())

	consumer := ikafka.NewConsumer(&ikafka.ConsumerConfig{
		Brokers:        kf.Brokers,
		Topic:          topic,
		GroupID:        group,
		StartOffset:    "first",
		ProcessTimeout: 5 * time.Second,
		RetryInitial:   200 * time.Millisecond,
		RetryMax:       2 * time.Second,
	}, svc, logg)

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() { _ = consumer.Run(runCtx) }()

	// даём консьюмеру присоединиться к группе/получить assignment
	time.Sleep(1500 * time.Millisecond)

	// генерим валидный уникальный заказ
	ord := testutil.MakeOrder()
	require.NoError(t, validate.NewOrderValidator().Validate(context.Background(), &ord))

	raw, _ := json.Marshal(ord)

	w := &kafka.Writer{
		Addr:         kafka.TCP(kf.Brokers...),
		Topic:        topic,
		RequiredAcks: kafka.RequireAll,
		Balancer:     &kafka.LeastBytes{},
	}
	defer w.Close()

	require.NoError(t, w.WriteMessages(ctx, kafka.Message{Value: raw}))

	// ждём появления в БД
	deadline := time.Now().Add(20 * time.Second)
	for {
		got, err := repo.GetByUID(ctx, ord.OrderUID)
		require.NoError(t, err)
		if got != nil {
			require.Equal(t, ord.OrderUID, got.OrderUID)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("order %s not saved in time", ord.OrderUID)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// 2) Не-JSON сообщение пропускается, валидное после него — сохраняется
func TestKafka_Skip_InvalidJSON_Then_SaveValid_TC(t *testing.T) {
	ctx, cancel, pool, repo, logg, cleanup, kf, _ := newStack(t)
	defer cancel()
	defer cleanup()

	topic, group := testutil.UniqueTopicAndGroup(kf.BaseTopic + "-invalid-json-" + safe(t))
	require.NoError(t, testutil.EnsureTopic(ctx, kf.Brokers[0], topic))

	// Консьюмер с обычным сервисом
	svc := usecase.NewOrderService(repo, cachemem.NewLRUCacheTTL(100, time.Minute), logg, validate.NewOrderValidator())
	consumer := ikafka.NewConsumer(&ikafka.ConsumerConfig{
		Brokers:        kf.Brokers,
		Topic:          topic,
		GroupID:        group,
		StartOffset:    "first",
		ProcessTimeout: 3 * time.Second,
		RetryInitial:   200 * time.Millisecond,
		RetryMax:       2 * time.Second,
	}, svc, logg)

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() { _ = consumer.Run(runCtx) }()

	time.Sleep(1500 * time.Millisecond)

	// 1) Шлём мусор
	writeMsg(t, ctx, kf.Brokers, topic, []byte("not-a-json"))

	// 2) Шлём валидный заказ
	ord := testutil.MakeOrder()
	raw, _ := json.Marshal(ord)
	writeMsg(t, ctx, kf.Brokers, topic, raw)

	// 3) Ждём появления валидного в БД, не валидный не должен попасть
	deadline := time.Now().Add(20 * time.Second)
	for {
		got, err := repo.GetByUID(ctx, ord.OrderUID)
		require.NoError(t, err)
		if got != nil {
			require.Equal(t, ord.OrderUID, got.OrderUID)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("order %s not saved in time", ord.OrderUID)
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = pool // чтобы линтер не ругался, pool нам не нужен дальше
}

// 3) Валидационная ошибка (entry пуст) пропускается; следующий валидный — сохраняется
func TestKafka_Skip_ValidationError_Then_SaveValid_TC(t *testing.T) {
	ctx, cancel, _, repo, logg, cleanup, kf, _ := newStack(t)
	defer cancel()
	defer cleanup()

	topic, group := testutil.UniqueTopicAndGroup(kf.BaseTopic + "-invalid-order-" + safe(t))
	require.NoError(t, testutil.EnsureTopic(ctx, kf.Brokers[0], topic))

	svc := usecase.NewOrderService(repo, cachemem.NewLRUCacheTTL(100, time.Minute), logg, validate.NewOrderValidator())
	consumer := ikafka.NewConsumer(&ikafka.ConsumerConfig{
		Brokers:        kf.Brokers,
		Topic:          topic,
		GroupID:        group,
		StartOffset:    "first",
		ProcessTimeout: 3 * time.Second,
		RetryInitial:   200 * time.Millisecond,
		RetryMax:       2 * time.Second,
	}, svc, logg)

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() { _ = consumer.Run(runCtx) }()

	time.Sleep(1500 * time.Millisecond)

	// 1) Валидный заказ, но испортим entry => валидация свалится
	bad := testutil.MakeOrder()
	bad.Entry = "" // триггер валидатора
	braw, _ := json.Marshal(bad)
	writeMsg(t, ctx, kf.Brokers, topic, braw)

	// 2) Следом валидный
	ok := testutil.MakeOrder()
	orab, _ := json.Marshal(ok)
	writeMsg(t, ctx, kf.Brokers, topic, orab)

	// 3) Ждём появления только валидного в БД
	deadline := time.Now().Add(20 * time.Second)
	for {
		got, err := repo.GetByUID(ctx, ok.OrderUID)
		require.NoError(t, err)
		if got != nil {
			require.Equal(t, ok.OrderUID, got.OrderUID)
			// убедимся, что испорченного нет
			gotBad, err := repo.GetByUID(ctx, bad.OrderUID)
			require.NoError(t, err)
			require.Nil(t, gotBad)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("order %s not saved in time", ok.OrderUID)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// 4) StartOffset="last": сообщения, опубликованные до старта консьюмера, игнорируются
func TestKafka_StartOffset_Last_IgnoresOld_TC(t *testing.T) {
	ctx, cancel, _, repo, logg, cleanup, kf, _ := newStack(t)
	defer cancel()
	defer cleanup()

	topic, group := testutil.UniqueTopicAndGroup(kf.BaseTopic + "-last-" + safe(t))
	require.NoError(t, testutil.EnsureTopic(ctx, kf.Brokers[0], topic))

	// 1) Публикуем "старое" ДО консьюмера
	old := testutil.MakeOrder()
	rold, _ := json.Marshal(old)
	writeMsg(t, ctx, kf.Brokers, topic, rold)

	// 2) Запускаем консьюмера с StartOffset="last"
	svc := usecase.NewOrderService(repo, cachemem.NewLRUCacheTTL(100, time.Minute), logg, validate.NewOrderValidator())
	consumer := ikafka.NewConsumer(&ikafka.ConsumerConfig{
		Brokers:     kf.Brokers,
		Topic:       topic,
		GroupID:     group,
		StartOffset: "last",
	}, svc, logg)

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() { _ = consumer.Run(runCtx) }()

	// 3) Публикуем новое несколько раз до появления в БД — так мы гарантируем, что одно из
	//    сообщений окажется после базовой позиции, с которой читает консьюмер.
	newOrd := testutil.MakeOrder()
	rnew, _ := json.Marshal(newOrd)

	deadline := time.Now().Add(20 * time.Second)
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		// публикуем повторно, пока не увидим сохранение
		writeMsg(t, ctx, kf.Brokers, topic, rnew)

		gotNew, err := repo.GetByUID(ctx, newOrd.OrderUID)
		require.NoError(t, err)
		if gotNew != nil {
			require.Equal(t, newOrd.OrderUID, gotNew.OrderUID)
			// и убеждаемся, что "старое" не попало
			gotOld, err := repo.GetByUID(ctx, old.OrderUID)
			require.NoError(t, err)
			require.Nil(t, gotOld)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("new order %s not saved in time", newOrd.OrderUID)
		}
		<-ticker.C
	}
}

// 5) At-least-once через рестарт: при временной ошибке и отсутствии коммита — передоставка после перезапуска
func TestKafka_Redelivery_AfterRestart_NoCommit_TC(t *testing.T) {
	ctxStart, cancelStart := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelStart()

	kf, stopKF, err := testutil.StartKafkaTC(ctxStart, "orders-itc")
	require.NoError(t, err)
	defer func() { _ = stopKF(context.Background()) }()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	logg, closer, err := logger.NewZapLogger(false)
	require.NoError(t, err)
	defer func() { _ = closer() }()

	topic, group := testutil.UniqueTopicAndGroup(kf.BaseTopic + "-redelivery-" + safe(t))
	require.NoError(t, testutil.EnsureTopic(ctx, kf.Brokers[0], topic))

	ord := testutil.MakeOrder()
	raw, _ := json.Marshal(ord)
	writeMsg(t, ctx, kf.Brokers, topic, raw)

	// Фаза 1: всегда временная ошибка => оффсет НЕ коммитится
	consumerFail := ikafka.NewConsumer(&ikafka.ConsumerConfig{
		Brokers:        kf.Brokers,
		Topic:          topic,
		GroupID:        group,
		StartOffset:    "first",
		ProcessTimeout: 300 * time.Millisecond, // короткий процесс-таймаут
		RetryInitial:   100 * time.Millisecond,
		RetryMax:       300 * time.Millisecond,
	}, alwaysTempFailSaver{}, logg)

	runCtx1, cancelRun1 := context.WithCancel(ctx)
	go func() { _ = consumerFail.Run(runCtx1) }()

	// Ждём немного, чтобы сообщение точно было Fetch'ed и обработка упала
	time.Sleep(2 * time.Second)
	cancelRun1() // выходим без коммита

	// Фаза 2: поднимаем PG и нормальный сервис
	pg, stopPG, err := testutil.StartPostgresTC(ctxStart)
	require.NoError(t, err)
	defer func() { _ = stopPG(context.Background()) }()
	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	pool, err := pgxpool.New(ctx, pg.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := pgrepo.NewOrderRepository(pool)
	svc := usecase.NewOrderService(repo, cachemem.NewLRUCacheTTL(100, time.Minute), logg, validate.NewOrderValidator())

	consumerOK := ikafka.NewConsumer(&ikafka.ConsumerConfig{
		Brokers:     kf.Brokers,
		Topic:       topic,
		GroupID:     group, // та же группа — перехватываем некоммиченное
		StartOffset: "first",
	}, svc, logg)

	runCtx2, cancelRun2 := context.WithCancel(ctx)
	defer cancelRun2()
	go func() { _ = consumerOK.Run(runCtx2) }()

	// Ждём появления заказа
	deadline := time.Now().Add(25 * time.Second)
	for {
		got, err := repo.GetByUID(ctx, ord.OrderUID)
		require.NoError(t, err)
		if got != nil {
			require.Equal(t, ord.OrderUID, got.OrderUID)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("order %s not redelivered/saved in time", ord.OrderUID)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// 6) Идемпотентность: дважды публикуем один и тот же заказ — в БД одна финальная запись
func TestKafka_Idempotent_DuplicateMessage_TC(t *testing.T) {
	ctx, cancel, _, repo, logg, cleanup, kf, _ := newStack(t)
	defer cancel()
	defer cleanup()

	topic, group := testutil.UniqueTopicAndGroup(kf.BaseTopic + "-dup-" + safe(t))
	require.NoError(t, testutil.EnsureTopic(ctx, kf.Brokers[0], topic))

	svc := usecase.NewOrderService(repo, cachemem.NewLRUCacheTTL(100, time.Minute), logg, validate.NewOrderValidator())
	consumer := ikafka.NewConsumer(&ikafka.ConsumerConfig{
		Brokers:     kf.Brokers,
		Topic:       topic,
		GroupID:     group,
		StartOffset: "first",
	}, svc, logg)

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() { _ = consumer.Run(runCtx) }()
	time.Sleep(1500 * time.Millisecond)

	ord := testutil.MakeOrder(testutil.WithItems(3))
	raw, _ := json.Marshal(ord)

	// Публикуем дважды подряд
	writeMsg(t, ctx, kf.Brokers, topic, raw)
	writeMsg(t, ctx, kf.Brokers, topic, raw)

	// Ждём и проверяем, что запись одна и итемы не «раздуты»
	deadline := time.Now().Add(20 * time.Second)
	for {
		got, err := repo.GetByUID(ctx, ord.OrderUID)
		require.NoError(t, err)
		if got != nil {
			require.Equal(t, ord.OrderUID, got.OrderUID)
			require.Len(t, got.Items, 3) // replace-логика items сохранила ровно 3
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("order %s not saved in time", ord.OrderUID)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// -----------------функции-помощники-----------------

func newStack(t *testing.T) (
	ctx context.Context,
	cancel func(),
	pool *pgxpool.Pool,
	repo *pgrepo.OrderRepository,
	logg ports.Logger,
	cleanup func(),
	kf *testutil.KafkaEnv,
	stopKF func(context.Context) error,
) {
	t.Helper()

	// Длинный контекст — на контейнеры
	ctxStart, cancelStart := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancelStart)

	pg, stopPG, err := testutil.StartPostgresTC(ctxStart)
	require.NoError(t, err)
	t.Cleanup(func() { _ = stopPG(context.Background()) })
	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	kf, stopKF, err = testutil.StartKafkaTC(ctxStart, "orders-itc")
	require.NoError(t, err)
	t.Cleanup(func() { _ = stopKF(context.Background()) })

	// Короткий контекст — сам тест
	ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)

	// Пул
	pool, err = pgxpool.New(ctx, pg.DSN)
	require.NoError(t, err)

	// Логгер (+ обёртка cleanup)
	var closer func() error
	logg, closer, err = logger.NewZapLogger(false)
	require.NoError(t, err)
	cleanup = func() { _ = closer() }

	repo = pgrepo.NewOrderRepository(pool)
	return
}

func writeMsg(t *testing.T, ctx context.Context, brokers []string, topic string, payload []byte) {
	t.Helper()
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		RequiredAcks: kafka.RequireAll,
		Balancer:     &kafka.LeastBytes{},
	}
	defer w.Close()
	require.NoError(t, w.WriteMessages(ctx, kafka.Message{Value: payload}))
}

// временная ошибка для имитации "временных сбоев"
type tmpError struct{ msg string }

func (e tmpError) Error() string   { return e.msg }
func (e tmpError) Temporary() bool { return true }

// сервис-заглушка, который всегда возвращает временную ошибку (чтобы не коммитить оффсет)
type alwaysFailSaver struct{}

func (alwaysFailSaver) SaveFromMessage(ctx context.Context, raw []byte) error {
	return tmpError{"temporary failure"}
}

// временная "сетеподобная" ошибка
type tempNetErr struct{}

func (tempNetErr) Error() string   { return "temporary failure" }
func (tempNetErr) Temporary() bool { return true }
func (tempNetErr) Timeout() bool   { return true } // как у net.Error

type alwaysTempFailSaver struct{}

func (alwaysTempFailSaver) SaveFromMessage(ctx context.Context, _ []byte) error {
	return tempNetErr{}
}
