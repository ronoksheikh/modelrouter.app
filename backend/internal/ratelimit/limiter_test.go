package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	l := New()
	now := time.Now()
	l.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("k", 3); !ok {
			t.Fatalf("request %d should pass", i)
		}
	}
	ok, wait := l.Allow("k", 3)
	if ok || wait <= 0 || wait > 20*time.Second {
		t.Fatalf("4th request should be limited with ~20s wait: %v %v", ok, wait)
	}
	now = now.Add(20 * time.Second) // refills 1 token
	if ok, _ := l.Allow("k", 3); !ok {
		t.Fatal("should refill")
	}
	if ok, _ := l.Allow("other", 3); !ok {
		t.Fatal("keys are independent")
	}
	if ok, _ := l.Allow("unlimited", 0); !ok {
		t.Fatal("0 = unlimited")
	}
	now = now.Add(time.Hour)
	l.Sweep(time.Minute)
	for i := range l.shards {
		if len(l.shards[i].m) != 0 {
			t.Fatal("sweep should remove idle buckets")
		}
	}
}
