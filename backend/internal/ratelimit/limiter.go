// Package ratelimit implements a sharded, memory-bounded token bucket limiter.
package ratelimit

import (
	"hash/maphash"
	"math"
	"sync"
	"time"
)

const shards = 64

type bucket struct {
	tokens float64
	last   int64 // unix nanos
}

type shard struct {
	mu sync.Mutex
	m  map[string]*bucket
}

type Limiter struct {
	seed   maphash.Seed
	shards [shards]shard
	now    func() time.Time
}

func New() *Limiter {
	l := &Limiter{seed: maphash.MakeSeed(), now: time.Now}
	for i := range l.shards {
		l.shards[i].m = make(map[string]*bucket)
	}
	return l
}

// Allow consumes one token from the bucket for key, where the bucket holds
// `perMinute` tokens and refills continuously. It returns whether the request is
// allowed and, if not, how long until the next token is available.
func (l *Limiter) Allow(key string, perMinute int) (bool, time.Duration) {
	if perMinute <= 0 {
		return true, 0
	}
	capacity := float64(perMinute)
	rate := capacity / float64(time.Minute) // tokens per nanosecond
	now := l.now().UnixNano()
	s := &l.shards[maphash.String(l.seed, key)%shards]
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.m[key]
	if !ok {
		b = &bucket{tokens: capacity, last: now}
		s.m[key] = b
	}
	b.tokens = math.Min(capacity, b.tokens+float64(now-b.last)*rate)
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / rate)
	return false, wait
}

// Sweep removes buckets idle for longer than idle. Call periodically.
func (l *Limiter) Sweep(idle time.Duration) {
	cutoff := l.now().Add(-idle).UnixNano()
	for i := range l.shards {
		s := &l.shards[i]
		s.mu.Lock()
		for k, b := range s.m {
			if b.last < cutoff {
				delete(s.m, k)
			}
		}
		s.mu.Unlock()
	}
}
