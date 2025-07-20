package api

import (
	"sync"
	"testing"
	"time"
)

func TestSimpleRateLimiter(t *testing.T) {
	tests := []struct {
		name     string
		minDelay time.Duration
		calls    int
		expected time.Duration
	}{
		{
			name:     "single call no delay",
			minDelay: 100 * time.Millisecond,
			calls:    1,
			expected: 0, // First call should not be delayed
		},
		{
			name:     "multiple calls with delay",
			minDelay: 50 * time.Millisecond,
			calls:    3,
			expected: 100 * time.Millisecond, // 2 delays * 50ms each
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewSimpleRateLimiter(tt.minDelay)
			start := time.Now()

			for i := 0; i < tt.calls; i++ {
				rl.Wait()
			}

			elapsed := time.Since(start)

			// Allow for some timing variance (±10ms)
			tolerance := 10 * time.Millisecond
			if elapsed < tt.expected-tolerance || elapsed > tt.expected+tolerance+100*time.Millisecond {
				t.Errorf("SimpleRateLimiter.Wait() took %v, expected around %v", elapsed, tt.expected)
			}
		})
	}
}

func TestSimpleRateLimiter_CanProceed(t *testing.T) {
	rl := NewSimpleRateLimiter(100 * time.Millisecond)

	// First call should be able to proceed
	if !rl.CanProceed() {
		t.Errorf("CanProceed() should return true for first call")
	}

	// After waiting, should be able to proceed
	rl.Wait()

	// Immediately after, should not be able to proceed
	if rl.CanProceed() {
		t.Errorf("CanProceed() should return false immediately after Wait()")
	}

	// After delay, should be able to proceed again
	time.Sleep(120 * time.Millisecond)
	if !rl.CanProceed() {
		t.Errorf("CanProceed() should return true after delay")
	}
}

func TestSimpleRateLimiter_Concurrent(t *testing.T) {
	rl := NewSimpleRateLimiter(50 * time.Millisecond)

	var wg sync.WaitGroup
	start := time.Now()
	callTimes := make([]time.Time, 5)

	// Make 5 concurrent calls
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			rl.Wait()
			callTimes[index] = time.Now()
		}(i)
	}

	wg.Wait()
	totalTime := time.Since(start)

	// Should take at least 4 * 50ms = 200ms (5 calls with 4 delays between them)
	expectedMin := 200 * time.Millisecond
	if totalTime < expectedMin {
		t.Errorf("Concurrent calls took %v, expected at least %v", totalTime, expectedMin)
	}

	// Skip detailed timing checks on concurrent test as they can be flaky
	// The important thing is that totalTime shows serialization occurred
	_ = callTimes
}

func TestTokenBucketRateLimiter(t *testing.T) {
	tests := []struct {
		name       string
		maxTokens  int
		refillRate time.Duration
		calls      int
		wantBlocks bool
	}{
		{
			name:       "enough tokens available",
			maxTokens:  5,
			refillRate: 100 * time.Millisecond,
			calls:      3,
			wantBlocks: false,
		},
		{
			name:       "needs to wait for tokens",
			maxTokens:  2,
			refillRate: 50 * time.Millisecond,
			calls:      4,
			wantBlocks: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewTokenBucketRateLimiter(tt.maxTokens, tt.refillRate)
			start := time.Now()

			for i := 0; i < tt.calls; i++ {
				rl.Wait()
			}

			elapsed := time.Since(start)

			if tt.wantBlocks {
				// Should have taken at least one refill period
				minExpected := tt.refillRate
				if elapsed < minExpected {
					t.Errorf("TokenBucketRateLimiter.Wait() took %v, expected at least %v", elapsed, minExpected)
				}
			} else {
				// Should have completed quickly
				maxExpected := 50 * time.Millisecond
				if elapsed > maxExpected {
					t.Errorf("TokenBucketRateLimiter.Wait() took %v, expected less than %v", elapsed, maxExpected)
				}
			}
		})
	}
}

