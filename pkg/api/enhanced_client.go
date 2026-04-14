package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// EnhancedClientConfig configures the enhanced HTTP client
type EnhancedClientConfig struct {
	BaseClient     *http.Client
	RateLimiter    RateLimiter
	RetryPolicy    *RetryPolicy
	UserAgent      string
	DefaultHeaders map[string]string
}

// EnhancedClient provides HTTP client functionality with rate limiting, retries, and standard headers
type EnhancedClient struct {
	client         *http.Client
	rateLimiter    RateLimiter
	retryPolicy    *RetryPolicy
	userAgent      string
	defaultHeaders map[string]string
}

// NewEnhancedClient creates a new enhanced HTTP client with the provided configuration
func NewEnhancedClient(config *EnhancedClientConfig) *EnhancedClient {
	// Set defaults if not provided
	if config.BaseClient == nil {
		config.BaseClient = &http.Client{Timeout: 30 * time.Second}
	}
	if config.RateLimiter == nil {
		config.RateLimiter = NewNoOpRateLimiter()
	}
	if config.RetryPolicy == nil {
		config.RetryPolicy = DefaultRetryPolicy()
	}
	if config.UserAgent == "" {
		config.UserAgent = "FeedForge/1.0"
	}
	if config.DefaultHeaders == nil {
		config.DefaultHeaders = make(map[string]string)
	}

	return &EnhancedClient{
		client:         config.BaseClient,
		rateLimiter:    config.RateLimiter,
		retryPolicy:    config.RetryPolicy,
		userAgent:      config.UserAgent,
		defaultHeaders: config.DefaultHeaders,
	}
}

// GetAndDecode performs an HTTP GET request with rate limiting, retries, and JSON decoding.
func (ec *EnhancedClient) GetAndDecode(url string, target any, additionalHeaders map[string]string) error {
	return ec.GetAndDecodeWithContext(context.Background(), url, target, additionalHeaders)
}

