package ports

import (
	"context"

	"github.com/Gunvolt24/wb_l0/internal/domain"
)

// OrderCache — интерфейс кэша заказов.
// Требования к реализации: потокобезопасность; доступ по ключу не хуже O(1); возврат копий сущности.
type OrderCache interface {
	// Get — вернуть заказ по UID; (order, true) при попадании, (nil, false) при промахе/истечении.
	Get(ctx context.Context, orderUID string) (*domain.Order, bool)

	// Set — сохранить/обновить заказ в кэше.
	Set(ctx context.Context, order *domain.Order) error

	// WarmUp — массовая загрузка кэша (например, при старте).
	// Реализация должна поддерживать отмену контекста.
	WarmUp(ctx context.Context, orders []*domain.Order) error
}
