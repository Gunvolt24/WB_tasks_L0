package postgres

import (
	"context"
	"errors"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/internal/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ ports.OrderRepository = (*OrderRepository)(nil)

type OrderRepository struct {
	pool *pgxpool.Pool
}

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository { return &OrderRepository{pool: pool} }

// Save - транзакцонно сохраняет order + deliveries + payments + items.
func (r *OrderRepository) Save(ctx context.Context, order *domain.Order) error {
	if order == nil || order.OrderUID == "" {
		return errors.New("order is empty or order_uid is required")
	}
	if order.CustomerID == "" {
		return errors.New("customer_id is required")
	}

	transaction, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	if _, err = transaction.Exec(ctx, `
		INSERT INTO customers (id) VALUES ($1) 
		ON CONFLICT (id) DO NOTHING 
	`, order.CustomerID); err != nil {
		return err
	}

	// orders (upsert по Primary Key order_uid)
	if _, err = transaction.Exec(ctx, `
		INSERT INTO orders (
			order_uid, track_number, entry, locale, internal_signature, 
			customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (order_uid) DO UPDATE SET
			track_number = EXCLUDED.track_number,
			entry = EXCLUDED.entry,
			locale = EXCLUDED.locale,
			internal_signature = EXCLUDED.internal_signature,
			customer_id = EXCLUDED.customer_id,
			delivery_service = EXCLUDED.delivery_service,
			shardkey = EXCLUDED.shardkey,
			sm_id = EXCLUDED.sm_id,
			date_created = EXCLUDED.date_created,
			oof_shard = EXCLUDED.oof_shard
	`,
		order.OrderUID, order.TrackNumber, order.Entry, order.Locale, order.InternalSignature,
		order.CustomerID, order.DeliveryService, order.ShardKey, order.SmID, order.DateCreated, order.OofShard,
	); err != nil {
		return err
	}

	// deliveries (1:1)
	if _, err = transaction.Exec(ctx, `
		INSERT INTO deliveries (
			order_uid, name, phone, zip, city, address, region, email
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (order_uid) DO UPDATE SET
			name = EXCLUDED.name,
			phone = EXCLUDED.phone,
			zip = EXCLUDED.zip,
			city = EXCLUDED.city,
			address = EXCLUDED.address,
			region = EXCLUDED.region,
			email = EXCLUDED.email
	`,
		order.OrderUID, order.Delivery.Name, order.Delivery.Phone, order.Delivery.Zip,
		order.Delivery.City, order.Delivery.Address, order.Delivery.Region, order.Delivery.Email,
	); err != nil {
		return err
	}

	// payments (транзакции по Primary Key, order_uid UNIQUE NOT NULL)
	if _, err = transaction.Exec(ctx, `
		INSERT INTO payments (
			transaction, order_uid, request_id, currency, provider,
			amount, payment_dt, bank, delivery_cost, goods_total, custom_fee
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (order_uid) DO UPDATE SET
			order_uid = EXCLUDED.order_uid,
			request_id = EXCLUDED.request_id,
			currency = EXCLUDED.currency,
			provider = EXCLUDED.provider,
			amount = EXCLUDED.amount,
			payment_dt = EXCLUDED.payment_dt,
			bank = EXCLUDED.bank,
			delivery_cost = EXCLUDED.delivery_cost,
			goods_total = EXCLUDED.goods_total,
			custom_fee = EXCLUDED.custom_fee
	`,
		order.Payment.Transaction, order.OrderUID, order.Payment.RequestID, order.Payment.Currency,
		order.Payment.Provider, order.Payment.Amount, order.Payment.PaymentDT, order.Payment.Bank,
		order.Payment.DeliveryCost, order.Payment.GoodsTotal, order.Payment.CustomFee,
	); err != nil {
		return err
	}

	// items: DELETE + INSERT
	if _, err = transaction.Exec(ctx, `DELETE FROM items WHERE order_uid = $1`, order.OrderUID); err != nil {
		return err
	}
	for _, item := range order.Items {
		if _, err = transaction.Exec(ctx, `
			INSERT INTO items (
				order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`,
			order.OrderUID, item.ChrtID, item.TrackNumber, item.Price, item.RID, item.Name,
			item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand, item.Status,
		); err != nil {
			return err
		}
	}

	err = transaction.Commit(ctx)
	return err
}

func (r *OrderRepository) GetByUID(ctx context.Context, uid string) (*domain.Order, error) {
	var order domain.Order

	// orders
	err := r.pool.QueryRow(ctx, `
		SELECT order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service,
			shardkey, sm_id, date_created, oof_shard
		FROM orders WHERE order_uid = $1
	`, uid).Scan(&order.OrderUID, &order.TrackNumber, &order.Entry, &order.Locale, &order.InternalSignature,
		&order.CustomerID, &order.DeliveryService, &order.ShardKey, &order.SmID, &order.DateCreated, &order.OofShard,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// deliveries
	if err := r.pool.QueryRow(ctx, `
		SELECT name, phone, zip, city, address, region, email
		FROM deliveries WHERE order_uid = $1
	`, uid).Scan(&order.Delivery.Name, &order.Delivery.Phone, &order.Delivery.Zip, &order.Delivery.City,
		&order.Delivery.Address, &order.Delivery.Region, &order.Delivery.Email,
	); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// payments
	if err := r.pool.QueryRow(ctx, `
		SELECT transaction, request_id, currency, provider, amount, payment_dt, bank,
			delivery_cost, goods_total, custom_fee
		FROM payments WHERE order_uid = $1
	`, uid).Scan(&order.Payment.Transaction, &order.Payment.RequestID, &order.Payment.Currency,
		&order.Payment.Provider, &order.Payment.Amount, &order.Payment.PaymentDT, &order.Payment.Bank,
		&order.Payment.DeliveryCost, &order.Payment.GoodsTotal, &order.Payment.CustomFee,
	); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// items
	rows, err := r.pool.Query(ctx, `
		SELECT chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status
		FROM items WHERE order_uid = $1
	`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.Item
		if err := rows.Scan(
			&item.ChrtID, &item.TrackNumber, &item.Price, &item.RID, &item.Name, &item.Sale, &item.Size,
			&item.TotalPrice, &item.NmID, &item.Brand, &item.Status,
		); err != nil {
			return nil, err
		}
		order.Items = append(order.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, rows.Err()
	}

	return &order, nil
}

func (r *OrderRepository) ListByCustomer(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, error) {
    if limit <= 0 { limit = 20 }
    if offset < 0 { offset = 0 }

    rows, err := r.pool.Query(ctx, `
        SELECT order_uid
        FROM orders
        WHERE customer_id = $1
        ORDER BY date_created DESC
        LIMIT $2 OFFSET $3
    `, customerID, limit, offset)
    if err != nil { return nil, err }
    defer rows.Close()

    var uids []string
    for rows.Next() {
        var uid string
        if err := rows.Scan(&uid); err != nil {
            return nil, err
        }
        uids = append(uids, uid)
    }
    if err := rows.Err(); err != nil { return nil, err }

    result := make([]*domain.Order, 0, len(uids))
    for _, uid := range uids {
        o, err := r.GetByUID(ctx, uid)
        if err != nil { return nil, err }
        if o != nil { result = append(result, o) }
    }
    return result, nil
}

func (r *OrderRepository) LastN(ctx context.Context, n int) ([]*domain.Order, error) {
	if n <= 0 {
		return nil, nil
	}

	// получаем только order_uid и дочитываем полные заказы по одному
	rows, err := r.pool.Query(ctx, `
		SELECT order_uid
		FROM orders
		ORDER BY date_created DESC
		LIMIT $1
	`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.Order
	for rows.Next() {
		var orderUID string
		if err := rows.Scan(&orderUID); err != nil {
			return nil, err
		}
		order, err := r.GetByUID(ctx, orderUID)
		if err != nil {
			return nil, err
		}
		if order != nil {
			result = append(result, order)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, rows.Err()
	}

	return result, nil
}
