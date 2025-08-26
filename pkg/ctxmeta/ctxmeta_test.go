package ctxmeta_test

import (
	"context"
	"testing"

	"github.com/Gunvolt24/wb_l0/pkg/ctxmeta"
)

func TestWithRequestID_PutAndGet(t *testing.T) {
	parent := context.Background()

	ctx := ctxmeta.WithRequestID(parent, "req-123")
	got, ok := ctxmeta.RequestIDFromContext(ctx)
	if !ok || got != "req-123" {
		t.Fatalf("want ok=true, id=req-123; got ok=%v id=%q", ok, got)
	}

	// Родитель не должен содержать request_id
	if _, parentOk := ctxmeta.RequestIDFromContext(parent); parentOk {
		t.Fatalf("parent context must not contain request_id")
	}
}

func TestWithRequestID_EmptyID_NoChange(t *testing.T) {
	parent := context.Background()
	ctx := ctxmeta.WithRequestID(parent, "")
	if ctx != parent {
		t.Fatalf("WithRequestID with empty id must return the same ctx")
	}
}

func TestWithRequestID_NilCtx(t *testing.T) {
	var nilCtx context.Context
	ctx := ctxmeta.WithRequestID(nilCtx, "req-1")
	if ctx != nil {
		t.Fatalf("WithRequestID(nil, ...) must return nil")
	}
	id, ok := ctxmeta.RequestIDFromContext(context.Background())
	if ok || id != "" {
		t.Fatalf("RequestIDFromContext(nil) must be empty/false, got id=%q ok=%v", id, ok)
	}
}

func TestRequestIDFromContext_NoValue(t *testing.T) {
	id, ok := ctxmeta.RequestIDFromContext(context.Background())
	if ok || id != "" {
		t.Fatalf("empty ctx must return empty/false, got id=%q ok=%v", id, ok)
	}
}

func TestRequestIDFromContext_EmptyStoredValue(t *testing.T) {
	// Даже если ключ верный, пустое значение считаем отсутствующим
	ctx := context.WithValue(context.Background(), ctxmeta.KeyRequestID, "")
	id, ok := ctxmeta.RequestIDFromContext(ctx)
	if ok || id != "" {
		t.Fatalf("empty stored value must be treated as absent, got id=%q ok=%v", id, ok)
	}
}

func TestRequestIDFromContext_StringKeyDoesNotWork(t *testing.T) {
	type otherKey struct{}
	// Кладём по строковому ключу — не должен доставаться,
	// т.к. библиотека использует собственный тип ключа (ctxKey)
	ctx := context.WithValue(context.Background(), otherKey{}, "req-xyz")
	id, ok := ctxmeta.RequestIDFromContext(ctx)
	if ok || id != "" {
		t.Fatalf("string key must not be recognized, got id=%q ok=%v", id, ok)
	}
}
