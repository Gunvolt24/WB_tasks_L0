package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

// HTTP — конфигурация HTTP-сервера.
type HTTP struct {
	Addr string `default:":8080" envconfig:"ADDR"`

	// Конфигурация режима Gin
	GinMode string `default:"debug" envconfig:"GIN_MODE"`

	// Таймауты сервера (защита от медленных клиентов и висящих соединений)
	ReadTimeout       time.Duration `default:"10s" envconfig:"READ_TIMEOUT"`
	WriteTimeout      time.Duration `default:"10s" envconfig:"WRITE_TIMEOUT"`
	ReadHeaderTimeout time.Duration `default:"5s"  envconfig:"READ_HEADER_TIMEOUT"`
	IdleTimeout       time.Duration `default:"60s" envconfig:"IDLE_TIMEOUT"`
	HandlerTimeout    time.Duration `default:"3s"  envconfig:"HANDLER_TIMEOUT"`
	GracefulTimeout   time.Duration `default:"5s"  envconfig:"GRACEFUL_TIMEOUT"`
}

// Metrics — конфигурация метрик (Prometheus).
type Metrics struct {
	Addr string `default:":2112" envconfig:"ADDR"`
}

// TracingConfig — конфигурация OpenTelemetry.
type TracingConfig struct {
	Enabled     bool    `default:"false" envconfig:"OTEL_ENABLED"`
	ServiceName string  `default:"orders-app" envconfig:"OTEL_SERVICE_NAME"`
	Endpoint    string  `default:"jaeger:4318" envconfig:"OTEL_ENDPOINT"`
	SampleRatio float64 `default:"1" envconfig:"OTEL_SAMPLE_RATIO"`
}

// Postgres — конфигурация подключения к базе данных.
type Postgres struct {
	DSN      string `default:"postgres://app:app@postgres:5432/orders?sslmode=disable" envconfig:"DSN"`
	MaxConns int32  `default:"10" envconfig:"MAX_CONNS"`
}

// Kafka — конфигурация подключения к Kafka consumer.
type Kafka struct {
	Brokers     []string `default:"kafka:9092" envconfig:"BROKERS"`
	Topic       string   `default:"orders" envconfig:"TOPIC"`
	GroupID     string   `default:"orders" envconfig:"GROUP_ID"`
	StartOffset string   `default:"last" envconfig:"START_OFFSET"`

	ProcessTimeout time.Duration `default:"5s" envconfig:"PROCESS_TIMEOUT"` // время ожидания обработки сообщения
	RetryInitial   time.Duration `default:"1s" envconfig:"RETRY_INITIAL"`   // начальное время повтора
	RetryMax       time.Duration `default:"30s" envconfig:"RETRY_MAX"`      // максимальное время повтора
}

// Cache — конфигурация кэша в памяти.
type Cache struct {
	Capacity int           `default:"1000" envconfig:"CAPACITY"`
	TTL      time.Duration `default:"10m" envconfig:"TTL"`
	WarmUpN  int           `default:"0" envconfig:"WARM_UP_N"`
}

// Logger — конфигурация логгера.
type Logger struct {
	IsProd bool `default:"false" envconfig:"IS_PROD"`
}

// Config — общая конфигурация приложения.
type Config struct {
	HTTP     HTTP
	Metrics  Metrics
	Tracing  TracingConfig
	Postgres Postgres
	Kafka    Kafka
	Cache    Cache
	Logger   Logger
}

// LoadWithPrefix загружает конфигурацию из переменных окружения с префиксом.
func LoadWithPrefix(prefix string) (Config, error) {
	var c Config
	if err := envconfig.Process(prefix, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Load загружает конфигурацию из переменных окружения с префиксом ORDER.
func Load() (Config, error) {
	return LoadWithPrefix("ORDER")
}
