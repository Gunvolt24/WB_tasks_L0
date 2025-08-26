package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/internal/ports"
)

// OrderService — прикладная логика работы с заказами (без знаний о транспорте).
type OrderService struct {
	repo      ports.OrderRepository // прямой доступ к хранилищу
	cache     ports.OrderCache      // прямой доступ к кэшу
	log       ports.Logger          // прямой доступ к логгеру
	validator ports.OrderValidator  // прямой доступ к валидатору
}

// NewOrderService — DI-конструктор.
func NewOrderService(
	repo ports.OrderRepository,
	cache ports.OrderCache,
	log ports.Logger,
	validator ports.OrderValidator,
) *OrderService {
	return &OrderService{
		repo:      repo,
		cache:     cache,
		log:       log,
		validator: validator,
	}
}

// GetOrder — получить заказ по UID: сначала из кэша, при промахе — из БД с записью в кэш.
// Возвращает (*Order, nil) или (nil, nil), если записи нет.
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
		// Кэшируем результат
		if setErr := s.cache.Set(ctx, order); setErr != nil {
			s.log.Warnf(ctx, "cache.Set failed order_uid=%s err=%v", orderUID, setErr)
		}
	}

	s.log.Infof(ctx, "db fetch order_uid=%s took=%s", orderUID, time.Since(start))
	return order, nil
}

// OrdersByCustomer — проксирование в репозиторий (пагинация уже валидирована на верхнем уровне).
func (s *OrderService) OrdersByCustomer(
	ctx context.Context,
	customerID string,
	limit, offset int,
) ([]*domain.Order, error) {
	return s.repo.ListByCustomer(ctx, customerID, limit, offset)
}

// SaveFromMessage — сохранить заказ, пришедший из Kafka (raw JSON).
// Шаги:
//  1. строгий парсинг JSON (DisallowUnknownFields) —> отлавливаем незадокументированные поля;
//  2. доменная валидация (вернёт validate.ErrInvalidOrder при проблемах);
//  3. транзакционное сохранение в БД (идемпотентные upsert);
//  4. положить запись в кэш.
func (s *OrderService) SaveFromMessage(ctx context.Context, raw []byte) error {
	// Строгое декодирование: запрещаем неизвестные поля.
	var order domain.Order
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&order); err != nil {
		s.log.Warnf(ctx, "invalid json err=%v", err)
		return fmt.Errorf("invalid json: %w", err)
	}

	// Убеждаемся, что после объекта нет лишних данных.
	if err := dec.Decode(new(struct{})); err != io.EOF {
		s.log.Warnf(ctx, "invalid json: trailing data")
		return fmt.Errorf("invalid json: trailing data")
	}

	// Доменная валидация (обязательные поля, корректность email, суммы и т.д.).
	if err := s.validator.Validate(ctx, &order); err != nil {
		s.log.Warnf(ctx, "validation failed order_uid=%s err=%v", order.OrderUID, err)
		return fmt.Errorf("validation failed: %w", err)
	}

	// Сохранение в БД в транзакции.
	if err := s.repo.Save(ctx, &order); err != nil {
		s.log.Errorf(ctx, "repo.Save failed order_uid=%s err=%v", order.OrderUID, err)
		return fmt.Errorf("failed to save order: %w", err)
	}

	// Обновление кэша.
	if err := s.cache.Set(ctx, &order); err != nil {
		s.log.Warnf(ctx, "cache.Set failed order_uid=%s err=%v", order.OrderUID, err)
	}

	s.log.Infof(ctx, "order saved uid=%s items=%d", order.OrderUID, len(order.Items))
	return nil
}

// WarmUpCache — прогрев кэша последними N заказами из БД.
// Если n <= 0, прогрев не выполняется (но это не ошибка).
func (s *OrderService) WarmUpCache(ctx context.Context, n int) error {
	if n <= 0 {
		s.log.Warnf(ctx, "cache warm-up skipped: n <= 0 (n=%d)", n)
		return nil
	}

	start := time.Now()
	list, err := s.repo.LastN(ctx, n)
	if err != nil {
		s.log.Errorf(ctx, "repo.LastN failed n=%d err=%v", n, err)
		return err
	}
	if warmUpErr := s.cache.WarmUp(ctx, list); warmUpErr != nil {
		s.log.Warnf(ctx, "cache.WarmUp failed err=%v", warmUpErr)
	}
	s.log.Infof(ctx, "cache warmed with %d orders in %s", len(list), time.Since(start))
	return nil
}
