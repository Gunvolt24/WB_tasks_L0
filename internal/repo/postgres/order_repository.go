package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/internal/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Проверка, что OrderRepository удовлетворяет интерфейсу OrderRepository.
var _ ports.OrderRepository = (*OrderRepository)(nil)

// OrderRepository — реализация репозитория заказов на Postgres (pgxpool).
type OrderRepository struct {
	pool *pgxpool.Pool
}

// NewOrderRepository - конструктор OrderRepository.
func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository { return &OrderRepository{pool: pool} }

// Save — транзакционно сохраняет заказ (идемпотентный upsert всех частей).
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
	defer func() {
		// При уже завершённой транзакции Rollback вернёт ErrTxClosed — игнорируем.
		if rbErr := transaction.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			_ = rbErr
		}
	}()

	// 1) customers — upsert (оставляем, чтобы не падать на FK).
	if _, err = transaction.Exec(ctx, `
		INSERT INTO customers (id) VALUES ($1) 
		ON CONFLICT (id) DO NOTHING 
	`, order.CustomerID); err != nil {
		return fmt.Errorf("insert customer: %w", err)
	}

	// 2) orders — upsert по order_uid (PRIMARY KEY/UNIQUE).
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
		return fmt.Errorf("upsert order: %w", err)
	}

	// 3) deliveries — upsert 1:1 по order_uid.
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
		return fmt.Errorf("upsert delivery: %w", err)
	}

	// 4) payments — upsert по order_uid (order_uid не обновляем).
	if _, err = transaction.Exec(ctx, `
		INSERT INTO payments (
			transaction, order_uid, request_id, currency, provider,
			amount, payment_dt, bank, delivery_cost, goods_total, custom_fee
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (order_uid) DO UPDATE SET
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
		return fmt.Errorf("upsert payment: %w", err)
	}

	// 5) items — replace: удаляем и вставляем список заново.
	if _, err = transaction.Exec(ctx, `DELETE FROM items WHERE order_uid = $1`, order.OrderUID); err != nil {
		return fmt.Errorf("delete items: %w", err)
	}
	if len(order.Items) > 0 {
		if err = copyItems(ctx, transaction, order.OrderUID, order.Items); err != nil {
			return err
		}
	}

	// Завершаем транзакцию
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// GetByUID — получить заказ по uid (1:1). Если не нашли, возвращает (nil, nil).
func (r *OrderRepository) GetByUID(ctx context.Context, uid string) (*domain.Order, error) {
	var order domain.Order

	// orders (основная запись)
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
		return nil, fmt.Errorf("select order: %w", err)
	}

	// deliveries (может отсутствовать)
	if err := r.pool.QueryRow(ctx, `
		SELECT name, phone, zip, city, address, region, email
		FROM deliveries WHERE order_uid = $1
	`, uid).Scan(&order.Delivery.Name, &order.Delivery.Phone, &order.Delivery.Zip, &order.Delivery.City,
		&order.Delivery.Address, &order.Delivery.Region, &order.Delivery.Email,
	); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("select delivery: %w", err)
	}

	// payments (может отсутствовать)
	if err := r.pool.QueryRow(ctx, `
		SELECT transaction, request_id, currency, provider, amount, payment_dt, bank,
			delivery_cost, goods_total, custom_fee
		FROM payments WHERE order_uid = $1
	`, uid).Scan(&order.Payment.Transaction, &order.Payment.RequestID, &order.Payment.Currency,
		&order.Payment.Provider, &order.Payment.Amount, &order.Payment.PaymentDT, &order.Payment.Bank,
		&order.Payment.DeliveryCost, &order.Payment.GoodsTotal, &order.Payment.CustomFee,
	); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("select payment: %w", err)
	}

	// items (0..N)
	rows, err := r.pool.Query(ctx, `
		SELECT chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status
		FROM items WHERE order_uid = $1
	`, uid)
	if err != nil {
		return nil, fmt.Errorf("select items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.Item
		if err := rows.Scan(
			&item.ChrtID, &item.TrackNumber, &item.Price, &item.RID, &item.Name, &item.Sale, &item.Size,
			&item.TotalPrice, &item.NmID, &item.Brand, &item.Status,
		); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		order.Items = append(order.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("items rows: %w", err)
	}

	return &order, nil
}

// ListByCustomer — постраничный список заказов клиента.
// Делает 4 запроса на страницу (пагинация): базовые заказы + payments + deliveries + items,
// затем склеивает всё в памяти, сохраняя порядок.
func (r *OrderRepository) ListByCustomer(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// 1) База заказов для страницы (DESC).
	rows, err := r.pool.Query(ctx, `
		SELECT
			order_uid, track_number, entry, locale, internal_signature,
			customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard
		FROM orders
		WHERE customer_id = $1
		ORDER BY date_created DESC, order_uid DESC
		LIMIT $2 OFFSET $3
	`, customerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("select customer orders: %w", err)
	}
	defer rows.Close()

	orders := make([]*domain.Order, 0, limit)
	byUID := make(map[string]*domain.Order, limit)
	uids := make([]string, 0, limit)

	for rows.Next() {
		order := &domain.Order{}
		if err := rows.Scan(
			&order.OrderUID, &order.TrackNumber, &order.Entry, &order.Locale, &order.InternalSignature,
			&order.CustomerID, &order.DeliveryService, &order.ShardKey, &order.SmID, &order.DateCreated, &order.OofShard,
		); err != nil {
			return nil, fmt.Errorf("scan order base: %w", err)
		}
		orders = append(orders, order)
		byUID[order.OrderUID] = order
		uids = append(uids, order.OrderUID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orders rows: %w", err)
	}
	if len(orders) == 0 {
		return orders, nil // пустая страница
	}

	// 2) Payments для всех UID страницы
	pRows, err := r.pool.Query(ctx, `
		SELECT
			order_uid, transaction, request_id, currency, provider, amount,
			payment_dt, bank, delivery_cost, goods_total, custom_fee
		FROM payments
		WHERE order_uid = ANY($1::text[])
	`, uids)
	if err != nil {
		return nil, fmt.Errorf("select payments: %w", err)
	}
	for pRows.Next() {
		var uid string
		var payment domain.Payment
		if err := pRows.Scan(
			&uid, &payment.Transaction, &payment.RequestID, &payment.Currency, &payment.Provider, &payment.Amount,
			&payment.PaymentDT, &payment.Bank, &payment.DeliveryCost, &payment.GoodsTotal, &payment.CustomFee,
		); err != nil {
			pRows.Close()
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		if order := byUID[uid]; order != nil {
			pCopy := payment
			order.Payment = pCopy
		}
	}
	if err := pRows.Err(); err != nil {
		pRows.Close()
		return nil, fmt.Errorf("payments rows: %w", err)
	}
	pRows.Close()

	// 3) Deliveries для всех UID страницы
	dRows, err := r.pool.Query(ctx, `
		SELECT
			order_uid, name, phone, zip, city, address, region, email
		FROM deliveries
		WHERE order_uid = ANY($1::text[])
	`, uids)
	if err != nil {
		return nil, fmt.Errorf("select deliveries: %w", err)
	}
	for dRows.Next() {
		var uid string
		var delivery domain.Delivery
		if err := dRows.Scan(
			&uid, &delivery.Name, &delivery.Phone, &delivery.Zip, &delivery.City, &delivery.Address, &delivery.Region, &delivery.Email,
		); err != nil {
			dRows.Close()
			return nil, fmt.Errorf("scan delivery: %w", err)
		}
		if order := byUID[uid]; order != nil {
			cDelivery := delivery
			order.Delivery = cDelivery
		}
	}
	if err := dRows.Err(); err != nil {
		dRows.Close()
		return nil, fmt.Errorf("deliveries rows: %w", err)
	}
	dRows.Close()

	// 4) Items для всех UID (сбор в map).
	itemsByUID := make(map[string][]domain.Item, len(orders))
	iRows, err := r.pool.Query(ctx, `
		SELECT
			order_uid, chrt_id, track_number, price, rid, name, sale, size,
			total_price, nm_id, brand, status
		FROM items
		WHERE order_uid = ANY($1::text[])
		ORDER BY order_uid, chrt_id
	`, uids)
	if err != nil {
		return nil, fmt.Errorf("select items: %w", err)
	}
	for iRows.Next() {
		var uid string
		var item domain.Item
		if err := iRows.Scan(
			&uid, &item.ChrtID, &item.TrackNumber, &item.Price, &item.RID, &item.Name, &item.Sale, &item.Size,
			&item.TotalPrice, &item.NmID, &item.Brand, &item.Status,
		); err != nil {
			iRows.Close()
			return nil, fmt.Errorf("scan item: %w", err)
		}
		itemsByUID[uid] = append(itemsByUID[uid], item)
	}
	if err := iRows.Err(); err != nil {
		iRows.Close()
		return nil, fmt.Errorf("items rows: %w", err)
	}
	iRows.Close()

	// Склейка: добавляем связанные части, порядок базового SELECT сохраняется.
	for _, order := range orders {
		if items := itemsByUID[order.OrderUID]; len(items) > 0 {
			order.Items = items
		}
	}
	return orders, nil
}

// LastN — последние N заказов (для прогрева кэша).
// Используем подход N+1: берём только UID, затем дочитываем полные заказы.
func (r *OrderRepository) LastN(ctx context.Context, n int) ([]*domain.Order, error) {
	if n <= 0 {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT order_uid
		FROM orders
		ORDER BY date_created DESC
		LIMIT $1
	`, n)
	if err != nil {
		return nil, fmt.Errorf("select last uids: %w", err)
	}
	defer rows.Close()

	var result []*domain.Order
	for rows.Next() {
		var orderUID string
		if err := rows.Scan(&orderUID); err != nil {
			return nil, fmt.Errorf("scan uid: %w", err)
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
		return nil, fmt.Errorf("last rows: %w", err)
	}

	return result, nil
}

// copyItems — вставка items через COPY (CopyFromRows); быстрее, чем INSERT в цикле.
func copyItems(ctx context.Context, tx pgx.Tx, orderUID string, items []domain.Item) error {
	rows := make([][]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, []any{orderUID, item.ChrtID, item.TrackNumber, item.Price, item.RID, item.Name,
			item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand, item.Status})
	}

	_, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"items"},
		[]string{
			"order_uid", "chrt_id", "track_number", "price", "rid", "name",
			"sale", "size", "total_price", "nm_id", "brand", "status",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("copy items: %w", err)
	}
	return nil
}
