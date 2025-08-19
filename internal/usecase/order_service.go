package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/internal/ports"
)

type OrderService struct {
	repo      ports.OrderRepository
	cache     ports.OrderCache
	log       ports.Logger
	validator ports.OrderValidator
}

func NewOrderService(repo ports.OrderRepository, cache ports.OrderCache, log ports.Logger, validator ports.OrderValidator) *OrderService {
	return &OrderService{repo: repo, cache: cache, log: log, validator: validator}
}

func (s *OrderService) GetOrder(ctx context.Context, orderUID string) (*domain.Order, error) {
	if order, found := s.cache.Get(ctx, orderUID); found {
		s.log.Infof(ctx, "cache hit for order=%s", orderUID)
		return order, nil
	}
	s.log.Infof(ctx, "cache miss for order=%s", orderUID)

	start := time.Now()
	order, err := s.repo.GetByUID(ctx, orderUID)
	if err != nil {
		s.log.Errorf(ctx, "repo.GetByUID failed order_uid=%s err=%v", orderUID, err)
		return nil, err
	}

	if order != nil {
		if err := s.cache.Set(ctx, order); err != nil {
			s.log.Warnf(ctx, "cache.Set failed order_uid=%s err=%v", orderUID, err)
		}
	}

	s.log.Infof(ctx, "db fetch order_uid=%s took=%s", orderUID, time.Since(start))
	return order, nil
}

func (s *OrderService) OrdersByCustomer(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, error) {
    return s.repo.ListByCustomer(ctx, customerID, limit, offset)
}

func (s *OrderService) SaveFromMessage(ctx context.Context, raw []byte) error {
	var order domain.Order
	if err := json.Unmarshal(raw, &order); err != nil {
		s.log.Warnf(ctx, "invalid json err=%v", err)
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := s.validator.Validate(ctx, &order); err != nil {
		s.log.Warnf(ctx, "validation failed order_uid=%s err=%v", order.OrderUID, err)
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.Save(ctx, &order); err != nil {
		s.log.Errorf(ctx, "repo.Save failed order_uid=%s err=%v", order.OrderUID, err)
		return fmt.Errorf("failed to save order: %w", err)
	}
	if err := s.cache.Set(ctx, &order); err != nil {
		s.log.Warnf(ctx, "cache.Set failed order_uid=%s err=%v", order.OrderUID, err)
	}
	s.log.Infof(ctx, "order saved uid=%s items=%d", order.OrderUID, len(order.Items))
	return nil
}

func (s *OrderService) WarmUpCache(ctx context.Context, n int) error {
	start := time.Now()
	list, err := s.repo.LastN(ctx, n)
	if err != nil {
		s.log.Errorf(ctx, "repo.LastN failed n=%d err=%v", n, err)
		return err
	}
	if err := s.cache.WarmUp(ctx, list); err != nil {
		s.log.Warnf(ctx, "cache.WarmUp failed err=%v", err)
	}
	s.log.Infof(ctx, "cache warmed with %d orders in %s", len(list), time.Since(start))
	return nil
}
