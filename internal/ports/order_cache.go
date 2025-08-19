package ports

import (
	"context"

	"github.com/Gunvolt24/wb_l0/internal/domain"
)

type OrderCache interface {
	Get(ctx context.Context, orderUID string) (*domain.Order, bool)
	Set(ctx context.Context, order *domain.Order) error
	WarmUp(ctx context.Context, orders []*domain.Order) error
}
