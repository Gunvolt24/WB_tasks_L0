package ports

import (
	"context"

	"github.com/Gunvolt24/wb_l0/internal/domain"
)

// OrderValidator — валидатор входящей модели заказа (структура + простые бизнес-правила).
// Используется "sentinel errors" (pkg/validate),
// чтобы потребитель (Kafka) различал невалидные сообщения и временные сбои.
type OrderValidator interface {
	Validate(ctx context.Context, order *domain.Order) error
}
