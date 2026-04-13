package database

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func resetDBCache(t *testing.T) {
	t.Helper()

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	for path, db := range dbCache {
		if db != nil && db.db != nil {
			_ = db.db.Close()
		}
		delete(dbCache, path)
	}
}

func newTestDatabase(t *testing.T) *Database {
	t.Helper()
	resetDBCache(t)
	t.Cleanup(func() { resetDBCache(t) })

	path := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDatabase(Config{Path: path})
	if err != nil {
		t.Fatalf("NewDatabase() error = %v", err)
	}
	return db
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Driver != "sqlite" {
		t.Fatalf("DefaultConfig().Driver = %q, want sqlite", cfg.Driver)
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("DefaultConfig().Timeout = %v, want %v", cfg.Timeout, 30*time.Second)
	}
}

func TestNewDatabaseCachesByPathAndCloseRemovesCache(t *testing.T) {
	resetDBCache(t)
	defer resetDBCache(t)

	path := filepath.Join(t.TempDir(), "cached.db")

	first, err := NewDatabase(Config{Path: path})
	if err != nil {
		t.Fatalf("first NewDatabase() error = %v", err)
	}

	second, err := NewDatabase(Config{Path: path})
	if err != nil {
		t.Fatalf("second NewDatabase() error = %v", err)
	}

	if first != second {
		t.Fatal("NewDatabase() did not return cached instance for same path")
	}

	if got := first.Path(); got != path {
		t.Fatalf("Path() = %q, want %q", got, path)
	}
	if first.DB() == nil {
		t.Fatal("DB() = nil, want non-nil")
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	third, err := NewDatabase(Config{Path: path})
	if err != nil {
		t.Fatalf("third NewDatabase() error = %v", err)
	}
	defer func() { _ = third.Close() }()

	if third == first {
		t.Fatal("NewDatabase() returned old instance after Close()")
	}
}

func TestExecuteSchemaAndTransactionCommit(t *testing.T) {
	db := newTestDatabase(t)

	if err := db.ExecuteSchema(`CREATE TABLE items (id INTEGER PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatalf("ExecuteSchema() error = %v", err)
	}

	if err := db.Transaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO items (value) VALUES ('committed')`)
		return err
	}); err != nil {
		t.Fatalf("Transaction(commit) error = %v", err)
	}

	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM items WHERE value = 'committed'`).Scan(&count); err != nil {
		t.Fatalf("QueryRow().Scan() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("committed row count = %d, want 1", count)
	}
}

func TestTransactionRollsBackOnError(t *testing.T) {
	db := newTestDatabase(t)

	if err := db.ExecuteSchema(`CREATE TABLE items (id INTEGER PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatalf("ExecuteSchema() error = %v", err)
	}

	wantErr := errors.New("boom")
	if err := db.Transaction(func(tx *sql.Tx) error {
		if _, execErr := tx.Exec(`INSERT INTO items (value) VALUES ('rolled-back')`); execErr != nil {
			return execErr
		}
		return wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("Transaction(rollback) error = %v, want %v", err, wantErr)
	}

	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM items WHERE value = 'rolled-back'`).Scan(&count); err != nil {
		t.Fatalf("QueryRow().Scan() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("rolled back row count = %d, want 0", count)
	}
}

func TestTransactionRollsBackOnPanic(t *testing.T) {
	db := newTestDatabase(t)

	if err := db.ExecuteSchema(`CREATE TABLE items (id INTEGER PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatalf("ExecuteSchema() error = %v", err)
	}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("Transaction() did not re-panic")
			}
			if r != "panic-value" {
				t.Fatalf("panic value = %v, want panic-value", r)
			}
		}()

		_ = db.Transaction(func(tx *sql.Tx) error {
			if _, err := tx.Exec(`INSERT INTO items (value) VALUES ('panic-rolled-back')`); err != nil {
				return err
			}
			panic("panic-value")
		})
	}()

	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM items WHERE value = 'panic-rolled-back'`).Scan(&count); err != nil {
		t.Fatalf("QueryRow().Scan() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("panic rolled back row count = %d, want 0", count)
	}
}

func TestCacheLifecycle(t *testing.T) {
	db := newTestDatabase(t)
	cache := NewCache(db, "cache_entries")

	if err := cache.InitializeCache(); err != nil {
		t.Fatalf("InitializeCache() error = %v", err)
	}

	if err := cache.Set("live", "value", time.Hour); err != nil {
		t.Fatalf("Set(live) error = %v", err)
	}
	if err := cache.Set("expired", "old", time.Hour); err != nil {
		t.Fatalf("Set(expired) error = %v", err)
	}
	if _, err := db.DB().Exec(`UPDATE cache_entries SET expires_at = datetime('now', '-1 hour') WHERE key = 'expired'`); err != nil {
		t.Fatalf("force expire row error = %v", err)
	}

	value, found, err := cache.Get("live")
	if err != nil {
		t.Fatalf("Get(live) error = %v", err)
	}
	if !found || value != "value" {
		t.Fatalf("Get(live) = (%q, %v), want (%q, true)", value, found, "value")
	}

	value, found, err = cache.Get("expired")
	if err != nil {
		t.Fatalf("Get(expired) error = %v", err)
	}
	if found || value != "" {
		t.Fatalf("Get(expired) = (%q, %v), want (\"\", false)", value, found)
	}

	value, found, err = cache.Get("missing")
	if err != nil {
		t.Fatalf("Get(missing) error = %v", err)
	}
	if found || value != "" {
		t.Fatalf("Get(missing) = (%q, %v), want (\"\", false)", value, found)
	}

	entries, err := cache.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Key != "live" {
		t.Fatalf("GetAll() = %#v, want one live entry", entries)
	}

	stats, err := cache.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats["total_entries"] != int64(2) {
		t.Fatalf("total_entries = %v, want 2", stats["total_entries"])
	}
	if stats["valid_entries"] != int64(1) {
		t.Fatalf("valid_entries = %v, want 1", stats["valid_entries"])
	}
	if stats["expired_entries"] != int64(1) {
		t.Fatalf("expired_entries = %v, want 1", stats["expired_entries"])
	}

	if err := cache.Delete("live"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, found, err := cache.Get("live"); err != nil {
		t.Fatalf("Get(after Delete) error = %v", err)
	} else if found {
		t.Fatal("Get(after Delete) found deleted key")
	}

	if err := cache.CleanupExpired(); err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}

	stats, err = cache.GetStats()
	if err != nil {
		t.Fatalf("GetStats() after cleanup error = %v", err)
	}
	if stats["total_entries"] != int64(0) {
		t.Fatalf("total_entries after cleanup = %v, want 0", stats["total_entries"])
	}

	if err := cache.Set("one", "1", time.Hour); err != nil {
		t.Fatalf("Set(one) error = %v", err)
	}
	if err := cache.Set("two", "2", time.Hour); err != nil {
		t.Fatalf("Set(two) error = %v", err)
	}
	if err := cache.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	entries, err = cache.GetAll()
	if err != nil {
		t.Fatalf("GetAll() after Clear error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("GetAll() after Clear len = %d, want 0", len(entries))
	}
}
