package config_test

import (
	"slices"
	"testing"
	"time"

	cfg "github.com/Gunvolt24/wb_l0/config"
)

const orders = "orders"

// TestLoadWithPrefix_Defaults — проверка наличия значений по умолчанию.
func TestLoadWithPrefix_Defaults(t *testing.T) {
	t.Parallel()

	c, err := cfg.LoadWithPrefix("ORDER_TEST_DEFAULTS")
	if err != nil {
		t.Fatalf("LoadWithPrefix error: %v", err)
	}

	// HTTP
	if c.HTTP.Addr != ":8080" {
		t.Fatalf("HTTP.Addr: want :8080, got %q", c.HTTP.Addr)
	}
	if c.HTTP.GinMode != "debug" {
		t.Fatalf("HTTP.GinMode: want debug, got %q", c.HTTP.GinMode)
	}
	if c.HTTP.ReadTimeout != 10*time.Second || c.HTTP.WriteTimeout != 10*time.Second {
		t.Fatalf("HTTP timeouts wrong: %+v", c.HTTP)
	}
	if c.HTTP.ReadHeaderTimeout != 5*time.Second || c.HTTP.IdleTimeout != 60*time.Second {
		t.Fatalf("HTTP header/idle timeouts wrong: %+v", c.HTTP)
	}
	if c.HTTP.HandlerTimeout != 3*time.Second {
		t.Fatalf("HTTP.HandlerTimeout: want 3s, got %v", c.HTTP.HandlerTimeout)
	}

	// Metrics
	if c.Metrics.Addr != ":2112" {
		t.Fatalf("Metrics.Addr: want :2112, got %q", c.Metrics.Addr)
	}

	// Tracing
	if c.Tracing.Enabled {
		t.Fatalf("Tracing.Enabled: want false, got true")
	}
	if c.Tracing.ServiceName != "orders-app" || c.Tracing.Endpoint != "jaeger:4318" || c.Tracing.SampleRatio != 1 {
		t.Fatalf("Tracing defaults wrong: %+v", c.Tracing)
	}

	// Postgres
	if c.Postgres.DSN == "" {
		t.Fatalf("Postgres.DSN should have default, got empty")
	}
	if c.Postgres.MaxConns != 10 {
		t.Fatalf("Postgres.MaxConns: want 10, got %d", c.Postgres.MaxConns)
	}

	// Kafka
	if !slices.Equal(c.Kafka.Brokers, []string{"kafka:9092"}) {
		t.Fatalf("Kafka.Brokers: want [kafka:9092], got %v", c.Kafka.Brokers)
	}
	if c.Kafka.Topic != orders || c.Kafka.GroupID != orders || c.Kafka.StartOffset != "last" {
		t.Fatalf("Kafka defaults wrong: %+v", c.Kafka)
	}
	if c.Kafka.ProcessTimeout != 5*time.Second || c.Kafka.RetryInitial != 1*time.Second || c.Kafka.RetryMax != 30*time.Second {
		t.Fatalf("Kafka timeouts wrong: %+v", c.Kafka)
	}

	// Cache
	if c.Cache.Capacity != 1000 || c.Cache.TTL != 10*time.Minute {
		t.Fatalf("Cache defaults wrong: %+v", c.Cache)
	}

	// Logger
	if c.Logger.IsProd {
		t.Fatalf("Logger.IsProd: want false, got true")
	}
}

