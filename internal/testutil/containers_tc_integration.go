//go:build integration

package testutil

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ----------------------------------------------------------------------------
// Красивые логи жизненного цикла
// ----------------------------------------------------------------------------

func shortID(c tc.Container) string {
	id := c.GetContainerID()
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func logHooks(l *log.Logger) tc.ContainerLifecycleHooks {
	return tc.ContainerLifecycleHooks{
		PreCreates: []tc.ContainerRequestHook{
			func(_ context.Context, req tc.ContainerRequest) error {
				l.Printf("🐳 creating container image=%s", req.Image)
				return nil
			},
		},
		PostCreates: []tc.ContainerHook{
			func(ctx context.Context, c tc.Container) error {
				n, _ := c.Name(ctx)
				l.Printf("✅ created id=%s name=%s", shortID(c), n)
				return nil
			},
		},
		PreStarts: []tc.ContainerHook{
			func(_ context.Context, c tc.Container) error {
				l.Printf("🐳 starting id=%s", shortID(c))
				return nil
			},
		},
		PostStarts: []tc.ContainerHook{
			func(_ context.Context, c tc.Container) error {
				l.Printf("✅ started id=%s", shortID(c))
				return nil
			},
		},
		PostReadies: []tc.ContainerHook{
			func(_ context.Context, c tc.Container) error {
				l.Printf("🔔 ready id=%s", shortID(c))
				return nil
			},
		},
		PreTerminates: []tc.ContainerHook{
			func(_ context.Context, c tc.Container) error {
				l.Printf("🛑 terminating id=%s", shortID(c))
				return nil
			},
		},
		PostTerminates: []tc.ContainerHook{
			func(_ context.Context, c tc.Container) error {
				l.Printf("🚫 terminated id=%s", shortID(c))
				return nil
			},
		},
	}
}

// Общий логгер для testcontainers (можно подключить свой)
var tcLogger = log.New(os.Stdout, "[tc] ", log.LstdFlags)

// ----------------------------------------------------------------------------
// Postgres
// ----------------------------------------------------------------------------

type PGContainer struct {
	Container *postgres.PostgresContainer
	Pool      *pgxpool.Pool
	DSN       string
}

func StartPostgresTC(ctx context.Context) (*PGContainer, func(context.Context) error, error) {
	pg, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		// красиво логируем этапы жизни контейнера
		tc.WithLifecycleHooks(logHooks(tcLogger)),
		// обязательно экспонируем 5432
		tc.WithExposedPorts("5432/tcp"),
		// базовые параметры БД
		postgres.WithDatabase("orders"),
		postgres.WithUsername("app"),
		postgres.WithPassword("app"),
		// подождём, пока порт начнёт слушаться и Postgres поднимется
		tc.WithWaitStrategy(
			wait.ForAll(
				wait.ForListeningPort("5432/tcp"),
				wait.ForLog("database system is ready to accept connections"),
			).WithDeadline(60*time.Second),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("run postgres: %w", err)
	}

	// Готовый DSN от контейнера (учтёт реальный host:port)
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pg.Terminate(ctx)
		return nil, nil, fmt.Errorf("conn string: %w", err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		_ = pg.Terminate(ctx)
		return nil, nil, fmt.Errorf("parse cfg: %w", err)
	}
	cfg.MaxConns = 5
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		_ = pg.Terminate(ctx)
		return nil, nil, fmt.Errorf("new pool: %w", err)
	}

	stop := func(c context.Context) error {
		pool.Close()
		return pg.Terminate(c)
	}

	return &PGContainer{Container: pg, DSN: dsn, Pool: pool}, stop, nil
}

// ----------------------------------------------------------------------------
// Kafka
// ----------------------------------------------------------------------------

type KafkaEnv struct {
	Container *redpanda.Container
	Brokers   []string
	BaseTopic string
}

func StartKafkaTC(ctx context.Context, baseTopic string) (*KafkaEnv, func(context.Context) error, error) {
	rp, err := redpanda.Run(
		ctx,
		"docker.redpanda.com/redpandadata/redpanda:v23.3.8",
		tc.WithLifecycleHooks(logHooks(tcLogger)),
		redpanda.WithAutoCreateTopics(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("run redpanda: %w", err)
	}

	seed, err := rp.KafkaSeedBroker(ctx) // вернёт "host:port" для клиента
	if err != nil {
		_ = tc.TerminateContainer(rp)
		return nil, nil, fmt.Errorf("seed broker: %w", err)
	}

	env := &KafkaEnv{
		Container: rp,
		Brokers:   []string{seed},
		BaseTopic: baseTopic,
	}
	stop := func(_ context.Context) error { return tc.TerminateContainer(rp) }
	return env, stop, nil
}
