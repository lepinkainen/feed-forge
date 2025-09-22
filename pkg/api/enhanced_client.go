package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	httputil "github.com/lepinkainen/feed-forge/pkg/http"
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

// GetAndDecode performs an HTTP GET request with rate limiting, retries, and JSON decoding
func (ec *EnhancedClient) GetAndDecode(url string, target any, additionalHeaders map[string]string) error {
	operation := func() error {
		// Apply rate limiting
		ec.rateLimiter.Wait()

		// Create request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Set User-Agent
		req.Header.Set("User-Agent", ec.userAgent)

		// Set default headers
		for key, value := range ec.defaultHeaders {
			req.Header.Set(key, value)
		}

		// Set additional headers (these override defaults)
		for key, value := range additionalHeaders {
			req.Header.Set(key, value)
		}

		// Perform request
		start := time.Now()
		res, err := ec.client.Do(req)
		duration := time.Since(start)

		if err != nil {
			ec.logAPICall(url, duration, false, err)
			return fmt.Errorf("failed to perform GET request: %w", err)
		}
		defer func() { _ = res.Body.Close() }()

		// Check status code
		if err := httputil.EnsureStatusOK(res); err != nil {
			ec.logAPICall(url, duration, false, err)
			// Convert to our HTTPError type for retry logic
			return &HTTPError{
				StatusCode: res.StatusCode,
				Message:    err.Error(),
			}
		}

		// Decode JSON
		if err := json.NewDecoder(res.Body).Decode(target); err != nil {
			ec.logAPICall(url, duration, false, err)
			return fmt.Errorf("failed to decode json response: %w", err)
		}

		ec.logAPICall(url, duration, true, nil)
		return nil
	}

	return ExecuteWithRetry(operation, ec.retryPolicy, fmt.Sprintf("GET %s", url))
}

// Get performs an HTTP GET request with rate limiting and retries, returning the response
func (ec *EnhancedClient) Get(url string, additionalHeaders map[string]string) (*http.Response, error) {
	var response *http.Response

	operation := func() error {
		// Apply rate limiting
		ec.rateLimiter.Wait()

		// Create request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Set User-Agent
		req.Header.Set("User-Agent", ec.userAgent)

		// Set default headers
		for key, value := range ec.defaultHeaders {
			req.Header.Set(key, value)
		}

		// Set additional headers (these override defaults)
		for key, value := range additionalHeaders {
			req.Header.Set(key, value)
		}

		// Perform request
		start := time.Now()
		res, err := ec.client.Do(req)
		duration := time.Since(start)

		if err != nil {
			ec.logAPICall(url, duration, false, err)
			return fmt.Errorf("failed to perform GET request: %w", err)
		}

		// Check status code
		if err := httputil.EnsureStatusOK(res); err != nil {
			ec.logAPICall(url, duration, false, err)
			res.Body.Close() // Close body on error
			// Convert to our HTTPError type for retry logic
			return &HTTPError{
				StatusCode: res.StatusCode,
				Message:    err.Error(),
			}
		}

		response = res
		ec.logAPICall(url, duration, true, nil)
		return nil
	}

	err := ExecuteWithRetry(operation, ec.retryPolicy, fmt.Sprintf("GET %s", url))
	if err != nil {
		return nil, err
	}

	return response, nil
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

// NewRedditClient creates an enhanced client configured for Reddit API
func NewRedditClient(baseClient *http.Client) *EnhancedClient {
	return NewEnhancedClient(&EnhancedClientConfig{
		BaseClient:  baseClient,
		RateLimiter: NewSimpleRateLimiter(1 * time.Second), // Reddit rate limit
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
