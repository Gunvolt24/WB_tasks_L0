package kafka

import (
	"context"
	"errors"
	"time"

	"github.com/Gunvolt24/wb_l0/pkg/metrics"
	"github.com/Gunvolt24/wb_l0/pkg/validate"
	"github.com/segmentio/kafka-go"
)

// handleMessage обрабатывает одно сообщение и определяет нужно ли коммитить оффсет.
func (c *Consumer) handleMessage(ctx context.Context, topic string, msg *kafka.Message) bool {
	ctxTimeout, cancel := context.WithTimeout(ctx, c.processTimeout)
	err := c.service.SaveFromMessage(ctxTimeout, msg.Value)
	cancel()

	switch {
	case err == nil:
		// Успешная обработка: фиксируем метрику и коммитим оффсет
		metrics.KafkaMessagesProcessed.WithLabelValues(topic).Inc()
		return true
	case errors.Is(err, validate.ErrInvalidOrder):
		// Невалидные данные: логируем и коммитим, чтобы не обрабатывать повторно
		metrics.KafkaMessagesFailed.WithLabelValues(topic).Inc()
		c.log.Warnf(ctx, "invalid message offset=%d: %v (skipped)", msg.Offset, err)
		return true
	default:
		// Временная ошибка(БД/сеть/таймаут): НЕ коммитим - будем обрабатывать повторно
		metrics.KafkaMessagesFailed.WithLabelValues(topic).Inc()
		c.log.Warnf(ctx, "process failed offset=%d: %v (will retry without commit)", msg.Offset, err)
		return false
	}
}

// commitSafely пытается закоммитить оффсет и залогировать ошибку.
func (c *Consumer) commitSafely(ctx context.Context, msg *kafka.Message) {
	if commitErr := c.reader.CommitMessages(ctx, *msg); commitErr != nil {
		c.log.Warnf(ctx, "commit failed offset=%d: %v", msg.Offset, commitErr)
	}
}

// sleepWithBackoff ждет backoff или останавливается по контексту.
func (c *Consumer) sleepWithBackoff(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// nextBackoff возвращает следующее время ожидания повтора с учетом retryMax.
func (c *Consumer) nextBackoff(current time.Duration) time.Duration {
	current *= 2
	if current > c.retryMax {
		return c.retryMax
	}
	return current
}

// withJitterEqual — умеренная случайность: половина задержки фиксирована,
// вторая половина — случайная. Баланс между стабильностью и случайностью.
func (c *Consumer) withJitterEqual(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	half := d / 2
	jitter := time.Duration(c.jitterRand.Int63n(int64(d-half) + 1))
	return half + jitter
}

// minDuration возвращает минимальное время из двух.
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
