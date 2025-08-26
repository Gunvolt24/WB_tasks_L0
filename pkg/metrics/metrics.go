package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Регистрируем метрики один раз, чтобы избежать паник.
var registerOnce sync.Once

// -------------- Kafka --------------

// KafkaMessagesConsumed — количество сообщений, прочитанных из Kafka Reader'ом.
var KafkaMessagesConsumed = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kafka_messages_consumed_total",
		Help: "Number of messages fetched from Kafka",
	},
	[]string{"topic"},
)

// KafkaMessagesProcessed — количество успешно обработанных сообщений.
var KafkaMessagesProcessed = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kafka_messages_processed_total",
		Help: "Number of messages processed successfully",
	},
	[]string{"topic"},
)

// KafkaMessagesFailed — количество сообщений, обработка которых завершилась ошибкой.
var KafkaMessagesFailed = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kafka_messages_failed_total",
		Help: "Number of messages failed to process",
	},
	[]string{"topic"},
)

// -------------- Cache --------------

// CacheOps — счётчик операций кэша.
// Лейбл "op" принимает ограниченный набор значений: hit|miss|evicted|expired.
var CacheOps = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "cache_operations_total",
		Help: "Cache operations",
	},
	[]string{"op"}, // hit|miss|evicted|expired
)

// CacheSize — текущий размер кэша (количество элементов).
var CacheSize = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "cache_size",
		Help: "Number of items currently in cache",
	},
)

// MustRegister — регистрирует метрики.
func MustRegister() {
	registerOnce.Do(func() {
		prometheus.MustRegister(KafkaMessagesConsumed, KafkaMessagesProcessed, KafkaMessagesFailed, CacheOps, CacheSize)
	})
}
