package api

import (
	"sync"
	"time"
)

// RateLimiter defines the interface for rate limiting implementations
type RateLimiter interface {
	// Wait blocks until it's safe to make another API call
	Wait()
	// CanProceed returns true if a request can be made without waiting
	CanProceed() bool
}

// SimpleRateLimiter implements basic rate limiting with minimum delay between calls
type SimpleRateLimiter struct {
	mu       sync.Mutex
	lastCall time.Time
	minDelay time.Duration
}

// NewSimpleRateLimiter creates a new simple rate limiter with minimum delay between calls
func NewSimpleRateLimiter(minDelay time.Duration) *SimpleRateLimiter {
	return &SimpleRateLimiter{
		minDelay: minDelay,
	}
}

// Wait blocks until it's safe to make another API call
func (rl *SimpleRateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	elapsed := time.Since(rl.lastCall)
	if elapsed < rl.minDelay {
		time.Sleep(rl.minDelay - elapsed)
	}
	rl.lastCall = time.Now()
}

// CanProceed returns true if a request can be made without waiting
func (rl *SimpleRateLimiter) CanProceed() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	elapsed := time.Since(rl.lastCall)
	return elapsed >= rl.minDelay
}

// TokenBucketRateLimiter implements token bucket algorithm for rate limiting
type TokenBucketRateLimiter struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate time.Duration
	lastRefill time.Time
}

// NewTokenBucketRateLimiter creates a new token bucket rate limiter
func NewTokenBucketRateLimiter(maxTokens int, refillRate time.Duration) *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available
func (rl *TokenBucketRateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on elapsed time
	rl.refillTokens()

	// If no tokens available, wait until one is refilled
	for rl.tokens <= 0 {
		rl.mu.Unlock()
		time.Sleep(rl.refillRate)
		rl.mu.Lock()
		rl.refillTokens()
	}

	// Consume a token
	rl.tokens--
}

// CanProceed returns true if a token is available
func (rl *TokenBucketRateLimiter) CanProceed() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refillTokens()
	return rl.tokens > 0
}

// refillTokens adds tokens based on elapsed time (internal method)
func (rl *TokenBucketRateLimiter) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed / rl.refillRate)

	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		rl.lastRefill = now
	}
}

// NoOpRateLimiter implements the RateLimiter interface but performs no rate limiting
type NoOpRateLimiter struct{}

// NewNoOpRateLimiter creates a rate limiter that performs no limiting
func NewNoOpRateLimiter() *NoOpRateLimiter {
	return &NoOpRateLimiter{}
}

// Wait does nothing (no rate limiting)
func (rl *NoOpRateLimiter) Wait() {
	// No operation
}

// CanProceed always returns true (no rate limiting)
func (rl *NoOpRateLimiter) CanProceed() bool {
	return true
}
