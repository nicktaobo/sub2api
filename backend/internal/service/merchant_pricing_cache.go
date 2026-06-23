// MERCHANT-SYSTEM v1.0
// 简单的 TTL 缓存（替代外部 LRU 依赖；目标规模小，足够）。

package service

import (
	"sync"
	"time"
)

type ttlCacheEntry[V any] struct {
	val       V
	expiresAt time.Time
}

// ttlCache 是按 key 失效的并发安全 TTL 缓存。
// TTL 为兜底过期；admin 改价时主动调 Remove(key) 立即失效（RFC §5.2.1 Step 2.1）。
type ttlCache[K comparable, V any] struct {
	mu      sync.RWMutex
	data    map[K]ttlCacheEntry[V]
	ttl     time.Duration
	maxSize int
}

func newTTLCache[K comparable, V any](maxSize int, ttl time.Duration) *ttlCache[K, V] {
	if maxSize <= 0 {
		maxSize = 1024
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &ttlCache[K, V]{
		data:    make(map[K]ttlCacheEntry[V], maxSize),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

func (c *ttlCache[K, V]) Get(k K) (V, bool) {
	c.mu.RLock()
	e, ok := c.data[k]
	c.mu.RUnlock()
	var zero V
	if !ok {
		return zero, false
	}
	if time.Now().After(e.expiresAt) {
		c.mu.Lock()
		delete(c.data, k)
		c.mu.Unlock()
		return zero, false
	}
	return e.val, true
}

func (c *ttlCache[K, V]) Set(k K, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.data) >= c.maxSize {
		// 简单 evict：删一个过期 entry，找不到就删一个任意 entry
		for kk, ee := range c.data {
			if time.Now().After(ee.expiresAt) {
				delete(c.data, kk)
				break
			}
		}
		if len(c.data) >= c.maxSize {
			for kk := range c.data {
				delete(c.data, kk)
				break
			}
		}
	}
	c.data[k] = ttlCacheEntry[V]{val: v, expiresAt: time.Now().Add(c.ttl)}
}

func (c *ttlCache[K, V]) Remove(k K) {
	c.mu.Lock()
	delete(c.data, k)
	c.mu.Unlock()
}

func (c *ttlCache[K, V]) Clear() {
	c.mu.Lock()
	c.data = make(map[K]ttlCacheEntry[V], c.maxSize)
	c.mu.Unlock()
}
