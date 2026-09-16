package services

import (
	"testing"
	"time"
)

func TestRewriteControlLimitsStartsInRollingWindowAndRefundsUnusedStart(t *testing.T) {
	control := newRewriteControl([]string{"batch-key"}, 2, time.Minute, time.Minute, 15*time.Minute)
	now := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)

	first, wait := control.tryReserveJobStart(now)
	if wait != 0 || first.id == 0 {
		t.Fatalf("first reservation = %#v, wait = %s", first, wait)
	}
	second, wait := control.tryReserveJobStart(now.Add(time.Second))
	if wait != 0 || second.id == 0 {
		t.Fatalf("second reservation = %#v, wait = %s", second, wait)
	}
	if _, wait := control.tryReserveJobStart(now.Add(2 * time.Second)); wait != 58*time.Second {
		t.Fatalf("third wait = %s, want 58s", wait)
	}

	control.ReleaseJobStart(second)
	if replacement, wait := control.tryReserveJobStart(now.Add(2 * time.Second)); wait != 0 || replacement.id == 0 {
		t.Fatalf("replacement reservation = %#v, wait = %s", replacement, wait)
	}
	if _, wait := control.tryReserveJobStart(now.Add(time.Minute)); wait != 0 {
		t.Fatalf("reservation at rolling boundary wait = %s, want 0", wait)
	}
}

func TestRewriteControlOpensGloballyAndEscalates429Cooldown(t *testing.T) {
	control := newRewriteControl([]string{"batch-key-a", "batch-key-b"}, 10, time.Minute, time.Minute, 15*time.Minute)
	now := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	control.now = func() time.Time { return now }
	control.jitter = func(time.Duration) time.Duration { return 0 }

	until := control.recordRateLimit(90 * time.Second)
	if want := now.Add(90 * time.Second); !until.Equal(want) {
		t.Fatalf("first open until = %s, want %s", until, want)
	}
	if _, wait := control.tryReserveJobStart(now); wait != 90*time.Second {
		t.Fatalf("global breaker wait = %s, want 90s", wait)
	}

	now = now.Add(10 * time.Second)
	until = control.recordRateLimit(0)
	if want := now.Add(2 * time.Minute); !until.Equal(want) {
		t.Fatalf("escalated open until = %s, want %s", until, want)
	}

	now = until
	if reservation, wait := control.tryReserveJobStart(now); wait != 0 || reservation.id == 0 {
		t.Fatalf("post-cooldown reservation = %#v, wait = %s", reservation, wait)
	}
	control.recordSuccess()
	if control.consecutive != 0 {
		t.Fatalf("consecutive 429 count = %d, want reset", control.consecutive)
	}
}

func TestRewriteControlCapsCooldownIncludingRetryAfterAndJitter(t *testing.T) {
	control := newRewriteControl([]string{"batch-key"}, 10, time.Minute, time.Minute, 15*time.Minute)
	now := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	control.now = func() time.Time { return now }
	control.jitter = func(max time.Duration) time.Duration { return max }

	until := control.recordRateLimit(time.Hour)
	if want := now.Add(15 * time.Minute); !until.Equal(want) {
		t.Fatalf("capped open until = %s, want %s", until, want)
	}
}