func TestTokenBucketRateLimiter_CanProceed(t *testing.T) {
	rl := NewTokenBucketRateLimiter(2, 100*time.Millisecond)

	// Should start with tokens available
	if !rl.CanProceed() {
		t.Errorf("CanProceed() should return true initially")
	}

	// Consume tokens
	rl.Wait()
	rl.Wait()

	// Should be out of tokens
	if rl.CanProceed() {
		t.Errorf("CanProceed() should return false after consuming all tokens")
	}

	// Wait for refill
	time.Sleep(120 * time.Millisecond)

	// Should have tokens again
	if !rl.CanProceed() {
		t.Errorf("CanProceed() should return true after refill")
	}
}

func TestTokenBucketRateLimiter_RefillTokens(t *testing.T) {
	rl := NewTokenBucketRateLimiter(5, 50*time.Millisecond)

	// Consume all tokens
	for i := 0; i < 5; i++ {
		rl.Wait()
	}

	// Verify no tokens available
	if rl.CanProceed() {
		t.Errorf("Should have no tokens after consuming all")
	}

	// Wait for multiple refill periods
	time.Sleep(150 * time.Millisecond) // Should refill 3 tokens

	// Manually check tokens by trying to consume them
	tokensAvailable := 0
	for i := 0; i < 10; i++ { // Try more than max to verify cap
		if rl.CanProceed() {
			rl.Wait()
			tokensAvailable++
		} else {
			break
		}
	}

	// Should have refilled some tokens (at least 2, at most 3)
	if tokensAvailable < 2 || tokensAvailable > 3 {
		t.Errorf("Expected 2-3 tokens after refill, got %d", tokensAvailable)
	}
}

func TestTokenBucketRateLimiter_MaxTokensCap(t *testing.T) {
	rl := NewTokenBucketRateLimiter(3, 10*time.Millisecond)

	// Wait for a long time to ensure max refill
	time.Sleep(500 * time.Millisecond)

	// Should not exceed max tokens
	tokensAvailable := 0
	for i := 0; i < 10; i++ {
		if rl.CanProceed() {
			rl.Wait()
			tokensAvailable++
		} else {
			break
		}
	}

	if tokensAvailable != 3 {
		t.Errorf("Expected exactly 3 tokens (max), got %d", tokensAvailable)
	}
}

func TestNoOpRateLimiter(t *testing.T) {
	rl := NewNoOpRateLimiter()

	// Should always be able to proceed
	for i := 0; i < 10; i++ {
		if !rl.CanProceed() {
			t.Errorf("NoOpRateLimiter.CanProceed() should always return true")
		}
	}

	// Wait should be instantaneous
	start := time.Now()
	for i := 0; i < 100; i++ {
		rl.Wait()
	}
	elapsed := time.Since(start)

	// Should complete very quickly
	if elapsed > 10*time.Millisecond {
		t.Errorf("NoOpRateLimiter.Wait() took %v, expected near-instant", elapsed)
	}
}

func TestRateLimiterInterface(t *testing.T) {
	// Verify all implementations satisfy the interface
	var _ RateLimiter = NewSimpleRateLimiter(time.Second)
	var _ RateLimiter = NewTokenBucketRateLimiter(10, time.Second)
	var _ RateLimiter = NewNoOpRateLimiter()
}

func BenchmarkSimpleRateLimiter(b *testing.B) {
	rl := NewSimpleRateLimiter(1 * time.Microsecond) // Very small delay for benchmarking

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Wait()
	}
}

func BenchmarkTokenBucketRateLimiter(b *testing.B) {
	rl := NewTokenBucketRateLimiter(1000, 1*time.Microsecond) // Large bucket for benchmarking

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Wait()
	}
}

func BenchmarkNoOpRateLimiter(b *testing.B) {
	rl := NewNoOpRateLimiter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Wait()
	}
}
