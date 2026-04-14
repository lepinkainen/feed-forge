package api

import (
	"context"
	"sync"
	"time"
)

// RateLimiter defines the interface for rate limiting implementations.
type RateLimiter interface {
	// Wait blocks until it's safe to make another API call.
	Wait()
	// WaitContext blocks until it's safe to make another API call or the context is cancelled.
	WaitContext(ctx context.Context) error
	// CanProceed returns true if a request can be made without waiting.
	CanProceed() bool
}

// SimpleRateLimiter implements basic rate limiting with minimum delay between calls.
type SimpleRateLimiter struct {
	mu       sync.Mutex
	lastCall time.Time
	minDelay time.Duration
}

// NewSimpleRateLimiter creates a new simple rate limiter with minimum delay between calls.
func NewSimpleRateLimiter(minDelay time.Duration) *SimpleRateLimiter {
	return &SimpleRateLimiter{minDelay: minDelay}
}

// Wait blocks until it's safe to make another API call.
func (rl *SimpleRateLimiter) Wait() {
	_ = rl.WaitContext(context.Background())
}

// WaitContext blocks until it's safe to make another API call or the context is cancelled.
func (rl *SimpleRateLimiter) WaitContext(ctx context.Context) error {
	rl.mu.Lock()
	waitFor := rl.minDelay - time.Since(rl.lastCall)
	if waitFor < 0 {
		waitFor = 0
	}
	nextCall := time.Now().Add(waitFor)
	rl.lastCall = nextCall
	rl.mu.Unlock()

	return waitWithContext(ctx, waitFor)
}

// CanProceed returns true if a request can be made without waiting.
func (rl *SimpleRateLimiter) CanProceed() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	elapsed := time.Since(rl.lastCall)
	return elapsed >= rl.minDelay
}

// TokenBucketRateLimiter implements token bucket algorithm for rate limiting.
type TokenBucketRateLimiter struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate time.Duration
	lastRefill time.Time
}

// NewTokenBucketRateLimiter creates a new token bucket rate limiter.
func NewTokenBucketRateLimiter(maxTokens int, refillRate time.Duration) *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available.
func (rl *TokenBucketRateLimiter) Wait() {
	_ = rl.WaitContext(context.Background())
}

// WaitContext blocks until a token is available or the context is cancelled.
func (rl *TokenBucketRateLimiter) WaitContext(ctx context.Context) error {
	for {
		rl.mu.Lock()
		rl.refillTokens()
		if rl.tokens > 0 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}
		waitFor := rl.refillRate - time.Since(rl.lastRefill)
		if waitFor <= 0 {
			waitFor = rl.refillRate
		}
		rl.mu.Unlock()

		if err := waitWithContext(ctx, waitFor); err != nil {
			return err
		}
	}
}

// CanProceed returns true if a token is available.
func (rl *TokenBucketRateLimiter) CanProceed() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refillTokens()
	return rl.tokens > 0
}

// refillTokens adds tokens based on elapsed time (internal method).
func (rl *TokenBucketRateLimiter) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed / rl.refillRate)

	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		rl.lastRefill = rl.lastRefill.Add(time.Duration(tokensToAdd) * rl.refillRate)
	}
}

// NoOpRateLimiter implements the RateLimiter interface but performs no rate limiting.
type NoOpRateLimiter struct{}

// NewNoOpRateLimiter creates a rate limiter that performs no limiting.
func NewNoOpRateLimiter() *NoOpRateLimiter {
	return &NoOpRateLimiter{}
}

// Wait does nothing (no rate limiting).
func (rl *NoOpRateLimiter) Wait() {
	_ = rl.WaitContext(context.Background())
}

// WaitContext does nothing (no rate limiting).
func (rl *NoOpRateLimiter) WaitContext(context.Context) error {
	return nil
}

// CanProceed always returns true (no rate limiting).
func (rl *NoOpRateLimiter) CanProceed() bool {
	return true
}
