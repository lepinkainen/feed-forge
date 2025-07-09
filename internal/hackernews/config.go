package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// DomainConfig represents the configuration structure for domain mappings
type DomainConfig struct {
	CategoryDomains map[string][]string `json:"category_domains"`
}

// CategoryMapper provides methods for domain categorization
type CategoryMapper struct {
	config           *DomainConfig
	domainToCategory map[string]string // reverse lookup for efficient searching
}

// Default configuration URL
const DefaultConfigURL = "https://raw.githubusercontent.com/lepinkainen/hntop-rss/refs/heads/main/configs/domains.json"

// loadConfigFromURL loads configuration from a remote URL with timeout
func loadConfigFromURL(url string) (*DomainConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var config DomainConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &config, nil
}

// loadConfigFromFile loads configuration from a local file
func loadConfigFromFile(filepath string) (*DomainConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config DomainConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &config, nil
}

// LoadConfig loads configuration with fallback priority:
// 1. Local file (if specified)
// 2. Remote URL (default or custom)
// If no configuration can be loaded, returns nil to disable domain mapping
func LoadConfig(configPath, configURL string) *CategoryMapper {
	var config *DomainConfig
	var err error

	// Try loading from local file first (if specified)
	if configPath != "" {
		slog.Debug("Loading config from local file", "path", configPath)
		config, err = loadConfigFromFile(configPath)
		if err != nil {
			slog.Warn("Failed to load local config, trying remote", "error", err)
		} else {
			slog.Info("Successfully loaded config from local file", "path", configPath)
		}
	}

	// Try loading from remote URL if local file failed or wasn't specified
	if config == nil {
		url := configURL
		if url == "" {
			url = DefaultConfigURL
		}

		slog.Debug("Loading config from remote URL", "url", url)
		config, err = loadConfigFromURL(url)
		if err != nil {
			slog.Warn("Failed to load remote config, domain mapping will be disabled", "error", err)
		} else {
			slog.Info("Successfully loaded config from remote URL", "url", url)
		}
	}

	// Return nil if no configuration could be loaded
	if config == nil {
		slog.Info("No domain configuration available, domain mapping disabled")
		return nil
	}

	return NewCategoryMapper(config)
}

// NewCategoryMapper creates a new CategoryMapper with reverse lookup optimization
func NewCategoryMapper(config *DomainConfig) *CategoryMapper {
	mapper := &CategoryMapper{
		config:           config,
		domainToCategory: make(map[string]string),
	}

	// Build reverse lookup map for efficient domain matching
	for category, domains := range config.CategoryDomains {
		for _, domain := range domains {
			mapper.domainToCategory[strings.ToLower(domain)] = category
		}
	}

	slog.Debug("CategoryMapper initialized", "categories", len(config.CategoryDomains), "domain_mappings", len(mapper.domainToCategory))
	return mapper
}

// GetCategoryForDomain returns the category for a given domain, or empty string if not found
func (cm *CategoryMapper) GetCategoryForDomain(domain string) string {
	domain = strings.ToLower(domain)

	// Check for exact match first
	if category, exists := cm.domainToCategory[domain]; exists {
		return category
	}

	// Check for partial matches (domain contains mapped domain)
	for mappedDomain, category := range cm.domainToCategory {
		if strings.Contains(domain, mappedDomain) {
			return category
		}
	}

	return ""
}

// GetAllCategories returns all available categories
func (cm *CategoryMapper) GetAllCategories() []string {
	categories := make([]string, 0, len(cm.config.CategoryDomains))
	for category := range cm.config.CategoryDomains {
		categories = append(categories, category)
	}
	return categories
}
