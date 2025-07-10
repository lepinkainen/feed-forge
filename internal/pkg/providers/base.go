package providers

import (
	"github.com/lepinkainen/feed-forge/pkg/database"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
)

// BaseProvider provides common functionality for all feed providers
type BaseProvider struct {
	// Database connections
	contentDB *database.Database
	ogDB      *opengraph.Database

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

	base.ogDB, err = opengraph.NewDatabase(ogDBPath)
	if err != nil {
		return nil, err
	}

	// Initialize content database if needed
	if dbConfig.UseContentDB && dbConfig.ContentDBName != "" {
		contentDBPath, err := database.GetDefaultPath(dbConfig.ContentDBName)
		if err != nil {
			base.ogDB.Close()
			return nil, err
		}

		base.contentDB, err = database.NewDatabase(database.Config{Path: contentDBPath})
		if err != nil {
			base.ogDB.Close()
			return nil, err
		}
	}

	return base, nil
}

// Close cleans up database connections
func (b *BaseProvider) Close() error {
	var lastErr error

	if b.contentDB != nil {
		if err := b.contentDB.Close(); err != nil {
			lastErr = err
		}
	}

	if b.ogDB != nil {
		if err := b.ogDB.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// GetOpenGraphDB returns the OpenGraph database connection
func (b *BaseProvider) GetOpenGraphDB() *opengraph.Database {
	return b.ogDB
}

// GetContentDB returns the content database connection
func (b *BaseProvider) GetContentDB() *database.Database {
	return b.contentDB
}

// EnsureOutputDirectory creates the output directory if it doesn't exist
func (b *BaseProvider) EnsureOutputDirectory(outfile string) error {
	return filesystem.EnsureDirectoryExists(outfile)
}

// CleanupExpired removes expired OpenGraph cache entries
func (b *BaseProvider) CleanupExpired() error {
	if b.ogDB == nil {
		return nil
	}
	return b.ogDB.CleanupExpired()
}
