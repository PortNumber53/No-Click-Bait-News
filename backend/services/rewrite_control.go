package services

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"sync"
	"time"
)

const (
	defaultRewriteStartsPerInterval = 10
	defaultRewriteInterval          = time.Minute
	defaultRewrite429Base           = time.Minute
	defaultRewrite429Max            = 15 * time.Minute
)

// RewriteStartReservation is an opaque admission slot that can be returned
// when a worker finds no eligible database job.
type RewriteStartReservation struct {
	id uint64
}

type rewriteStart struct {
	id uint64
	at time.Time
}

// RewriteControl coordinates credentials, provider cooldowns, and job admission
// across every article rewrite worker and configured model in this process.
type RewriteControl struct {
	mu sync.Mutex

	apiKeys     []string
	nextKey     int
	starts      []rewriteStart
	nextStartID uint64
	startLimit  int
	startWindow time.Duration
	backoffBase time.Duration
	backoffMax  time.Duration
	openUntil   time.Time
	consecutive int
	breakerOpen bool
	jitter      func(time.Duration) time.Duration
	now         func() time.Time
}

func newRewriteControl(apiKeys []string, startLimit int, startWindow, backoffBase, backoffMax time.Duration) *RewriteControl {
	if startLimit < 1 {
		startLimit = defaultRewriteStartsPerInterval
	}
	if startWindow <= 0 {
		startWindow = defaultRewriteInterval
	}
	if backoffBase <= 0 {
		backoffBase = defaultRewrite429Base
	}
	if backoffMax < backoffBase {
		backoffMax = defaultRewrite429Max
	}
	return &RewriteControl{
		apiKeys:     append([]string(nil), apiKeys...),
		startLimit:  startLimit,
		startWindow: startWindow,
		backoffBase: backoffBase,
		backoffMax:  backoffMax,
		now:         time.Now,
		jitter: func(max time.Duration) time.Duration {
			if max <= 0 {
				return 0
			}
			return time.Duration(rand.Int64N(int64(max) + 1))
		},
	}
}

func (c *RewriteControl) nextAPIKey() (string, int, error) {
	if c == nil {
		return "", 0, fmt.Errorf("rewrite control is not configured")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.apiKeys) == 0 {
		return "", 0, fmt.Errorf("batch API key pool is empty")
	}
	slot := c.nextKey % len(c.apiKeys)
	c.nextKey = (c.nextKey + 1) % len(c.apiKeys)
	return c.apiKeys[slot], slot + 1, nil
}

// WaitForJobStart blocks until both the provider circuit and rolling start
// limit allow another article job to be claimed.
func (c *RewriteControl) WaitForJobStart(ctx context.Context) (RewriteStartReservation, error) {
	if c == nil {
		return RewriteStartReservation{}, fmt.Errorf("rewrite control is not configured")
	}
	for {
		now := c.now()
		reservation, wait := c.tryReserveJobStart(now)
		if wait <= 0 {
			return reservation, nil
		}
		log.Printf("[articles.rewrite.admission] status=delayed wait_ms=%d", wait.Milliseconds())
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return RewriteStartReservation{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *RewriteControl) tryReserveJobStart(now time.Time) (RewriteStartReservation, time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closeExpiredBreaker(now)
	if c.breakerOpen {
		return RewriteStartReservation{}, c.openUntil.Sub(now)
	}

	cutoff := now.Add(-c.startWindow)
	firstCurrent := 0
	for firstCurrent < len(c.starts) && !c.starts[firstCurrent].at.After(cutoff) {
		firstCurrent++
	}
	if firstCurrent > 0 {
		c.starts = append([]rewriteStart(nil), c.starts[firstCurrent:]...)
	}
	if len(c.starts) >= c.startLimit {
		return RewriteStartReservation{}, c.starts[0].at.Add(c.startWindow).Sub(now)
	}

	c.nextStartID++
	reservation := RewriteStartReservation{id: c.nextStartID}
	c.starts = append(c.starts, rewriteStart{id: reservation.id, at: now})
	return reservation, 0
}

func (c *RewriteControl) cooldownUntil(now time.Time) (time.Time, bool) {
	if c == nil {
		return time.Time{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeExpiredBreaker(now)
	return c.openUntil, c.breakerOpen
}

func (c *RewriteControl) closeExpiredBreaker(now time.Time) {
	if c.breakerOpen && !now.Before(c.openUntil) {
		c.breakerOpen = false
		c.openUntil = time.Time{}
		log.Printf("[articles.rewrite.breaker] status=closed")
	}
}

// ReleaseJobStart returns a reservation when no database job was claimed.
func (c *RewriteControl) ReleaseJobStart(reservation RewriteStartReservation) {
	if c == nil || reservation.id == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.starts {
		if c.starts[i].id == reservation.id {
			c.starts = append(c.starts[:i], c.starts[i+1:]...)
			return
		}
	}
}

func (c *RewriteControl) recordRateLimit(retryAfter time.Duration) time.Time {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutive++
	delay := c.backoffBase
	for i := 1; i < c.consecutive && delay < c.backoffMax; i++ {
		if delay >= c.backoffMax/2 {
			delay = c.backoffMax
			break
		}
		delay *= 2
	}
	if retryAfter > delay {
		delay = retryAfter
	}
	if delay > c.backoffMax {
		delay = c.backoffMax
	}
	if remaining := c.backoffMax - delay; remaining > 0 {
		jitterMax := delay / 5
		if jitterMax > remaining {
			jitterMax = remaining
		}
		delay += c.jitter(jitterMax)
	}

	until := now.Add(delay)
	action := "opened"
	if c.breakerOpen && until.After(c.openUntil) {
		action = "extended"
	} else if c.breakerOpen {
		action = "maintained"
	}
	if action != "maintained" {
		c.openUntil = until
	}
	c.breakerOpen = true
	log.Printf("[articles.rewrite.breaker] status=%s consecutive_429=%d cooldown_ms=%d open_until=%s", action, c.consecutive, c.openUntil.Sub(now).Milliseconds(), c.openUntil.UTC().Format(time.RFC3339))
	return c.openUntil
}

func (c *RewriteControl) recordSuccess() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.breakerOpen {
		c.consecutive = 0
	}
}
