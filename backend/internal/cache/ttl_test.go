package cache

import (
	"fmt"
	"testing"
	"time"
)

func TestTTL(t *testing.T) {
	c := NewTTL[int](1000)
	now := time.Now()
	c.now = func() time.Time { return now }
	c.Set("a", 1, time.Second)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatal("get")
	}
	now = now.Add(2 * time.Second)
	if _, ok := c.Get("a"); ok {
		t.Fatal("should expire")
	}
	c.Set("b", 2, time.Minute)
	c.Delete("b")
	if _, ok := c.Get("b"); ok {
		t.Fatal("delete")
	}
	c.Set("x1", 1, time.Minute)
	c.Set("x2", 2, time.Minute)
	c.DeleteFunc(func(_ string, v int) bool { return v == 2 })
	if _, ok := c.Get("x2"); ok {
		t.Fatal("DeleteFunc")
	}
}

func TestTTLBounded(t *testing.T) {
	c := NewTTL[int](320)
	for i := 0; i < 10_000; i++ {
		c.Set(fmt.Sprint(i), i, time.Hour)
	}
	if c.Len() > 320 {
		t.Fatalf("cache grew unbounded: %d", c.Len())
	}
}
