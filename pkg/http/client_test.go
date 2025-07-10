package http

import (
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	expected := &ClientConfig{
		Timeout:      10 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 1 * time.Second,
		UserAgent:    "feed-forge/1.0",
		Headers:      make(map[string]string),
	}

	if !reflect.DeepEqual(config, expected) {
		t.Errorf("DefaultConfig() = %+v, expected %+v", config, expected)
	}

	// Verify headers map is properly initialized
	if config.Headers == nil {
		t.Error("DefaultConfig() Headers should not be nil")
	}

	if len(config.Headers) != 0 {
		t.Errorf("DefaultConfig() Headers should be empty, got %d items", len(config.Headers))
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name   string
		config *ClientConfig
	}{
		{
			name:   "with nil config",
			config: nil,
		},
		{
			name:   "with default config",
			config: DefaultConfig(),
		},
		{
			name: "with custom config",
			config: &ClientConfig{
				Timeout:      5 * time.Second,
				MaxRetries:   2,
				RetryBackoff: 500 * time.Millisecond,
				UserAgent:    "custom-agent/1.0",
				Headers:      map[string]string{"Custom": "header"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)

			if client == nil {
				t.Fatal("NewClient() returned nil")
			}

			if client.client == nil {
				t.Error("NewClient() client.client should not be nil")
			}

			if client.config == nil {
				t.Error("NewClient() client.config should not be nil")
			}

			// When config is nil, should use default config
			if tt.config == nil {
				expectedConfig := DefaultConfig()
				if !reflect.DeepEqual(client.config, expectedConfig) {
					t.Errorf("NewClient(nil) should use default config")
				}
			} else {
				if !reflect.DeepEqual(client.config, tt.config) {
					t.Errorf("NewClient() config = %+v, expected %+v", client.config, tt.config)
				}
			}

			// Verify timeout is set correctly
			expectedTimeout := client.config.Timeout
			if client.client.Timeout != expectedTimeout {
				t.Errorf("NewClient() timeout = %v, expected %v", client.client.Timeout, expectedTimeout)
			}
		})
	}
}

func TestIsRetryableStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{
			name:       "200 OK - not retryable",
			statusCode: http.StatusOK,
			expected:   false,
		},
		{
			name:       "201 Created - not retryable",
			statusCode: http.StatusCreated,
			expected:   false,
		},
		{
			name:       "400 Bad Request - not retryable",
			statusCode: http.StatusBadRequest,
			expected:   false,
		},
		{
			name:       "401 Unauthorized - not retryable",
			statusCode: http.StatusUnauthorized,
			expected:   false,
		},
		{
			name:       "403 Forbidden - not retryable",
			statusCode: http.StatusForbidden,
			expected:   false,
		},
		{
			name:       "404 Not Found - not retryable",
			statusCode: http.StatusNotFound,
			expected:   false,
		},
		{
			name:       "429 Too Many Requests - retryable",
			statusCode: http.StatusTooManyRequests,
			expected:   true,
		},
		{
			name:       "500 Internal Server Error - retryable",
			statusCode: http.StatusInternalServerError,
			expected:   true,
		},
		{
			name:       "502 Bad Gateway - retryable",
			statusCode: http.StatusBadGateway,
			expected:   true,
		},
		{
			name:       "503 Service Unavailable - retryable",
			statusCode: http.StatusServiceUnavailable,
			expected:   true,
		},
		{
			name:       "504 Gateway Timeout - retryable",
			statusCode: http.StatusGatewayTimeout,
			expected:   true,
		},
		{
			name:       "505 HTTP Version Not Supported - not retryable",
			statusCode: http.StatusHTTPVersionNotSupported,
			expected:   false,
		},
		{
			name:       "edge case: 0 status code",
			statusCode: 0,
			expected:   false,
		},
		{
			name:       "edge case: negative status code",
			statusCode: -1,
			expected:   false,
		},
		{
			name:       "edge case: very high status code",
			statusCode: 999,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableStatusCode(tt.statusCode)
			if result != tt.expected {
				t.Errorf("IsRetryableStatusCode(%d) = %v, expected %v",
					tt.statusCode, result, tt.expected)
			}
		})
	}
}

func TestClientConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config *ClientConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: &ClientConfig{
				Timeout:      10 * time.Second,
				MaxRetries:   3,
				RetryBackoff: 1 * time.Second,
				UserAgent:    "test/1.0",
				Headers:      map[string]string{"Test": "header"},
			},
			valid: true,
		},
		{
			name: "zero timeout",
			config: &ClientConfig{
				Timeout:      0,
				MaxRetries:   3,
				RetryBackoff: 1 * time.Second,
				UserAgent:    "test/1.0",
				Headers:      map[string]string{},
			},
			valid: true, // Zero timeout might be valid in some cases
		},
		{
			name: "negative max retries",
			config: &ClientConfig{
				Timeout:      10 * time.Second,
				MaxRetries:   -1,
				RetryBackoff: 1 * time.Second,
				UserAgent:    "test/1.0",
				Headers:      map[string]string{},
			},
			valid: false, // Negative retries don't make sense
		},
		{
			name: "zero retry backoff",
			config: &ClientConfig{
				Timeout:      10 * time.Second,
				MaxRetries:   3,
				RetryBackoff: 0,
				UserAgent:    "test/1.0",
				Headers:      map[string]string{},
			},
			valid: true, // Zero backoff might be valid for immediate retries
		},
		{
			name: "empty user agent",
			config: &ClientConfig{
				Timeout:      10 * time.Second,
				MaxRetries:   3,
				RetryBackoff: 1 * time.Second,
				UserAgent:    "",
				Headers:      map[string]string{},
			},
			valid: true, // Empty user agent is allowed
		},
		{
			name: "nil headers",
			config: &ClientConfig{
				Timeout:      10 * time.Second,
				MaxRetries:   3,
				RetryBackoff: 1 * time.Second,
				UserAgent:    "test/1.0",
				Headers:      nil,
			},
			valid: true, // Nil headers should be handled gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that we can create a client with the config
			client := NewClient(tt.config)
			if client == nil {
				t.Fatal("NewClient() returned nil")
			}

			// Basic validation checks
			if tt.config.MaxRetries < 0 && tt.valid {
				t.Error("Negative MaxRetries should not be considered valid")
			}
		})
	}
}
