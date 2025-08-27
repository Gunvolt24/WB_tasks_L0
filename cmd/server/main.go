package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/Gunvolt24/wb_l0/config"
	"github.com/Gunvolt24/wb_l0/internal/app"
)

func main() {
	// Контекст для остановки приложения
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		println("failed to load config:", err.Error())
		return
	}

	// Сборка зависимостей и запуск приложения
	application, cleanup, err := app.Bootstrap(ctx, &cfg)
	if err != nil {
		println("failed to bootstrap application:", err.Error())
		return
	}
	defer cleanup()

	// Запуск HTTP и Kafka + корректное завершение
	if errApp := application.Run(ctx); errApp != nil {
		application.Logger.Errorf(ctx, "failed to run application: %v", err)
	}
}
