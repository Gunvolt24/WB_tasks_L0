package logger

import (
	"context"

	"github.com/Gunvolt24/wb_l0/internal/ports"
	"github.com/Gunvolt24/wb_l0/pkg/ctxmeta"
	"go.uber.org/zap"
)

// Проверка, что ZapLogger удовлетворяет интерфейсу ports.Logger.
var _ ports.Logger = (*ZapLogger)(nil)

// ZapLogger — обёртка над zap.Logger/SugaredLogger,
// которая удовлетворяет интерфейсу ports.Logger (Infof/Warnf/Errorf)
// и даёт cleanup для корректного flush (Sync).
type ZapLogger struct {
	base   *zap.Logger
	sugar  *zap.SugaredLogger
	isProd bool
}

// NewZapLogger — конструктор логгера.
// 2 режима: isProd/isDev.
func NewZapLogger(isProd bool) (*ZapLogger, func() error, error) {
	var (
		logger *zap.Logger
		err    error
	)

	if isProd {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}

	if err != nil {
		return nil, nil, err
	}

	loggerWrap := &ZapLogger{
		base:   logger,
		sugar:  logger.Sugar(),
		isProd: isProd,
	}

	// cleanup — функция для корректного завершения приложения
	// используется в app.Bootstrap для сброса буферов.
	cleanup := func() error { return loggerWrap.base.Sync() }

	return loggerWrap, cleanup, nil
}

// withCtx — возвращает SugaredLogger, c добавленными полями из контекста.
// По умолчанию добавляем request_id.
// При сборке с -tags=otel также пробуем достать trace_id/span_id из активного спана.
func (z *ZapLogger) withCtx(ctx context.Context) *zap.SugaredLogger {
	logger := z.sugar
	if rid, ok := ctxmeta.RequestIDFromContext(ctx); ok {
		logger = logger.With("request_id", rid)
	}
	if tid, ok := ctxmeta.TraceIDFromContext(ctx); ok {
		logger = logger.With("trace_id", tid)
	}
	if sid, ok := ctxmeta.SpanIDFromContext(ctx); ok {
		logger = logger.With("span_id", sid)
	}
	return logger
}

// Infof - лог уровня INFO.
// Контекст используется только для извлечения метаданных (request_id/trace_id/span_id),
// на поведение самого zap не влияет.
func (z *ZapLogger) Infof(ctx context.Context, format string, args ...any) {
	z.withCtx(ctx).Infof(format, args...)
}

// Warnf - лог уровня WARN.
func (z *ZapLogger) Warnf(ctx context.Context, format string, args ...any) {
	z.withCtx(ctx).Warnf(format, args...)
}

// Errorf - лог уровня ERROR.
func (z *ZapLogger) Errorf(ctx context.Context, format string, args ...any) {
	z.withCtx(ctx).Errorf(format, args...)
}

// Base - доступ к базовому логгеру zap.
func (z *ZapLogger) Base() *zap.Logger           { return z.base }
func (z *ZapLogger) Sugared() *zap.SugaredLogger { return z.sugar }
