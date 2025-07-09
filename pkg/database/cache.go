package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// CacheEntry represents a generic cache entry
type CacheEntry struct {
	ID        int64
	Key       string
	Value     string
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Cache provides a generic caching layer on top of the database
type Cache struct {
	db        *Database
	tableName string
}

// NewCache creates a new cache instance
func NewCache(db *Database, tableName string) *Cache {
	return &Cache{
		db:        db,
		tableName: tableName,
	}
}

// InitializeCache creates the cache table if it doesn't exist
func (c *Cache) InitializeCache() error {
	schema := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE INDEX IF NOT EXISTS idx_%s_key ON %s(key);
		CREATE INDEX IF NOT EXISTS idx_%s_expires ON %s(expires_at);
	`, c.tableName, c.tableName, c.tableName, c.tableName, c.tableName)

	return c.db.ExecuteSchema(schema)
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) (string, bool, error) {
	query := fmt.Sprintf(`
		SELECT value FROM %s 
		WHERE key = ? AND expires_at > CURRENT_TIMESTAMP
	`, c.tableName)

	var value string
	err := c.db.DB().QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("failed to get cache value: %w", err)
	}

	return value, true, nil
}

// Set stores a value in the cache
func (c *Cache) Set(key, value string, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)

	query := fmt.Sprintf(`
		INSERT OR REPLACE INTO %s (key, value, expires_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, c.tableName)

	_, err := c.db.DB().Exec(query, key, value, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to set cache value: %w", err)
	}

	return nil
}

// Delete removes a value from the cache
func (c *Cache) Delete(key string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE key = ?`, c.tableName)

	_, err := c.db.DB().Exec(query, key)
	if err != nil {
		return fmt.Errorf("failed to delete cache value: %w", err)
	}

	return nil
}

// CleanupExpired removes expired entries from the cache
func (c *Cache) CleanupExpired() error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE expires_at < CURRENT_TIMESTAMP`, c.tableName)

	result, err := c.db.DB().Exec(query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired entries: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		slog.Debug("Cleaned up expired cache entries", "table", c.tableName, "count", rowsAffected)
	}

	return nil
}

// GetStats returns cache statistics
func (c *Cache) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total entries
	var totalEntries int64
	err := c.db.DB().QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, c.tableName)).Scan(&totalEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to get total entries: %w", err)
	}
	stats["total_entries"] = totalEntries

	// Valid entries (not expired)
	var validEntries int64
	err = c.db.DB().QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE expires_at > CURRENT_TIMESTAMP`, c.tableName)).Scan(&validEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to get valid entries: %w", err)
	}
	stats["valid_entries"] = validEntries

	// Expired entries
	var expiredEntries int64
	err = c.db.DB().QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE expires_at < CURRENT_TIMESTAMP`, c.tableName)).Scan(&expiredEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired entries: %w", err)
	}
	stats["expired_entries"] = expiredEntries

	return stats, nil
}

// Clear removes all entries from the cache
func (c *Cache) Clear() error {
	query := fmt.Sprintf(`DELETE FROM %s`, c.tableName)

	_, err := c.db.DB().Exec(query)
	if err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	return nil
}

// GetAll returns all valid entries from the cache
func (c *Cache) GetAll() ([]CacheEntry, error) {
	query := fmt.Sprintf(`
		SELECT id, key, value, expires_at, created_at, updated_at
		FROM %s 
		WHERE expires_at > CURRENT_TIMESTAMP
		ORDER BY updated_at DESC
	`, c.tableName)

	rows, err := c.db.DB().Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all cache entries: %w", err)
	}
	defer rows.Close()

	var entries []CacheEntry
	for rows.Next() {
		var entry CacheEntry
		err := rows.Scan(&entry.ID, &entry.Key, &entry.Value, &entry.ExpiresAt, &entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan cache entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
