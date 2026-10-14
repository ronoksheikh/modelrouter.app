// Package bus broadcasts cache invalidations to every replica using Postgres
// LISTEN/NOTIFY, so revoking a key or changing a model takes effect cluster-wide
// within milliseconds.
package bus

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const channel = "modelrouter_events"

const (
	KindKey     = "key"     // value: key hash
	KindUser    = "user"    // value: user id
	KindCatalog = "catalog" // value: ignored
)

type Bus struct {
	pool     *pgxpool.Pool
	log      *slog.Logger
	mu       sync.RWMutex
	handlers map[string][]func(string)
}

func New(pool *pgxpool.Pool, log *slog.Logger) *Bus {
	return &Bus{pool: pool, log: log, handlers: map[string][]func(string){}}
}

func (b *Bus) On(kind string, fn func(string)) {
	b.mu.Lock()
	b.handlers[kind] = append(b.handlers[kind], fn)
	b.mu.Unlock()
}

func (b *Bus) dispatch(kind, val string) {
	b.mu.RLock()
	hs := b.handlers[kind]
	b.mu.RUnlock()
	for _, h := range hs {
		h(val)
	}
}

// Publish applies the event locally and broadcasts it to other replicas.
func (b *Bus) Publish(ctx context.Context, kind, val string) {
	b.dispatch(kind, val)
	if b.pool == nil {
		return
	}
	if _, err := b.pool.Exec(ctx, `SELECT pg_notify($1, $2)`, channel, kind+":"+val); err != nil {
		b.log.Warn("bus publish failed", "err", err)
	}
}

// Listen blocks, reconnecting on failure, until ctx is done.
func (b *Bus) Listen(ctx context.Context) {
	for ctx.Err() == nil {
		if err := b.listenOnce(ctx); err != nil && ctx.Err() == nil {
			b.log.Warn("bus listener disconnected; reconnecting", "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (b *Bus) listenOnce(ctx context.Context) error {
	conn, err := b.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
		return err
	}
	// Anything may have changed while we were disconnected.
	b.dispatch(KindCatalog, "")
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		kind, val, _ := strings.Cut(n.Payload, ":")
		b.dispatch(kind, val)
	}
}
