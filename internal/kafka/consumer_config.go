package kafka

import "github.com/segmentio/kafka-go"

type ConsumerConfig struct {
	Brokers     []string
	Topic       string
	GroupID     string
	StartOffset string
}

func (c *ConsumerConfig) readerConfig() kafka.ReaderConfig {
	rc := kafka.ReaderConfig{
		Brokers:        c.Brokers,
		GroupID:        c.GroupID,
		Topic:          c.Topic,
		CommitInterval: 0,
	}

	switch c.StartOffset {
	case "first":
		rc.StartOffset = kafka.FirstOffset
	default:
		rc.StartOffset = kafka.LastOffset
	}

	return rc
}
