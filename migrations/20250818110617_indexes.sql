-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_orders_date_created ON orders (date_created DESC);

-- join’ы/фильтры по заказу
CREATE INDEX IF NOT EXISTS idx_deliveries_order_uid ON deliveries (order_uid);
CREATE INDEX IF NOT EXISTS idx_payments_order_uid   ON payments (order_uid);
CREATE INDEX IF NOT EXISTS idx_items_order_uid      ON items (order_uid);

-- потенциально полезные для поиска
CREATE INDEX IF NOT EXISTS idx_orders_track_number ON orders (track_number);
CREATE INDEX IF NOT EXISTS idx_items_rid           ON items (rid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_items_rid;
DROP INDEX IF EXISTS idx_orders_track_number;
DROP INDEX IF EXISTS idx_items_order_uid;
DROP INDEX IF EXISTS idx_payments_order_uid;
DROP INDEX IF EXISTS idx_deliveries_order_uid;
DROP INDEX IF EXISTS idx_orders_date_created;
-- +goose StatementEnd
