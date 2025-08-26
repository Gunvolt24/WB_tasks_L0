package memory

import (
	"container/list"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/domain"
	"github.com/Gunvolt24/wb_l0/pkg/metrics"
)

// evictLRU — удаляет наименее используемый элемент.
func (c *LRUCacheTTL) evictLRU() {
	if back := c.ll.Back(); back != nil {
		c.removeElement(back)
		metrics.CacheOps.WithLabelValues("evicted").Inc()
		metrics.CacheSize.Set(float64(c.ll.Len()))
	}
}

// removeElement — удаляет элемент из списка и индекса.
func (c *LRUCacheTTL) removeElement(elem *list.Element) {
	if elem == nil {
		return
	}
	if ent, ok := elem.Value.(*entry); ok {
		delete(c.cache, ent.id)
	}
	c.ll.Remove(elem)
}

// isExpired — проверяет истечение TTL.
func (c *LRUCacheTTL) isExpired(ent *entry, now time.Time) bool {
	if c.ttl <= 0 {
		return false
	}
	return now.After(ent.expiresAt)
}

// expiryFrom — вычисляет момент истечения для текущего времени.
func (c *LRUCacheTTL) expiryFrom(now time.Time) time.Time {
	if c.ttl <= 0 {
		return time.Time{}
	}
	return now.Add(c.ttl)
}

// pruneExpiredFromBack — удаляет элементы с истекшим TTL из хвоста до первого актуального.
func (c *LRUCacheTTL) pruneExpiredFromBack(now time.Time) {
	if c.ttl <= 0 {
		return
	}
	for {
		back := c.ll.Back()
		if back == nil {
			return
		}
		ent, ok := back.Value.(*entry)
		if !ok {
			c.removeElement(back)
			metrics.CacheSize.Set(float64(c.ll.Len()))
			continue
		}
		if now.After(ent.expiresAt) {
			c.removeElement(back)
			metrics.CacheOps.WithLabelValues("expired").Inc()
			metrics.CacheSize.Set(float64(c.ll.Len()))
			continue
		}
		return
	}
}

// cloneOrder — возвращает копию заказа, чтобы внешние изменения
// не отражались на данных внутри кэша.
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
