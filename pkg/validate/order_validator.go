package validate

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/internal/ports"
)

// Проверка, что OrderValidator удовлетворяет интерфейсу OrderValidator.
var _ ports.OrderValidator = (*OrderValidator)(nil)

// ErrInvalidOrder — базовая (sentinel error) ошибка валидации.
var ErrInvalidOrder = errors.New("order validation failed")

// OrderValidator — структура для валидации заказа.
type OrderValidator struct{}

// NewOrderValidator — конструктор OrderValidator.
// Возвращает ErrInvalidOrder (с обёрнутой причиной) при любой проблеме.
func NewOrderValidator() *OrderValidator { return &OrderValidator{} }

// Validate — проверяет корректность полей заказа.
func (v *OrderValidator) Validate(_ context.Context, order *domain.Order) error {
	if err := v.validateCore(order); err != nil {
		return err
	}
	if err := v.validatePayment(&order.Payment); err != nil {
		return err
	}
	if err := v.validateDelivery(&order.Delivery); err != nil {
		return err
	}
	return v.validateItems(order.Items)
}

// validateCore — валидация основных полей заказа.
func (v *OrderValidator) validateCore(order *domain.Order) error {
	if order == nil {
		return fmt.Errorf("%w: заказ не может быть nil", ErrInvalidOrder)
	}
	if order.OrderUID == "" {
		return fmt.Errorf("%w: order_uid обязателен", ErrInvalidOrder)
	}
	if order.TrackNumber == "" {
		return fmt.Errorf("%w: track_number обязателен", ErrInvalidOrder)
	}
	if order.Entry == "" {
		return fmt.Errorf("%w: entry (канал поступления) обязателен", ErrInvalidOrder)
	}
	if order.DateCreated.IsZero() || order.DateCreated.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return fmt.Errorf("%w: date_created некорректен", ErrInvalidOrder)
	}
	return nil
}

// Валидация платежа
func (v *OrderValidator) validatePayment(p *domain.Payment) error {
	if p.Transaction == "" {
		return fmt.Errorf("%w: payment.transaction обязателен", ErrInvalidOrder)
	}
	if p.Currency == "" {
		return fmt.Errorf("%w: payment.currency обязателен", ErrInvalidOrder)
	}
	if p.Amount < 0 {
		return fmt.Errorf("%w: payment.amount должен быть неотрицательным", ErrInvalidOrder)
	}
	return nil
}

// Валидация доставки
func (v *OrderValidator) validateDelivery(d *domain.Delivery) error {
	if d.Email == "" {
		return fmt.Errorf("%w: delivery.email обязателен", ErrInvalidOrder)
	}
	if _, err := mail.ParseAddress(d.Email); err != nil {
		return fmt.Errorf("%w: delivery.email некорректен", ErrInvalidOrder)
	}
	return nil
}

// Валидация товаров
func (v *OrderValidator) validateItems(items []domain.Item) error {
	if len(items) == 0 {
		return fmt.Errorf("%w: items не должен быть пустым", ErrInvalidOrder)
	}

	for i := range items {
		item := &items[i]
		idx := strconv.Itoa(i)

		if item.Name == "" {
			return fmt.Errorf("%w: items[%s].name обязателен", ErrInvalidOrder, idx)
		}
		if item.Price < 0 {
			return fmt.Errorf("%w: items[%s].price должен быть неотрицательным", ErrInvalidOrder, idx)
		}
		if item.TotalPrice < 0 {
			return fmt.Errorf("%w: items[%s].total_price должен быть неотрицательным", ErrInvalidOrder, idx)
		}
	}
	return nil
}
