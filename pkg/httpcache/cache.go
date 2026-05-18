// Package httpcache stores HTTP conditional request validators.
package httpcache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/lepinkainen/feed-forge/pkg/api"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	_ "modernc.org/sqlite" // SQLite driver
)

// ErrNotModified signals that upstream returned HTTP 304 Not Modified.
var ErrNotModified = errors.New("upstream not modified")

// CachedGet performs a conditional GET using validators from store.
func CachedGet(ctx context.Context, client *api.EnhancedClient, store *Store, url string, headers map[string]string) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("http cache get: client is nil")
	}

	var prev api.CacheValidators
	if store != nil {
		if validators, ok := store.GetContext(ctx, url); ok {
			prev = validators
		}
	}

	resp, err := client.GetConditional(ctx, url, prev, headers)
	if err != nil {
		return nil, err
	}
	if resp.NotModified {
		return nil, ErrNotModified
	}

	if store != nil {
		validators := api.CacheValidators{ETag: resp.ETag, LastModified: resp.LastModified}
		if saveErr := store.SaveContext(ctx, url, validators); saveErr != nil {
			slog.Warn("Failed to save HTTP validators", "url", url, "error", saveErr)
		}
	}

	return resp.Body, nil
}

// Store persists HTTP validators by URL.
type Store struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewStore opens or creates a validator cache database.
func NewStore(dbPath string) (*Store, error) {
	if dbPath == "" {
		var err error
		dbPath, err = filesystem.GetDefaultPath("http_cache.db")
		if err != nil {
			return nil, err
		}
	}

	if err := filesystem.EnsureDirectoryExists(dbPath); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open http cache: %w", err)
	}

	if err := configureSQLite(db); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("Failed to close HTTP cache database", "error", closeErr)
		}
		return nil, err
	}

	store := &Store{db: db, dbPath: dbPath}
	if err := store.createSchema(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("Failed to close HTTP cache database", "error", closeErr)
		}
		return nil, fmt.Errorf("create http cache schema: %w", err)
	}

	return store, nil
}

func configureSQLite(db *sql.DB) error {
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		return fmt.Errorf("set busy timeout: %w", err)
	}

	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		return fmt.Errorf("read journal mode: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("set journal mode: %w", err)
		}
	}

	for _, pragma := range []string{
		"PRAGMA synchronous=NORMAL",
		"PRAGMA temp_store=memory",
		"PRAGMA mmap_size=268435456",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("set pragma %q: %w", pragma, err)
		}
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	return nil
}

func (s *Store) createSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS http_validators (
		url TEXT PRIMARY KEY,
		etag TEXT DEFAULT '',
		last_modified TEXT DEFAULT '',
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := s.db.ExecContext(context.Background(), schema)
	return err
}

// Close closes the backing database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// Get returns cached validators for URL.
func (s *Store) Get(url string) (api.CacheValidators, bool) {
	return s.GetContext(context.Background(), url)
}

// GetContext returns cached validators for URL.
func (s *Store) GetContext(ctx context.Context, url string) (api.CacheValidators, bool) {
	if s == nil || s.db == nil || url == "" {
		return api.CacheValidators{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var v api.CacheValidators
	err := s.db.QueryRowContext(ctx, `SELECT etag, last_modified FROM http_validators WHERE url = ?`, url).Scan(&v.ETag, &v.LastModified)
	if errors.Is(err, sql.ErrNoRows) {
		return api.CacheValidators{}, false
	}
	if err != nil {
		slog.Warn("Failed to read HTTP validators", "url", url, "error", err)
		return api.CacheValidators{}, false
	}

	if v.ETag == "" && v.LastModified == "" {
		return api.CacheValidators{}, false
	}
	return v, true
}

// Save stores validators for URL.
func (s *Store) Save(url string, v api.CacheValidators) error {
	return s.SaveContext(context.Background(), url, v)
}

// SaveContext stores validators for URL.
func (s *Store) SaveContext(ctx context.Context, url string, v api.CacheValidators) error {
	if s == nil || s.db == nil || url == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
	INSERT INTO http_validators (url, etag, last_modified, updated_at)
	VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(url) DO UPDATE SET
		etag = excluded.etag,
		last_modified = excluded.last_modified,
		updated_at = CURRENT_TIMESTAMP
	`, url, v.ETag, v.LastModified)
	if err != nil {
		return fmt.Errorf("save HTTP validators: %w", err)
	}
	return nil
}
