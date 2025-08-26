-- +goose Up
-- +goose StatementBegin
-- 1) Таблица клиентов
CREATE TABLE IF NOT EXISTS customers (
  id TEXT PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2) Бэкфилл клиентов из уже существующих заказов
INSERT INTO customers (id)
SELECT DISTINCT customer_id
FROM orders
WHERE customer_id IS NOT NULL
ON CONFLICT (id) DO NOTHING;

-- 3) Внешний ключ + индекс для быстрых выборок по клиенту
ALTER TABLE orders
  ADD CONSTRAINT fk_orders_customer
  FOREIGN KEY (customer_id) REFERENCES customers(id)
  ON UPDATE CASCADE
  ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_orders_customer_date_uid
  ON orders (customer_id, date_created DESC, order_uid DESC);

-- 4) Делаем customer_id обязательным 
ALTER TABLE orders ALTER COLUMN customer_id SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE orders ALTER COLUMN customer_id DROP NOT NULL;
DROP INDEX IF EXISTS idx_orders_customer_date_uid;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS fk_orders_customer;
DROP TABLE IF EXISTS customers;
-- +goose StatementEnd
