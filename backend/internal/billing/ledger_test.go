package billing

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
)

type fakeDB struct {
	mu       sync.Mutex
	balances map[string]int64
	logs     []store.UsageRecord
	keys     map[string]store.KeyDebit
	fail     bool
}

func (f *fakeDB) GetBalance(_ context.Context, id string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.balances[id], nil
}

func (f *fakeDB) FlushUsage(_ context.Context, recs []store.UsageRecord, debits map[string]int64, keys map[string]store.KeyDebit) (map[string]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return nil, errors.New("db down")
	}
	f.logs = append(f.logs, recs...)
	out := map[string]int64{}
	for id, d := range debits {
		f.balances[id] -= d
		out[id] = f.balances[id]
	}
	for id, d := range keys {
		k := f.keys[id]
		k.Amount += d.Amount
		k.Requests += d.Requests
		f.keys[id] = k
	}
	return out, nil
}

func newTestLedger(db *fakeDB) *Ledger {
	return NewLedger(db, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Hour)
}

func TestLedgerChargesImmediatelyAndFlushes(t *testing.T) {
	db := &fakeDB{balances: map[string]int64{"u1": 1000}, keys: map[string]store.KeyDebit{}}
	l := newTestLedger(db)
	ctx := context.Background()

	if v, _ := l.Available(ctx, "u1"); v != 1000 {
		t.Fatalf("initial available = %d", v)
	}
	l.Record(store.UsageRecord{UserID: "u1", APIKeyID: "k1", Cost: 300})
	l.Record(store.UsageRecord{UserID: "u1", APIKeyID: "k1", Cost: 200})
	if v, _ := l.Available(ctx, "u1"); v != 500 {
		t.Fatalf("available after charges = %d, want 500", v)
	}
	if v := l.PendingKeySpend("u1", "k1"); v != 500 {
		t.Fatalf("pending key spend = %d", v)
	}
	if err := l.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if db.balances["u1"] != 500 || len(db.logs) != 2 || db.keys["k1"].Requests != 2 {
		t.Fatalf("db state wrong: %+v %d %+v", db.balances, len(db.logs), db.keys)
	}
	if v, _ := l.Available(ctx, "u1"); v != 500 {
		t.Fatalf("available after flush = %d", v)
	}
	if v := l.PendingKeySpend("u1", "k1"); v != 0 {
		t.Fatalf("pending key spend after flush = %d", v)
	}
}

func TestLedgerRetriesFailedFlush(t *testing.T) {
	db := &fakeDB{balances: map[string]int64{"u1": 1000}, keys: map[string]store.KeyDebit{}, fail: true}
	l := newTestLedger(db)
	ctx := context.Background()
	_, _ = l.Available(ctx, "u1")
	l.Record(store.UsageRecord{UserID: "u1", Cost: 100})
	if err := l.Flush(ctx); err == nil {
		t.Fatal("expected failure")
	}
	if l.QueueLen() != 1 {
		t.Fatalf("record should be re-queued, queue=%d", l.QueueLen())
	}
	if v, _ := l.Available(ctx, "u1"); v != 900 {
		t.Fatalf("pending charge must still count: %d", v)
	}
	db.fail = false
	if err := l.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if db.balances["u1"] != 900 || l.QueueLen() != 0 {
		t.Fatalf("not persisted after retry: %v", db.balances)
	}
}

func TestLedgerStopFlushes(t *testing.T) {
	db := &fakeDB{balances: map[string]int64{"u1": 50}, keys: map[string]store.KeyDebit{}}
	l := newTestLedger(db)
	go l.Run()
	l.Record(store.UsageRecord{UserID: "u1", Cost: 5})
	l.Stop()
	if len(db.logs) != 1 {
		t.Fatalf("stop must flush pending usage, got %d logs", len(db.logs))
	}
}

func TestLedgerConcurrent(t *testing.T) {
	db := &fakeDB{balances: map[string]int64{"u1": 1_000_000}, keys: map[string]store.KeyDebit{}}
	l := newTestLedger(db)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				l.Record(store.UsageRecord{UserID: "u1", Cost: 1})
				if j%25 == 0 {
					_ = l.Flush(context.Background())
				}
			}
		}()
	}
	wg.Wait()
	_ = l.Flush(context.Background())
	if db.balances["u1"] != 1_000_000-5000 {
		t.Fatalf("balance = %d", db.balances["u1"])
	}
}
