package opengraph

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"

	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/interfaces"
	_ "modernc.org/sqlite"
)

// Database wraps database operations with thread safety
type Database struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// Ensure Database implements interfaces
var _ interfaces.Database = (*Database)(nil)
var _ interfaces.StatsProvider = (*Database)(nil)
var _ interfaces.CleanupProvider = (*Database)(nil)

// NewDatabase creates a new OpenGraph database instance
func NewDatabase(dbPath string) (*Database, error) {
	if dbPath == "" {
		// Default to current directory
		dbPath = DefaultDBFile
	}

	// Ensure directory exists
	if err := filesystem.EnsureDirectoryExists(dbPath); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure SQLite for better concurrency and performance
	pragmas := []string{
		"PRAGMA journal_mode=WAL",    // Enable WAL mode for concurrent readers/writers
		"PRAGMA busy_timeout=5000",   // 5 second timeout for lock contention
		"PRAGMA synchronous=NORMAL",  // Balance between performance and safety
		"PRAGMA temp_store=memory",   // Store temp tables in memory
		"PRAGMA mmap_size=268435456", // 256MB memory mapped I/O
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma %q: %w", pragma, err)
		}
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	ogDB := &Database{
		db:     db,
		dbPath: dbPath,
	}

	// Create schema
	if err := ogDB.createSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	slog.Info("OpenGraph database initialized", "path", dbPath)
	return ogDB, nil
}

// createSchema creates the necessary tables
func (db *Database) createSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS opengraph_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL UNIQUE,
		title TEXT DEFAULT '',
		description TEXT DEFAULT '',
		image TEXT DEFAULT '',
		site_name TEXT DEFAULT '',
		fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP NOT NULL,
		fetch_success BOOLEAN DEFAULT 0
	);
	
	CREATE INDEX IF NOT EXISTS idx_opengraph_url ON opengraph_cache(url);
	CREATE INDEX IF NOT EXISTS idx_opengraph_expires ON opengraph_cache(expires_at);
	`

	_, err := db.db.Exec(schema)
	return err
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
	SELECT url, title, description, image, site_name, fetched_at, expires_at, fetch_success
	FROM opengraph_cache 
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
		&data.FetchedAt,
		&data.ExpiresAt,
		&fetchSuccess,
	)

	if err == sql.ErrNoRows {
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
	INSERT OR REPLACE INTO opengraph_cache 
	(url, title, description, image, site_name, fetched_at, expires_at, fetch_success)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.db.Exec(query,
		data.URL,
		data.Title,
		data.Description,
		data.Image,
		data.SiteName,
		data.FetchedAt,
		data.ExpiresAt,
		fetchSuccess,
	)

	if err != nil {
		return fmt.Errorf("failed to save cached data: %w", err)
	}

	return nil
}

// CleanupExpired removes expired cache entries
func (db *Database) CleanupExpired() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `DELETE FROM opengraph_cache WHERE expires_at < CURRENT_TIMESTAMP`
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
func (db *Database) GetStats() (map[string]interface{}, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := make(map[string]interface{})

	// Total entries
	var totalEntries int
	err := db.db.QueryRow("SELECT COUNT(*) FROM opengraph_cache").Scan(&totalEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to get total entries: %w", err)
	}
	stats["total_entries"] = totalEntries

	// Successful entries
	var successfulEntries int
	err = db.db.QueryRow("SELECT COUNT(*) FROM opengraph_cache WHERE fetch_success = 1").Scan(&successfulEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to get successful entries: %w", err)
	}
	stats["successful_entries"] = successfulEntries

	// Expired entries
	var expiredEntries int
	err = db.db.QueryRow("SELECT COUNT(*) FROM opengraph_cache WHERE expires_at < CURRENT_TIMESTAMP").Scan(&expiredEntries)
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
	SELECT COUNT(*) FROM opengraph_cache 
	WHERE url = ? AND fetch_success = 0 AND fetched_at > datetime('now', '-1 hour')
	`

	var count int
	err := db.db.QueryRow(query, url).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check recent failure: %w", err)
	}

	return count > 0, nil
}
