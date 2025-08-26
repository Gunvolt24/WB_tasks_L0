package memory

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/internal/ports"
	"github.com/Gunvolt24/wb_l0/pkg/metrics"
)

// Проверка, что LRUCacheTTL удовлетворяет интерфейсу OrderCache.
var _ ports.OrderCache = (*LRUCacheTTL)(nil)

// ErrInvalidOrder — некорректные данные для сохранения в кэше.
var ErrInvalidOrder = errors.New("invalid order: nil or empty order_uid")

// entry — элемент LRU-списка с данными заказа и временем истечения TTL.
type entry struct {
	id        string
	order     *domain.Order
	expiresAt time.Time
}

// LRUCacheTTL — потокобезопасный LRU-кэш с TTL.
// Get перемещает элемент в начало и при наличии TTL продлевает срок жизни.
type LRUCacheTTL struct {
	capacity int           // максимальное количество элементов
	ttl      time.Duration // время истечения TTL

	ll    *list.List               // двухсвязный список для порядка LRU
	cache map[string]*list.Element // индекс по UID

	mu sync.Mutex // защита структур от параллельных доступов
}

// NewLRUCacheTTL — создаёт кэш с заданной ёмкостью и TTL.
// Если capacity <= 0, используется 1.
func NewLRUCacheTTL(capacity int, ttl time.Duration) *LRUCacheTTL {
	if capacity <= 0 {
		capacity = 1
	}
	return &LRUCacheTTL{
		capacity: capacity,
		ttl:      ttl,
		ll:       list.New(),
		cache:    make(map[string]*list.Element),
	}
}

// Get — вернуть заказ по id.
// (order, true) при попадании; (nil, false) при промахе или истёкшем TTL.
// Возвращает копию сущности.
func (c *LRUCacheTTL) Get(_ context.Context, id string) (*domain.Order, bool) {
	if id == "" {
		metrics.CacheOps.WithLabelValues("miss").Inc()
		return nil, false
	}

	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.cache[id]
	if !ok {
		metrics.CacheOps.WithLabelValues("miss").Inc()
		return nil, false
	}

	ent, ok := elem.Value.(*entry)
	if !ok {
		metrics.CacheOps.WithLabelValues("expired").Inc()
		c.removeElement(elem)
		metrics.CacheSize.Set(float64(c.ll.Len()))
		return nil, false
	}

	if c.isExpired(ent, now) {
		metrics.CacheOps.WithLabelValues("expired").Inc()
		c.removeElement(elem)
		metrics.CacheSize.Set(float64(c.ll.Len()))
		return nil, false
	}

	// Перемещаем элемент в начало LRU
	c.ll.MoveToFront(elem)

	// Обновляем TTL при попадании
	if c.ttl > 0 {
		ent.expiresAt = c.expiryFrom(now)
	}

	metrics.CacheOps.WithLabelValues("hit").Inc()
	return cloneOrder(ent.order), true
}

// Set — сохранить/обновить заказ.
// Возвращает ErrInvalidOrder при пустом UID или nil-значении.
func (c *LRUCacheTTL) Set(_ context.Context, order *domain.Order) error {
	if order == nil || order.OrderUID == "" {
		return ErrInvalidOrder
	}
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Обновление существующего элемента
	if elem, found := c.cache[order.OrderUID]; found {
		if ent, isEntry := elem.Value.(*entry); isEntry {
			ent.order = cloneOrder(order)
			ent.expiresAt = c.expiryFrom(now)
			c.ll.MoveToFront(elem)
			return nil
		}
		// Некорректное состояние - удалить и вставить заново
		c.removeElement(elem)
	}

	// Перед вставкой удаляем устаревшие элементы
	c.pruneExpiredFromBack(now)

	// Вставка нового элемента
	elem := c.ll.PushFront(&entry{
		id:        order.OrderUID,
		order:     cloneOrder(order),
		expiresAt: c.expiryFrom(now),
	})
	c.cache[order.OrderUID] = elem
	metrics.CacheSize.Set(float64(c.ll.Len()))

	// Вытеснение при превышении ёмкости
	if c.ll.Len() > c.capacity {
		c.evictLRU()
	}
	return nil
}

// WarmUp — массовая загрузка кэша (например, при запуске).
func (c *LRUCacheTTL) WarmUp(ctx context.Context, orders []*domain.Order) error {
	for _, order := range orders {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := c.Set(ctx, order); err != nil {
				return err
			}
		}
	}
	return nil
}


