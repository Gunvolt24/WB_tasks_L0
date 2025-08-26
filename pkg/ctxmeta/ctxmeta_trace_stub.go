//go:build !otel || gopls

package ctxmeta

import "context"

// Сборка без тега `otel`: заглушки для trace/span.
func TraceIDFromContext(context.Context) (string, bool) { return "", false }
func SpanIDFromContext(context.Context) (string, bool)  { return "", false }
