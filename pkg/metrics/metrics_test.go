package metrics_test

import (
	"testing"

	"github.com/Gunvolt24/wb_l0/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMustRegister_IsIdempotent(t *testing.T) {
	// Должно выполняться без паники даже при повторном вызове.
	t.Helper()
	metrics.MustRegister()
	metrics.MustRegister()
}

func TestKafkaCounters_Inc(t *testing.T) {
	metrics.MustRegister()

	beforeConsumed := testutil.ToFloat64(metrics.KafkaMessagesConsumed.WithLabelValues("orders"))
	beforeProcessed := testutil.ToFloat64(metrics.KafkaMessagesProcessed.WithLabelValues("orders"))
	beforeFailed := testutil.ToFloat64(metrics.KafkaMessagesFailed.WithLabelValues("orders"))

	metrics.KafkaMessagesConsumed.WithLabelValues("orders").Inc()
	metrics.KafkaMessagesProcessed.WithLabelValues("orders").Inc()
	metrics.KafkaMessagesFailed.WithLabelValues("orders").Inc()

	if got := testutil.ToFloat64(metrics.KafkaMessagesConsumed.WithLabelValues("orders")); got != beforeConsumed+1 {
		t.Fatalf("KafkaMessagesConsumed: got=%v want=%v", got, beforeConsumed+1)
	}
	if got := testutil.ToFloat64(metrics.KafkaMessagesProcessed.WithLabelValues("orders")); got != beforeProcessed+1 {
		t.Fatalf("KafkaMessagesProcessed: got=%v want=%v", got, beforeProcessed+1)
	}
	if got := testutil.ToFloat64(metrics.KafkaMessagesFailed.WithLabelValues("orders")); got != beforeFailed+1 {
		t.Fatalf("KafkaMessagesFailed: got=%v want=%v", got, beforeFailed+1)
	}
}

func TestCacheOps_CountersByLabel(t *testing.T) {
	metrics.MustRegister()

	hitBefore := testutil.ToFloat64(metrics.CacheOps.WithLabelValues("hit"))
	missBefore := testutil.ToFloat64(metrics.CacheOps.WithLabelValues("miss"))

	metrics.CacheOps.WithLabelValues("hit").Inc()
	metrics.CacheOps.WithLabelValues("hit").Inc()

	if got := testutil.ToFloat64(metrics.CacheOps.WithLabelValues("hit")); got != hitBefore+2 {
		t.Fatalf("CacheOps(hit): got=%v want=%v", got, hitBefore+2)
	}
	if got := testutil.ToFloat64(metrics.CacheOps.WithLabelValues("miss")); got != missBefore {
		t.Fatalf("CacheOps(miss): got=%v want=%v", got, missBefore)
	}
}

func TestCacheSize_GaugeSet(t *testing.T) {
	metrics.MustRegister()

	cur := testutil.ToFloat64(metrics.CacheSize)

	metrics.CacheSize.Set(cur + 5)
	if got := testutil.ToFloat64(metrics.CacheSize); got != cur+5 {
		t.Fatalf("CacheSize after +5: got=%v want=%v", got, cur+5)
	}

	metrics.CacheSize.Set(cur) // вернуть как было
	if got := testutil.ToFloat64(metrics.CacheSize); got != cur {
		t.Fatalf("CacheSize restore: got=%v want=%v", got, cur)
	}
}
