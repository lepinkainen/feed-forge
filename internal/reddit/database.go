package reddit

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	_ "modernc.org/sqlite" // SQLite driver
)

// OpenGraphDB wraps database operations with thread safety
type OpenGraphDB struct {
	db *sql.DB
	mu sync.RWMutex
}

// InitOpenGraphDB initializes the SQLite database for OpenGraph caching
func InitOpenGraphDB() (*OpenGraphDB, error) {
	db, err := sql.Open("sqlite", opengraph.DefaultDBFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	ogDB := &OpenGraphDB{db: db}

	// Create schema
	if err := ogDB.createSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	// Run migrations
	if err := ogDB.runMigrations(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("OpenGraph database initialized successfully")
	return ogDB, nil
}

// Close closes the database connection
func (ogDB *OpenGraphDB) Close() error {
	ogDB.mu.Lock()
	defer ogDB.mu.Unlock()

	if ogDB.db != nil {
		return ogDB.db.Close()
	}
	return nil
}

// createSchema creates the initial database schema
func (ogDB *OpenGraphDB) createSchema() error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS opengraph_cache (
		url TEXT PRIMARY KEY,
		title TEXT,
		description TEXT,
		image TEXT,
		site_name TEXT,
		fetched_at DATETIME,
		expires_at DATETIME,
		version INTEGER DEFAULT 1
	);
	
	CREATE INDEX IF NOT EXISTS idx_expires_at ON opengraph_cache(expires_at);
	CREATE INDEX IF NOT EXISTS idx_fetched_at ON opengraph_cache(fetched_at);
	`

	_, err := ogDB.db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// runMigrations runs database migrations
func (ogDB *OpenGraphDB) runMigrations() error {
	// Check if version column exists, if not add it
	var columnExists bool
	checkColumnSQL := `
	SELECT COUNT(*) FROM pragma_table_info('opengraph_cache') 
	WHERE name = 'version'
	`

	row := ogDB.db.QueryRow(checkColumnSQL)
	var count int
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("failed to check version column: %w", err)
	}

	columnExists = count > 0

	if !columnExists {
		alterTableSQL := `ALTER TABLE opengraph_cache ADD COLUMN version INTEGER DEFAULT 1`
		_, err := ogDB.db.Exec(alterTableSQL)
		if err != nil {
			return fmt.Errorf("failed to add version column: %w", err)
		}
		slog.Info("Added version column to opengraph_cache table")
	}

	return nil
}

// CleanupExpiredEntries removes expired OpenGraph entries from the database
func (ogDB *OpenGraphDB) CleanupExpiredEntries() error {
	ogDB.mu.Lock()
	defer ogDB.mu.Unlock()

	query := `DELETE FROM opengraph_cache WHERE expires_at <= datetime('now')`

	result, err := ogDB.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired entries: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		slog.Info("Cleaned up expired OpenGraph entries", "count", rowsAffected)
	}

	return nil
}

// GetCacheStats returns statistics about the OpenGraph cache
func (ogDB *OpenGraphDB) GetCacheStats() (*CacheStats, error) {
	ogDB.mu.RLock()
	defer ogDB.mu.RUnlock()

	stats := &CacheStats{}

	// Total entries
	row := ogDB.db.QueryRow(`SELECT COUNT(*) FROM opengraph_cache`)
	if err := row.Scan(&stats.TotalEntries); err != nil {
		return nil, fmt.Errorf("failed to get total entries: %w", err)
	}

	// Expired entries
	row = ogDB.db.QueryRow(`SELECT COUNT(*) FROM opengraph_cache WHERE expires_at <= datetime('now')`)
	if err := row.Scan(&stats.ExpiredEntries); err != nil {
		return nil, fmt.Errorf("failed to get expired entries: %w", err)
	}

	// Valid entries
	stats.ValidEntries = stats.TotalEntries - stats.ExpiredEntries

	// Oldest entry
	row = ogDB.db.QueryRow(`SELECT MIN(fetched_at) FROM opengraph_cache`)
	var oldestStr sql.NullString
	if err := row.Scan(&oldestStr); err != nil {
		return nil, fmt.Errorf("failed to get oldest entry: %w", err)
	}
	if oldestStr.Valid {
		oldest, err := time.Parse("2006-01-02 15:04:05", oldestStr.String)
		if err == nil {
			stats.OldestEntry = &oldest
		}
	}

	// Newest entry
	row = ogDB.db.QueryRow(`SELECT MAX(fetched_at) FROM opengraph_cache`)
	var newestStr sql.NullString
	if err := row.Scan(&newestStr); err != nil {
		return nil, fmt.Errorf("failed to get newest entry: %w", err)
	}
	if newestStr.Valid {
		newest, err := time.Parse("2006-01-02 15:04:05", newestStr.String)
		if err == nil {
			stats.NewestEntry = &newest
		}
	}

	return stats, nil
}

// CacheStats represents statistics about the OpenGraph cache
type CacheStats struct {
	TotalEntries   int64
	ValidEntries   int64
	ExpiredEntries int64
	OldestEntry    *time.Time
	NewestEntry    *time.Time
}

// VacuumDatabase performs database maintenance operations
func (ogDB *OpenGraphDB) VacuumDatabase() error {
	ogDB.mu.Lock()
	defer ogDB.mu.Unlock()

	slog.Info("Starting database vacuum operation")

	_, err := ogDB.db.Exec(`VACUUM`)
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	slog.Info("Database vacuum completed")
	return nil
}

// GetDatabaseSize returns the size of the database file
func (ogDB *OpenGraphDB) GetDatabaseSize() (int64, error) {
	ogDB.mu.RLock()
	defer ogDB.mu.RUnlock()

	var size int64
	row := ogDB.db.QueryRow(`SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`)
	err := row.Scan(&size)
	if err != nil {
		return 0, fmt.Errorf("failed to get database size: %w", err)
	}

	return size, nil
}
