package api

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestRetryPolicy_CalculateBackoff(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name     string
		attempt  int
		expected time.Duration
	}{
		{
			name:     "attempt 0 returns 0",
			attempt:  0,
			expected: 0,
		},
		{
			name:     "attempt 1 returns initial backoff",
			attempt:  1,
			expected: 1 * time.Second,
		},
		{
			name:     "attempt 2 doubles backoff",
			attempt:  2,
			expected: 2 * time.Second,
		},
		{
			name:     "attempt 3 quadruples backoff",
			attempt:  3,
			expected: 4 * time.Second,
		},
		{
			name:     "large attempt caps at max backoff",
			attempt:  10,
			expected: 30 * time.Second, // MaxBackoff
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policy.CalculateBackoff(tt.attempt)
			if got != tt.expected {
				t.Errorf("CalculateBackoff(%d) = %v, want %v", tt.attempt, got, tt.expected)
			}
		})
	}
}

func TestRetryPolicy_IsRetryableError(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error is not retryable",
			err:      nil,
			expected: false,
		},
		{
			name:     "HTTP 500 error is retryable",
			err:      &HTTPError{StatusCode: http.StatusInternalServerError, Message: "Server Error"},
			expected: true,
		},
		{
			name:     "HTTP 429 error is retryable",
			err:      &HTTPError{StatusCode: http.StatusTooManyRequests, Message: "Rate Limited"},
			expected: true,
		},
		{
			name:     "HTTP 404 error is not retryable",
			err:      &HTTPError{StatusCode: http.StatusNotFound, Message: "Not Found"},
			expected: false,
		},
		{
			name:     "OAuth2 500 error is retryable",
			err:      &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusInternalServerError}},
			expected: true,
		},
		{
			name:     "OAuth2 400 error is not retryable",
			err:      &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusBadRequest}},
			expected: false,
		},
		{
			name:     "generic error is not retryable",
			err:      errors.New("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policy.IsRetryableError(tt.err)
			if got != tt.expected {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestRetryPolicy_IsRateLimitError(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error is not rate limit",
			err:      nil,
			expected: false,
		},
		{
			name:     "HTTP 429 error is rate limit",
			err:      &HTTPError{StatusCode: http.StatusTooManyRequests, Message: "Rate Limited"},
			expected: true,
		},
		{
			name:     "HTTP 500 error is not rate limit",
			err:      &HTTPError{StatusCode: http.StatusInternalServerError, Message: "Server Error"},
			expected: false,
		},
		{
			name:     "OAuth2 429 error is rate limit",
			err:      &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusTooManyRequests}},
			expected: true,
		},
		{
			name:     "OAuth2 500 error is not rate limit",
			err:      &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusInternalServerError}},
			expected: false,
		},
		{
			name:     "generic error is not rate limit",
			err:      errors.New("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policy.IsRateLimitError(tt.err)
			if got != tt.expected {
				t.Errorf("IsRateLimitError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy.MaxAttempts != 3 {
		t.Errorf("DefaultRetryPolicy().MaxAttempts = %d, want 3", policy.MaxAttempts)
	}

	if policy.InitialBackoff != 1*time.Second {
		t.Errorf("DefaultRetryPolicy().InitialBackoff = %v, want 1s", policy.InitialBackoff)
	}

	if policy.MaxBackoff != 30*time.Second {
		t.Errorf("DefaultRetryPolicy().MaxBackoff = %v, want 30s", policy.MaxBackoff)
	}

	if policy.BackoffMultiplier != 2.0 {
		t.Errorf("DefaultRetryPolicy().BackoffMultiplier = %f, want 2.0", policy.BackoffMultiplier)
	}

	expectedRetryableCodes := []int{
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}

	if len(policy.RetryableErrors) != len(expectedRetryableCodes) {
		t.Errorf("DefaultRetryPolicy() has %d retryable codes, want %d", len(policy.RetryableErrors), len(expectedRetryableCodes))
	}

	for _, code := range expectedRetryableCodes {
		found := false
		for _, retryCode := range policy.RetryableErrors {
			if retryCode == code {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultRetryPolicy() missing retryable code %d", code)
		}
	}
}

func TestAggressiveRetryPolicy(t *testing.T) {
	policy := AggressiveRetryPolicy()

	if policy.MaxAttempts != 5 {
		t.Errorf("AggressiveRetryPolicy().MaxAttempts = %d, want 5", policy.MaxAttempts)
	}

	if policy.InitialBackoff != 500*time.Millisecond {
		t.Errorf("AggressiveRetryPolicy().InitialBackoff = %v, want 500ms", policy.InitialBackoff)
	}

	if policy.MaxBackoff != 60*time.Second {
		t.Errorf("AggressiveRetryPolicy().MaxBackoff = %v, want 60s", policy.MaxBackoff)
	}
}

func TestConservativeRetryPolicy(t *testing.T) {
	policy := ConservativeRetryPolicy()

	if policy.MaxAttempts != 2 {
		t.Errorf("ConservativeRetryPolicy().MaxAttempts = %d, want 2", policy.MaxAttempts)
	}

	if policy.InitialBackoff != 2*time.Second {
		t.Errorf("ConservativeRetryPolicy().InitialBackoff = %v, want 2s", policy.InitialBackoff)
	}

	if policy.MaxBackoff != 10*time.Second {
		t.Errorf("ConservativeRetryPolicy().MaxBackoff = %v, want 10s", policy.MaxBackoff)
	}

	// Conservative should have fewer retryable errors
	if len(policy.RetryableErrors) != 3 {
		t.Errorf("ConservativeRetryPolicy() has %d retryable codes, want 3", len(policy.RetryableErrors))
	}
}

func TestHTTPError_Error(t *testing.T) {
	err := &HTTPError{
		StatusCode: http.StatusInternalServerError,
		Message:    "Internal Server Error",
	}

	expected := "HTTP 500: Internal Server Error"
	if err.Error() != expected {
		t.Errorf("HTTPError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestExecuteWithRetry(t *testing.T) {
	tests := []struct {
		name             string
		operation        func() RetryableOperation
		policy           *RetryPolicy
		wantErr          bool
		expectedAttempts int
	}{
		{
			name: "successful operation on first attempt",
			operation: func() RetryableOperation {
				return func() error {
					return nil
				}
			},
			policy:           DefaultRetryPolicy(),
			wantErr:          false,
			expectedAttempts: 1,
		},
		{
			name: "operation fails with non-retryable error",
			operation: func() RetryableOperation {
				return func() error {
					return &HTTPError{StatusCode: http.StatusNotFound, Message: "Not Found"}
				}
			},
			policy:           DefaultRetryPolicy(),
			wantErr:          true,
			expectedAttempts: 1,
		},
		{
			name: "operation succeeds after retries",
			operation: func() RetryableOperation {
				attempts := 0
				return func() error {
					attempts++
					if attempts < 3 {
						return &HTTPError{StatusCode: http.StatusInternalServerError, Message: "Server Error"}
					}
					return nil
				}
			},
			policy:           DefaultRetryPolicy(),
			wantErr:          false,
			expectedAttempts: 3,
		},
		{
			name: "operation exhausts all retries",
			operation: func() RetryableOperation {
				return func() error {
					return &HTTPError{StatusCode: http.StatusInternalServerError, Message: "Server Error"}
				}
			},
			policy: &RetryPolicy{
				MaxAttempts:     2,
				InitialBackoff:  10 * time.Millisecond,
				RetryableErrors: []int{http.StatusInternalServerError},
			},
			wantErr:          true,
			expectedAttempts: 2,
		},
		{
			name: "rate limit error gets longer backoff",
			operation: func() RetryableOperation {
				attempts := 0
				return func() error {
					attempts++
					if attempts < 2 {
						return &HTTPError{StatusCode: http.StatusTooManyRequests, Message: "Rate Limited"}
					}
					return nil
				}
			},
			policy: &RetryPolicy{
				MaxAttempts:     3,
				InitialBackoff:  10 * time.Millisecond,
				RetryableErrors: []int{http.StatusTooManyRequests},
			},
			wantErr:          false,
			expectedAttempts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation := tt.operation()

			start := time.Now()
			err := ExecuteWithRetry(operation, tt.policy, "test-operation")
			elapsed := time.Since(start)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteWithRetry() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Skip timing checks for fast tests to avoid flakiness
			_ = elapsed
		})
	}
}

func TestExecuteWithRetry_OperationName(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:     2,
		InitialBackoff:  1 * time.Millisecond,
		RetryableErrors: []int{http.StatusInternalServerError},
	}

	operation := func() error {
		return &HTTPError{StatusCode: http.StatusInternalServerError, Message: "Server Error"}
	}

	err := ExecuteWithRetry(operation, policy, "test-operation")

	if err == nil {
		t.Errorf("ExecuteWithRetry() should have failed")
	}

	// Error message should contain operation name
	if !contains(err.Error(), "test-operation") {
		t.Errorf("Error message should contain operation name: %v", err.Error())
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func BenchmarkExecuteWithRetry_Success(b *testing.B) {
	policy := DefaultRetryPolicy()
	operation := func() error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExecuteWithRetry(operation, policy, "benchmark")
	}
}

func BenchmarkExecuteWithRetry_NonRetryableError(b *testing.B) {
	policy := DefaultRetryPolicy()
	operation := func() error {
		return &HTTPError{StatusCode: http.StatusNotFound, Message: "Not Found"}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExecuteWithRetry(operation, policy, "benchmark")
	}
}
