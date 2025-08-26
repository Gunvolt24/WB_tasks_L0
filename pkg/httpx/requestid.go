package httpx

import (
	"github.com/Gunvolt24/wb_l0/pkg/ctxmeta"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDMiddleware:
// - принимает X-Request-ID от клиента или генерирует UUID
// - кладёт request_id в контекст
// - возвращает его в ответном заголовке X-Request-ID
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Header("X-Request-ID", requestID)

		ctx := ctxmeta.WithRequestID(c.Request.Context(), requestID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
