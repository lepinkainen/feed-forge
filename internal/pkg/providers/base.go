package providers

import (
	"github.com/lepinkainen/feed-forge/pkg/database"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
)

// BaseProvider provides common functionality for all feed providers
type BaseProvider struct {
	// Database connections
	ContentDB *database.Database
	OgDB      *opengraph.Database

	// Common configuration
	outputDir string
}

// DatabaseConfig holds database configuration for providers
type DatabaseConfig struct {
	ContentDBName string // e.g., "hackernews.db", "reddit.db"
	UseContentDB  bool   // Whether this provider needs a content database
}

// NewBaseProvider creates a new base provider with common setup
func NewBaseProvider(dbConfig DatabaseConfig) (*BaseProvider, error) {
	base := &BaseProvider{}

	// Initialize OpenGraph database (all providers use this)
	ogDBPath, err := database.GetDefaultPath("opengraph.db")
	if err != nil {
		return nil, err
	}

	base.OgDB, err = opengraph.NewDatabase(ogDBPath)
	if err != nil {
		return nil, err
	}

	// Initialize content database if needed
	if dbConfig.UseContentDB && dbConfig.ContentDBName != "" {
		contentDBPath, err := database.GetDefaultPath(dbConfig.ContentDBName)
		if err != nil {
			base.OgDB.Close()
			return nil, err
		}

		base.ContentDB, err = database.NewDatabase(database.Config{Path: contentDBPath})
		if err != nil {
			base.OgDB.Close()
			return nil, err
		}
	}

	return base, nil
}

// Close cleans up database connections
func (b *BaseProvider) Close() error {
	var lastErr error

	if b.ContentDB != nil {
		if err := b.ContentDB.Close(); err != nil {
			lastErr = err
		}
	}

	if b.OgDB != nil {
		if err := b.OgDB.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// CleanupExpired removes expired OpenGraph cache entries
func (b *BaseProvider) CleanupExpired() error {
	if b.OgDB == nil {
		return nil
	}
	return b.OgDB.CleanupExpired()
}
