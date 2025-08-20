package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Gunvolt24/wb_l0/config"
	cachemem "github.com/Gunvolt24/wb_l0/internal/cache/memory"
	"github.com/Gunvolt24/wb_l0/internal/kafka"
	"github.com/Gunvolt24/wb_l0/internal/repo/postgres"
	rest "github.com/Gunvolt24/wb_l0/internal/transport/http"
	"github.com/Gunvolt24/wb_l0/internal/usecase"
	"github.com/Gunvolt24/wb_l0/pkg/logger"
	"github.com/Gunvolt24/wb_l0/pkg/metrics"
	"github.com/Gunvolt24/wb_l0/pkg/validate"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env.local")

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logg, cleanup, err := logger.NewZapLogger(false)
	if err != nil {
		panic(err)
	}
	defer func() { _ = cleanup() }()

	metrics.MustRegister()

	pool, err := postgres.NewPool(ctx, cfg.Postgres.DSN, cfg.Postgres.MaxConns)
	if err != nil {
		logg.Errorf(ctx, "failed to create postgres pool: %v", err)
		return
	}
	defer pool.Close()

	// сборка зависимостей
	cache := cachemem.NewLRUCacheTTL(cfg.Cache.Capacity, cfg.Cache.TTL)
	repo := postgres.NewOrderRepository(pool)
	validator := validate.NewOrderValidator()
	service := usecase.NewOrderService(repo, cache, logg, validator)

	// прогреваем кэш
	if err := service.WarmUpCache(ctx, cfg.Cache.Capacity); err != nil {
		logg.Warnf(ctx, "warm-up cache failed: %v", err)
	}

	// HTTP (GIN)
	h := rest.NewHandler(service, logg)
	router := rest.NewRouter(h, "./web")
	httpSrv := &http.Server{
		Addr:    cfg.HTTP.Addr,
		Handler: router,
	}

	// Kafka consumer
	kafkaCfg := kafka.ConsumerConfig{
		Brokers:     cfg.Kafka.Brokers,
		GroupID:     cfg.Kafka.GroupID,
		Topic:       cfg.Kafka.Topic,
		StartOffset: cfg.Kafka.StartOffset,
	}
	consumer := kafka.NewConsumer(kafkaCfg, service, logg)

	// start kafka consumer
	go func() {
		if err := consumer.Run(ctx); err != nil {
			logg.Warnf(ctx, "kafka consumer stopped: %v", err)
			cancel()
		}
	}()

	// start http server
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logg.Warnf(ctx, "http server stopped: %v", err)
			cancel()
		}
	}()

	// graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()

	shCtx, shCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shCancel()
	_ = httpSrv.Shutdown(shCtx)
	if err := consumer.Close(); err != nil {
		logg.Warnf(ctx, "consumer close error: %v", err)
	}
}
