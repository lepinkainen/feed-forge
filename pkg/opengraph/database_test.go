package opengraph

import (
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
