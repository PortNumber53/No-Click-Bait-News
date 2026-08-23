package services

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestClaimCrawlerSlotHonorsLimitConcurrently(t *testing.T) {
	const limit = int64(8)
	var claimed atomic.Int64
	var successful atomic.Int64
	var workers sync.WaitGroup
	for range 100 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if claimCrawlerSlot(&claimed, limit) {
				successful.Add(1)
			}
		}()
	}
	workers.Wait()
	if got := successful.Load(); got != limit {
		t.Fatalf("successful claims = %d, want %d", got, limit)
	}
}
