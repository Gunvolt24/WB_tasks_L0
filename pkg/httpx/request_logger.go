package httpx

import (
	"time"

	"github.com/Gunvolt24/wb_l0/internal/ports"
	"github.com/Gunvolt24/wb_l0/pkg/ctxmeta"
	"github.com/gin-gonic/gin"
)

// requestLogger — middleware для логирования HTTP-запросов.
func RequestLogger(log ports.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		// не логируем /metrics, /ping
		switch c.FullPath() {
		case "/metrics", "/ping":
			return
		}

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		rid, _ := ctxmeta.RequestIDFromContext(c.Request.Context())
		tr, _ := ctxmeta.TraceIDFromContext(c.Request.Context())
		sp, _ := ctxmeta.SpanIDFromContext(c.Request.Context())

		log.Infof(
			c.Request.Context(),
			"request id=%s trace=%s span=%s method=%s path=%s status=%d ip=%s duration=%s size=%d",
			rid, tr, sp,
			c.Request.Method,
			path,
			c.Writer.Status(),
			c.ClientIP(),
			time.Since(start),
			c.Writer.Size(),
		)
	}
}
