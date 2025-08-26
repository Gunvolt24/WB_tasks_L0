//go:build otel && !gopls

package ctxmeta

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// В сборке с тегом `otel` достаём trace/span из активного спана и
// возвращаем их как строки для логов.

func TraceIDFromContext(ctx context.Context) (string, bool) {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return "", false
	}
	return sc.TraceID().String(), true
}

func SpanIDFromContext(ctx context.Context) (string, bool) {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return "", false
	}
	return sc.SpanID().String(), true
}
