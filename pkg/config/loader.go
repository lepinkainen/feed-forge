package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	httputil "github.com/lepinkainen/feed-forge/pkg/http"
	"gopkg.in/yaml.v3"
)

// LoaderConfig represents configuration loading options
type LoaderConfig struct {
	RemoteURL         string
	LocalPath         string
	Timeout           time.Duration
	MaxRetries        int
	FallbackToDefault bool
}

// DefaultLoaderConfig returns default loader configuration
func DefaultLoaderConfig() *LoaderConfig {
	return &LoaderConfig{
		Timeout:           10 * time.Second,
		MaxRetries:        3,
		FallbackToDefault: true,
	}
}

// LoadOrFetch loads configuration with fallback priority: local file -> remote URL
// This is the main function providers should use for configuration loading
func LoadOrFetch(localPath, remoteURL string, target interface{}) error {
	config := DefaultLoaderConfig()
	config.LocalPath = localPath
	config.RemoteURL = remoteURL
	return LoadFromURLWithFallback(config, target)
}

// LoadFromURLWithFallback loads configuration from URL with local fallback
func LoadFromURLWithFallback(config *LoaderConfig, target interface{}) error {
	// Try remote URL first if provided
	if config.RemoteURL != "" {
		if err := loadFromURL(config.RemoteURL, config.Timeout, target); err == nil {
			return nil
		}
	}

	// Try local file if remote failed
	if config.LocalPath != "" {
		if err := loadFromFile(config.LocalPath, target); err == nil {
			return nil
		}
	}

	// Return error if both failed and no fallback
	if !config.FallbackToDefault {
		return fmt.Errorf("failed to load configuration from URL and local file")
	}

	return nil
}

// loadFromURL loads configuration from a remote URL using shared HTTP utilities
func loadFromURL(url string, timeout time.Duration, target interface{}) error {
	httpConfig := httputil.DefaultConfig()
	httpConfig.Timeout = timeout

	client := httputil.NewClient(httpConfig)
	resp, err := client.GetWithContext(context.Background(), url)
	if err != nil {
		return fmt.Errorf("failed to fetch config from URL: %w", err)
	}
	defer resp.Body.Close()

	if err := httputil.EnsureStatusOK(resp); err != nil {
		return fmt.Errorf("HTTP error fetching config: %w", err)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode configuration: %w", err)
	}

	return nil
}

// loadFromFile loads configuration from a local file with automatic format detection
func loadFromFile(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Detect format based on file extension and content
	format := detectFormat(path, data)

	switch format {
	case "json":
		if err := json.Unmarshal(data, target); err != nil {
			return fmt.Errorf("failed to parse JSON from %s: %w", path, err)
		}
	case "yaml":
		if err := yaml.Unmarshal(data, target); err != nil {
			return fmt.Errorf("failed to parse YAML from %s: %w", path, err)
		}
	default:
		return fmt.Errorf("unsupported file format for %s (detected: %s)", path, format)
	}

	return nil
}

// detectFormat determines the file format based on extension and content
func detectFormat(path string, data []byte) string {
	// Check file extension first
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	}

	// Fall back to content-based detection
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "json"
	}

	// Assume YAML for other cases (YAML is more permissive)
	return "yaml"
}
