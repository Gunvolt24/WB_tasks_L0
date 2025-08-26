//go:build integration

package postgres_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	pgrepo "github.com/Gunvolt24/wb_l0/internal/repo/postgres"
	"github.com/Gunvolt24/wb_l0/internal/testutil"
)

// 1) Сохранение и получение заказа
func TestRepo_SaveAndGet_TC(t *testing.T) {
	t.Parallel()

	// длинный контекст — только на подъём контейнера
	ctxStart, cancelStart := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelStart()

	pg, stopPG, err := testutil.StartPostgresTC(ctxStart)
	require.NoError(t, err)
	defer func() { _ = stopPG(context.Background()) }()

	// миграции
	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	// короткий контекст — на сами БД-операции
	ctxTest, cancelTest := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelTest()

	pool, err := pgxpool.New(ctxTest, pg.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := pgrepo.NewOrderRepository(pool)

	ord := testutil.MakeOrder() // генерит валидный уникальный заказ
	require.NoError(t, repo.Save(ctxTest, &ord))

	got, err := repo.GetByUID(ctxTest, ord.OrderUID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, ord.OrderUID, got.OrderUID)
}

// 2) Повторный Save — апдейт базовых полей и полная замена items
func TestRepo_Save_UpsertAndItemsReplace_TC(t *testing.T) {
	t.Parallel()

	ctxStart, cancelStart := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelStart()

	pg, stopPG, err := testutil.StartPostgresTC(ctxStart)
	require.NoError(t, err)
	defer func() { _ = stopPG(context.Background()) }()
	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, pg.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := pgrepo.NewOrderRepository(pool)

	// 1-й Save: заказ с 2 товарами
	ord := testutil.MakeOrder(testutil.WithItems(2))
	ord.Entry = "WBIL"
	ord.Locale = "ru"
	require.NoError(t, repo.Save(ctx, &ord))

	// 2-й Save: меняем track_number, delivery.city, payment.amount и заменяем items на 1 шт
	ord.TrackNumber = "TR-UPDATED"
	ord.Delivery.City = "Gotham"
	ord.Payment.Amount = 999
	ord.Items = []domain.Item{{
		ChrtID:      777,
		TrackNumber: "TR-ITEM",
		Price:       777,
		RID:         "RID-777",
		Name:        "OnlyOne",
		Sale:        0,
		Size:        "XL",
		TotalPrice:  777,
		NmID:        777,
		Brand:       "onebrand",
		Status:      200,
	}}
	require.NoError(t, repo.Save(ctx, &ord))

	got, err := repo.GetByUID(ctx, ord.OrderUID)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.Equal(t, "TR-UPDATED", got.TrackNumber)
	require.Equal(t, "Gotham", got.Delivery.City)
	require.Equal(t, 999, got.Payment.Amount)

	require.Len(t, got.Items, 1)
	require.Equal(t, 777, got.Items[0].ChrtID)
	require.Equal(t, "OnlyOne", got.Items[0].Name)
}

// 3) GetByUID допускает отсутствие связанных deliveries/payments (возвращает заказ)
func TestRepo_GetByUID_AllowsMissingRelated_TC(t *testing.T) {
	t.Parallel()

	ctxStart, cancelStart := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelStart()

	pg, stopPG, err := testutil.StartPostgresTC(ctxStart)
	require.NoError(t, err)
	defer func() { _ = stopPG(context.Background()) }()
	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, pg.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := pgrepo.NewOrderRepository(pool)

	ord := testutil.MakeOrder()
	require.NoError(t, repo.Save(ctx, &ord))

	// Удаляем связанные записи напрямую — имитируем неполные данные
	_, err = pool.Exec(ctx, `DELETE FROM deliveries WHERE order_uid = $1`, ord.OrderUID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM payments WHERE order_uid = $1`, ord.OrderUID)
	require.NoError(t, err)

	got, err := repo.GetByUID(ctx, ord.OrderUID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, ord.OrderUID, got.OrderUID)

	// Delivery и Payment должны быть «пустыми», но без ошибки
	require.Empty(t, got.Delivery.Email)
	require.Empty(t, got.Payment.Currency)
	require.Zero(t, got.Payment.Amount)
}

// 4) ListByCustomer — пагинация и сортировка по date_created DESC, затем order_uid DESC
func TestRepo_ListByCustomer_PaginationAndOrder_TC(t *testing.T) {
	t.Parallel()

	ctxStart, cancelStart := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelStart()

	pg, stopPG, err := testutil.StartPostgresTC(ctxStart)
	require.NoError(t, err)
	defer func() { _ = stopPG(context.Background()) }()
	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, pg.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := pgrepo.NewOrderRepository(pool)

	const cust = "cust-list"
	base := time.Now().UTC().Add(-time.Hour)

	// Сохраняем 5 заказов одного клиента с контролируемыми датами + 1 другого клиента
	var uids []string
	for i := 0; i < 5; i++ {
		o := testutil.MakeOrder(testutil.WithCustomer(cust))
		o.DateCreated = base.Add(time.Duration(i) * time.Minute) // возрастающее время
		require.NoError(t, repo.Save(ctx, &o))
		uids = append(uids, o.OrderUID)
	}
	// другой клиент
	other := testutil.MakeOrder(testutil.WithCustomer("cust-other"))
	require.NoError(t, repo.Save(ctx, &other))

	// Ожидаемый порядок DESC по дате → у нас uids с возрастающими датами,
	// значит в ответе порядок должен быть обратным.
	expectedDesc := append([]string(nil), uids...)
	sort.Slice(expectedDesc, func(i, j int) bool { return expectedDesc[i] > expectedDesc[j] }) // при равных секундах сработает доп. сортировка по UID
	// но основное — проверим на страницах

	// Страница 1: limit=2 offset=0 → 2 последних заказа клиента
	page1, err := repo.ListByCustomer(ctx, cust, 2, 0)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	require.Equal(t, cust, page1[0].CustomerID)
	require.Equal(t, cust, page1[1].CustomerID)
	// Даты должны быть нерастущими
	require.True(t, !page1[0].DateCreated.Before(page1[1].DateCreated))

	// Страница 2: limit=2 offset=2 → ещё 2
	page2, err := repo.ListByCustomer(ctx, cust, 2, 2)
	require.NoError(t, err)
	require.Len(t, page2, 2)
	require.True(t, !page2[0].DateCreated.Before(page2[1].DateCreated))

	// Страница 3: limit=2 offset=4 → только 1 оставшийся
	page3, err := repo.ListByCustomer(ctx, cust, 2, 4)
	require.NoError(t, err)
	require.Len(t, page3, 1)
	require.Equal(t, cust, page3[0].CustomerID)

	// Убедимся, что ни на одной странице нет заказов другого клиента
	for _, pg := range [][]*domain.Order{page1, page2, page3} {
		for _, o := range pg {
			require.Equal(t, cust, o.CustomerID)
		}
	}
}

// 5) LastN — возвращает последние N заказов и подгружает полные сущности
func TestRepo_LastN_ReturnsLatestFull_TC(t *testing.T) {
	t.Parallel()

	ctxStart, cancelStart := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelStart()

	pg, stopPG, err := testutil.StartPostgresTC(ctxStart)
	require.NoError(t, err)
	defer func() { _ = stopPG(context.Background()) }()
	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, pg.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := pgrepo.NewOrderRepository(pool)

	base := time.Now().UTC().Add(-time.Hour)
	var saved []domain.Order
	for i := 0; i < 4; i++ {
		o := testutil.MakeOrder()
		o.DateCreated = base.Add(time.Duration(i) * time.Minute)
		require.NoError(t, repo.Save(ctx, &o))
		saved = append(saved, o)
	}

	latest3, err := repo.LastN(ctx, 3)
	require.NoError(t, err)
	require.Len(t, latest3, 3)

	// Сравним, что это действительно 3 самых поздних по date_created
	// saved[3] — самый поздний, затем [2], затем [1]
	expect := []string{saved[3].OrderUID, saved[2].OrderUID, saved[1].OrderUID}
	actual := []string{latest3[0].OrderUID, latest3[1].OrderUID, latest3[2].OrderUID}
	require.Equal(t, expect, actual)

	// И что подгружены связанные части (payment/delivery/items)
	for _, o := range latest3 {
		require.NotEmpty(t, o.Payment.Currency)
		require.NotEmpty(t, o.Delivery.Email)
		require.NotEmpty(t, o.Items) // MakeOrder создаёт хотя бы 1 item
	}
}

// 6) Save — ошибки валидации входа (nil / пустые обязательные поля)
func TestRepo_Save_ValidationErrors_TC(t *testing.T) {
	t.Parallel()

	ctxStart, cancelStart := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelStart()

	pg, stopPG, err := testutil.StartPostgresTC(ctxStart)
	require.NoError(t, err)
	defer func() { _ = stopPG(context.Background()) }()
	require.NoError(t, testutil.ApplyMigrationsGoose(pg.DSN))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, pg.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := pgrepo.NewOrderRepository(pool)

	// nil
	require.Error(t, repo.Save(ctx, nil))

	// пустой order_uid
	o1 := testutil.MakeOrder()
	o1.OrderUID = ""
	require.Error(t, repo.Save(ctx, &o1))

	// пустой customer_id
	o2 := testutil.MakeOrder()
	o2.CustomerID = ""
	require.Error(t, repo.Save(ctx, &o2))
}
