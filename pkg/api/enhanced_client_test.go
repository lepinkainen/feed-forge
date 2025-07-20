package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewEnhancedClient(t *testing.T) {
	tests := []struct {
		name   string
		config *EnhancedClientConfig
		want   func(*EnhancedClient) bool
	}{
		{
			name:   "empty config gets defaults",
			config: &EnhancedClientConfig{},
			want: func(ec *EnhancedClient) bool {
				return ec.client.Timeout == 30*time.Second &&
					ec.userAgent == "FeedForge/1.0" &&
					ec.rateLimiter != nil &&
					ec.retryPolicy != nil &&
					ec.defaultHeaders != nil
			},
		},
		{
			name: "custom config preserved",
			config: &EnhancedClientConfig{
				BaseClient:  &http.Client{Timeout: 5 * time.Second},
				RateLimiter: NewSimpleRateLimiter(2 * time.Second),
				UserAgent:   "CustomAgent/1.0",
				DefaultHeaders: map[string]string{
					"Accept": "application/json",
				},
			},
			want: func(ec *EnhancedClient) bool {
				return ec.client.Timeout == 5*time.Second &&
					ec.userAgent == "CustomAgent/1.0" &&
					ec.defaultHeaders["Accept"] == "application/json"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewEnhancedClient(tt.config)
			if !tt.want(got) {
				t.Errorf("NewEnhancedClient() validation failed")
			}
		})
	}
}

func TestEnhancedClient_GetAndDecode(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		target         interface{}
		headers        map[string]string
		wantErr        bool
		wantTarget     interface{}
	}{
		{
			name: "successful JSON decode",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"message": "success"})
			},
			target:     &map[string]string{},
			wantErr:    false,
			wantTarget: &map[string]string{"message": "success"},
		},
		{
			name: "server error triggers retry",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Server Error"))
			},
			target:  &map[string]string{},
			wantErr: true,
		},
		{
			name: "invalid JSON returns error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("invalid json"))
			},
			target:  &map[string]string{},
			wantErr: true,
		},
		{
			name: "custom headers are set",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Custom-Header") != "test-value" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"header": "received"})
			},
			target:     &map[string]string{},
			headers:    map[string]string{"X-Custom-Header": "test-value"},
			wantErr:    false,
			wantTarget: &map[string]string{"header": "received"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			client := NewEnhancedClient(&EnhancedClientConfig{
				RetryPolicy: &RetryPolicy{
					MaxAttempts:     2,
					InitialBackoff:  10 * time.Millisecond,
					RetryableErrors: []int{http.StatusInternalServerError},
				},
				RateLimiter: NewNoOpRateLimiter(),
			})

			err := client.GetAndDecode(server.URL, tt.target, tt.headers)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetAndDecode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.wantTarget != nil {
				targetMap := tt.target.(*map[string]string)
				wantMap := tt.wantTarget.(*map[string]string)
				if (*targetMap)["message"] != (*wantMap)["message"] && (*targetMap)["header"] != (*wantMap)["header"] {
					t.Errorf("GetAndDecode() target = %v, want %v", *targetMap, *wantMap)
				}
			}
		})
	}
}

func TestEnhancedClient_Get(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		headers        map[string]string
		wantErr        bool
		wantStatus     int
	}{
		{
			name: "successful GET request",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
		},
		{
			name: "user agent is set correctly",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.Header.Get("User-Agent"), "FeedForge") {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
		},
		{
			name: "server error returns error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			client := NewEnhancedClient(&EnhancedClientConfig{
				RetryPolicy: &RetryPolicy{
					MaxAttempts:     1,
					RetryableErrors: []int{},
				},
				RateLimiter: NewNoOpRateLimiter(),
			})

			resp, err := client.Get(server.URL, tt.headers)

			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				defer resp.Body.Close()
				if resp.StatusCode != tt.wantStatus {
					t.Errorf("Get() status = %v, want %v", resp.StatusCode, tt.wantStatus)
				}
			}
		})
	}
}

func TestEnhancedClient_HeaderManagement(t *testing.T) {
	client := NewEnhancedClient(&EnhancedClientConfig{
		RateLimiter: NewNoOpRateLimiter(),
	})

	// Test SetUserAgent
	client.SetUserAgent("TestAgent/2.0")
	if client.userAgent != "TestAgent/2.0" {
		t.Errorf("SetUserAgent() failed, got %s", client.userAgent)
	}

	// Test SetDefaultHeader
	client.SetDefaultHeader("X-Test", "value1")
	if client.defaultHeaders["X-Test"] != "value1" {
		t.Errorf("SetDefaultHeader() failed")
	}

	// Test RemoveDefaultHeader
	client.RemoveDefaultHeader("X-Test")
	if _, exists := client.defaultHeaders["X-Test"]; exists {
		t.Errorf("RemoveDefaultHeader() failed")
	}
}

func TestEnhancedClient_CanProceed(t *testing.T) {
	// Test with NoOpRateLimiter
	client1 := NewEnhancedClient(&EnhancedClientConfig{
		RateLimiter: NewNoOpRateLimiter(),
	})
	if !client1.CanProceed() {
		t.Errorf("CanProceed() with NoOpRateLimiter should return true")
	}

	// Test with SimpleRateLimiter
	client2 := NewEnhancedClient(&EnhancedClientConfig{
		RateLimiter: NewSimpleRateLimiter(100 * time.Millisecond),
	})
	// First call should be able to proceed
	if !client2.CanProceed() {
		t.Errorf("CanProceed() first call should return true")
	}
}

func TestNewRedditClient(t *testing.T) {
	baseClient := &http.Client{Timeout: 10 * time.Second}
	client := NewRedditClient(baseClient)

	if client.client != baseClient {
		t.Errorf("NewRedditClient() didn't use provided base client")
	}

	if !strings.Contains(client.userAgent, "theshrike79") {
		t.Errorf("NewRedditClient() user agent incorrect: %s", client.userAgent)
	}

	if client.defaultHeaders["Accept"] != "application/json" {
		t.Errorf("NewRedditClient() missing Accept header")
	}
}

func TestNewHackerNewsClient(t *testing.T) {
	client := NewHackerNewsClient()

	if client.client.Timeout != 30*time.Second {
		t.Errorf("NewHackerNewsClient() timeout incorrect")
	}

	if client.userAgent != "FeedForge/1.0" {
		t.Errorf("NewHackerNewsClient() user agent incorrect: %s", client.userAgent)
	}

	if client.defaultHeaders["Accept"] != "application/json" {
		t.Errorf("NewHackerNewsClient() missing Accept header")
	}
}

func TestNewGenericClient(t *testing.T) {
	client := NewGenericClient()

	if client.client.Timeout != 30*time.Second {
		t.Errorf("NewGenericClient() timeout incorrect")
	}

	if client.userAgent != "FeedForge/1.0" {
		t.Errorf("NewGenericClient() user agent incorrect: %s", client.userAgent)
	}

	// Should use NoOpRateLimiter
	if !client.CanProceed() {
		t.Errorf("NewGenericClient() should have no rate limiting")
	}
}
