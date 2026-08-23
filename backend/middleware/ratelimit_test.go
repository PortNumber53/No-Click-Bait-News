package middleware

import (
	"testing"
	"time"
)

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	start := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	if !limiter.allow("client", start) || !limiter.allow("client", start.Add(time.Second)) {
		t.Fatal("requests within the configured limit should be allowed")
	}
	if limiter.allow("client", start.Add(2*time.Second)) {
		t.Fatal("request above the configured limit should be rejected")
	}
	if !limiter.allow("client", start.Add(time.Minute)) {
		t.Fatal("request should be allowed after the window resets")
	}
}
