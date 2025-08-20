package kafka

import (
	"context"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/ports"
	"github.com/Gunvolt24/wb_l0/internal/usecase"
	"github.com/Gunvolt24/wb_l0/pkg/metrics"
	"github.com/segmentio/kafka-go"
)

var _ ports.MessageConsumer = (*Consumer)(nil)

type Consumer struct {
	reader  *kafka.Reader
	service *usecase.OrderService
	log     ports.Logger
}

func NewConsumer(cfg ConsumerConfig, service *usecase.OrderService, log ports.Logger) *Consumer {
	reader := kafka.NewReader(cfg.readerConfig())
	return &Consumer{reader: reader, service: service, log: log}
}

func (c *Consumer) Run(ctx context.Context) error {
	rc := c.reader.Config()
	c.log.Infof(ctx, "kafka consumer started topic=%s group_id=%s brokers=%v", rc.Topic, rc.GroupID, rc.Brokers)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.log.Warnf(ctx, "consumer stopped: %v", ctx.Err())
				return ctx.Err()
			}
			c.log.Warnf(ctx, "consumer fetch message err=%v", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(300 * time.Second):
			}
			continue
		}

		metrics.KafkaMessagesConsumed.WithLabelValues(rc.Topic).Inc()

		if err := c.service.SaveFromMessage(ctx, msg.Value); err != nil {
			metrics.KafkaMessagesFailed.WithLabelValues(rc.Topic).Inc()
			c.log.Warnf(ctx, "process failed offset=%d: %v (message skipped)", msg.Offset, err)
		} else {
			metrics.KafkaMessagesProcessed.WithLabelValues(rc.Topic).Inc()
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.Warnf(ctx, "commit failed offset=%d: %v", msg.Offset, err)
		}
	}
}

func (c *Consumer) Close() error { return c.reader.Close() }
