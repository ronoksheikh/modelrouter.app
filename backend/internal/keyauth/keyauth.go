// Package keyauth resolves API keys to their owners with an in-memory cache.
package keyauth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/cache"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/secret"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
)

var ErrInvalidKey = errors.New("invalid API key")

type Store interface {
	GetKeyAuth(ctx context.Context, hash string) (*store.KeyAuth, error)
}

type Cache struct {
	st     Store
	pos    *cache.TTL[*store.KeyAuth]
	neg    *cache.TTL[struct{}]
	ttl    time.Duration
	negTTL time.Duration
}

func New(st Store) *Cache {
	return &Cache{
		st:     st,
		pos:    cache.NewTTL[*store.KeyAuth](200_000),
		neg:    cache.NewTTL[struct{}](100_000),
		ttl:    60 * time.Second,
		negTTL: 10 * time.Second,
	}
}

// FromHeaders extracts a key from `Authorization: Bearer` or `x-api-key`.
func FromHeaders(authorization, xAPIKey string) string {
	if xAPIKey != "" {
		return strings.TrimSpace(xAPIKey)
	}
	if len(authorization) > 7 && strings.EqualFold(authorization[:7], "bearer ") {
		return strings.TrimSpace(authorization[7:])
	}
	return ""
}

func (c *Cache) Lookup(ctx context.Context, rawKey string) (*store.KeyAuth, error) {
	if !strings.HasPrefix(rawKey, secret.KeyPrefix) || len(rawKey) < len(secret.KeyPrefix)+20 || len(rawKey) > 200 {
		return nil, ErrInvalidKey
	}
	hash := secret.HashKey(rawKey)
	if a, ok := c.pos.Get(hash); ok {
		return a, nil
	}
	if _, ok := c.neg.Get(hash); ok {
		return nil, ErrInvalidKey
	}
	a, err := c.st.GetKeyAuth(ctx, hash)
	if errors.Is(err, store.ErrNotFound) {
		c.neg.Set(hash, struct{}{}, c.negTTL)
		return nil, ErrInvalidKey
	}
	if err != nil {
		return nil, err
	}
	ttl := c.ttl
	if a.SpendLimit != nil {
		// Spend-limited keys re-read their persisted spend often so the limit stays tight.
		ttl = 5 * time.Second
	}
	c.pos.Set(hash, a, ttl)
	return a, nil
}

func (c *Cache) InvalidateHash(hash string) {
	c.pos.Delete(hash)
	c.neg.Delete(hash)
}

func (c *Cache) InvalidateUser(userID string) {
	c.pos.DeleteFunc(func(_ string, a *store.KeyAuth) bool { return a.UserID == userID })
}
