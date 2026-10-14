package billing

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
)

// Backend is the persistence the ledger needs. *store.Store satisfies it.
type Backend interface {
	GetBalance(ctx context.Context, userID string) (int64, error)
	FlushUsage(ctx context.Context, recs []store.UsageRecord, userDebits map[string]int64, keyDebits map[string]store.KeyDebit) (map[string]int64, error)
}

type account struct {
	balance  int64 // last balance confirmed by the database
	pending  int64 // debits recorded locally but not yet flushed
	keySpent map[string]int64
	loadedAt time.Time
}

// Ledger tracks user balances in memory and persists usage in batches.
//
// Every request is charged immediately against the in-memory balance (so
// concurrent requests see each other), while the database is updated in a
// single transaction per flush interval. This keeps the hot path free of DB
// writes and lets one instance handle very high request rates.
type Ledger struct {
	db       Backend
	log      *slog.Logger
	ttl      time.Duration
	interval time.Duration
	maxBatch int

	mu       sync.Mutex
	accounts map[string]*account
	queue    []store.UsageRecord
	notify   chan struct{}
	done     chan struct{}
	stopped  chan struct{}
}

func NewLedger(db Backend, log *slog.Logger, flushInterval time.Duration) *Ledger {
	if flushInterval <= 0 {
		flushInterval = time.Second
	}
	return &Ledger{
		db: db, log: log, ttl: 30 * time.Second, interval: flushInterval, maxBatch: 5000,
		accounts: make(map[string]*account),
		notify:   make(chan struct{}, 1),
		done:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

// Available returns the spendable balance (DB balance minus unflushed debits).
func (l *Ledger) Available(ctx context.Context, userID string) (int64, error) {
	l.mu.Lock()
	a, ok := l.accounts[userID]
	fresh := ok && time.Since(a.loadedAt) < l.ttl
	if fresh {
		v := a.balance - a.pending
		l.mu.Unlock()
		return v, nil
	}
	l.mu.Unlock()

	bal, err := l.db.GetBalance(ctx, userID)
	if err != nil {
		return 0, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok = l.accounts[userID]
	if !ok {
		a = &account{keySpent: map[string]int64{}}
		l.accounts[userID] = a
	}
	a.balance = bal
	a.loadedAt = time.Now()
	return a.balance - a.pending, nil
}

// PendingKeySpend returns unflushed spend for an API key (for spend limits).
func (l *Ledger) PendingKeySpend(userID, keyID string) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if a, ok := l.accounts[userID]; ok {
		return a.keySpent[keyID]
	}
	return 0
}

// Invalidate forces the next Available call to reload from the database
// (e.g. after a top-up or an admin adjustment).
func (l *Ledger) Invalidate(userID string) {
	l.mu.Lock()
	if a, ok := l.accounts[userID]; ok {
		a.loadedAt = time.Time{}
	}
	l.mu.Unlock()
}

// Record charges a request and queues its usage log.
func (l *Ledger) Record(rec store.UsageRecord) {
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	l.mu.Lock()
	a, ok := l.accounts[rec.UserID]
	if !ok {
		a = &account{keySpent: map[string]int64{}}
		l.accounts[rec.UserID] = a
	}
	a.pending += rec.Cost
	if rec.APIKeyID != "" {
		a.keySpent[rec.APIKeyID] += rec.Cost
	}
	l.queue = append(l.queue, rec)
	n := len(l.queue)
	l.mu.Unlock()
	if n >= l.maxBatch {
		select {
		case l.notify <- struct{}{}:
		default:
		}
	}
}

// Run flushes periodically until Stop is called.
func (l *Ledger) Run() {
	defer close(l.stopped)
	t := time.NewTicker(l.interval)
	defer t.Stop()
	evict := time.NewTicker(5 * time.Minute)
	defer evict.Stop()
	for {
		select {
		case <-l.done:
			// Final flush with retries so no usage is lost on shutdown.
			for i := 0; i < 5; i++ {
				if l.Flush(context.Background()) == nil {
					return
				}
				time.Sleep(time.Duration(i+1) * 200 * time.Millisecond)
			}
			return
		case <-t.C:
			_ = l.Flush(context.Background())
		case <-l.notify:
			_ = l.Flush(context.Background())
		case <-evict.C:
			l.evictIdle()
		}
	}
}

func (l *Ledger) Stop() {
	close(l.done)
	<-l.stopped
}

// Flush writes queued usage to the database. On failure the batch is put back
// at the front of the queue and retried on the next tick.
func (l *Ledger) Flush(ctx context.Context) error {
	l.mu.Lock()
	if len(l.queue) == 0 {
		l.mu.Unlock()
		return nil
	}
	batch := l.queue
	if len(batch) > l.maxBatch {
		batch = batch[:l.maxBatch]
		l.queue = append([]store.UsageRecord(nil), l.queue[l.maxBatch:]...)
	} else {
		l.queue = nil
	}
	l.mu.Unlock()

	userDebits := map[string]int64{}
	keyDebits := map[string]store.KeyDebit{}
	for _, r := range batch {
		userDebits[r.UserID] += r.Cost
		if r.APIKeyID != "" {
			d := keyDebits[r.APIKeyID]
			d.Amount += r.Cost
			d.Requests++
			keyDebits[r.APIKeyID] = d
		}
	}

	fctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	balances, err := l.db.FlushUsage(fctx, batch, userDebits, keyDebits)
	if err != nil {
		l.log.Error("usage flush failed; will retry", "records", len(batch), "err", err)
		l.mu.Lock()
		l.queue = append(batch, l.queue...)
		l.mu.Unlock()
		return err
	}

	l.mu.Lock()
	for _, r := range batch {
		if a, ok := l.accounts[r.UserID]; ok && r.APIKeyID != "" {
			a.keySpent[r.APIKeyID] -= r.Cost
			if a.keySpent[r.APIKeyID] <= 0 {
				delete(a.keySpent, r.APIKeyID)
			}
		}
	}
	for uid, debit := range userDebits {
		a, ok := l.accounts[uid]
		if !ok {
			continue
		}
		a.pending -= debit
		if bal, ok := balances[uid]; ok {
			a.balance = bal
			a.loadedAt = time.Now()
		}
	}
	l.mu.Unlock()
	return nil
}

func (l *Ledger) evictIdle() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for id, a := range l.accounts {
		if a.pending == 0 && len(a.keySpent) == 0 && time.Since(a.loadedAt) > 10*time.Minute {
			delete(l.accounts, id)
		}
	}
}

// QueueLen reports the number of unflushed usage records (for health checks).
func (l *Ledger) QueueLen() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.queue)
}
