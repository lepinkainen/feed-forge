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
	"time"

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

// CachedGetWithStale performs a conditional GET that also persists the response
// body, so a previously fetched copy can be served when the upstream later fails
// (for example an intermittent HTTP 404). On a 200 the fresh body is cached and
// returned with stale=false. On 304 the cached body is returned with stale=false
// and its timestamp refreshed (a 304 proves the cached copy is still current).
// If the request fails but a cached body exists that is no older than maxStale,
// that body is returned with stale=true alongside the original error, letting
// callers degrade gracefully; maxStale <= 0 means no age limit. With no cached
// body, or one older than maxStale, an error is returned.
func CachedGetWithStale(ctx context.Context, client *api.EnhancedClient, store *Store, url string, headers map[string]string, maxStale time.Duration) (body []byte, stale bool, err error) {
	if client == nil {
		return nil, false, fmt.Errorf("http cache get: client is nil")
	}

	var prev api.CacheValidators
	if store != nil {
		if validators, ok := store.GetContext(ctx, url); ok {
			prev = validators
		}
	}

	resp, getErr := client.GetConditional(ctx, url, prev, headers)
	if getErr != nil {
		if store != nil {
			if cached, fetchedAt, ok := store.GetBodyContext(ctx, url); ok {
				age := time.Since(fetchedAt)
				if maxStale > 0 && age > maxStale {
					return nil, false, fmt.Errorf("upstream failing and cached copy is %s old (max %s): %w", age.Round(time.Minute), maxStale, getErr)
				}
				return cached, true, getErr
			}
		}
		return nil, false, getErr
	}

	if resp.NotModified {
		if store != nil {
			if cached, _, ok := store.GetBodyContext(ctx, url); ok {
				if touchErr := store.TouchContext(ctx, url); touchErr != nil {
					slog.Warn("Failed to refresh cached HTTP body timestamp", "url", url, "error", touchErr)
				}
				return cached, false, nil
			}
		}
		return nil, false, ErrNotModified
	}

	if store != nil {
		validators := api.CacheValidators{ETag: resp.ETag, LastModified: resp.LastModified}
		if saveErr := store.SaveBodyContext(ctx, url, validators, resp.Body); saveErr != nil {
			slog.Warn("Failed to save cached HTTP body", "url", url, "error", saveErr)
		}
	}

	return resp.Body, false, nil
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
		body BLOB,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := s.db.ExecContext(context.Background(), schema); err != nil {
		return err
	}
	return s.ensureBodyColumn()
}

// ensureBodyColumn adds the body column to databases created before stale-body
// caching existed. ALTER TABLE ADD COLUMN is a no-op-safe migration for existing rows.
func (s *Store) ensureBodyColumn() error {
	ctx := context.Background()
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info(http_validators)")
	if err != nil {
		return fmt.Errorf("inspect http cache schema: %w", err)
	}
	defer func() { _ = rows.Close() }()

	hasBody := false
	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notNull    int
			dflt       sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &primaryKey); err != nil {
			return fmt.Errorf("scan http cache schema: %w", err)
		}
		if name == "body" {
			hasBody = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read http cache schema: %w", err)
	}

	if !hasBody {
		if _, err := s.db.ExecContext(ctx, "ALTER TABLE http_validators ADD COLUMN body BLOB"); err != nil {
			return fmt.Errorf("add body column: %w", err)
		}
	}
	return nil
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

// GetBodyContext returns the cached response body for URL and the time it was
// last fetched or verified via 304, if one was stored.
func (s *Store) GetBodyContext(ctx context.Context, url string) ([]byte, time.Time, bool) {
	if s == nil || s.db == nil || url == "" {
		return nil, time.Time{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var body []byte
	var fetchedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT body, updated_at FROM http_validators WHERE url = ?`, url).Scan(&body, &fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, time.Time{}, false
	}
	if err != nil {
		slog.Warn("Failed to read cached HTTP body", "url", url, "error", err)
		return nil, time.Time{}, false
	}
	if len(body) == 0 {
		return nil, time.Time{}, false
	}
	return body, fetchedAt.Time, true
}

// TouchContext refreshes the updated_at timestamp for URL, marking the cached
// body as verified-current (used after an HTTP 304).
func (s *Store) TouchContext(ctx context.Context, url string) error {
	if s == nil || s.db == nil || url == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `UPDATE http_validators SET updated_at = ? WHERE url = ?`, time.Now().UTC(), url)
	if err != nil {
		return fmt.Errorf("touch HTTP cache entry: %w", err)
	}
	return nil
}

// SaveBodyContext stores validators and the response body for URL.
func (s *Store) SaveBodyContext(ctx context.Context, url string, v api.CacheValidators, body []byte) error {
	if s == nil || s.db == nil || url == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
	INSERT INTO http_validators (url, etag, last_modified, body, updated_at)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(url) DO UPDATE SET
		etag = excluded.etag,
		last_modified = excluded.last_modified,
		body = excluded.body,
		updated_at = excluded.updated_at
	`, url, v.ETag, v.LastModified, body, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("save HTTP body: %w", err)
	}
	return nil
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
