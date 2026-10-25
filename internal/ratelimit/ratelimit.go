package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	rate     float64
	last     time.Time
	now      func() time.Time
}

func New(limit int) *Limiter {
	if limit <= 0 {
		limit = 1
	}
	return &Limiter{
		tokens:   float64(limit),
		capacity: float64(limit),
		rate:     float64(limit) / 60.0,
		last:     time.Now(),
		now:      time.Now,
	}
}

func (l *Limiter) Allow() (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	elapsed := now.Sub(l.last).Seconds()
	if elapsed > 0 {
		l.tokens += elapsed * l.rate
		if l.tokens > l.capacity {
			l.tokens = l.capacity
		}
		l.last = now
	}
	if l.tokens >= 1 {
		l.tokens--
		return true, 0
	}
	need := (1 - l.tokens) / l.rate
	return false, time.Duration(need * float64(time.Second))
}

func (l *Limiter) Limit() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return int(l.capacity)
}
