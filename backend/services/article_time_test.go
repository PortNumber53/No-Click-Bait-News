package services

import (
	"testing"
	"time"
)

func TestClampPublishedAtUsesCurrentTimeForFutureDate(t *testing.T) {
	now := time.Date(2026, time.September, 12, 21, 0, 0, 0, time.UTC)
	future := now.Add(48 * time.Hour)

	if got := clampPublishedAt(future, now); !got.Equal(now) {
		t.Fatalf("clampPublishedAt() = %s, want %s", got, now)
	}
}

func TestClampPublishedAtPreservesPastDate(t *testing.T) {
	now := time.Date(2026, time.September, 12, 21, 0, 0, 0, time.UTC)
	past := now.Add(-2 * time.Hour)

	if got := clampPublishedAt(past, now); !got.Equal(past) {
		t.Fatalf("clampPublishedAt() = %s, want %s", got, past)
	}
}
