package memory

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/pkg/metrics"
)

type entry struct {
	id        string
	order     *domain.Order
	expiresAt time.Time
}

type LRUCacheTTL struct {
	capacity int
	ttl      time.Duration

	ll    *list.List
	index map[string]*list.Element

	mu sync.Mutex
}

func NewLRUCacheTTL(capacity int, ttl time.Duration) *LRUCacheTTL {
	if capacity <= 0 {
		capacity = 1
	}
	return &LRUCacheTTL{
		capacity: capacity,
		ttl:      ttl,
		ll:       list.New(),
		index:    make(map[string]*list.Element),
	}
}

func (c *LRUCacheTTL) Get(_ context.Context, id string) (*domain.Order, bool) {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.index[id]
	if !ok {
		metrics.CacheOps.WithLabelValues("miss").Inc()
		return nil, false
	}
	ent := elem.Value.(*entry)
	if c.isExpired(ent, now) {
		metrics.CacheOps.WithLabelValues("expired").Inc()
		c.removeElement(elem)
		metrics.CacheSize.Set(float64(len(c.index)))
		return nil, false
	}
	c.ll.MoveToFront(elem)

	if c.ttl > 0 {
		ent.expiresAt = c.expiryFrom(now)
	}

	metrics.CacheOps.WithLabelValues("hit").Inc()
	return cloneOrder(ent.order), true
}

func (c *LRUCacheTTL) Set(_ context.Context, order *domain.Order) error {
	if order == nil || order.OrderUID == "" {
		return nil
	}
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.index[order.OrderUID]; ok {
		ent := elem.Value.(*entry)
		ent.order = cloneOrder(order)
		ent.expiresAt = c.expiryFrom(now)
		c.ll.MoveToFront(elem)
		return nil
	}

	c.pruneExpiredFromBack(now)

	elem := c.ll.PushFront(&entry{
		id:        order.OrderUID,
		order:     cloneOrder(order),
		expiresAt: c.expiryFrom(now),
	})
	c.index[order.OrderUID] = elem
	metrics.CacheSize.Set(float64(len(c.index)))

	if c.ll.Len() > c.capacity {
		c.evictLRU()
	}
	return nil
}

func (c *LRUCacheTTL) WarmUp(ctx context.Context, orders []*domain.Order) error {
	for _, order := range orders {
		if err := c.Set(ctx, order); err != nil {
			return err
		}
	}
	return nil
}

// ------вспомогательные функции------

func (c *LRUCacheTTL) evictLRU() {
	if back := c.ll.Back(); back != nil {
		c.removeElement(back)
		metrics.CacheOps.WithLabelValues("evicted").Inc()
		metrics.CacheSize.Set(float64(len(c.index)))
	}
}

func (c *LRUCacheTTL) removeElement(elem *list.Element) {
	ent := elem.Value.(*entry)
	delete(c.index, ent.id)
	c.ll.Remove(elem)
}

func (c *LRUCacheTTL) isExpired(ent *entry, now time.Time) bool {
	if c.ttl <= 0 {
		return false
	}
	return now.After(ent.expiresAt)
}

func (c *LRUCacheTTL) expiryFrom(now time.Time) time.Time {
	if c.ttl <= 0 {
		return time.Time{}
	}
	return now.Add(c.ttl)
}

func (c *LRUCacheTTL) pruneExpiredFromBack(now time.Time) {
	if c.ttl <= 0 {
		return
	}
	for {
		back := c.ll.Back()
		if back == nil {
			return
		}
		ent := back.Value.(*entry)
		if now.After(ent.expiresAt) {
			c.removeElement(back)
			metrics.CacheOps.WithLabelValues("expired").Inc()
			metrics.CacheSize.Set(float64(len(c.index)))
			continue
		}
		return
	}
}

func cloneOrder(order *domain.Order) *domain.Order {
	if order == nil {
		return nil
	}
	clonedOrder := *order
	if order.Items != nil {
		clonedOrder.Items = append([]domain.Item(nil), order.Items...)
	}
	return &clonedOrder
}
