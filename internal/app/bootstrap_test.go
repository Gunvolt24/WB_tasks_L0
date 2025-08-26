package app_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/app"
)

// логгер-заглушка
type nopLogger struct{}

func (nopLogger) Infof(context.Context, string, ...any)  {}
func (nopLogger) Warnf(context.Context, string, ...any)  {}
func (nopLogger) Errorf(context.Context, string, ...any) {}

// фейковый консьюмер, который ждёт отмены контекста
type fakeConsumer struct {
	runCalls   int32
	closeCalls int32
}

func (f *fakeConsumer) Run(ctx context.Context) error {
	atomic.AddInt32(&f.runCalls, 1)
	<-ctx.Done()
	return ctx.Err()
}
func (f *fakeConsumer) Close() error {
	atomic.AddInt32(&f.closeCalls, 1)
	return nil
}

func TestAppRun_GracefulShutdown(t *testing.T) {
	// HTTP-сервер на случайном свободном порту
	srv := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: http.NewServeMux(),
	}

	fc := &fakeConsumer{}
	a := &app.App{
		Logger:        nopLogger{},
		HTTPServer:    srv,
		KafkaConsumer: fc,
	}

	// Запуск и быстрая остановка
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := a.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if atomic.LoadInt32(&fc.runCalls) == 0 {
		t.Fatalf("consumer.Run should be called")
	}
	if atomic.LoadInt32(&fc.closeCalls) == 0 {
		t.Fatalf("consumer.Close should be called")
	}
}
