package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/internal/ports/mocks"
	"github.com/Gunvolt24/wb_l0/internal/usecase"
	"github.com/Gunvolt24/wb_l0/pkg/validate"
	"github.com/golang/mock/gomock"
)

const orderUID = "order-1"

type noopLogger struct{}

func (noopLogger) Infof(context.Context, string, ...any)  {}
func (noopLogger) Warnf(context.Context, string, ...any)  {}
func (noopLogger) Errorf(context.Context, string, ...any) {}

func TestGetOrder_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	o := &domain.Order{OrderUID: orderUID}

	cache.EXPECT().Get(gomock.Any(), orderUID).Return(o, true)

	svc := usecase.NewOrderService(repo, cache, log, validator)

	got, err := svc.GetOrder(context.Background(), orderUID)
	if err != nil || got == nil || got.OrderUID != orderUID {
		t.Fatalf("expected hit, got err=%v, order=%+v", err, got)
	}
}

func TestGetOrder_CacheMiss_FetchAndCache(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	o := &domain.Order{OrderUID: orderUID}

	gomock.InOrder(
		cache.EXPECT().Get(gomock.Any(), orderUID).Return(nil, false),
		repo.EXPECT().GetByUID(gomock.Any(), orderUID).Return(o, nil),
		cache.EXPECT().Set(gomock.Any(), o),
	)

	svc := usecase.NewOrderService(repo, cache, log, validator)

	got, err := svc.GetOrder(context.Background(), orderUID)
	if err != nil || got == nil || got.OrderUID != orderUID {
		t.Fatalf("expected miss, got err=%v, order=%+v", err, got)
	}
}

func TestSaveFromMessage_InvalidJson(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	svc := usecase.NewOrderService(repo, cache, log, validator)

	err := svc.SaveFromMessage(context.Background(), []byte("{"))
	if err == nil || !strings.Contains(err.Error(), "invalid json") {
		t.Fatalf("expected invalid json error, got err=%v", err)
	}
}

func TestSaveFromMessage_ValidationFailed(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	validator.EXPECT().Validate(gomock.Any(), gomock.AssignableToTypeOf(&domain.Order{})).Return(validate.ErrInvalidOrder)

	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	raw, err1 := json.Marshal(&domain.Order{OrderUID: orderUID, TrackNumber: "track-1", Entry: "entry-1",
		Delivery: domain.Delivery{Email: "email@email"}, Items: []domain.Item{{Name: "item"}}})

	if err1 != nil {
		t.Fatalf("unexpected error: %v", err1)
	}

	svc := usecase.NewOrderService(repo, cache, log, validator)

	err2 := svc.SaveFromMessage(context.Background(), raw)
	if err2 == nil || !errors.Is(err2, validate.ErrInvalidOrder) {
		t.Fatalf("want wrapped ErrInvalidOrder, got %v", err2)
	}
}

