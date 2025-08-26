// Пакет ctxmeta — нейтральный слой для работы с метаданными запроса,
// которые прокидываются через context.Context (request_id, trace_id и т.д.).
// Идея: HTTP-слой и логгер зависят от небольшого общего пакета, но не друг от друга.
package ctxmeta

import "context"

type ctxKey string

const (
	// Ключи контекста (неэкспортируемые типы — чтобы избежать коллизий).
	KeyRequestID ctxKey = "request_id"
)

// WithRequestID кладёт request_id в контекст (если пусто — ничего не делает).
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil || requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, KeyRequestID, requestID)
}

// RequestIDFromContext достаёт request_id из контекста.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	if v, ok := ctx.Value(KeyRequestID).(string); ok && v != "" {
		return v, true
	}
	return "", false
}
