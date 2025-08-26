package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Gunvolt24/wb_l0/config"
	cachemem "github.com/Gunvolt24/wb_l0/internal/cache/memory"
	"github.com/Gunvolt24/wb_l0/internal/kafka"
	"github.com/Gunvolt24/wb_l0/internal/ports"
	"github.com/Gunvolt24/wb_l0/internal/repo/postgres"
	rest "github.com/Gunvolt24/wb_l0/internal/transport/http"
	"github.com/Gunvolt24/wb_l0/internal/usecase"
	"github.com/Gunvolt24/wb_l0/pkg/logger"
	"github.com/Gunvolt24/wb_l0/pkg/metrics"
	"github.com/Gunvolt24/wb_l0/pkg/telemetry"
	"github.com/Gunvolt24/wb_l0/pkg/validate"
	"github.com/gin-gonic/gin"
)

// App — собранное приложение и его внешние интерфейсы (HTTP, consumer).
type App struct {
	Logger          ports.Logger          // логгер
	HTTPServer      *http.Server          // HTTP-сервер
	KafkaConsumer   ports.MessageConsumer // консьюмер сообщений
	gracefulTimeout time.Duration         // время ожидания завершения HTTP-сервера
}

// Cleanup — функция освобождения ресурсов.
type Cleanup func()

// applyGinMode — устанавливает режим Gin по строке;
// неизвестное значение → debug и предупреждение в лог.
func applyGinMode(ctx context.Context, mode string, log ports.Logger) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	case "", "debug":
		gin.SetMode(gin.DebugMode)
	default:
		gin.SetMode(gin.DebugMode)
		log.Warnf(ctx, "unknown GIN_MODE=%q, fallback to debug", mode)
	}
}

// Bootstrap — собирает зависимости и возвращает приложение, функцию очистки и ошибку.
func Bootstrap(ctx context.Context, cfg *config.Config) (*App, Cleanup, error) {
	// Логгер (dev/prod режим задаётся конфигурацией).
	logg, cleanupLogger, err := logger.NewZapLogger(cfg.Logger.IsProd)
	if err != nil {
		return nil, func() {}, err
	}

	// Регистрация метрик (Prometheus).
	metrics.MustRegister()

	// Пул подключений Postgres
	pool, err := postgres.NewPool(ctx, cfg.Postgres.DSN, cfg.Postgres.MaxConns)
	if err != nil {
		if cErr := cleanupLogger(); cErr != nil {
			logg.Warnf(ctx, "cleanup logger: %v", cErr)
		}
		return nil, func() {}, err
	}

	// Трейсинг OTEL (при включённой конфигурации); по умолчанию — no-op.
	shutdownTrace := func(context.Context) error { return nil }
	if cfg.Tracing.Enabled {
		setup, tErr := telemetry.SetupTracing(ctx, cfg.Tracing.ServiceName, cfg.Tracing.Endpoint, cfg.Tracing.SampleRatio)
		if tErr != nil {
			logg.Warnf(ctx, "failed to setup tracing: %v", tErr)
		} else {
			logg.Infof(ctx, "otel tracing enabled service=%s endpoint=%s sample=%.2f",
				cfg.Tracing.ServiceName, cfg.Tracing.Endpoint, cfg.Tracing.SampleRatio)
			shutdownTrace = setup
		}
	}

	// Сборка зависимостей доменного слоя.
	orderCache := cachemem.NewLRUCacheTTL(cfg.Cache.Capacity, cfg.Cache.TTL)
	orderRepo := postgres.NewOrderRepository(pool)
	orderValidator := validate.NewOrderValidator()
	orderService := usecase.NewOrderService(orderRepo, orderCache, logg, orderValidator)

	// Прогрев кэша
	if n := cfg.Cache.WarmUpN; n > 0 {
		if err := orderService.WarmUpCache(ctx, n); err != nil {
			logg.Warnf(ctx, "warm-up cache failed: %v", err)
		}
	}

	// Режим Gin.
	applyGinMode(ctx, cfg.HTTP.GinMode, logg)

	// Имя сервиса для otelgin (только при включённом трейсинге).
	otelServiceName := ""
	if cfg.Tracing.Enabled {
		otelServiceName = cfg.Tracing.ServiceName
	}

	// Роутер и HTTP-сервер.
	httpHandler := rest.NewHandler(orderService, logg, cfg.HTTP.HandlerTimeout)
	router := rest.NewRouter(httpHandler, "./web", otelServiceName)

	httpSrv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	// Конфигурация и создание консьюмера Kafka.
	kafkaCfg := kafka.ConsumerConfig{
		Brokers:        cfg.Kafka.Brokers,
		GroupID:        cfg.Kafka.GroupID,
		Topic:          cfg.Kafka.Topic,
		StartOffset:    cfg.Kafka.StartOffset,
		ProcessTimeout: cfg.Kafka.ProcessTimeout,
		RetryInitial:   cfg.Kafka.RetryInitial,
		RetryMax:       cfg.Kafka.RetryMax,
	}
	consumer := kafka.NewConsumer(&kafkaCfg, orderService, logg)

	app := &App{
		Logger:          logg,
		HTTPServer:      httpSrv,
		KafkaConsumer:   consumer,
		gracefulTimeout: cfg.HTTP.GracefulTimeout,
	}

	// Очистка ресурсов (в обратном порядке).
	cleanup := func() {
		if terr := shutdownTrace(context.Background()); terr != nil {
			logg.Warnf(ctx, "shutdown tracing: %v", terr)
		}
		if err := consumer.Close(); err != nil {
			logg.Warnf(ctx, "kafka consumer close error: %v", err)
		}

		pool.Close()
		if cerr := cleanupLogger(); cerr != nil {
			logg.Warnf(ctx, "cleanup logger: %v", cerr)
		}
	}

	return app, cleanup, nil
}

// Run — запускает HTTP-сервер и консьюмера; ждёт отмены контекста или ошибки и останавливает их.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 2)

	// Запуск консьюмера.
	go func() {
		a.Logger.Infof(ctx, "kafka consumer starting")
		if err := a.KafkaConsumer.Run(ctx); err != nil {
			errCh <- err
		}
	}()

	// Запуск HTTP-сервера.
	go func() {
		a.Logger.Infof(ctx, "http server starting (addr=%s)", a.HTTPServer.Addr)
		if err := a.HTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// Ожидание сигнала остановки или фоновой ошибки.
	select {
	case <-ctx.Done():
		a.Logger.Infof(ctx, "shutdown requested, starting graceful shutdown")
	case err := <-errCh:
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			a.Logger.Infof(ctx, "background component stopped: %v", err)
		} else {
			a.Logger.Warnf(ctx, "background error: %v", err)
		}
	}

	gt := a.gracefulTimeout
	if gt <= 0 {
		gt = 5 * time.Second
	}

	// Корректная остановка HTTP-сервера.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), gt)
	defer cancel()

	if err := a.HTTPServer.Shutdown(shutdownCtx); err != nil {
		a.Logger.Warnf(ctx, "http server shutdown failed: %v", err)
	} else {
		a.Logger.Infof(ctx, "http server stopped gracefully")
	}

	// Остановка Kafka-консьюмера
	if err := a.KafkaConsumer.Close(); err != nil {
		a.Logger.Warnf(ctx, "kafka consumer close error: %v", err)
	}

	a.Logger.Infof(ctx, "service stopped")
	return nil
}