// Меняем окружение.
func TestLoadWithPrefix_Overrides(t *testing.T) {
	const p = "ORDER_TEST_OVR"

	// HTTP
	t.Setenv(p+"_HTTP_ADDR", ":9999")
	t.Setenv(p+"_HTTP_GIN_MODE", "release")
	t.Setenv(p+"_HTTP_READ_TIMEOUT", "2s")
	t.Setenv(p+"_HTTP_WRITE_TIMEOUT", "3s")
	t.Setenv(p+"_HTTP_READ_HEADER_TIMEOUT", "1s")
	t.Setenv(p+"_HTTP_IDLE_TIMEOUT", "15s")
	t.Setenv(p+"_HTTP_HANDLER_TIMEOUT", "4500ms")

	// Metrics
	t.Setenv(p+"_METRICS_ADDR", ":9998")

	// Tracing
	t.Setenv(p+"_TRACING_OTEL_ENABLED", "true")
	t.Setenv(p+"_TRACING_OTEL_SERVICE_NAME", "svc")
	t.Setenv(p+"_TRACING_OTEL_ENDPOINT", "collector:4318")
	t.Setenv(p+"_TRACING_OTEL_SAMPLE_RATIO", "0.25")

	// Postgres
	t.Setenv(p+"_POSTGRES_DSN", "postgres://u:p@h:5432/db?sslmode=disable")
	t.Setenv(p+"_POSTGRES_MAX_CONNS", "42")

	// Kafka
	t.Setenv(p+"_KAFKA_BROKERS", "k1:9092,k2:9093")
	t.Setenv(p+"_KAFKA_TOPIC", "orders-test")
	t.Setenv(p+"_KAFKA_GROUP_ID", "g-test")
	t.Setenv(p+"_KAFKA_START_OFFSET", "first")
	t.Setenv(p+"_KAFKA_PROCESS_TIMEOUT", "7s")
	t.Setenv(p+"_KAFKA_RETRY_INITIAL", "250ms")
	t.Setenv(p+"_KAFKA_RETRY_MAX", "2m")

	// Cache
	t.Setenv(p+"_CACHE_CAPACITY", "777")
	t.Setenv(p+"_CACHE_TTL", "30m")

	// Logger
	t.Setenv(p+"_LOGGER_IS_PROD", "true")

	c, err := cfg.LoadWithPrefix(p)
	if err != nil {
		t.Fatalf("LoadWithPrefix error: %v", err)
	}

	// Проверки
	if c.HTTP.Addr != ":9999" || c.HTTP.GinMode != "release" {
		t.Fatalf("HTTP overrides wrong: %+v", c.HTTP)
	}
	if c.HTTP.ReadTimeout != 2*time.Second || c.HTTP.WriteTimeout != 3*time.Second ||
		c.HTTP.ReadHeaderTimeout != 1*time.Second || c.HTTP.IdleTimeout != 15*time.Second ||
		c.HTTP.HandlerTimeout != 4500*time.Millisecond {
		t.Fatalf("HTTP timeouts override wrong: %+v", c.HTTP)
	}
	if c.Metrics.Addr != ":9998" {
		t.Fatalf("Metrics.Addr override wrong: %q", c.Metrics.Addr)
	}
	if !c.Tracing.Enabled || c.Tracing.ServiceName != "svc" || c.Tracing.Endpoint != "collector:4318" || c.Tracing.SampleRatio != 0.25 {
		t.Fatalf("Tracing overrides wrong: %+v", c.Tracing)
	}
	if c.Postgres.DSN != "postgres://u:p@h:5432/db?sslmode=disable" || c.Postgres.MaxConns != 42 {
		t.Fatalf("Postgres overrides wrong: %+v", c.Postgres)
	}
	if !slices.Equal(c.Kafka.Brokers, []string{"k1:9092", "k2:9093"}) ||
		c.Kafka.Topic != "orders-test" || c.Kafka.GroupID != "g-test" || c.Kafka.StartOffset != "first" {
		t.Fatalf("Kafka basic overrides wrong: %+v", c.Kafka)
	}
	if c.Kafka.ProcessTimeout != 7*time.Second || c.Kafka.RetryInitial != 250*time.Millisecond || c.Kafka.RetryMax != 2*time.Minute {
		t.Fatalf("Kafka timeouts override wrong: %+v", c.Kafka)
	}
	if c.Cache.Capacity != 777 || c.Cache.TTL != 30*time.Minute {
		t.Fatalf("Cache overrides wrong: %+v", c.Cache)
	}
	if !c.Logger.IsProd {
		t.Fatalf("Logger.IsProd override wrong: %+v", c.Logger)
	}
}

// Тоже меняем окружение — но с невалидным значением.
func TestLoadWithPrefix_InvalidValue_ReturnsError(t *testing.T) {
	const p = "ORDER_TEST_BAD"
	t.Setenv(p+"_HTTP_READ_TIMEOUT", "not-a-duration")

	if _, err := cfg.LoadWithPrefix(p); err == nil {
		t.Fatalf("expected error for invalid duration, got nil")
	}
}
