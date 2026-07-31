package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFromURL_Success(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"name": "remote-config",
			"version": "3.0.0",
			"debug": true,
			"timeout": 45
		}`))
	}))
	defer server.Close()

	var config testConfig
	err := loadFromURL(server.URL, 5*time.Second, &config)

	if err != nil {
		t.Errorf("loadFromURL() error = %v", err)
		return
	}

	// Verify loaded values
	if config.Name != "remote-config" {
		t.Errorf("config.Name = %q, want %q", config.Name, "remote-config")
	}

	if config.Version != "3.0.0" {
		t.Errorf("config.Version = %q, want %q", config.Version, "3.0.0")
	}
}

func TestLoadFromURL_Errors(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		wantErr     bool
		errorSubstr string
	}{
		{
			name: "HTTP 404 error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			wantErr:     true,
			errorSubstr: "HTTP 404",
		},
		{
			name: "invalid JSON response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{invalid json}`))
				}))
			},
			wantErr:     true,
			errorSubstr: "failed to decode json response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			var config testConfig
			err := loadFromURL(server.URL, 5*time.Second, &config)

			if (err != nil) != tt.wantErr {
				t.Errorf("loadFromURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && !containsString(err.Error(), tt.errorSubstr) {
				t.Errorf("loadFromURL() error = %v, should contain %q", err, tt.errorSubstr)
			}
		})
	}
}

func TestLoadFromURL_Timeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than timeout
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name": "test"}`))
	}))
	defer server.Close()

	var config testConfig
	err := loadFromURL(server.URL, 500*time.Millisecond, &config) // Short timeout

	if err == nil {
		t.Errorf("loadFromURL() should have timed out")
	}

	if !containsString(err.Error(), "failed to fetch config from URL") {
		t.Errorf("loadFromURL() error should indicate URL fetch failure, got: %v", err)
	}
}

func TestLoadFromURLWithFallback_ResetsTargetOnDefaultFallback(t *testing.T) {
	cfg := testConfig{Name: "stale", Version: "old", Debug: true, Timeout: 99}
	err := LoadFromURLWithFallback(&LoaderConfig{
		RemoteURL:         "http://invalid-url.example.com",
		LocalPath:         "/nonexistent/path",
		Timeout:           1 * time.Second,
		FallbackToDefault: true,
	}, &cfg)
	if err != nil {
		t.Fatalf("LoadFromURLWithFallback() error = %v", err)
	}
	if cfg != (testConfig{}) {
		t.Fatalf("LoadFromURLWithFallback() fallback cfg = %#v, want zero-value default", cfg)
	}
}

func TestLoadFromURLWithFallback(t *testing.T) {
	tempDir := t.TempDir()

	// Create a local fallback file
	localContent := `{
		"name": "local-fallback",
		"version": "1.0.0",
		"debug": false,
		"timeout": 20
	}`

	localFile := filepath.Join(tempDir, "fallback.json")
	err := os.WriteFile(localFile, []byte(localContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create local fallback file: %v", err)
	}

	tests := []struct {
		name           string
		config         *LoaderConfig
		setupServer    func() *httptest.Server
		expectedSource string // "remote" or "local"
		wantErr        bool
	}{
		{
			name: "remote success",
			config: &LoaderConfig{
				RemoteURL:         "", // Will be set from server
				LocalPath:         localFile,
				Timeout:           5 * time.Second,
				FallbackToDefault: true,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"name": "remote-success", "version": "2.0.0", "debug": true, "timeout": 30}`))
				}))
			},
			expectedSource: "remote",
			wantErr:        false,
		},
		{
			name: "remote fail, local success",
			config: &LoaderConfig{
				RemoteURL:         "http://invalid-url-that-should-fail.example.com",
				LocalPath:         localFile,
				Timeout:           1 * time.Second,
				FallbackToDefault: true,
			},
			setupServer:    func() *httptest.Server { return nil },
			expectedSource: "local",
			wantErr:        false,
		},
		{
			name: "both fail, fallback enabled",
			config: &LoaderConfig{
				RemoteURL:         "http://invalid-url.example.com",
				LocalPath:         "/nonexistent/path",
				Timeout:           1 * time.Second,
				FallbackToDefault: true,
			},
			setupServer:    func() *httptest.Server { return nil },
			expectedSource: "default",
			wantErr:        false,
		},
		{
			name: "both fail, no fallback",
			config: &LoaderConfig{
				RemoteURL:         "http://invalid-url.example.com",
				LocalPath:         "/nonexistent/path",
				Timeout:           1 * time.Second,
				FallbackToDefault: false,
			},
			setupServer:    func() *httptest.Server { return nil },
			expectedSource: "",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupServer != nil {
				server := tt.setupServer()
				if server != nil {
					defer server.Close()
					tt.config.RemoteURL = server.URL
				}
			}

			var config testConfig
			err := LoadFromURLWithFallback(tt.config, &config)

			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromURLWithFallback() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				switch tt.expectedSource {
				case "remote":
					if config.Name != "remote-success" {
						t.Errorf("Expected remote config, got name = %q", config.Name)
					}
				case "local":
					if config.Name != "local-fallback" {
						t.Errorf("Expected local config, got name = %q", config.Name)
					}
				case "default":
					if config != (testConfig{}) {
						t.Errorf("Expected zero-value default config with fallback, got %#v", config)
					}
				}
			}
		})
	}
}

func TestLoadOrFetch(t *testing.T) {
	tempDir := t.TempDir()

	// Create a local test file
	localContent := `{"name": "load-or-fetch-test", "version": "1.0.0"}`
	localFile := filepath.Join(tempDir, "test.json")
	err := os.WriteFile(localFile, []byte(localContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test successful load from local file
	var config testConfig
	err = LoadOrFetch(localFile, "http://invalid-url.example.com", &config)

	if err != nil {
		t.Errorf("LoadOrFetch() error = %v", err)
		return
	}

	if config.Name != "load-or-fetch-test" {
		t.Errorf("config.Name = %q, want %q", config.Name, "load-or-fetch-test")
	}

	// Test with invalid paths (should succeed due to default fallback)
	var config2 testConfig
	err = LoadOrFetch("/nonexistent", "http://invalid-url.example.com", &config2)

	if err != nil {
		t.Errorf("LoadOrFetch() with fallback should not error, got %v", err)
	}
}
