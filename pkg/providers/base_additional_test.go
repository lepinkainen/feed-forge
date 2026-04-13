package providers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lepinkainen/feed-forge/pkg/filesystem"
)

func TestNewBaseProvider_WithoutContentDB(t *testing.T) {
	cacheDir := t.TempDir()
	filesystem.SetCacheDir(cacheDir)
	t.Cleanup(func() { filesystem.SetCacheDir("") })

	base, err := NewBaseProvider(DatabaseConfig{UseContentDB: false})
	if err != nil {
		t.Fatalf("NewBaseProvider() error = %v", err)
	}
	defer func() { _ = base.Close() }()

	if base.OgDB == nil {
		t.Fatal("OgDB = nil, want initialized")
	}
	if base.ContentDB != nil {
		t.Fatal("ContentDB != nil, want nil")
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "opengraph.db")); err != nil {
		t.Fatalf("opengraph db file missing: %v", err)
	}
}

func TestNewBaseProvider_WithContentDB(t *testing.T) {
	cacheDir := t.TempDir()
	filesystem.SetCacheDir(cacheDir)
	t.Cleanup(func() { filesystem.SetCacheDir("") })

	base, err := NewBaseProvider(DatabaseConfig{ContentDBName: "content.db", UseContentDB: true})
	if err != nil {
		t.Fatalf("NewBaseProvider() error = %v", err)
	}
	defer func() { _ = base.Close() }()

	if base.OgDB == nil || base.ContentDB == nil {
		t.Fatalf("databases not initialized: OgDB=%v ContentDB=%v", base.OgDB, base.ContentDB)
	}
	if got := base.ContentDB.Path(); got != filepath.Join(cacheDir, "content.db") {
		t.Fatalf("ContentDB.Path() = %q, want %q", got, filepath.Join(cacheDir, "content.db"))
	}
	for _, path := range []string{filepath.Join(cacheDir, "opengraph.db"), filepath.Join(cacheDir, "content.db")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected db file %q missing: %v", path, err)
		}
	}
}

func TestBaseProvider_CloseAndCleanupExpiredNilSafe(t *testing.T) {
	base := &BaseProvider{}
	if err := base.CleanupExpired(); err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}
	if err := base.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
