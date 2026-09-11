package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"slices"
	"syscall"
	"time"
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
		RetryableErrors:   []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout},
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

// ErrEmptyBody reports a successful HTTP status carrying no body. It is
// transient, so it is retryable.
var ErrEmptyBody = errors.New("empty response body")

func asHTTPError(err error) (*HTTPError, bool) {
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		return nil, false
	}
	return httpErr, true
}

// IsRetryableError checks if an error should trigger a retry
func (rp *RetryPolicy) IsRetryableError(err error) bool {
	if errors.Is(err, ErrEmptyBody) {
		return true
	}
	if httpErr, ok := asHTTPError(err); ok {
		return rp.isRetryableStatusCode(httpErr.StatusCode)
	}
	return false
}

// IsTransientUpstreamError reports whether err is the kind of failure a later
// run is likely to clear on its own: an HTTP 4xx/5xx from an upstream
// service, an empty response body, or a network-level hiccup (timeout,
// connection reset/refused, unreachable host, DNS failure, or unexpected
// EOF). Callers demote these in logs and skip alerting on them, since a
// retry or the next scheduled run will probably succeed.
func IsTransientUpstreamError(err error) bool {
	if err == nil {
		return false
	}
	if code, ok := UpstreamStatusCode(err); ok && code >= 400 && code < 600 {
		return true
	}
	if errors.Is(err, ErrEmptyBody) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EHOSTUNREACH) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

// UpstreamStatusCode returns the HTTP status code from a wrapped HTTPError.
func UpstreamStatusCode(err error) (int, bool) {
	httpErr, ok := asHTTPError(err)
	if !ok {
		return 0, false
	}
	return httpErr.StatusCode, true
}

// IsRateLimitError checks if an error is specifically due to rate limiting
func (rp *RetryPolicy) IsRateLimitError(err error) bool {
	httpErr, ok := asHTTPError(err)
	return ok && httpErr.StatusCode == http.StatusTooManyRequests
}

// isRetryableStatusCode checks if a status code should trigger retries
func (rp *RetryPolicy) isRetryableStatusCode(statusCode int) bool {
	return slices.Contains(rp.RetryableErrors, statusCode)
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Message    string
	Err        error // wrapped error
}

// Error implements the error interface
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// Unwrap returns the wrapped error
func (e *HTTPError) Unwrap() error {
	return e.Err
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func() error

// ExecuteWithRetry executes an operation with retry logic.
func ExecuteWithRetry(operation RetryableOperation, policy *RetryPolicy, operationName string) error {
	return ExecuteWithRetryContext(context.Background(), operation, policy, operationName)
}

// ExecuteWithRetryContext executes an operation with retry logic that can be cancelled.
func ExecuteWithRetryContext(ctx context.Context, operation RetryableOperation, policy *RetryPolicy, operationName string) error {
	var (
		lastErr  error
		attempts int
	)

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if attempt > 1 {
			if err := waitBeforeAttempt(ctx, policy, attempt, lastErr, operationName); err != nil {
				return fmt.Errorf("operation %s cancelled before attempt %d: %w", operationName, attempt, err)
			}
		}

		attempts = attempt
		err := operation()
		if err == nil {
			if attempt > 1 {
				slog.Info("Operation succeeded after retry",
					"operation", operationName,
					"attempt", attempt)
			}
			return nil
		}

		lastErr = err

		if !policy.IsRetryableError(err) {
			slog.Debug("Error is not retryable, stopping",
				"operation", operationName,
				"attempt", attempt,
				"error", err)
			break
		}
	}

	return fmt.Errorf("operation %s failed after %d attempt(s): %w", operationName, attempts, lastErr)
}

func waitBeforeAttempt(ctx context.Context, policy *RetryPolicy, attempt int, lastErr error, operationName string) error {
	backoff := policy.CalculateBackoff(attempt - 1)
	if policy.IsRateLimitError(lastErr) {
		backoff *= 2
		slog.Debug("Rate limited, using longer backoff",
			"operation", operationName,
			"attempt", attempt,
			"backoff", backoff)
	}

	slog.Debug("Retrying operation",
		"operation", operationName,
		"attempt", attempt,
		"maxAttempts", policy.MaxAttempts,
		"backoff", backoff,
		"lastError", lastErr)

	return waitWithContext(ctx, backoff)
}
