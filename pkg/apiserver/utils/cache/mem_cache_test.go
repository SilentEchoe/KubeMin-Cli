package cache

import (
    "testing"
    "time"
)

func TestMemCache_BasicStoreLoad(t *testing.T) {
    c := NewMemCache(false)
    // shrink TTL for fast tests
    mc := c.(*MemCache)
    mc.ttl = time.Second

    if err := c.Store("k", "v"); err != nil {
        t.Fatalf("store error: %v", err)
    }
    if !c.Exists("k") {
        t.Fatalf("expected key to exist")
    }
    got, _ := c.Load("k")
    if got != "v" {
        t.Fatalf("expected v, got %q", got)
    }
}

func TestMemCache_Expiration(t *testing.T) {
    c := NewMemCache(false)
    mc := c.(*MemCache)
    mc.ttl = 50 * time.Millisecond

    if err := c.Store("k", "v"); err != nil {
        t.Fatalf("store error: %v", err)
    }
    // Before expiry
    if got, _ := c.Load("k"); got != "v" {
        t.Fatalf("expected v before expiry, got %q", got)
    }
    time.Sleep(80 * time.Millisecond)
    // After expiry Load should return empty and prune key
    if got, _ := c.Load("k"); got != "" {
        t.Fatalf("expected empty after expiry, got %q", got)
    }
    if c.Exists("k") {
        t.Fatalf("expected key to be expired and removed")
    }
}

