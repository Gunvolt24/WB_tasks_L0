package ports

import "context"

// Logger — минимальный контракт логгера для внешних слоёв.
type Logger interface {
	Infof(ctx context.Context, format string, args ...any)  // Infof — информационные сообщения.
	Warnf(ctx context.Context, format string, args ...any)  // Warnf — предупреждения.
	Errorf(ctx context.Context, format string, args ...any) // Errorf — ошибки.
}
