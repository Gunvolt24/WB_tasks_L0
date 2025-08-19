-- +goose Up
-- +goose StatementBegin
CREATE TABLE orders (
  order_uid           TEXT PRIMARY KEY,
  track_number        TEXT        NOT NULL,
  entry               TEXT        NOT NULL,
  locale              TEXT,
  internal_signature  TEXT,
  customer_id         TEXT,
  delivery_service    TEXT,
  shardkey            TEXT,
  sm_id               INTEGER,
  date_created        TIMESTAMPTZ NOT NULL,
  oof_shard           TEXT
);

CREATE TABLE deliveries (
  order_uid TEXT PRIMARY KEY
            REFERENCES orders(order_uid) ON DELETE CASCADE,
  name      TEXT,
  phone     TEXT,
  zip       TEXT,
  city      TEXT,
  address   TEXT,
  region    TEXT,
  email     TEXT
);

-- PK по transaction, а order_uid — UNIQUE NOT NULL 
CREATE TABLE payments (
  transaction   TEXT PRIMARY KEY,
  order_uid     TEXT NOT NULL UNIQUE
                REFERENCES orders(order_uid) ON DELETE CASCADE,
  request_id    TEXT,
  currency      TEXT,
  provider      TEXT,
  amount        INTEGER NOT NULL CHECK (amount >= 0),
  payment_dt    BIGINT  NOT NULL CHECK (payment_dt >= 0),
  bank          TEXT,
  delivery_cost INTEGER NOT NULL CHECK (delivery_cost >= 0),
  goods_total   INTEGER NOT NULL CHECK (goods_total >= 0),
  custom_fee    INTEGER NOT NULL CHECK (custom_fee >= 0)
);

CREATE TABLE items (
  id           BIGSERIAL PRIMARY KEY,
  order_uid    TEXT NOT NULL
               REFERENCES orders(order_uid) ON DELETE CASCADE,
  chrt_id      INTEGER,
  track_number TEXT,
  price        INTEGER NOT NULL CHECK (price >= 0),
  rid          TEXT,
  name         TEXT,
  sale         INTEGER NOT NULL CHECK (sale >= 0),
  size         TEXT,
  total_price  INTEGER NOT NULL CHECK (total_price >= 0),
  nm_id        INTEGER,
  brand        TEXT,
  status       INTEGER
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS deliveries;
DROP TABLE IF EXISTS orders;
-- +goose StatementEnd
