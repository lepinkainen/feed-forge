// Package config provides configuration loading utilities with fallback support.
package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	httputil "github.com/lepinkainen/feed-forge/pkg/http"
	"gopkg.in/yaml.v3"
)

// Configuration loading errors
var (
	ErrConfigNotFound    = errors.New("configuration not found")
	ErrConfigInvalid     = errors.New("configuration is invalid")
	ErrUnsupportedFormat = errors.New("unsupported configuration format")
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
func LoadOrFetch(localPath, remoteURL string, target any) error {
	config := DefaultLoaderConfig()
	config.LocalPath = localPath
	config.RemoteURL = remoteURL
	return LoadFromURLWithFallback(config, target)
}

// LoadFromURLWithFallback loads configuration from URL with local fallback
func LoadFromURLWithFallback(config *LoaderConfig, target any) error {
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
		return fmt.Errorf("%w: tried URL and local file", ErrConfigNotFound)
	}

	return nil
}

// loadFromURL loads configuration from a remote URL using shared HTTP utilities
func loadFromURL(url string, timeout time.Duration, target any) error {
	httpConfig := httputil.DefaultConfig()
	httpConfig.Timeout = timeout

	client := httputil.NewClient(httpConfig)
	resp, err := client.GetWithContext(context.Background(), url)
	if err != nil {
		return fmt.Errorf("failed to fetch config from URL: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if err := httputil.EnsureStatusOK(resp); err != nil {
		return fmt.Errorf("HTTP error fetching config: %w", err)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode configuration: %w", err)
	}

	return nil
}

// loadFromFile loads configuration from a local file with automatic format detection
func loadFromFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrConfigNotFound, path)
		}
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Detect format based on file extension and content
	format := detectFormat(path, data)

	switch format {
	case "json":
		if err := json.Unmarshal(data, target); err != nil {
			return fmt.Errorf("%w: failed to parse JSON from %s: %v", ErrConfigInvalid, path, err)
		}
	case "yaml":
		if err := yaml.Unmarshal(data, target); err != nil {
			return fmt.Errorf("%w: failed to parse YAML from %s: %v", ErrConfigInvalid, path, err)
		}
	default:
		return fmt.Errorf("%w: %s (detected: %s)", ErrUnsupportedFormat, path, format)
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
