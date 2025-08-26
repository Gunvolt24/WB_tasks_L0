package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Gunvolt24/wb_l0/pkg/httpx"
	"github.com/gin-gonic/gin"
)

// Утилита для создания *gin.Context с query-строкой
func ctxWithQuery(rawQuery string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/?"+rawQuery, http.NoBody)
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c
}

func TestClampInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		v, min, max int
		want        int
	}{
		{"below_min", 0, 1, 10, 1},
		{"above_max", 11, 1, 10, 10},
		{"inside", 5, 1, 10, 5},
		{"equal_min", 1, 1, 10, 1},
		{"equal_max", 10, 1, 10, 10},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := httpx.ClampInt(tt.v, tt.min, tt.max); got != tt.want {
				t.Fatalf("ClampInt(%d,%d,%d) = %d, want %d", tt.v, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestParseLimitOffset_Defaults_NoQuery(t *testing.T) {
	t.Parallel()

	{
		c := ctxWithQuery("")
		limit, offset := httpx.ParseLimitOffset(c, 20, 50)
		if limit != 20 || offset != 0 {
			t.Fatalf("got limit=%d offset=%d, want 20/0", limit, offset)
		}
	}

	{
		c := ctxWithQuery("")
		limit, offset := httpx.ParseLimitOffset(c, 100, 50)
		if limit != 50 || offset != 0 {
			t.Fatalf("got limit=%d offset=%d, want 50/0", limit, offset)
		}
	}

	{
		c := ctxWithQuery("")
		limit, offset := httpx.ParseLimitOffset(c, 0, 50)
		if limit != 1 || offset != 0 {
			t.Fatalf("got limit=%d offset=%d, want 1/0", limit, offset)
		}
	}
}

func TestParseLimitOffset_QueryProvided(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		rawQuery     string
		defaultLimit int
		maxLimit     int
		wantLimit    int
		wantOffset   int
	}{
		// корректные значения
		{"ok_both", "limit=25&offset=10", 20, 50, 25, 10},
		{"ok_only_limit", "limit=5", 20, 50, 5, 0},
		{"ok_only_offset", "offset=7", 20, 50, 20, 7},

		// клампинг limit
		{"limit_zero_clamped_to_min", "limit=0", 20, 50, 1, 0},
		{"limit_negative_clamped_to_min", "limit=-5", 20, 50, 1, 0},
		{"limit_above_max_clamped", "limit=999", 20, 50, 50, 0},

		// нечисловые значения
		{"limit_non_int_uses_default", "limit=foo", 20, 50, 20, 0},
		{"offset_non_int_ignored", "offset=bar", 20, 50, 20, 0},

		// отрицательный offset игнорируется
		{"offset_negative_ignored", "limit=10&offset=-3", 20, 50, 10, 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := ctxWithQuery(tt.rawQuery)
			limit, offset := httpx.ParseLimitOffset(c, tt.defaultLimit, tt.maxLimit)
			if limit != tt.wantLimit || offset != tt.wantOffset {
				t.Fatalf("got limit=%d offset=%d, want %d/%d (query=%q)",
					limit, offset, tt.wantLimit, tt.wantOffset, tt.rawQuery)
			}
		})
	}
}
