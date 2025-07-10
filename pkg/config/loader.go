package config

import (
	"encoding/json"
	"fmt"
	"time"

	httputil "github.com/lepinkainen/feed-forge/pkg/http"
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
	resp, err := client.Get(url)
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

// loadFromFile loads configuration from a local file
func loadFromFile(path string, target interface{}) error {
	return fmt.Errorf("local file loading not implemented yet - use existing provider-specific loaders")
}
