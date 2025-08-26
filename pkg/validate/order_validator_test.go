package validate_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/pkg/validate"
)

func validOrder() *domain.Order {
	return &domain.Order{
		OrderUID:    "1",
		TrackNumber: "T1",
		Entry:       "WBIL",
		DateCreated: time.Date(2025, 11, 26, 6, 22, 19, 0, time.UTC),

		Payment: domain.Payment{
			Transaction: "1",
			Currency:    "RUB",
			Amount:      1,
		},

		Delivery: domain.Delivery{
			Email: "test@example.com",
		},

		Items: []domain.Item{
			{
				Name:       "Item 1",
				Price:      50,
				TotalPrice: 50,
			},
		},
	}
}

func TestOrderValidator_Validate(t *testing.T) {
	v := validate.NewOrderValidator()
	ctx := context.Background()

	t.Run("valid order", func(t *testing.T) {
		o := validOrder()
		if err := v.Validate(ctx, o); err != nil {
			t.Fatalf("expected valid order, got: %v", err)
		}
	})

	type testCase struct {
		name      string
		makeOrder func() *domain.Order
		msg       string
	}

	cases := []testCase{
		{
			name:      "nil order",
			makeOrder: func() *domain.Order { return nil },
			msg:       "заказ не может быть nil",
		},
		{
			name: "empty order_uid",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.OrderUID = ""
				return o
			},
			msg: "order_uid обязателен",
		},
		{
			name: "empty track_number",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.TrackNumber = ""
				return o
			},
			msg: "track_number обязателен",
		},
		{
			name: "empty entry",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.Entry = ""
				return o
			},
			msg: "entry (канал поступления) обязателен",
		},
		{
			name: "zero date_created",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.DateCreated = time.Time{}
				return o
			},
			msg: "date_created некорректен",
		},
		{
			name: "old date_created",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.DateCreated = time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)
				return o
			},
			msg: "date_created некорректен",
		},
		{
			name: "empty payment.transaction",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.Payment.Transaction = ""
				return o
			},
			msg: "payment.transaction обязателен",
		},
		{
			name: "empty payment.currency",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.Payment.Currency = ""
				return o
			},
			msg: "payment.currency обязателен",
		},
		{
			name: "negative payment.amount",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.Payment.Amount = -1
				return o
			},
			msg: "payment.amount должен быть неотрицательным",
		},
		{
			name: "empty delivery.email",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.Delivery.Email = ""
				return o
			},
			msg: "delivery.email обязателен",
		},
		{
			name: "invalid delivery.email",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.Delivery.Email = "not-an-email"
				return o
			},
			msg: "delivery.email некорректен",
		},
		{
			name: "empty items",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.Items = nil
				return o
			},
			msg: "items не должен быть пустым",
		},
		{
			name: "empty item.name",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.Items[0].Name = ""
				return o
			},
			msg: "items[0].name обязателен",
		},
		{
			name: "negative item.price",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.Items[0].Price = -1
				return o
			},
			msg: "items[0].price должен быть неотрицательным",
		},
		{
			name: "negative item.total_price",
			makeOrder: func() *domain.Order {
				o := validOrder()
				o.Items[0].TotalPrice = -1
				return o
			},
			msg: "items[0].total_price должен быть неотрицательным",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := tc.makeOrder()
			err := v.Validate(ctx, o)
			if err == nil {
				t.Errorf("expected error, got nil")
			}

			if !errors.Is(err, validate.ErrInvalidOrder) {
				t.Errorf("expected ErrInvalidOrder, got %v", err)
			}

			if !strings.Contains(err.Error(), tc.msg) {
				t.Errorf("expected error message to contain %q, got %q", tc.msg, err.Error())
			}
		})
	}
}
