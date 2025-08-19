package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	KafkaMessagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_consumed_total",
			Help: "Number of messages fetched from Kafka",
		},
		[]string{"topic"},
	)
	KafkaMessagesProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_processed_total",
			Help: "Number of messages processed successfully",
		},
		[]string{"topic"},
	)
	KafkaMessagesFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_failed_total",
			Help: "Number of messages failed to process",
		},
		[]string{"topic"},
	)
)

var (
    CacheOps = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_operations_total",
            Help: "Cache operations",
        },
        []string{"op"}, // hit|miss|evicted|expired
    )
    CacheSize = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "cache_size",
            Help: "Number of items currently in cache",
        },
    )
)

func MustRegister() {
	prometheus.MustRegister(KafkaMessagesConsumed, KafkaMessagesProcessed, KafkaMessagesFailed, CacheOps, CacheSize)
}