//go:build !otel
// +build !otel

package ctxmeta_test

import (
	"context"
	"testing"

	"github.com/Gunvolt24/wb_l0/pkg/ctxmeta"
)

func TestTraceAndSpanIDs_NoOtelBuild_ReturnEmpty(t *testing.T) {
	if id, ok := ctxmeta.TraceIDFromContext(context.Background()); ok || id != "" {
		t.Fatalf("TraceIDFromContext => %q,%v; want \"\", false", id, ok)
	}
	if id, ok := ctxmeta.SpanIDFromContext(context.Background()); ok || id != "" {
		t.Fatalf("SpanIDFromContext => %q,%v; want \"\", false", id, ok)
	}
}
