// Package cache provides a small sharded in-memory TTL cache with a size bound.
package cache

import (
	"hash/maphash"
	"sync"
	"time"
)

const shardCount = 32

type entry[V any] struct {
	val     V
	expires int64
}

type shard[V any] struct {
	mu sync.RWMutex
	m  map[string]entry[V]
}

// TTL is safe for concurrent use. When a shard exceeds its capacity, expired
// entries are evicted first, then arbitrary ones, keeping memory bounded.
type TTL[V any] struct {
	shards   [shardCount]shard[V]
	seed     maphash.Seed
	perShard int
	now      func() time.Time
}

func NewTTL[V any](maxEntries int) *TTL[V] {
	if maxEntries < shardCount {
		maxEntries = shardCount
	}
	c := &TTL[V]{seed: maphash.MakeSeed(), perShard: maxEntries / shardCount, now: time.Now}
	for i := range c.shards {
		c.shards[i].m = make(map[string]entry[V])
	}
	return c
}

func (c *TTL[V]) shard(k string) *shard[V] {
	return &c.shards[maphash.String(c.seed, k)%shardCount]
}

func (c *TTL[V]) Get(k string) (V, bool) {
	s := c.shard(k)
	s.mu.RLock()
	e, ok := s.m[k]
	s.mu.RUnlock()
	if !ok || c.now().UnixNano() > e.expires {
		var zero V
		return zero, false
	}
	return e.val, true
}

func (c *TTL[V]) Set(k string, v V, ttl time.Duration) {
	s := c.shard(k)
	now := c.now().UnixNano()
	s.mu.Lock()
	if len(s.m) >= c.perShard {
		if _, exists := s.m[k]; !exists {
			c.evict(s, now)
		}
	}
	s.m[k] = entry[V]{val: v, expires: now + int64(ttl)}
	s.mu.Unlock()
}

func (c *TTL[V]) evict(s *shard[V], now int64) {
	removed := 0
	for k, e := range s.m {
		if now > e.expires {
			delete(s.m, k)
			removed++
		}
	}
	if removed > 0 {
		return
	}
	// Drop ~10% of entries; map iteration order is random.
	n := len(s.m)/10 + 1
	for k := range s.m {
		delete(s.m, k)
		if n--; n == 0 {
			break
		}
	}
}

func (c *TTL[V]) Delete(k string) {
	s := c.shard(k)
	s.mu.Lock()
	delete(s.m, k)
	s.mu.Unlock()
}

// DeleteFunc removes all entries for which fn returns true.
func (c *TTL[V]) DeleteFunc(fn func(k string, v V) bool) {
	for i := range c.shards {
		s := &c.shards[i]
		s.mu.Lock()
		for k, e := range s.m {
			if fn(k, e.val) {
				delete(s.m, k)
			}
		}
		s.mu.Unlock()
	}
}

func (c *TTL[V]) Len() int {
	n := 0
	for i := range c.shards {
		s := &c.shards[i]
		s.mu.RLock()
		n += len(s.m)
		s.mu.RUnlock()
	}
	return n
}
