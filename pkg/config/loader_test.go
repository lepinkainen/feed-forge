package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test configuration structure
type testConfig struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	Debug   bool   `json:"debug" yaml:"debug"`
	Timeout int    `json:"timeout" yaml:"timeout"`
}

func TestDefaultLoaderConfig(t *testing.T) {
	config := DefaultLoaderConfig()

	if config == nil {
		t.Errorf("DefaultLoaderConfig() returned nil")
		return
	}

	if config.Timeout != 10*time.Second {
		t.Errorf("DefaultLoaderConfig().Timeout = %v, want %v", config.Timeout, 10*time.Second)
	}

	if config.MaxRetries != 3 {
		t.Errorf("DefaultLoaderConfig().MaxRetries = %d, want %d", config.MaxRetries, 3)
	}

	if !config.FallbackToDefault {
		t.Errorf("DefaultLoaderConfig().FallbackToDefault should be true")
	}

	if config.RemoteURL != "" {
		t.Errorf("DefaultLoaderConfig().RemoteURL should be empty")
	}

	if config.LocalPath != "" {
		t.Errorf("DefaultLoaderConfig().LocalPath should be empty")
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		data     []byte
		expected string
	}{
		{
			name:     "JSON file extension",
			path:     "config.json",
			data:     []byte(`{"test": true}`),
			expected: "json",
		},
		{
			name:     "YAML file extension",
			path:     "config.yaml",
			data:     []byte(`test: true`),
			expected: "yaml",
		},
		{
			name:     "YML file extension",
			path:     "config.yml",
			data:     []byte(`test: true`),
			expected: "yaml",
		},
		{
			name:     "JSON content detection",
			path:     "config",
			data:     []byte(`{"test": true}`),
			expected: "json",
		},
		{
			name:     "JSON array content detection",
			path:     "config",
			data:     []byte(`[{"test": true}]`),
			expected: "json",
		},
		{
			name:     "YAML content fallback",
			path:     "config",
			data:     []byte(`test: true`),
			expected: "yaml",
		},
		{
			name:     "Mixed extension vs content - extension wins",
			path:     "config.json",
			data:     []byte(`test: true`),
			expected: "json",
		},
		{
			name:     "Whitespace handling",
			path:     "config",
			data:     []byte(`  {"test": true}  `),
			expected: "json",
		},
		{
			name:     "Empty content defaults to YAML",
			path:     "config",
			data:     []byte(``),
			expected: "yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectFormat(tt.path, tt.data)
			if result != tt.expected {
				t.Errorf("detectFormat(%q, %q) = %q, want %q", tt.path, string(tt.data), result, tt.expected)
			}
		})
	}
}

func TestLoadFromFile_JSON(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test JSON file
	jsonContent := `{
		"name": "test-config",
		"version": "1.0.0",
		"debug": true,
		"timeout": 30
	}`

	jsonFile := filepath.Join(tempDir, "config.json")
	err := os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}

	var config testConfig
	err = loadFromFile(jsonFile, &config)

	if err != nil {
		t.Errorf("loadFromFile() error = %v", err)
		return
	}

	// Verify loaded values
	if config.Name != "test-config" {
		t.Errorf("config.Name = %q, want %q", config.Name, "test-config")
	}

	if config.Version != "1.0.0" {
		t.Errorf("config.Version = %q, want %q", config.Version, "1.0.0")
	}

	if !config.Debug {
		t.Errorf("config.Debug = %v, want %v", config.Debug, true)
	}

	if config.Timeout != 30 {
		t.Errorf("config.Timeout = %d, want %d", config.Timeout, 30)
	}
}

func TestLoadFromFile_YAML(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test YAML file
	yamlContent := `name: test-config-yaml
version: 2.0.0
debug: false
timeout: 60`

	yamlFile := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test YAML file: %v", err)
	}

	var config testConfig
	err = loadFromFile(yamlFile, &config)

	if err != nil {
		t.Errorf("loadFromFile() error = %v", err)
		return
	}

	// Verify loaded values
	if config.Name != "test-config-yaml" {
		t.Errorf("config.Name = %q, want %q", config.Name, "test-config-yaml")
	}

	if config.Version != "2.0.0" {
		t.Errorf("config.Version = %q, want %q", config.Version, "2.0.0")
	}

	if config.Debug {
		t.Errorf("config.Debug = %v, want %v", config.Debug, false)
	}

	if config.Timeout != 60 {
		t.Errorf("config.Timeout = %d, want %d", config.Timeout, 60)
	}
}

func TestLoadFromFile_Errors(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		filename    string
		content     string
		wantErr     bool
		errorSubstr string
	}{
		{
			name:        "file not found",
			filename:    "nonexistent.json",
			content:     "",
			wantErr:     true,
			errorSubstr: "failed to read file",
		},
		{
			name:        "invalid JSON",
			filename:    "invalid.json",
			content:     `{"name": "test", invalid}`,
			wantErr:     true,
			errorSubstr: "failed to parse JSON",
		},
		{
			name:        "invalid YAML",
			filename:    "invalid.yaml",
			content:     "name: test\n  invalid: : yaml",
			wantErr:     true,
			errorSubstr: "failed to parse YAML",
		},
		{
			name:        "XML parsed as YAML fails",
			filename:    "config.xml",
			content:     `<config><name>test</name></config>`,
			wantErr:     true,
			errorSubstr: "failed to parse YAML", // XML content fails YAML parsing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config testConfig
			var err error

			if tt.name == "file not found" {
				// Test with non-existent file
				err = loadFromFile(filepath.Join(tempDir, tt.filename), &config)
			} else {
				// Create file with test content
				filePath := filepath.Join(tempDir, tt.filename)
				writeErr := os.WriteFile(filePath, []byte(tt.content), 0644)
				if writeErr != nil {
					t.Fatalf("Failed to create test file: %v", writeErr)
				}
				err = loadFromFile(filePath, &config)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("loadFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && !containsString(err.Error(), tt.errorSubstr) {
				t.Errorf("loadFromFile() error = %v, should contain %q", err, tt.errorSubstr)
			}
		})
	}
}

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
			errorSubstr: "HTTP error",
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
			errorSubstr: "failed to decode configuration",
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
					// With fallback enabled and both sources failing, config should remain empty/default
					if config.Name != "" {
						t.Errorf("Expected default/empty config with fallback, got name = %q", config.Name)
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

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
