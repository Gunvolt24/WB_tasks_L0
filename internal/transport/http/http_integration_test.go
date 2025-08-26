//go:build integration

package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cachemem "github.com/Gunvolt24/wb_l0/internal/cache/memory"
	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/stretchr/testify/require"

	pgrepo "github.com/Gunvolt24/wb_l0/internal/repo/postgres"
	"github.com/Gunvolt24/wb_l0/internal/testutil"
	rest "github.com/Gunvolt24/wb_l0/internal/transport/http"
	"github.com/Gunvolt24/wb_l0/internal/usecase"
	"github.com/Gunvolt24/wb_l0/pkg/logger"
	"github.com/Gunvolt24/wb_l0/pkg/validate"
)

// 1) GET /order/:id - 200 успешная обработка, 404 когда заказа нет
func TestHTTP_GetOrder_TC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	pg, stop, err := testutil.StartPostgresTC(ctx)
	require.NoError(t, err)
	defer func() { _ = stop(context.Background()) }()
	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	logg, cleanup, err := logger.NewZapLogger(false)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	repo := pgrepo.NewOrderRepository(pg.Pool)
	svc := usecase.NewOrderService(repo, cachemem.NewLRUCacheTTL(100, time.Minute), logg, validate.NewOrderValidator())

	// seed: генерим уникальный заказ
	ord := testutil.MakeOrder()
	require.NoError(t, repo.Save(ctx, &ord))

	// http
	h := rest.NewHandler(svc, logg, 2*time.Second)
	r := rest.NewRouter(h, "", "")
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/order/" + ord.OrderUID)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
}

// 2) GET /order/:id — 404 когда заказа нет
func TestHTTP_GetOrder_NotFound_TC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	pg, stop, err := testutil.StartPostgresTC(ctx)
	require.NoError(t, err)
	defer func() { _ = stop(context.Background()) }()
	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	logg, cleanup, err := logger.NewZapLogger(false)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	repo := pgrepo.NewOrderRepository(pg.Pool)
	svc := usecase.NewOrderService(repo, cachemem.NewLRUCacheTTL(100, time.Minute), logg, validate.NewOrderValidator())

	h := rest.NewHandler(svc, logg, 2*time.Second)
	r := rest.NewRouter(h, "", "")
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/order/not-existing-uid")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	var got map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, "order not found", got["error"])
}

// 3) POST /order/:id — 405 Method Not Allowed + заголовок Allow: GET
func TestHTTP_GetOrder_MethodNotAllowed_TC(t *testing.T) {
	logg, cleanup, err := logger.NewZapLogger(false)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	h := rest.NewHandler(noOpService{}, logg, 2*time.Second)
	r := rest.NewRouter(h, "", "")
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/order/some-id", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	require.Equal(t, "GET", resp.Header.Get("Allow"))

	var got map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, "method not allowed", got["error"])
}

// 3) GET /customer/:id/orders — пагинация (limit/offset) и фильтрация по customer_id
func TestHTTP_ListOrdersByCustomer_Pagination_TC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	pg, stop, err := testutil.StartPostgresTC(ctx)
	require.NoError(t, err)
	defer func() { _ = stop(context.Background()) }()
	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	logg, cleanup, err := logger.NewZapLogger(false)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	repo := pgrepo.NewOrderRepository(pg.Pool)
	svc := usecase.NewOrderService(repo, cachemem.NewLRUCacheTTL(100, time.Minute), logg, validate.NewOrderValidator())

	// seed: 3 заказа одного клиента + 1 другого
	const cust = "cust-pagination"
	for i := 0; i < 3; i++ {
		o := testutil.MakeOrder(testutil.WithCustomer(cust))
		require.NoError(t, repo.Save(ctx, &o))
	}
	oOther := testutil.MakeOrder(testutil.WithCustomer("cust-other"))
	require.NoError(t, repo.Save(ctx, &oOther))

	// router
	h := rest.NewHandler(svc, logg, 2*time.Second)
	r := rest.NewRouter(h, "", "")
	ts := httptest.NewServer(r)
	defer ts.Close()

	// limit=2 offset=1 — ожидаем 2 заказа данного клиента
	resp, err := http.Get(ts.URL + fmt.Sprintf("/customer/%s/orders?limit=2&offset=1", cust))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got []domain.Order
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))

	require.Len(t, got, 2)
	for _, ord := range got {
		require.Equal(t, cust, ord.CustomerID)
	}
}

// 4) /ping, /metrics, 404 на неизвестный маршрут
func TestHTTP_Health_Metrics_And_404_TC(t *testing.T) {
	logg, cleanup, err := logger.NewZapLogger(false)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	h := rest.NewHandler(noOpService{}, logg, 2*time.Second)
	r := rest.NewRouter(h, "", "")
	ts := httptest.NewServer(r)
	defer ts.Close()

	// /ping
	resp, err := http.Get(ts.URL + "/ping")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := readAll(t, resp.Body)
	require.Equal(t, "pong", string(body))

	// /metrics
	respM, err := http.Get(ts.URL + "/metrics")
	require.NoError(t, err)
	defer respM.Body.Close()
	require.Equal(t, http.StatusOK, respM.StatusCode)
	require.NotEmpty(t, readAll(t, respM.Body)) // достаточно, что не пусто

	// 404
	resp404, err := http.Get(ts.URL + "/no/such/route")
	require.NoError(t, err)
	defer resp404.Body.Close()
	require.Equal(t, http.StatusNotFound, resp404.StatusCode)

	var got map[string]any
	require.NoError(t, json.NewDecoder(resp404.Body).Decode(&got))
	require.Equal(t, "route not found", got["error"])
}

// 5) Таймаут запросов: Handler с коротким reqTimeout должен вернуть 500
func TestHTTP_GetOrder_Timeout_500_TC(t *testing.T) {
	// Логгер и роутер со slowService, таймаут очень короткий
	logg, cleanup, err := logger.NewZapLogger(false)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	h := rest.NewHandler(slowService{}, logg, 10*time.Millisecond)
	r := rest.NewRouter(h, "", "")
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/order/any")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Ожидаем 500, так как slowService вернёт ctx.Err() по таймауту
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var got map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))

	// Сообщение у вас стандартизировано "internal server error"
	require.Equal(t, "internal server error", got["error"])
}

// --- функции помощники ---

// noOpService — простая заглушка для роутера, где неважно, что вернёт бизнес-логика.
type noOpService struct{}

func (noOpService) GetOrder(context.Context, string) (*domain.Order, error) { return nil, nil }
func (noOpService) OrdersByCustomer(context.Context, string, int, int) ([]*domain.Order, error) {
	return nil, nil
}

// slowService — всегда ждёт ctx.Done() и возвращает ошибку контекста (для проверки таймаута 500).
type slowService struct{}

func (slowService) GetOrder(ctx context.Context, _ string) (*domain.Order, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (slowService) OrdersByCustomer(ctx context.Context, _ string, _, _ int) ([]*domain.Order, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// readAll — просто прочитать тело.
func readAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return b
}
