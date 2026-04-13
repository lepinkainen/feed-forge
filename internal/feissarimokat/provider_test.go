package feissarimokat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/providers"
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
	if !strings.Contains(err.Error(), "create feissarimokat provider") {
		t.Fatalf("factory() error = %q, want provider context", err)
	}
	if !strings.Contains(err.Error(), filepath.Base(cacheMarker.Name())) {
		t.Fatalf("factory() error = %q, want underlying cache path context", err)
	}
}

func TestItem_FeedItemInterface(t *testing.T) {
	item := &Item{
		RSSItem: RSSItem{
			ItemTitle:   "Test Post",
			Description: "Test description",
			ItemLink:    "https://www.feissarimokat.com/2024/01/test-post/",
		},
		Images:      []string{"https://static.feissarimokat.com/img/test.jpg"},
		ContentHTML: `Test description<img src="https://static.feissarimokat.com/img/test.jpg" alt="Test Post">`,
	}

	if item.Title() != "Test Post" {
		t.Errorf("Title() = %s, want Test Post", item.Title())
	}

	if item.Link() != "https://www.feissarimokat.com/2024/01/test-post/" {
		t.Errorf("Link() = %s, want https://www.feissarimokat.com/2024/01/test-post/", item.Link())
	}

	if item.CommentsLink() != item.Link() {
		t.Error("CommentsLink() should equal Link()")
	}

	if item.Author() != "Feissarimokat" {
		t.Errorf("Author() = %s, want Feissarimokat", item.Author())
	}

	if item.Score() != 0 {
		t.Errorf("Score() = %d, want 0", item.Score())
	}

	if item.CommentCount() != 0 {
		t.Errorf("CommentCount() = %d, want 0", item.CommentCount())
	}

	if item.CreatedAt().IsZero() {
		t.Error("CreatedAt() should not be zero")
	}

	if len(item.Categories()) != 2 {
		t.Errorf("Categories() length = %d, want 2", len(item.Categories()))
	}

	if item.ImageURL() != "https://static.feissarimokat.com/img/test.jpg" {
		t.Errorf("ImageURL() = %s, want https://static.feissarimokat.com/img/test.jpg", item.ImageURL())
	}

	if item.Content() == "" {
		t.Error("Content() should not be empty")
	}
}

func TestItem_EmptyImages(t *testing.T) {
	item := &Item{
		RSSItem: RSSItem{ItemTitle: "No Images"},
	}

	if item.ImageURL() != "" {
		t.Errorf("ImageURL() = %s, want empty string", item.ImageURL())
	}
}

func TestProviderRegistration(t *testing.T) {
	info, err := providers.DefaultRegistry.Get("feissarimokat")
	if err != nil {
		t.Fatalf("Provider not registered: %v", err)
	}

	if info.Name != "feissarimokat" {
		t.Errorf("Name = %s, want feissarimokat", info.Name)
	}

	// Test factory creates a valid provider
	cfg := &Config{}
	provider, err := info.Factory(cfg)
	if err != nil {
		t.Fatalf("Factory failed: %v", err)
	}
	if provider == nil {
		t.Fatal("Factory returned nil provider")
	}
}
