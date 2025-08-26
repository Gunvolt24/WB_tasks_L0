package httpx

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// ClampInt — ограничение значения v в диапазоне [min, max].
func ClampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ParseLimitOffset - читает limit/offset из query с дефолтами и границами.
func ParseLimitOffset(c *gin.Context, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	if v, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit))); err == nil {
		limit = ClampInt(v, 1, maxLimit)
	}
	if v, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && v >= 0 {
		offset = v
	}
	return
}
