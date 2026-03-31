package main

import (
	"sync"
	"time"
)

type bucket struct {
	mu        sync.Mutex
	capacity   float64
	refillRate float64 // tokens per second
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

type RateLimiter struct {
	buckets map[string]*bucket
	mu sync.Mutex
	capacity float64
	refillRate float64
	bucketTTL time.Duration
}

func NewRateLimiter(capacity, refillRate float64, ttl time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		capacity: capacity,
		refillRate: refillRate,
		bucketTTL: ttl,
	}
	// Background Cleanup
	go rl.CleanupLoop()

	return rl
}

func (rl *RateLimiter) Allow(key string) bool {
	// get or create bucket
	rl.mu.Lock()
	b, exists := rl.buckets[key]
	if !exists {
		b = &bucket{
			tokens: rl.capacity,
			capacity: rl.capacity,
			refillRate: rl.refillRate,
			lastRefill: time.Now(),
			lastSeen: time.Now(),
		}
		rl.buckets[key] = b
	}
	rl.mu.Unlock()

	now := time.Now()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.lastRefill = now

	// Check Allowance
	if b.tokens >= 1 {
		b.tokens -= 1
		b.lastSeen = now
		return true
	}

	b.lastSeen = now
	return false
}

func (rl *RateLimiter) CleanupLoop() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		rl.mu.Lock()
		for k, b := range rl.buckets {
			b.mu.Lock()
			if now.Sub(b.lastSeen) > rl.bucketTTL {
				delete(rl.buckets, k)
			}
			b.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}