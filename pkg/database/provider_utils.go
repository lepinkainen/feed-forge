package database

import (
	"log/slog"

	"github.com/lepinkainen/feed-forge/pkg/opengraph"
)

// ProviderDatabases holds the database connections for a provider
type ProviderDatabases struct {
	ContentDB   *Database
	OpenGraphDB *opengraph.Database
}

// Close closes all database connections
func (pd *ProviderDatabases) Close() error {
	var lastErr error

	if pd.ContentDB != nil {
		if err := pd.ContentDB.Close(); err != nil {
			lastErr = err
		}
	}

	if pd.OpenGraphDB != nil {
		if err := pd.OpenGraphDB.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// InitializeProviderDatabases sets up databases for a provider
func InitializeProviderDatabases(contentDBName string, useContentDB bool) (*ProviderDatabases, error) {
	pd := &ProviderDatabases{}

	// Initialize OpenGraph database (all providers use this)
	ogDBPath, err := GetDefaultPath("opengraph.db")
	if err != nil {
		return nil, err
	}

	pd.OpenGraphDB, err = opengraph.NewDatabase(ogDBPath)
	if err != nil {
		return nil, err
	}

	// Clean up expired OpenGraph cache entries
	if err := pd.OpenGraphDB.CleanupExpiredEntries(); err != nil {
		slog.Warn("Failed to cleanup expired OpenGraph cache", "error", err)
	}

	// Initialize content database if needed
	if useContentDB && contentDBName != "" {
		contentDBPath, err := GetDefaultPath(contentDBName)
		if err != nil {
			pd.OpenGraphDB.Close()
			return nil, err
		}

		pd.ContentDB, err = NewDatabase(Config{Path: contentDBPath})
		if err != nil {
			pd.OpenGraphDB.Close()
			return nil, err
		}
	}

	return pd, nil
}
