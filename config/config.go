package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type HTTP struct {
	Addr string `default:":8080" envconfig:"ADDR"`
}

type Metrics struct {
	Addr string `default:":2112" envconfig:"ADDR"`
}

type Postgres struct {
	DSN      string `defualt:"postgres://app:app@postgres:5432/orders?sslmode=disable" envconfig:"DSN"`
	MaxConns int32  `default:"10" envconfig:"MAX_CONNS"`
}

type Kafka struct {
	Brokers     []string `default:"kafka:9092" envconfig:"BROKERS"`
	Topic       string   `default:"orders" envconfig:"TOPIC"`
	GroupID     string   `default:"orders" envconfig:"GROUP_ID"`
	StartOffset string   `default:"last" envconfig:"START_OFFSET"`
}

type Cache struct {
	Capacity int           `default:"1000" envconfig:"CAPACITY"`
	TTL      time.Duration `default:"10m" envconfig:"TTL"`
}

type Config struct {
	HTTP     HTTP
	Metrics  Metrics
	Postgres Postgres
	Kafka    Kafka
	Cache    Cache
}

func Load() (Config, error) { 
	var c Config

	if err := envconfig.Process("ORDER", &c); err != nil {
		return Config{}, err
	}

	return c, nil
}
