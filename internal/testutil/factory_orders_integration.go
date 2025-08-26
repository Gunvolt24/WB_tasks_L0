//go:build integration

package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/domain"
)

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func UniqSuffix() string { return randHex(6) }

// Мини-генератор валидного заказа
func MakeOrder(opts ...func(*domain.Order)) domain.Order {
	uid := "ord-" + UniqSuffix()
	now := time.Now().UTC().Truncate(time.Second)

	o := domain.Order{
		OrderUID:    uid,
		TrackNumber: "TR-" + UniqSuffix(),
		CustomerID:  "cust-" + UniqSuffix(),
		DateCreated: now,

		// Обязотаельное поле по валидатору
		Entry: "WBIL",

		Delivery: domain.Delivery{
			Name:    "John Smith",
			Phone:   "+1-202-555-01",
			Zip:     "000000",
			City:    "Metropolis",
			Address: "Main st 1",
			Region:  "NA",
			Email:   "john@example.com",
		},
		Payment: domain.Payment{
			Transaction:  uid, // txn == order_uid
			RequestID:    "",
			Currency:     "USD",
			Provider:     "test",
			Amount:       123,
			PaymentDT:    now.Unix(),
			Bank:         "TC-BANK",
			DeliveryCost: 10,
			GoodsTotal:   113,
			CustomFee:    0,
		},
		Items: []domain.Item{
			{
				ChrtID:      1001,
				TrackNumber: "TR-" + UniqSuffix(),
				Price:       100,
				RID:         "RID-1",
				Name:        "Widget",
				Sale:        0,
				Size:        "M",
				TotalPrice:  100,
				NmID:        1,
				Brand:       "brand",
				Status:      200,
			},
		},
	}

	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// Доп. опция — если нужно переопределить Entry в тесте
func WithEntry(entry string) func(*domain.Order) {
	return func(o *domain.Order) { o.Entry = entry }
}

func WithCustomer(cust string) func(*domain.Order) {
	return func(o *domain.Order) { o.CustomerID = cust }
}

func WithOrderUID(uid string) func(*domain.Order) {
	return func(o *domain.Order) {
		o.OrderUID = uid
		o.Payment.Transaction = uid
	}
}

func WithItems(n int) func(*domain.Order) {
	return func(o *domain.Order) {
		o.Items = make([]domain.Item, 0, n)
		for i := 0; i < n; i++ {
			o.Items = append(o.Items, domain.Item{
				ChrtID:      1000 + i,
				TrackNumber: "TR-" + UniqSuffix(),
				Price:       10 * (i + 1),
				RID:         "RID-" + UniqSuffix(),
				Name:        "Item",
				Sale:        0,
				Size:        "M",
				TotalPrice:  10 * (i + 1),
				NmID:        i + 1,
				Brand:       "brand",
				Status:      200,
			})
		}
	}
}
