package memory

import (
	"context"
	"testing"
	"time"

	"github.com/Gunvolt24/wb_l0/internal/domain"
)

func newOrder(id string) *domain.Order {
	return &domain.Order{
		OrderUID: id,
		Items:    []domain.Item{{Name: "x"}},
	}
}

func mustSet(t *testing.T, c *LRUCacheTTL, order *domain.Order) {
	t.Helper()
	if err := c.Set(context.Background(), order); err != nil {
		t.Fatalf("Set(%q) error: %v", order.OrderUID, err)
	}
}

func TestGetSet_HitMiss(t *testing.T) {
	c := NewLRUCacheTTL(2, 5*time.Minute)
	ctx := context.Background()

	// miss
	if _, ok := c.Get(ctx, "id-1"); ok {
		t.Fatalf("expected miss before Set")
	}

	// hit после Set
	mustSet(t, c, newOrder("id-1"))
	got, ok := c.Get(ctx, "id-1")
	if !ok || got.OrderUID != "id-1" {
		t.Fatalf("expected hit for id-1")
	}
}

func TestTTL_Expiry(t *testing.T) {
	c := NewLRUCacheTTL(2, 100*time.Millisecond)
	ctx := context.Background()

	mustSet(t, c, newOrder("ttl"))
	if _, ok := c.Get(ctx, "ttl"); !ok {
		t.Fatalf("expected hit right after Set")
	}
	time.Sleep(150 * time.Millisecond)
	if _, ok := c.Get(ctx, "ttl"); ok {
		t.Fatalf("expected miss after TTL expires")
	}
}

func TestLRUEviction(t *testing.T) {
	c := NewLRUCacheTTL(2, 0)
	ctx := context.Background()

	mustSet(t, c, newOrder("A"))
	mustSet(t, c, newOrder("B"))
	// Сделать свежим значение "A"
	if _, ok := c.Get(ctx, "A"); !ok {
		t.Fatalf("expected hit for A")
	}
	// Добавляем "C" — вытеснит "B" (самый старый)
	mustSet(t, c, newOrder("C"))

	if _, ok := c.Get(ctx, "B"); ok {
		t.Fatalf("expected B to be evicted")
	}
	if _, ok := c.Get(ctx, "A"); !ok || c.ll.Len() != 2 {
		t.Fatalf("expected A & C to stay in cache")
	}
}

func TestCloneImmutability(t *testing.T) {
	const changedName = "changed"

	c := NewLRUCacheTTL(1, 0)
	ctx := context.Background()
	orig := newOrder("Z")
	mustSet(t, c, orig)

	// меняем то, что вернул Get — не должно влиять на кэш
	o1, _ := c.Get(ctx, "Z")
	o1.Items[0].Name = changedName

	o2, _ := c.Get(ctx, "Z")
	if o2.Items[0].Name == changedName {
		t.Fatalf("cache should return clones, not pointers to internal value")
	}
}
