//go:build !integration

package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/internal/testutil"
)

// --- Бенчмарки ---

// Базовый бенч: GetOrder (валидный заказ) — сравниваем LEAN vs FULL пайплайн
func BenchmarkHTTP_GetOrder(b *testing.B) {
	log := nopLogger{}
	ord := testutil.MakeOrder(testutil.WithCustomer("bench-cust"))
	h := NewHandler(svcOne{o: &ord}, log, 2*time.Second)

	lean := makeLeanRouter(h)
	full := makeFullRouter(h)

	b.Run("lean/no-mw", func(b *testing.B) {
		benchServeGET(b, lean, "/order/"+ord.OrderUID)
	})
	b.Run("full/prod-mw", func(b *testing.B) {
		benchServeGET(b, full, "/order/"+ord.OrderUID)
	})
}

// Потолок без маршалинга: тот же заказ, но заранее закодированный JSON
// Показывает, сколько «ест» encoding/json в вашем хендлере.
func BenchmarkHTTP_GetOrder_PreMarshaledBytes(b *testing.B) {
	ord := testutil.MakeOrder()
	raw, _ := json.Marshal(ord)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// отдельный эндпоинт, который просто отдаёт готовый []byte
	r.GET("/order/:id", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", raw)
	})

	benchServeGET(b, r, "/order/"+ord.OrderUID)
}

// Пагинация: 10/50/100 — измеряем рост аллокаций и времени
func BenchmarkHTTP_ListByCustomer(b *testing.B) {
	log := nopLogger{}

	for _, n := range []int{10, 50, 100} {
		b.Run("N="+strconv.Itoa(n), func(b *testing.B) {
			// готовим список из n заказов
			list := make([]*domain.Order, 0, n)
			for i := 0; i < n; i++ {
				o := testutil.MakeOrder(testutil.WithCustomer("bench-cust"))
				oo := o // копия, чтобы адрес был уникальным
				list = append(list, &oo)
			}
			h := NewHandler(svcList{list: list}, log, 2*time.Second)

			lean := makeLeanRouter(h)
			benchServeGET(b, lean, "/customer/bench-cust/orders?limit="+strconv.Itoa(n))
		})
	}
}

// Ошибочный путь (404): "цена" роутера и 404-хендлера
func BenchmarkHTTP_404(b *testing.B) {
	log := nopLogger{}
	ord := testutil.MakeOrder()
	h := NewHandler(svcOne{o: &ord}, log, 2*time.Second)
	r := makeLeanRouter(h)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest(http.MethodGet, "/nope", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			_, _ = io.Copy(io.Discard, w.Body)
			if w.Code != http.StatusNotFound {
				b.Fatalf("status=%d", w.Code)
			}
		}
	})
}

// --- nopLogger — логгер, который не делает ничего. ---

type nopLogger struct{}

func (nopLogger) Infof(context.Context, string, ...any)  {}
func (nopLogger) Warnf(context.Context, string, ...any)  {}
func (nopLogger) Errorf(context.Context, string, ...any) {}

// --- Стабы ---

type svcOne struct{ o *domain.Order }

func (s svcOne) GetOrder(context.Context, string) (*domain.Order, error) { return s.o, nil }
func (s svcOne) OrdersByCustomer(context.Context, string, int, int) ([]*domain.Order, error) {
	return []*domain.Order{s.o}, nil
}

// для списка: заранее подготовленная выборка N элементов (без аллокаций на каждом вызове)
type svcList struct{ list []*domain.Order }

func (s svcList) GetOrder(context.Context, string) (*domain.Order, error) { return s.list[0], nil }
func (s svcList) OrdersByCustomer(context.Context, string, int, int) ([]*domain.Order, error) {
	return s.list, nil
}

// --- функции-помощники ---

func makeLeanRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New() // без Recovery/otel/logger — получаем меньшую аллокацию
	r.GET("/order/:id", h.getOrderByID)
	r.GET("/customer/:id/orders", h.listOrdersByCustomer)
	return r
}

func makeFullRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	// prod пайплайн из NewRouter
	return NewRouter(h, "", "")
}

func benchServeGET(b *testing.B, r *gin.Engine, path string) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()

	// Параллельный режим ближе к реальности без TCP
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			// вычитываем тело
			_, _ = io.Copy(io.Discard, w.Body)
			if w.Code != http.StatusOK {
				b.Fatalf("status=%d", w.Code)
			}
		}
	})
}
