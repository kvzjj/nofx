package api

import (
	"testing"
	"time"
)

func TestRateLimiterAllowsWithinLimit(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4") {
			t.Fatalf("request %d within limit should be allowed", i+1)
		}
	}
}

func TestRateLimiterBlocksOverLimit(t *testing.T) {
	rl := newRateLimiter(2, time.Minute)
	rl.allow("ip")
	rl.allow("ip")
	if rl.allow("ip") {
		t.Fatal("request over limit should be blocked")
	}
	if rl.retryAfter("ip") <= 0 {
		t.Fatal("retryAfter should be positive while blocked")
	}
}

func TestRateLimiterKeysAreIndependent(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	if !rl.allow("a") {
		t.Fatal("first key should pass")
	}
	if rl.allow("a") {
		t.Fatal("first key should now be blocked")
	}
	if !rl.allow("b") {
		t.Fatal("different key must not be affected")
	}
}

func TestRateLimiterWindowSlides(t *testing.T) {
	rl := newRateLimiter(1, 30*time.Millisecond)
	if !rl.allow("k") {
		t.Fatal("first hit should pass")
	}
	if rl.allow("k") {
		t.Fatal("second hit within window should fail")
	}
	time.Sleep(40 * time.Millisecond)
	if !rl.allow("k") {
		t.Fatal("after window expiry the key should be allowed again")
	}
}