// GetAndDecodeWithContext performs an HTTP GET request with cancellation support.
func (ec *EnhancedClient) GetAndDecodeWithContext(ctx context.Context, url string, target any, additionalHeaders map[string]string) error {
	operation := func() error {
		if err := ec.rateLimiter.WaitContext(ctx); err != nil {
			return fmt.Errorf("rate limiter wait: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		ec.applyHeaders(req, additionalHeaders)

		start := time.Now()
		res, err := ec.client.Do(req)
		duration := time.Since(start)

		if err != nil {
			ec.logAPICall(url, duration, false, err)
			return fmt.Errorf("failed to perform GET request: %w", err)
		}
		defer func() { _ = res.Body.Close() }()

		if err := ensureStatusOK(res); err != nil {
			ec.logAPICall(url, duration, false, err)
			return &HTTPError{StatusCode: res.StatusCode, Message: err.Error(), Err: err}
		}

		if err := json.NewDecoder(res.Body).Decode(target); err != nil {
			ec.logAPICall(url, duration, false, err)
			return fmt.Errorf("failed to decode json response: %w", err)
		}

		ec.logAPICall(url, duration, true, nil)
		return nil
	}

	return ExecuteWithRetryContext(ctx, operation, ec.retryPolicy, fmt.Sprintf("GET %s", url))
}

// Get performs an HTTP GET request with rate limiting and retries, returning the response.
func (ec *EnhancedClient) Get(url string, additionalHeaders map[string]string) (*http.Response, error) {
	return ec.GetWithContext(context.Background(), url, additionalHeaders)
}

// GetWithContext performs an HTTP GET request with cancellation support.
func (ec *EnhancedClient) GetWithContext(ctx context.Context, url string, additionalHeaders map[string]string) (*http.Response, error) {
	var response *http.Response

	operation := func() error {
		if err := ec.rateLimiter.WaitContext(ctx); err != nil {
			return fmt.Errorf("rate limiter wait: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		ec.applyHeaders(req, additionalHeaders)

		start := time.Now()
		res, err := ec.client.Do(req)
		duration := time.Since(start)

		if err != nil {
			ec.logAPICall(url, duration, false, err)
			return fmt.Errorf("failed to perform GET request: %w", err)
		}

		if err := ensureStatusOK(res); err != nil {
			ec.logAPICall(url, duration, false, err)
			if closeErr := res.Body.Close(); closeErr != nil {
				slog.Error("Failed to close response body", "error", closeErr)
			}
			return &HTTPError{StatusCode: res.StatusCode, Message: err.Error(), Err: err}
		}

		response = res
		ec.logAPICall(url, duration, true, nil)
		return nil
	}

	if err := ExecuteWithRetryContext(ctx, operation, ec.retryPolicy, fmt.Sprintf("GET %s", url)); err != nil {
		return nil, err
	}

	return response, nil
}

func (ec *EnhancedClient) applyHeaders(req *http.Request, additionalHeaders map[string]string) {
	req.Header.Set("User-Agent", ec.userAgent)

	for key, value := range ec.defaultHeaders {
		req.Header.Set(key, value)
	}

	for key, value := range additionalHeaders {
		req.Header.Set(key, value)
	}
}

// CanProceed returns true if a request can be made without rate limiting delay
func (ec *EnhancedClient) CanProceed() bool {
	return ec.rateLimiter.CanProceed()
}

// SetUserAgent updates the User-Agent header for all requests
func (ec *EnhancedClient) SetUserAgent(userAgent string) {
	ec.userAgent = userAgent
}

// SetDefaultHeader sets a default header that will be included in all requests
func (ec *EnhancedClient) SetDefaultHeader(key, value string) {
	ec.defaultHeaders[key] = value
}

// RemoveDefaultHeader removes a default header
func (ec *EnhancedClient) RemoveDefaultHeader(key string) {
	delete(ec.defaultHeaders, key)
}

// ensureStatusOK checks if the response status is 200 OK
func ensureStatusOK(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, resp.Status)
	}
	return nil
}

// logAPICall logs API call statistics
func (ec *EnhancedClient) logAPICall(url string, duration time.Duration, success bool, err error) {
	status := "success"
	if !success {
		status = "failure"
	}

	fields := []any{
		"url", url,
		"duration", duration,
		"status", status,
	}

	if err != nil {
		fields = append(fields, "error", err)
	}

	if success {
		slog.Debug("API call completed", fields...)
	} else {
		slog.Warn("API call failed", fields...)
	}
}

// newBrowserTLSTransport creates an HTTP transport with a TLS fingerprint
// that avoids being blocked by sites that reject Go's default TLS client hello.
// Reddit in particular uses TLS fingerprinting to block automated clients.
func newBrowserTLSTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			},
		},
		ForceAttemptHTTP2: true,
	}
}

// NewRedditClient creates an enhanced client configured for Reddit API.
// Uses a custom TLS transport to avoid Reddit's TLS fingerprint blocking.
func NewRedditClient(baseClient *http.Client) *EnhancedClient {
	if baseClient == nil {
		baseClient = &http.Client{
			Timeout:   30 * time.Second,
			Transport: newBrowserTLSTransport(),
		}
	}
	return NewEnhancedClient(&EnhancedClientConfig{
		BaseClient:  baseClient,
		RateLimiter: NewSimpleRateLimiter(2 * time.Second), // Reddit rate limit - generous to avoid 429s
		RetryPolicy: DefaultRetryPolicy(),
		UserAgent:   "FeedForge/1.0 by theshrike79",
		DefaultHeaders: map[string]string{
			"Accept": "application/json",
		},
	})
}

// NewHackerNewsClient creates an enhanced client configured for Hacker News API
func NewHackerNewsClient() *EnhancedClient {
	return NewEnhancedClient(&EnhancedClientConfig{
		BaseClient:  &http.Client{Timeout: 30 * time.Second},
		RateLimiter: NewSimpleRateLimiter(500 * time.Millisecond), // Conservative rate limit
		RetryPolicy: ConservativeRetryPolicy(),
		UserAgent:   "FeedForge/1.0",
		DefaultHeaders: map[string]string{
			"Accept": "application/json",
		},
	})
}

// NewGenericClient creates an enhanced client with minimal configuration
func NewGenericClient() *EnhancedClient {
	return NewEnhancedClient(&EnhancedClientConfig{
		BaseClient:  &http.Client{Timeout: 30 * time.Second},
		RateLimiter: NewNoOpRateLimiter(), // No rate limiting by default
		RetryPolicy: ConservativeRetryPolicy(),
		UserAgent:   "FeedForge/1.0",
	})
}
