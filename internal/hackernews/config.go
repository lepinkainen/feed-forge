package hackernews

import (
	"log/slog"
	"strings"

	configloader "github.com/lepinkainen/feed-forge/pkg/config"
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

// DefaultConfigURL is the default configuration URL for domain mappings
const DefaultConfigURL = "https://raw.githubusercontent.com/lepinkainen/hntop-rss/refs/heads/main/configs/domains.json"

// LoadConfig loads configuration with fallback priority:
// 1. Local file (if specified)
// 2. Remote URL (default or custom)
// If no configuration can be loaded, returns nil to disable domain mapping
func LoadConfig(configPath, configURL string) *CategoryMapper {
	var config DomainConfig

	// Use default URL if none provided
	url := configURL
	if url == "" {
		url = DefaultConfigURL
	}

	slog.Debug("Loading HackerNews domain config", "path", configPath, "url", url)

	// Use shared configuration loading utility
	err := configloader.LoadOrFetch(configPath, url, &config)
	if err != nil {
		slog.Warn("Failed to load domain config, domain mapping will be disabled", "error", err)
		return nil
	}

	slog.Debug("Successfully loaded domain configuration")
	return NewCategoryMapper(&config)
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
