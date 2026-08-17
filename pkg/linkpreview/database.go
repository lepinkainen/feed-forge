// Package linkpreview fetches and caches link-preview data: full-text
// extraction via go-trafilatura, backfilled with OpenGraph metadata.
package linkpreview

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/lepinkainen/feed-forge/pkg/database"
	"github.com/lepinkainen/feed-forge/pkg/dbinterfaces"
)

// Database wraps database operations with thread safety
type Database struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// Ensure Database implements dbinterfaces
var _ dbinterfaces.Database = (*Database)(nil)
var _ dbinterfaces.StatsProvider = (*Database)(nil)
var _ dbinterfaces.CleanupProvider = (*Database)(nil)

// NewDatabase creates a new OpenGraph database instance
func NewDatabase(dbPath string) (*Database, error) {
	if dbPath == "" {
		dbPath = DefaultDBFile
	}

	db, err := database.OpenSQLite(database.SQLiteOptions{Path: dbPath})
	if err != nil {
		return nil, err
	}

	ogDB := &Database{
		db:     db,
		dbPath: dbPath,
	}
	if err := ogDB.createSchema(); err != nil {
		if closeErr := ogDB.Close(); closeErr != nil {
			slog.Error("Failed to close database", "error", closeErr)
		}
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	slog.Info("OpenGraph database initialized", "path", dbPath)
	return ogDB, nil
}

// createSchema creates the necessary tables
func (db *Database) createSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS linkpreview_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL UNIQUE,
		title TEXT DEFAULT '',
		description TEXT DEFAULT '',
		image TEXT DEFAULT '',
		site_name TEXT DEFAULT '',
		excerpt TEXT DEFAULT '',
		etag TEXT DEFAULT '',
		last_modified TEXT DEFAULT '',
		fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP NOT NULL,
		fetch_success BOOLEAN DEFAULT 0
	);
	
	CREATE INDEX IF NOT EXISTS idx_linkpreview_url ON linkpreview_cache(url);
	CREATE INDEX IF NOT EXISTS idx_linkpreview_expires ON linkpreview_cache(expires_at);
	`

	if _, err := db.db.Exec(schema); err != nil {
		return err
	}

	return nil
}

// Close closes the database connection
func (db *Database) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.db != nil {
		return db.db.Close()
	}
	return nil
}

// GetCachedData retrieves cached OpenGraph data for a URL
func (db *Database) GetCachedData(url string) (*Data, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
	SELECT url, title, description, image, site_name, excerpt, etag, last_modified, fetched_at, expires_at, fetch_success
	FROM linkpreview_cache
	WHERE url = ? AND expires_at > CURRENT_TIMESTAMP AND fetch_success = 1
	`

	var data Data
	var fetchSuccess bool

	err := db.db.QueryRow(query, url).Scan(
		&data.URL,
		&data.Title,
		&data.Description,
		&data.Image,
		&data.SiteName,
		&data.Excerpt,
		&data.ETag,
		&data.LastModified,
		&data.FetchedAt,
		&data.ExpiresAt,
		&fetchSuccess,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // No cached data found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query cached data: %w", err)
	}

	if !fetchSuccess {
		return nil, nil // Don't return failed fetches
	}

	return &data, nil
}

// SaveCachedData saves OpenGraph data to the cache
func (db *Database) SaveCachedData(data *Data, fetchSuccess bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
	INSERT OR REPLACE INTO linkpreview_cache
	(url, title, description, image, site_name, excerpt, etag, last_modified, fetched_at, expires_at, fetch_success)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.db.Exec(query,
		data.URL,
		data.Title,
		data.Description,
		data.Image,
		data.SiteName,
		data.Excerpt,
		data.ETag,
		data.LastModified,
		data.FetchedAt,
		data.ExpiresAt,
		fetchSuccess,
	)

	if err != nil {
		return fmt.Errorf("failed to save cached data: %w", err)
	}

	return nil
}

// GetExpiredValidators returns validators for an expired successful cached row.
func (db *Database) GetExpiredValidators(url string) (etag, lastModified string, found bool) {
	data, err := db.GetExpiredData(url)
	if err != nil || data == nil {
		return "", "", false
	}
	return data.ETag, data.LastModified, data.ETag != "" || data.LastModified != ""
}

// GetExpiredData retrieves expired successful cached OpenGraph data for conditional refresh.
func (db *Database) GetExpiredData(url string) (*Data, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
	SELECT url, title, description, image, site_name, excerpt, etag, last_modified, fetched_at, expires_at
	FROM linkpreview_cache
	WHERE url = ? AND expires_at <= CURRENT_TIMESTAMP AND fetch_success = 1
	`

	var data Data
	err := db.db.QueryRow(query, url).Scan(
		&data.URL,
		&data.Title,
		&data.Description,
		&data.Image,
		&data.SiteName,
		&data.Excerpt,
		&data.ETag,
		&data.LastModified,
		&data.FetchedAt,
		&data.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query expired cached data: %w", err)
	}
	return &data, nil
}

// CleanupExpired removes expired cache entries without reusable validators.
func (db *Database) CleanupExpired() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `DELETE FROM linkpreview_cache WHERE expires_at < CURRENT_TIMESTAMP AND etag = '' AND last_modified = ''`
	result, err := db.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired entries: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		slog.Debug("Cleaned up expired OpenGraph cache entries", "count", rowsAffected)
	}

	return nil
}

// GetStats returns statistics about the cache
func (db *Database) GetStats() (map[string]any, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := make(map[string]any)

	// Total entries
	var totalEntries int
	err := db.db.QueryRow("SELECT COUNT(*) FROM linkpreview_cache").Scan(&totalEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to get total entries: %w", err)
	}
	stats["total_entries"] = totalEntries

	// Successful entries
	var successfulEntries int
	err = db.db.QueryRow("SELECT COUNT(*) FROM linkpreview_cache WHERE fetch_success = 1").Scan(&successfulEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to get successful entries: %w", err)
	}
	stats["successful_entries"] = successfulEntries

	// Expired entries
	var expiredEntries int
	err = db.db.QueryRow("SELECT COUNT(*) FROM linkpreview_cache WHERE expires_at < CURRENT_TIMESTAMP").Scan(&expiredEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired entries: %w", err)
	}
	stats["expired_entries"] = expiredEntries

	return stats, nil
}

// HasRecentFailure checks if there was a recent failed fetch attempt
func (db *Database) HasRecentFailure(url string) (bool, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
	SELECT COUNT(*) FROM linkpreview_cache 
	WHERE url = ? AND fetch_success = 0 AND fetched_at > datetime('now', '-1 hour')
	`

	var count int
	err := db.db.QueryRow(query, url).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check recent failure: %w", err)
	}

	return count > 0, nil
}
