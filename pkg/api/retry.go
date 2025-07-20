package api

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

// RetryPolicy defines the configuration for retry behavior
type RetryPolicy struct {
	MaxAttempts       int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
	RetryableErrors   []int // HTTP status codes that should trigger retries
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:       3,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors:   []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout},
	}
}

// AggressiveRetryPolicy returns a retry policy with more aggressive retries
func AggressiveRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:       5,
		InitialBackoff:    500 * time.Millisecond,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors:   []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout},
	}
}

// ConservativeRetryPolicy returns a retry policy with minimal retries
func ConservativeRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:       2,
		InitialBackoff:    2 * time.Second,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors:   []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable},
	}
}

// CalculateBackoff calculates the backoff duration for a given attempt
func (rp *RetryPolicy) CalculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	backoff := float64(rp.InitialBackoff) * math.Pow(rp.BackoffMultiplier, float64(attempt-1))
	if backoff > float64(rp.MaxBackoff) {
		backoff = float64(rp.MaxBackoff)
	}

	return time.Duration(backoff)
}

// IsRetryableError checks if an error should trigger a retry
func (rp *RetryPolicy) IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for HTTP status code errors
	if httpErr, ok := err.(*HTTPError); ok {
		return rp.isRetryableStatusCode(httpErr.StatusCode)
	}

	// Check for OAuth2 retrieve errors
	if oauthErr, ok := err.(*oauth2.RetrieveError); ok {
		return rp.isRetryableStatusCode(oauthErr.Response.StatusCode)
	}

	// For other errors, default to not retrying
	return false
}

// IsRateLimitError checks if an error is specifically due to rate limiting
func (rp *RetryPolicy) IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	// Check for HTTP status code errors
	if httpErr, ok := err.(*HTTPError); ok {
		return httpErr.StatusCode == http.StatusTooManyRequests
	}

	// Check for OAuth2 retrieve errors
	if oauthErr, ok := err.(*oauth2.RetrieveError); ok {
		return oauthErr.Response.StatusCode == http.StatusTooManyRequests
	}

	return false
}

// isRetryableStatusCode checks if a status code should trigger retries
func (rp *RetryPolicy) isRetryableStatusCode(statusCode int) bool {
	for _, code := range rp.RetryableErrors {
		if statusCode == code {
			return true
		}
	}
	return false
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Message    string
}

// Error implements the error interface
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func() error

// ExecuteWithRetry executes an operation with retry logic
func ExecuteWithRetry(operation RetryableOperation, policy *RetryPolicy, operationName string) error {
	var lastErr error

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		// Log retry attempts
		if attempt > 1 {
			backoff := policy.CalculateBackoff(attempt - 1)
			slog.Warn("Retrying operation",
				"operation", operationName,
				"attempt", attempt,
				"maxAttempts", policy.MaxAttempts,
				"backoff", backoff,
				"lastError", lastErr)
			time.Sleep(backoff)
		}

		// Execute the operation
		err := operation()
		if err == nil {
			// Success
			if attempt > 1 {
				slog.Info("Operation succeeded after retry",
					"operation", operationName,
					"attempt", attempt)
			}
			return nil
		}

		lastErr = err

		// Check if we should retry this error
		if !policy.IsRetryableError(err) {
			slog.Debug("Error is not retryable, stopping",
				"operation", operationName,
				"attempt", attempt,
				"error", err)
			break
		}

		// Special handling for rate limit errors
		if policy.IsRateLimitError(err) {
			rateLimitBackoff := policy.CalculateBackoff(attempt) * 2 // Longer backoff for rate limits
			slog.Warn("Rate limited, using longer backoff",
				"operation", operationName,
				"attempt", attempt,
				"backoff", rateLimitBackoff)
			time.Sleep(rateLimitBackoff)
		}
	}

	return fmt.Errorf("operation %s failed after %d attempts: %w", operationName, policy.MaxAttempts, lastErr)
}
