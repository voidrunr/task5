package ratelimit

import (
	"testing"
	"time"
)

func TestAllowWithinLimit(t *testing.T) {
	l := New(3)
	for i := 0; i < 3; i++ {
		ok, _ := l.Allow()
		if !ok {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	ok, retry := l.Allow()
	if ok {
		t.Error("4th request should be rejected")
	}
	if retry <= 0 || retry > time.Minute {
		t.Errorf("retryAfter = %v, want within a minute", retry)
	}
}

func TestRefillOverTime(t *testing.T) {
	l := New(60)
	now := time.Now()
	l.now = func() time.Time { return now }

	for i := 0; i < 60; i++ {
		if ok, _ := l.Allow(); !ok {
			t.Fatalf("request %d should pass", i+1)
		}
	}
	if ok, _ := l.Allow(); ok {
		t.Error("bucket should be empty")
	}

	now = now.Add(2 * time.Second)
	if ok, _ := l.Allow(); !ok {
		t.Error("request should pass after refill")
	}
	if ok, _ := l.Allow(); !ok {
		t.Error("second request should pass after refill")
	}
	if ok, _ := l.Allow(); ok {
		t.Error("third request should be rejected")
	}
}

func TestZeroLimitClamped(t *testing.T) {
	l := New(0)
	ok, _ := l.Allow()
	if !ok {
		t.Error("clamped limiter should allow at least one request")
	}
	if ok, _ := l.Allow(); ok {
		t.Error("second request should be rejected")
	}
	if l.Limit() != 1 {
		t.Errorf("Limit() = %d, want 1", l.Limit())
	}
}
