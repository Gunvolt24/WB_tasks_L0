package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Gunvolt24/wb_l0/pkg/ctxmeta"
	"github.com/Gunvolt24/wb_l0/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestRequestIDMiddleware_GeneratesWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotID string
	var ok bool

	r := gin.New()
	r.Use(httpx.RequestIDMiddleware())
	r.GET("/", func(c *gin.Context) {
		gotID, ok = ctxmeta.RequestIDFromContext(c.Request.Context())
		c.Status(204)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", http.NoBody)
	r.ServeHTTP(w, req)

	rid := w.Header().Get("X-Request-ID")
	if rid == "" {
		t.Fatalf("header X-Request-ID должен быть установлен")
	}
	if _, err := uuid.Parse(rid); err != nil {
		t.Fatalf("сгенерированный X-Request-ID должен быть UUID, got=%q err=%v", rid, err)
	}
	if !ok || gotID != rid {
		t.Fatalf("request id в контексте должен совпадать с заголовком: ctx=%q ok=%v header=%q", gotID, ok, rid)
	}
}

func TestRequestIDMiddleware_UsesProvidedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const provided = "custom-id-42"
	var gotID string
	var ok bool

	r := gin.New()
	r.Use(httpx.RequestIDMiddleware())
	r.GET("/", func(c *gin.Context) {
		gotID, ok = ctxmeta.RequestIDFromContext(c.Request.Context())
		c.Status(204)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", http.NoBody)
	req.Header.Set("X-Request-ID", provided)
	r.ServeHTTP(w, req)

	rid := w.Header().Get("X-Request-ID")
	if rid != provided {
		t.Fatalf("middleware должен сохранять переданный X-Request-ID: got=%q want=%q", rid, provided)
	}
	if !ok || gotID != provided {
		t.Fatalf("в контексте должен лежать переданный X-Request-ID: ctx=%q ok=%v want=%q", gotID, ok, provided)
	}
}
