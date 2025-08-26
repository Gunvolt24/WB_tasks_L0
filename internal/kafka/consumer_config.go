package kafka

import (
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// ConsumerConfig — конфигурация подключения к Kafka consumer.
type ConsumerConfig struct {
	Brokers     []string // список брокеров kafka
	Topic       string   // топик подписки
	GroupID     string   // идентификатор группы
	StartOffset string   // начальная позиция чтения

	ProcessTimeout time.Duration // таймаут обработки одного сообщения
	RetryInitial   time.Duration // начальное время повтора
	RetryMax       time.Duration // максимальное время повтора
}

// ReaderConfig — враппер для kafka.ReaderConfig
func (c *ConsumerConfig) ReaderConfig() kafka.ReaderConfig {
	return c.readerConfig()
}

// readerConfig — сборка конфигурации для kafka.NewReader.
// Важно: CommitInterval=0 - отключаем авто-коммит; подтверждаем вручную
// только после успешной обработки (FetchMessage -> CommitMessages).
func (c *ConsumerConfig) readerConfig() kafka.ReaderConfig {
	rc := kafka.ReaderConfig{
		Brokers:        c.Brokers,
		GroupID:        c.GroupID,
		Topic:          c.Topic,
		CommitInterval: 0, // ручной коммит (обязательно используем FetchMessage/CommitMessages)
	}

	// Нормализуем StartOffset (обрезаем пробелы, приводим к нижнему регистру)
	switch strings.ToLower(strings.TrimSpace(c.StartOffset)) {
	case "first":
		rc.StartOffset = kafka.FirstOffset
	default:
		// По умолчанию — читать только новые сообщения
		rc.StartOffset = kafka.LastOffset
	}

	return rc
}
