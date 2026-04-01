package filesystem

import (
	"path/filepath"
	"testing"
)

func TestGetDefaultPath_UsesConfiguredCacheDir(t *testing.T) {
	oldCacheDir := cacheDir
	t.Cleanup(func() { cacheDir = oldCacheDir })

	SetCacheDir("/tmp/feed-forge-cache")

	got, err := GetDefaultPath("reddit.db")
	if err != nil {
		t.Fatalf("GetDefaultPath() error = %v", err)
	}

	want := filepath.Join("/tmp/feed-forge-cache", "reddit.db")
	if got != want {
		t.Fatalf("GetDefaultPath() = %q, want %q", got, want)
	}
}

func TestGetDefaultPath_UsesXDGCacheHome(t *testing.T) {
	oldCacheDir := cacheDir
	t.Cleanup(func() { cacheDir = oldCacheDir })
	cacheDir = ""

	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-cache")

	got, err := GetDefaultPath("hackernews.db")
	if err != nil {
		t.Fatalf("GetDefaultPath() error = %v", err)
	}

	want := filepath.Join("/tmp/xdg-cache", "feed-forge", "hackernews.db")
	if got != want {
		t.Fatalf("GetDefaultPath() = %q, want %q", got, want)
	}
}

func TestGetDefaultPath_FallsBackToHomeCache(t *testing.T) {
	oldCacheDir := cacheDir
	t.Cleanup(func() { cacheDir = oldCacheDir })
	cacheDir = ""

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CACHE_HOME", "")

	got, err := GetDefaultPath("og.db")
	if err != nil {
		t.Fatalf("GetDefaultPath() error = %v", err)
	}

	want := filepath.Join(homeDir, ".cache", "feed-forge", "og.db")
	if got != want {
		t.Fatalf("GetDefaultPath() = %q, want %q", got, want)
	}
}
