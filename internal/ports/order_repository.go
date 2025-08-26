package ports

import (
	"context"

	"github.com/Gunvolt24/wb_l0/internal/domain"
)

// OrderRepository — контракт хранилища заказов (PostgreSQL).
type OrderRepository interface {
	// Save — создать или обновить заказ по OrderUID. Операция должна быть атомарной.
	Save(ctx context.Context, order *domain.Order) error

	// GetByUID — вернуть заказ по UID; (nil, nil), если не найден.
	GetByUID(ctx context.Context, orderUID string) (*domain.Order, error)

	// ListByCustomer — заказы клиента с пагинацией; сортировка по DateCreated DESC.
	ListByCustomer(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, error)

	// LastN — последние N заказов по DateCreated DESC (например, для прогрева кэша).
	LastN(ctx context.Context, n int) ([]*domain.Order, error)
}
