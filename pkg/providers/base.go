package providers

import (
	"log/slog"

	"github.com/lepinkainen/feed-forge/pkg/database"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
)

// BaseProvider provides common functionality for all feed providers
type BaseProvider struct {
	// Database connections
	ContentDB *database.Database
	OgDB      *opengraph.Database
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
	ogDBPath, err := filesystem.GetDefaultPath("opengraph.db")
	if err != nil {
		return nil, err
	}

	base.OgDB, err = opengraph.NewDatabase(ogDBPath)
	if err != nil {
		return nil, err
	}

	// Initialize content database if needed
	if dbConfig.UseContentDB && dbConfig.ContentDBName != "" {
		contentDBPath, err := filesystem.GetDefaultPath(dbConfig.ContentDBName)
		if err != nil {
			if closeErr := base.OgDB.Close(); closeErr != nil {
				slog.Error("Failed to close OpenGraph database", "error", closeErr)
			}
			return nil, err
		}

		base.ContentDB, err = database.NewDatabase(database.Config{Path: contentDBPath})
		if err != nil {
			if closeErr := base.OgDB.Close(); closeErr != nil {
				slog.Error("Failed to close OpenGraph database", "error", closeErr)
			}
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
