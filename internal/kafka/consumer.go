package kafka

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/ports"
	"github.com/Gunvolt24/wb_l0/pkg/metrics"
	"github.com/segmentio/kafka-go"
)

// Проверка, что Consumer удовлетворяет интерфейсу верхнего уровня (порт приложения).
var _ ports.MessageConsumer = (*Consumer)(nil)

// reader — минимальный контракт над источником (kafka.Reader),
// чтобы легко подменять его моками в тестах.
type reader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Config() kafka.ReaderConfig
	Close() error
}

// messageSaver — зависимость на бизнес-логику,
// которая парсит/валидирует/сохраняет сообщение.
type messageSaver interface {
	SaveFromMessage(ctx context.Context, raw []byte) error
}

// Consumer — обёртка над kafka.Reader + зависимостями (usecase, logger).
type Consumer struct {
	reader         reader
	service        messageSaver
	log            ports.Logger
	processTimeout time.Duration
	retryInitial   time.Duration
	retryMax       time.Duration
	jitterRand     *rand.Rand
	closeOnce      sync.Once
}

// NewConsumer — конструктор. readerConfig() настроен на ручной коммит оффсетов.
func NewConsumer(cfg *ConsumerConfig, service messageSaver, log ports.Logger) *Consumer {
	reader := kafka.NewReader(cfg.readerConfig())

	// Параметры по умолчанию (если не заданы в конфиге)
	pt := cfg.ProcessTimeout
	if pt <= 0 {
		pt = 5 * time.Second
	}

	rInit := cfg.RetryInitial
	if rInit <= 0 {
		rInit = 1 * time.Second
	}

	rMax := cfg.RetryMax
	if rMax <= 0 {
		rMax = 30 * time.Second
	}

	return &Consumer{
		reader:         reader,
		service:        service,
		log:            log,
		processTimeout: pt,
		retryInitial:   rInit,
		retryMax:       rMax,
		// jitterRand — источник случайности, чтобы рассинхронизировать экспоненциальный backoff.
		jitterRand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Run — основной цикл:
// 1) читаем сообщение без авто-коммита;
// 2) успешная обработка → CommitMessages;
// 3) невалидные данные → лог и CommitMessages (пропускаем навсегда);
// 4) временная ошибка → без коммита (повторная обработка, at-least-once).
func (c *Consumer) Run(ctx context.Context) error {
	rc := c.reader.Config()
	c.log.Infof(ctx, "kafka consumer started topic=%s group_id=%s brokers=%v", rc.Topic, rc.GroupID, rc.Brokers)

	// Экспоненциальный backoff на ошибках FetchMessage с equal-jitter
	retry := c.retryInitial

	for {
		// Читаем сообщение (без автокоммита)
		msg, fetchErr := c.reader.FetchMessage(ctx)
		if fetchErr != nil {
			// Если контекст отменен -> выходим
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Иначе - временная ошибка брокера/сети. Ожидаем и повторяем
			sleep := c.withJitterEqual(retry)
			c.log.Warnf(ctx, "fetch failed: %v (will retry in %s)", fetchErr, sleep)
			if !c.sleepWithBackoff(ctx, sleep) {
				return ctx.Err()
			}
			// nextBackoff возвращает следующее время ожидания повтора с учетом retryMax.
			retry = c.nextBackoff(retry)
			continue
		}

		// Успешный FetchMessage -> сбрасываем интервал ожидания и инкрементим метрики
		retry = c.retryInitial
		metrics.KafkaMessagesConsumed.WithLabelValues(rc.Topic).Inc()

		// Обрабатываем сообщение(с таймаутом внутри)
		if shouldCommit := c.handleMessage(ctx, rc.Topic, &msg); shouldCommit {
			// Успешная обработка -> коммитим оффсет
			c.commitSafely(ctx, &msg)
		} else {
			// Пауза с джиттером после временной ошибки,
			// чтобы разнести повторные попытки во времени и снизить нагрузку на внешние зависимости.
			_ = c.sleepWithBackoff(ctx, c.withJitterEqual(minDuration(c.retryInitial, 500*time.Millisecond)))
		}
	}
}

// Close - закрывает reader. Вызывается при остановке приложения.
func (c *Consumer) Close() (retErr error) {
	c.closeOnce.Do(func() {
		retErr = c.reader.Close()
	})
	return retErr
}
