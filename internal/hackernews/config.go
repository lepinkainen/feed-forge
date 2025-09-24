package hackernews

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/lepinkainen/feed-forge/configs"
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

// LoadConfig loads configuration with fallback priority:
// 1. Local file (if specified)
// 2. Embedded domains.json file
// If no configuration can be loaded, returns nil to disable domain mapping
func LoadConfig(configPath string) *CategoryMapper {
	var config DomainConfig

	slog.Debug("Loading HackerNews domain config", "path", configPath)

	// Try local file first if specified
	if configPath != "" {
		err := configloader.LoadOrFetch(configPath, "", &config)
		if err == nil {
			slog.Debug("Successfully loaded domain configuration from local file")
			return NewCategoryMapper(&config)
		}
		slog.Warn("Failed to load local config file, falling back to embedded config", "error", err)
	}

	// Fall back to embedded configuration
	embeddedData, err := configs.EmbeddedConfigs.ReadFile("domains.json")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			slog.Warn("Embedded domain config not found, domain mapping will be disabled")
			return nil
		}
		slog.Warn("Failed to read embedded domain config, domain mapping will be disabled", "error", err)
		return nil
	}

	if err := json.Unmarshal(embeddedData, &config); err != nil {
		slog.Warn("Failed to load embedded domain config, domain mapping will be disabled", "error", err)
		return nil
	}

	slog.Debug("Successfully loaded domain configuration from embedded file")
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
