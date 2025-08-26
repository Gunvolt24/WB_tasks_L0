package kafka_test

import (
	"slices"
	"testing"
	"time"

	mykafka "github.com/Gunvolt24/wb_l0/internal/kafka"
	kafkago "github.com/segmentio/kafka-go"
)

func TestConsumerConfig_readerConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		startOffset string
		wantOffset  int64
	}{
		{"first lower", "first", kafkago.FirstOffset},
		{"first upper", "FIRST", kafkago.FirstOffset},
		{"first spaced", " FiRsT \n", kafkago.FirstOffset},
		{"first tabs", "\tFiRsT\t", kafkago.FirstOffset},
		{"empty -> last", "", kafkago.LastOffset},
		{"explicit last -> last", "last", kafkago.LastOffset},
		{"LAST -> last", "LAST", kafkago.LastOffset},
		{"unknown -> last", "unknown", kafkago.LastOffset},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := mykafka.ConsumerConfig{
				Brokers:     []string{"k1:9092", "k2:9092"},
				Topic:       "orders",
				GroupID:     "group-1",
				StartOffset: tt.startOffset,

				// Эти поля не участвуют в ReaderConfig, но зададим любые значения
				ProcessTimeout: 3 * time.Second,
				RetryInitial:   1 * time.Second,
				RetryMax:       5 * time.Second,
			}

			rc := cfg.ReaderConfig()

			// 1) StartOffset нормализован
			if rc.StartOffset != tt.wantOffset {
				t.Fatalf("StartOffset: want %d, got %d", tt.wantOffset, rc.StartOffset)
			}

			// 2) Прокинулись базовые поля
			if !slices.Equal(rc.Brokers, cfg.Brokers) {
				t.Fatalf("Brokers: want %v, got %v", cfg.Brokers, rc.Brokers)
			}
			if rc.Topic != cfg.Topic {
				t.Fatalf("Topic: want %s, got %s", cfg.Topic, rc.Topic)
			}
			if rc.GroupID != cfg.GroupID {
				t.Fatalf("GroupID: want %s, got %s", cfg.GroupID, rc.GroupID)
			}
			// 3) Ручной коммит
			if rc.CommitInterval != 0 {
				t.Fatalf("CommitInterval: want 0, got %v", rc.CommitInterval)

			}
		})
	}

}
