package opengraph

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDatabaseCacheLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opengraph.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase() error = %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	now := time.Now().UTC().Truncate(time.Second)
	success := &Data{
		URL:         "https://example.com/article",
		Title:       "Example",
		Description: "Example description",
		Image:       "https://example.com/image.jpg",
		SiteName:    "Example",
		FetchedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	if err := db.SaveCachedData(success, true); err != nil {
		t.Fatalf("SaveCachedData(success) error = %v", err)
	}

	cached, err := db.GetCachedData(success.URL)
	if err != nil {
		t.Fatalf("GetCachedData(success) error = %v", err)
	}
	if cached == nil || cached.Title != success.Title || cached.Image != success.Image {
		t.Fatalf("GetCachedData(success) = %#v", cached)
	}

	failed := &Data{
		URL:       "https://example.com/failed",
		FetchedAt: now,
		ExpiresAt: now.Add(1 * time.Hour),
	}
	if err := db.SaveCachedData(failed, false); err != nil {
		t.Fatalf("SaveCachedData(failed) error = %v", err)
	}

	cached, err = db.GetCachedData(failed.URL)
	if err != nil {
		t.Fatalf("GetCachedData(failed) error = %v", err)
	}
	if cached != nil {
		t.Fatalf("GetCachedData(failed) = %#v, want nil", cached)
	}

	hasFailure, err := db.HasRecentFailure(failed.URL)
	if err != nil {
		t.Fatalf("HasRecentFailure() error = %v", err)
	}
	if !hasFailure {
		t.Fatal("HasRecentFailure() = false, want true")
	}
}

func TestDatabaseCleanupExpiredAndStats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opengraph.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase() error = %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	now := time.Now().UTC().Truncate(time.Second)
	entries := []struct {
		data    *Data
		success bool
	}{
		{
			data: &Data{
				URL:       "https://example.com/live",
				Title:     "Live",
				FetchedAt: now,
				ExpiresAt: now.Add(24 * time.Hour),
			},
			success: true,
		},
		{
			data: &Data{
				URL:       "https://example.com/expired",
				Title:     "Expired",
				FetchedAt: now.Add(-48 * time.Hour),
				ExpiresAt: now.Add(-1 * time.Hour),
			},
			success: true,
		},
		{
			data: &Data{
				URL:       "https://example.com/failed",
				FetchedAt: now,
				ExpiresAt: now.Add(1 * time.Hour),
			},
			success: false,
		},
	}

	for _, entry := range entries {
		if err := db.SaveCachedData(entry.data, entry.success); err != nil {
			t.Fatalf("SaveCachedData(%s) error = %v", entry.data.URL, err)
		}
	}

	stats, err := db.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats["total_entries"] != 3 {
		t.Fatalf("total_entries = %v, want 3", stats["total_entries"])
	}
	if stats["successful_entries"] != 2 {
		t.Fatalf("successful_entries = %v, want 2", stats["successful_entries"])
	}
	if stats["expired_entries"] != 1 {
		t.Fatalf("expired_entries = %v, want 1", stats["expired_entries"])
	}

	if err := db.CleanupExpired(); err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}

	stats, err = db.GetStats()
	if err != nil {
		t.Fatalf("GetStats() after cleanup error = %v", err)
	}
	if stats["total_entries"] != 2 {
		t.Fatalf("total_entries after cleanup = %v, want 2", stats["total_entries"])
	}
	if stats["expired_entries"] != 0 {
		t.Fatalf("expired_entries after cleanup = %v, want 0", stats["expired_entries"])
	}
}

// TestSaveCachedDataUnderWriteContention covers the failure seen in production:
// the generate command runs its providers concurrently and every provider opens
// its own handle on one shared opengraph.db, so a save can land while another
// handle holds the write lock. The per-connection busy_timeout must make the save
// wait instead of returning "database is locked (5) (SQLITE_BUSY)".
func TestSaveCachedDataUnderWriteContention(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opengraph.db")

	writer, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase(writer) error = %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	blocker, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase(blocker) error = %v", err)
	}
	t.Cleanup(func() { _ = blocker.Close() })

	// Pin the writer's first connection so the save below runs on a connection the
	// pool creates afterwards. Only that first connection would be configured if
	// the pragmas were applied as statements instead of through the DSN.
	pinned, err := writer.db.Conn(context.Background())
	if err != nil {
		t.Fatalf("Conn() error = %v", err)
	}
	t.Cleanup(func() { _ = pinned.Close() })

	tx, err := blocker.db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if _, err := tx.Exec(
		"INSERT OR REPLACE INTO opengraph_cache (url, expires_at) VALUES (?, CURRENT_TIMESTAMP)",
		"https://example.com/blocker",
	); err != nil {
		t.Fatalf("blocker insert: %v", err)
	}

	released := make(chan struct{})
	go func() {
		time.Sleep(250 * time.Millisecond)
		close(released)
		if err := tx.Commit(); err != nil {
			t.Errorf("blocker commit: %v", err)
		}
	}()

	now := time.Now().UTC()
	data := &Data{
		URL:       "https://example.com/contended",
		Title:     "Contended",
		FetchedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}
	if err := writer.SaveCachedData(data, true); err != nil {
		t.Fatalf("SaveCachedData() failed instead of waiting for the write lock: %v", err)
	}

	select {
	case <-released:
	default:
		t.Error("SaveCachedData() returned before the write lock was released")
	}
}

func TestNewDatabase_DefaultPathAndMissingCache(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	db, err := NewDatabase("")
	if err != nil {
		t.Fatalf("NewDatabase(\"\") error = %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	if db.dbPath != DefaultDBFile {
		t.Fatalf("dbPath = %q, want %q", db.dbPath, DefaultDBFile)
	}
}
