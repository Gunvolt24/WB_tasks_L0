package ports

import (
	"context"

	"github.com/Gunvolt24/wb_l0/internal/domain"
)

type OrderRepository interface {
	Save(ctx context.Context, order *domain.Order) error
	GetByUID(ctx context.Context, orderUID string) (*domain.Order, error)
	ListByCustomer(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, error)
	LastN(ctx context.Context, n int) ([]*domain.Order, error)
}
