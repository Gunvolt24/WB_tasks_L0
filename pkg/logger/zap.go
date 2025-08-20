package logger

import (
	"context"

	"go.uber.org/zap"
)

type ZapLogger struct {
	base   *zap.Logger
	sugar  *zap.SugaredLogger
	isProd bool
}

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

	cleanup := func() error { return loggerWrap.base.Sync() }
	return loggerWrap, cleanup, nil
}

func (z *ZapLogger) Infof(_ context.Context, format string, args ...any) {
	z.sugar.Infof(format, args...)
}
func (z *ZapLogger) Warnf(_ context.Context, format string, args ...any) {
	z.sugar.Warnf(format, args...)
}
func (z *ZapLogger) Errorf(_ context.Context, format string, args ...any) {
	z.sugar.Errorf(format, args...)
}

func (z *ZapLogger) Base() *zap.Logger           { return z.base }
func (z *ZapLogger) Sugared() *zap.SugaredLogger { return z.sugar }
