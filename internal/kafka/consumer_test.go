package kafka

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/segmentio/kafka-go"

	"github.com/Gunvolt24/wb_l0/internal/kafka/mocks"
	"github.com/Gunvolt24/wb_l0/pkg/validate"
)

type nopLogger struct{}

func (nopLogger) Infof(context.Context, string, ...any)  {}
func (nopLogger) Warnf(context.Context, string, ...any)  {}
func (nopLogger) Errorf(context.Context, string, ...any) {}

// runAsync запускает Consumer.Run в отдельном горутине и возвращает канал с ошибкой.
func runAsync(ctx context.Context, c *Consumer) <-chan error {
	errCh := make(chan error, 1)
	go func() { errCh <- c.Run(ctx) }()
	return errCh
}

func newTestConsumer(r reader, s messageSaver) *Consumer {
	return &Consumer{
		reader: r, service: s, log: nopLogger{},
		processTimeout: 30 * time.Millisecond,
		retryInitial:   5 * time.Millisecond,
		retryMax:       10 * time.Millisecond,
		jitterRand:     rand.New(rand.NewSource(1)),
	}
}

// Успешная обработка + коммит
func TestRun_OK_Commits(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := mocks.NewMockreader(ctrl)
	s := mocks.NewMockmessageSaver(ctrl)

	rc := kafka.ReaderConfig{Topic: "orders", GroupID: "g1", Brokers: []string{"b:9092"}}
	r.EXPECT().Config().Return(rc).AnyTimes()
	// 1-й цикл: сообщение обрабатывается
	r.EXPECT().FetchMessage(gomock.Any()).
		Return(kafka.Message{Offset: 1, Value: []byte("ok")}, nil)
	s.EXPECT().SaveFromMessage(gomock.Any(), []byte("ok")).Return(nil)
	r.EXPECT().CommitMessages(gomock.Any(), gomock.Any()).Return(nil)
	// 2-й fetch блокируется до отмены контекста
	r.EXPECT().FetchMessage(gomock.Any()).
		DoAndReturn(func(ctx context.Context) (kafka.Message, error) {
			<-ctx.Done()
			return kafka.Message{}, ctx.Err()
		})

	c := newTestConsumer(r, s)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := runAsync(ctx, c)

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for Run to stop")
	}
}

// Невалидное сообщение => тоже коммитим (чтобы не ретраить мусор)
func TestRun_InvalidOrder_Commits(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := mocks.NewMockreader(ctrl)
	s := mocks.NewMockmessageSaver(ctrl)

	rc := kafka.ReaderConfig{Topic: "orders", GroupID: "g1", Brokers: []string{"b:9092"}}
	r.EXPECT().Config().Return(rc).AnyTimes()

	// 1-й цикл: получили сообщение, сервис вернул validate.ErrInvalidOrder, выполняем CommitMessages
	r.EXPECT().FetchMessage(gomock.Any()).
		Return(kafka.Message{Offset: 7, Value: []byte("bad")}, nil)
	s.EXPECT().SaveFromMessage(gomock.Any(), []byte("bad")).Return(validate.ErrInvalidOrder)
	r.EXPECT().CommitMessages(gomock.Any(), gomock.Any()).Return(nil)

	// 2-й fetch будет ждать отмены
	r.EXPECT().FetchMessage(gomock.Any()).
		DoAndReturn(func(ctx context.Context) (kafka.Message, error) {
			<-ctx.Done()
			return kafka.Message{}, ctx.Err()
		})

	c := newTestConsumer(r, s)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := runAsync(ctx, c)

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for Run to stop")
	}
}

// Временная ошибка сервиса (БД/сеть/таймаут) => НЕ коммитим
func TestRun_TemporaryFailure_NoCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := mocks.NewMockreader(ctrl)
	s := mocks.NewMockmessageSaver(ctrl)

	rc := kafka.ReaderConfig{Topic: "orders", GroupID: "g1", Brokers: []string{"b:9092"}}
	r.EXPECT().Config().Return(rc).AnyTimes()

	// 1-й цикл: получили сообщение, сервис упал "временной" ошибкой -> CommitMessages НЕ вызывается
	r.EXPECT().FetchMessage(gomock.Any()).
		Return(kafka.Message{Offset: 2, Value: []byte("x")}, nil)
	s.EXPECT().SaveFromMessage(gomock.Any(), []byte("x")).Return(errors.New("db down"))
	// Никаких r.EXPECT().CommitMessages(...) специально НЕ ставим:
	// если Consumer по ошибке его вызовет — тест упадёт как "unexpected call".

	// 2-й fetch блокируется до отмены
	r.EXPECT().FetchMessage(gomock.Any()).
		DoAndReturn(func(ctx context.Context) (kafka.Message, error) {
			<-ctx.Done()
			return kafka.Message{}, ctx.Err()
		})

	c := newTestConsumer(r, s)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := runAsync(ctx, c)

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for Run to stop")
	}
}

// Ошибки FetchMessage ретраятся; по отмене контекста — корректный выход
func TestRun_FetchError_RetryThenStopOnCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := mocks.NewMockreader(ctrl)
	s := mocks.NewMockmessageSaver(ctrl)

	rc := kafka.ReaderConfig{Topic: "orders", GroupID: "g1", Brokers: []string{"b:9092"}}
	r.EXPECT().Config().Return(rc).AnyTimes()

	// Всегда возвращаем ошибку брокера; Consumer будет ждать по backoff и ретраить,
	// пока не отменится контекст
	r.EXPECT().FetchMessage(gomock.Any()).
		DoAndReturn(func(_ context.Context) (kafka.Message, error) {
			return kafka.Message{}, errors.New("broker error")
		}).AnyTimes()

	c := newTestConsumer(r, s)

	// Короткий таймаут, чтобы быстро выйти
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	if err := c.Run(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
}

// CommitMessages вернул ошибку — получаем предупреждение; цикл живёт дальше
func TestRun_CommitWarnOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := mocks.NewMockreader(ctrl)
	s := mocks.NewMockmessageSaver(ctrl)

	rc := kafka.ReaderConfig{Topic: "orders", GroupID: "g1", Brokers: []string{"b:9092"}}
	r.EXPECT().Config().Return(rc).AnyTimes()

	// 1-й цикл: сервис работает, но CommitMessages возвращает ошибку — не должен падать
	r.EXPECT().FetchMessage(gomock.Any()).
		Return(kafka.Message{Offset: 3, Value: []byte("ok")}, nil)
	s.EXPECT().SaveFromMessage(gomock.Any(), []byte("ok")).Return(nil)
	r.EXPECT().CommitMessages(gomock.Any(), gomock.Any()).
		Return(errors.New("temporary"))

	// 2-й fetch блокируется до отмены
	r.EXPECT().FetchMessage(gomock.Any()).
		DoAndReturn(func(ctx context.Context) (kafka.Message, error) {
			<-ctx.Done()
			return kafka.Message{}, ctx.Err()
		})

	c := newTestConsumer(r, s)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := runAsync(ctx, c)

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for Run to stop")
	}
}

// 5) Проверка Close() прокидывает вызов в reader.Close()
func TestClose_DelegatesToReader(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := mocks.NewMockreader(ctrl)
	s := mocks.NewMockmessageSaver(ctrl)

	// Close должен быть вызван и вернуть nil
	r.EXPECT().Close().Return(nil)

	c := newTestConsumer(r, s)
	if err := c.Close(); err != nil {
		t.Fatalf("expected nil from Close, got %v", err)
	}
}
