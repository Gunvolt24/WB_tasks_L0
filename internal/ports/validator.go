package ports

import (
	"context"

	"github.com/Gunvolt24/wb_l0/internal/domain"
)

type OrderValidator interface {
	Validate(ctx context.Context, order *domain.Order) error
}
