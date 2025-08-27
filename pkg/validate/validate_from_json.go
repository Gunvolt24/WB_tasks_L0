package validate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/internal/ports"
)

// ValidateOrderFromJSON — валидация заказа из JSON.
func ValidateOrderFromJSON(ctx context.Context, validator ports.OrderValidator, raw []byte) (*domain.Order, error) {
	var order domain.Order
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&order); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	// гарантируем отсутствие полей вне структуры
	if err := dec.Decode(new(struct{})); err != io.EOF {
		return nil, fmt.Errorf("invalid json: trailing data")
	}
	if err := validator.Validate(ctx, &order); err != nil {
		return nil, err
	}
	return &order, nil
}
