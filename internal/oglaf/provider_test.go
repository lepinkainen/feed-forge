package oglaf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lepinkainen/feed-forge/pkg/filesystem"
)

func TestFactoryPropagatesConstructorError(t *testing.T) {
	cacheMarker, err := os.CreateTemp(t.TempDir(), "cache-file")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if err := cacheMarker.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	filesystem.SetCacheDir(cacheMarker.Name())
	t.Cleanup(func() {
		filesystem.SetCacheDir("")
	})

	provider, err := factory(&Config{})
	if err == nil {
		t.Fatal("factory() error = nil, want propagated constructor error")
	}
	if provider != nil {
		t.Fatalf("factory() provider = %#v, want nil", provider)
	}
	if !strings.Contains(err.Error(), "create oglaf provider") {
		t.Fatalf("factory() error = %q, want provider context", err)
	}
	if !strings.Contains(err.Error(), filepath.Base(cacheMarker.Name())) {
		t.Fatalf("factory() error = %q, want underlying cache path context", err)
	}
}
