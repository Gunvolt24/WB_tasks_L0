package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool — создаёт пул соединений к Postgres на базе DSN.
// Здесь задаём лимиты по времени жизни/простоя соединений.
// Если maxConns > 0 — переопределяем размер пула.
// В конце выполняем Ping для fail-fast (раньше узнаем о проблемах подключения).
func NewPool(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	// Парсим DSN.
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}

	// Жизненный цикл соединений — помогает избегать переполнение пула соединений.
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	// Пул соединений.
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Проверка соединений.
	if connErr := pool.Ping(ctx); connErr != nil {
		pool.Close()
		return nil, connErr
	}

	return pool, nil
}