func TestSaveFromMessage_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	raw, err := json.Marshal(&domain.Order{
		OrderUID:    orderUID,
		TrackNumber: "track-1",
		Entry:       "entry-1",
		Delivery:    domain.Delivery{Email: "email@email.ru"},
		Items:       []domain.Item{{Name: "item"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gomock.InOrder(
		validator.EXPECT().Validate(gomock.Any(), gomock.AssignableToTypeOf(&domain.Order{})).Return(nil),
		repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil),
		cache.EXPECT().Set(gomock.Any(), gomock.AssignableToTypeOf(&domain.Order{})).Return(nil),
	)

	svc := usecase.NewOrderService(repo, cache, log, validator)

	saveErr := svc.SaveFromMessage(context.Background(), raw)
	if saveErr != nil {
		t.Fatalf("unexpected error: %v", saveErr)
	}
}

func TestWarmUpCache_SkipWhenLessThanZero(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	svc := usecase.NewOrderService(repo, cache, log, validator)
	if err := svc.WarmUpCache(context.Background(), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetOrder_CacheMiss_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	cache.EXPECT().Get(gomock.Any(), orderUID).Return(nil, false)
	repoErr := errors.New("DB down")
	repo.EXPECT().GetByUID(gomock.Any(), orderUID).Return(nil, repoErr)

	svc := usecase.NewOrderService(repo, cache, log, validator)
	got, err := svc.GetOrder(context.Background(), orderUID)
	if err == nil || !errors.Is(err, repoErr) {
		t.Fatalf("expected repo error, got order=%v, err=%+v", got, err)
	}
}

func TestGetOrder_CacheMiss_NotFound_NoCache(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	cache.EXPECT().Get(gomock.Any(), orderUID).Return(nil, false)
	repo.EXPECT().GetByUID(gomock.Any(), orderUID).Return(nil, nil)
	cache.EXPECT().Set(gomock.Any(), gomock.Any()).Times(0)

	svc := usecase.NewOrderService(repo, cache, log, validator)
	got, err := svc.GetOrder(context.Background(), orderUID)
	if err != nil || got != nil {
		t.Fatalf("expected not found, got order=%v, err=%+v", got, err)
	}
}

func TestGetOrder_CacheMiss_CacheSetWarnOnly(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	o := &domain.Order{OrderUID: orderUID}

	gomock.InOrder(
		cache.EXPECT().Get(gomock.Any(), orderUID).Return(nil, false),
		repo.EXPECT().GetByUID(gomock.Any(), orderUID).Return(o, nil),
		cache.EXPECT().Set(gomock.Any(), o).Return(errors.New("cache set failed")),
	)

	svc := usecase.NewOrderService(repo, cache, log, validator)
	got, err := svc.GetOrder(context.Background(), orderUID)
	if err != nil || got == nil || got.OrderUID != orderUID {
		t.Fatalf("expected miss, got err=%v, order=%+v", err, got)
	}
}

func TestSaveFromMessage_TrailingData(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	base, err1 := json.Marshal(&domain.Order{
		OrderUID:    orderUID,
		TrackNumber: "track-1",
		Entry:       "entry-1",
		Delivery:    domain.Delivery{Email: "email@email"},
		Items:       []domain.Item{{Name: "item"}},
	})

	if err1 != nil {
		t.Fatalf("unexpected error: %v", err1)
	}

	raw := append([]byte{}, base...)
	raw = append(raw, []byte(" {}")...)

	svc := usecase.NewOrderService(repo, cache, log, validator)
	err2 := svc.SaveFromMessage(context.Background(), raw)
	if err2 == nil || !strings.Contains(err2.Error(), "trailing data") {
		t.Fatalf("want trailing data error, got %v", err2)
	}
}

func TestSaveFromMessage_RepoErr(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	raw, err1 := json.Marshal(&domain.Order{
		OrderUID:    orderUID,
		TrackNumber: "track-1",
		Entry:       "entry-1",
		Delivery:    domain.Delivery{Email: "email@email"},
		Items:       []domain.Item{{Name: "item"}},
	})

	if err1 != nil {
		t.Fatalf("unexpected error: %v", err1)
	}

	gomock.InOrder(
		validator.EXPECT().Validate(gomock.Any(), gomock.AssignableToTypeOf(&domain.Order{})).Return(nil),
		repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("insert failed")),
	)

	svc := usecase.NewOrderService(repo, cache, log, validator)
	err2 := svc.SaveFromMessage(context.Background(), raw)
	if err2 == nil || !strings.Contains(err2.Error(), "failed to save order") {
		t.Fatalf("want wrapped save error, got %v", err2)
	}
}

func TestWarmUpCache_RepoErr(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	repo.EXPECT().LastN(gomock.Any(), 3).Return(nil, errors.New("DB down"))
	cache.EXPECT().WarmUp(gomock.Any(), gomock.Any()).Times(0)

	svc := usecase.NewOrderService(repo, cache, log, validator)
	if err := svc.WarmUpCache(context.Background(), 3); err == nil {
		t.Fatalf("want wrapped repo error, got %v", err)
	}
}

func TestWarmUpCache_WarnOnly(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	validator := mocks.NewMockOrderValidator(ctrl)

	list := []*domain.Order{{OrderUID: orderUID}}
	gomock.InOrder(
		repo.EXPECT().LastN(gomock.Any(), 2).Return(list, nil),
		cache.EXPECT().WarmUp(gomock.Any(), list).Return(errors.New("cache warm up failed")),
	)

	svc := usecase.NewOrderService(repo, cache, log, validator)
	if err := svc.WarmUpCache(context.Background(), 2); err != nil {
		t.Fatalf("warmup warning must not fail, got %v", err)
	}
}

func TestOrdersByCustomer_Proxy(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := mocks.NewMockOrderRepository(ctrl)
	cache := mocks.NewMockOrderCache(ctrl)
	log := noopLogger{}
	val := mocks.NewMockOrderValidator(ctrl)

	want := []*domain.Order{{OrderUID: "a"}, {OrderUID: "b"}}
	repo.EXPECT().ListByCustomer(gomock.Any(), "cust-1", 10, 20).Return(want, nil)

	svc := usecase.NewOrderService(repo, cache, log, val)
	got, err := svc.OrdersByCustomer(context.Background(), "cust-1", 10, 20)
	if err != nil || len(got) != 2 || got[0].OrderUID != "a" || got[1].OrderUID != "b" {
		t.Fatalf("unexpected result: %+v, err=%v", got, err)
	}
}
