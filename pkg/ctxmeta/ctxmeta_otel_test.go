//go:build otel
// +build otel

package ctxmeta_test

import (
	"context"
	"testing"

	"github.com/Gunvolt24/wb_l0/pkg/ctxmeta"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestTraceAndSpanIDs_FromContext_Otel(t *testing.T) {
	// Локальный TracerProvider — без глобальной настройки.
	tp := sdktrace.NewTracerProvider()
	tr := tp.Tracer("test")

	ctx, span := tr.Start(context.Background(), "op")
	defer span.End()

	traceID, ok := ctxmeta.TraceIDFromContext(ctx)
	if !ok {
		t.Fatalf("TraceIDFromContext must return ok=true in otel build")
	}
	spanID, ok := ctxmeta.SpanIDFromContext(ctx)
	if !ok {
		t.Fatalf("SpanIDFromContext must return ok=true in otel build")
	}

	if got, want := traceID, span.SpanContext().TraceID().String(); got != want {
		t.Fatalf("traceID=%s, want %s", got, want)
	}
	if got, want := spanID, span.SpanContext().SpanID().String(); got != want {
		t.Fatalf("spanID=%s, want %s", got, want)
	}
}

func TestTraceAndSpanIDs_InvalidContext_Otel(t *testing.T) {
	// Без спана в контексте должен вернуться "", false
	if id, ok := ctxmeta.TraceIDFromContext(context.Background()); ok || id != "" {
		t.Fatalf("TraceIDFromContext(background) => %q,%v; want \"\", false", id, ok)
	}
	if id, ok := ctxmeta.SpanIDFromContext(context.Background()); ok || id != "" {
		t.Fatalf("SpanIDFromContext(background) => %q,%v; want \"\", false", id, ok)
	}
}
