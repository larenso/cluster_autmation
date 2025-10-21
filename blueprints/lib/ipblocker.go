package lib

import (
	"sync"
	"time"
)

type blockRecord struct {
	counter int
	lastAcc time.Time
}

type IPBlocker struct {
	limit    int
	resetDur time.Duration

	ac map[string]*blockRecord
	mu sync.RWMutex
}

func NewIPBlocker(limit int, resetDur time.Duration) *IPBlocker {
	tb := &IPBlocker{
		limit:    limit,
		resetDur: resetDur,
		ac:       make(map[string]*blockRecord, 3),
	}
	return tb
}

func (b *IPBlocker) NotifyFailure(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	x, ok := b.ac[ip]
	if !ok {
		b.ac[ip] = &blockRecord{counter: 1, lastAcc: time.Now()}
		return
	}

	if time.Since(x.lastAcc) > b.resetDur {
		x.counter = 1
	} else {
		x.counter++
	}
	x.lastAcc = time.Now()
}

func (b *IPBlocker) CheckBlocked(ip string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	x, ok := b.ac[ip]
	return ok && x.counter > b.limit
}
func (b *IPBlocker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	clear(b.ac)
}
