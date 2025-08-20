package validate

import (
	"context"
	"errors"
	"net/mail"
	"strconv"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/internal/ports"
)

var _ ports.OrderValidator = (*OrderValidator)(nil)

type OrderValidator struct{}

func NewOrderValidator() *OrderValidator { return &OrderValidator{} }

func (v *OrderValidator) Validate(_ context.Context, order *domain.Order) error {
	// Валидация обязательных полей

	if order == nil {
		return errors.New("заказ не может быть nil")
	}
	if order.OrderUID == "" {
		return errors.New("order_uid обязателен")
	}
	if order.TrackNumber == "" {
		return errors.New("track_number обязателен")
	}
	if order.Entry == "" {
		return errors.New("entry (канал поступления) обязателен")
	}
	if order.DateCreated.IsZero() || order.DateCreated.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return errors.New("date_created некорректен")
	}

	// Валидация платежа
	if order.Payment.Transaction == "" {
		return errors.New("payment.transaction обязателен")
	}
	if order.Payment.Currency == "" {
		return errors.New("payment.currency обязателен")
	}
	if order.Payment.Amount < 0 {
		return errors.New("payment.amount должен быть неотрицательным")
	}

	// Валидация доставки
	if order.Delivery.Email == "" {
		return errors.New("delivery.email обязателен")
	}
	if _, err := mail.ParseAddress(order.Delivery.Email); err != nil {
		return errors.New("delivery.email некорректен")
	}

	// Валидация товаров
	if len(order.Items) == 0 {
		return errors.New("items не должен быть пустым")
	}
	for i, item := range order.Items {
		idx := strconv.Itoa(i)
		if item.Name == "" {
			return errors.New("items[" + idx + "].name обязателен")
		}
		if item.Price < 0 {
			return errors.New("items[" + idx + "].price должен быть неотрицательным")
		}
		if item.TotalPrice < 0 {
			return errors.New("items[" + idx + "].total_price должен быть неотрицательным")
		}
	}

	return nil
}
