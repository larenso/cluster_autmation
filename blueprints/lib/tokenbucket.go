package lib

import (
	"sync"
	"time"
)

type TokenBucket struct {
	tokens     int
	capacity   int
	rateSec    int
	lastRefill time.Time
	mu         sync.Mutex
}

func NewTokenBucket(capacity, rateSec int) *TokenBucket {
	tb := &TokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		rateSec:    rateSec,
		lastRefill: time.Now(),
	}
	return tb
}

func (b *TokenBucket) GetToken() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	tokensToAdd := int(now.Sub(b.lastRefill).Seconds() * float64(b.rateSec))

	if tokensToAdd > 0 {
		b.lastRefill = now.Round(time.Second)
	}

	if b.tokens+tokensToAdd > 0 {
		b.tokens = min(tokensToAdd+b.tokens-1, b.capacity-1)
		return true
	}

	return false
}
