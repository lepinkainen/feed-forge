package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClientConfig represents HTTP client configuration
type ClientConfig struct {
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
	UserAgent    string
	Headers      map[string]string
}

// DefaultConfig returns default HTTP client configuration
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		Timeout:      10 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 1 * time.Second,
		UserAgent:    "feed-forge/1.0",
		Headers:      make(map[string]string),
	}
}

// Client represents an HTTP client with retry logic
type Client struct {
	client *http.Client
	config *ClientConfig
}

// NewClient creates a new HTTP client with the given configuration
func NewClient(config *ClientConfig) *Client {
	if config == nil {
		config = DefaultConfig()
	}

	return &Client{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		config: config,
	}
}

// GetWithContext performs an HTTP GET request with context and retry logic
func (c *Client) GetWithContext(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}

	return c.doWithRetry(req)
}

// PostWithContext performs an HTTP POST request with context and retry logic
func (c *Client) PostWithContext(ctx context.Context, url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return c.doWithRetry(req)
}

// DoRequest performs an HTTP request with retry logic
func (c *Client) DoRequest(req *http.Request) (*http.Response, error) {
	return c.doWithRetry(req)
}

// doWithRetry performs an HTTP request with retry logic
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	// Set default headers
	if c.config.UserAgent != "" {
		req.Header.Set("User-Agent", c.config.UserAgent)
	}

	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}

	var lastErr error
	backoff := c.config.RetryBackoff

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(backoff):
				backoff *= 2 // Exponential backoff
			}
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Check if we should retry based on status code
		if IsRetryableStatusCode(resp.StatusCode) && attempt < c.config.MaxRetries {
			resp.Body.Close()
			lastErr = fmt.Errorf("retryable HTTP status: %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.config.MaxRetries+1, lastErr)
}

// IsRetryableStatusCode determines if an HTTP status code should be retried
func IsRetryableStatusCode(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
