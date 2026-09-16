package handlers

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestNormalizeNewsURLRejectsUnsafeTargets(t *testing.T) {
	tests := []string{
		"file:///etc/passwd",
		"http://127.0.0.1/admin",
		"http://[::1]/admin",
		"http://169.254.169.254/latest/meta-data",
		"https://user:password@203.0.113.1/story",
		"https://" + strings.Repeat("a", 2050),
	}
	for _, raw := range tests {
		if _, _, err := normalizeNewsURL(context.Background(), raw); err == nil {
			t.Errorf("normalizeNewsURL(%q) unexpectedly succeeded", raw)
		}
	}
}

func TestRateLimitedRewriteRetryDoesNotConsumeAttempt(t *testing.T) {
	now := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	attempts, availableAt := rateLimitedRewriteRetry(articleRewriteJob{Attempts: 3}, now.Add(2*time.Minute), now)
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if !availableAt.Equal(now.Add(2 * time.Minute)) {
		t.Fatalf("availableAt = %s, want %s", availableAt, now.Add(2*time.Minute))
	}

	attempts, availableAt = rateLimitedRewriteRetry(articleRewriteJob{Attempts: 0}, now.Add(-time.Minute), now)
	if attempts != 0 || !availableAt.Equal(now) {
		t.Fatalf("clamped retry = attempts %d at %s", attempts, availableAt)
	}
}

func TestIsPublicIPRejectsReservedRanges(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.1.1", "198.18.0.1", "203.0.113.1", "::1", "2001:db8::1", "ff02::1"} {
		if isPublicIP(net.ParseIP(raw)) {
			t.Errorf("reserved address %q unexpectedly accepted", raw)
		}
	}
	if !isPublicIP(net.ParseIP("8.8.8.8")) {
		t.Error("public address was unexpectedly rejected")
	}
}
