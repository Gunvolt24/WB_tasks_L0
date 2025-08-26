package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/internal/ports/mocks"
	rest "github.com/Gunvolt24/wb_l0/internal/transport/http"
	"github.com/golang/mock/gomock"
)

type noopLogger struct{}

func (noopLogger) Infof(context.Context, string, ...any)  {}
func (noopLogger) Warnf(context.Context, string, ...any)  {}
func (noopLogger) Errorf(context.Context, string, ...any) {}

func TestGetOrder_Found(t *testing.T) {
	ctrl := gomock.NewController(t)

	svc := mocks.NewMockOrderReadService(ctrl)
	log := noopLogger{}

	want := &domain.Order{OrderUID: "order-1", Items: []domain.Item{{Name: "item"}}}
	svc.EXPECT().GetOrder(gomock.Any(), "order-1").Return(want, nil)

	h := rest.NewHandler(svc, log, 0)
	r := rest.NewRouter(h, "", "test")

	req := httptest.NewRequest(http.MethodGet, "/order/order-1", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got domain.Order
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got.OrderUID != "order-1" {
		t.Fatalf("wrong order uid: %v", got)
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)

	svc := mocks.NewMockOrderReadService(ctrl)
	log := noopLogger{}

	svc.EXPECT().GetOrder(gomock.Any(), "missing").Return(nil, nil)

	h := rest.NewHandler(svc, log, 0)
	r := rest.NewRouter(h, "", "test")

	req := httptest.NewRequest(http.MethodGet, "/order/missing", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestGetOrder_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)

	svc := mocks.NewMockOrderReadService(ctrl)
	log := noopLogger{}

	svc.EXPECT().GetOrder(gomock.Any(), "intErr").Return(nil, errors.New("db error"))

	h := rest.NewHandler(svc, log, 0)
	r := rest.NewRouter(h, "", "test")

	req := httptest.NewRequest(http.MethodGet, "/order/intErr", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestListOrdersByCustomer_OK_Default(t *testing.T) {
	ctrl := gomock.NewController(t)

	svc := mocks.NewMockOrderReadService(ctrl)
	log := noopLogger{}

	// В хендлере defaultLimit = 20, offset по умолчанию пусть будет 0
	ret := []*domain.Order{{OrderUID: "a"}, {OrderUID: "b"}}
	svc.EXPECT().OrdersByCustomer(gomock.Any(), "cust-1", 20, 0).Return(ret, nil)

	h := rest.NewHandler(svc, log, 0)
	r := rest.NewRouter(h, "", "test")

	req := httptest.NewRequest(http.MethodGet, "/customer/cust-1/orders", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got []*domain.Order
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(got) != 2 || got[0].OrderUID != "a" || got[1].OrderUID != "b" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestListOrdersByCustomer_OK_WithParams(t *testing.T) {
	ctrl := gomock.NewController(t)

	svc := mocks.NewMockOrderReadService(ctrl)
	log := noopLogger{}

	ret := []*domain.Order{{OrderUID: "x"}}
	svc.EXPECT().OrdersByCustomer(gomock.Any(), "cust-9", 3, 7).Return(ret, nil)

	h := rest.NewHandler(svc, log, 0)
	r := rest.NewRouter(h, "", "test")

	req := httptest.NewRequest(http.MethodGet, "/customer/cust-9/orders?limit=3&offset=7", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got []*domain.Order
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(got) != 1 || got[0].OrderUID != "x" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestListOrdersByCustomer_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)

	svc := mocks.NewMockOrderReadService(ctrl)
	log := noopLogger{}

	svc.EXPECT().OrdersByCustomer(gomock.Any(), "cust-err", 20, 0).Return(nil, errors.New("service error"))

	h := rest.NewHandler(svc, log, 0)
	r := rest.NewRouter(h, "", "test")

	req := httptest.NewRequest(http.MethodGet, "/customer/cust-err/orders", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestNoRoute_404(t *testing.T) {
	ctrl := gomock.NewController(t)

	svc := mocks.NewMockOrderReadService(ctrl)
	log := noopLogger{}

	h := rest.NewHandler(svc, log, 0)
	r := rest.NewRouter(h, "", "test")

	req := httptest.NewRequest(http.MethodGet, "/no-such-route", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestMethodNotAllowed_405(t *testing.T) {
	ctrl := gomock.NewController(t)

	svc := mocks.NewMockOrderReadService(ctrl)
	log := noopLogger{}

	h := rest.NewHandler(svc, log, 0)
	r := rest.NewRouter(h, "", "test")

	req := httptest.NewRequest(http.MethodPost, "/order/123", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d, body=%s", w.Code, w.Body.String())
	}
	if allow := w.Header().Get("Allow"); allow != "GET" {
		t.Fatalf("want Allow: GET, got %q", allow)
	}
}

func TestPing_200(t *testing.T) {
	ctrl := gomock.NewController(t)

	svc := mocks.NewMockOrderReadService(ctrl)
	log := noopLogger{}

	h := rest.NewHandler(svc, log, 0)
	r := rest.NewRouter(h, "", "test")

	req := httptest.NewRequest(http.MethodGet, "/ping", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestMetrics_200(t *testing.T) {
	ctrl := gomock.NewController(t)

	svc := mocks.NewMockOrderReadService(ctrl)
	log := noopLogger{}

	h := rest.NewHandler(svc, log, 0)
	r := rest.NewRouter(h, "", "test")

	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	// Содержимое может меняться — достаточно проверить, что не пусто.
	if w.Body.Len() == 0 {
		t.Fatal("metrics body is empty")
	}
}
