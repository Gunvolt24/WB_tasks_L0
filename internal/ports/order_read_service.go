package ports

import (
	"context"

	"github.com/Gunvolt24/wb_l0/internal/domain"
)

// OrderReadService — сервис чтения заказов.
type OrderReadService interface {
	GetOrder(ctx context.Context, orderUID string) (*domain.Order, error)
	OrdersByCustomer(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, error)
}
