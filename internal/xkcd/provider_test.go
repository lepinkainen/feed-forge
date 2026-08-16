package xkcd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// TestFetchItemsAgainstFixture serves the recorded RSS from testdata over an
// httptest server and verifies image/caption extraction, newest-first order,
// and pubDate parsing.
func TestFetchItemsAgainstFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "rss.xml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	orig := FeedURL
	FeedURL = srv.URL
	t.Cleanup(func() { FeedURL = orig })

	items, err := fetchItems(nil)
	if err != nil {
		t.Fatalf("fetchItems() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.Title() != "Accretionary Arc" {
		t.Errorf("Title() = %q, want Accretionary Arc", first.Title())
	}
	if first.Link() != "https://xkcd.com/3285/" {
		t.Errorf("Link() = %q", first.Link())
	}
	if first.ImageURL() != "https://imgs.xkcd.com/comics/accretionary_arc.png" {
		t.Errorf("ImageURL() = %q", first.ImageURL())
	}
	if first.CreatedAt().IsZero() {
		t.Error("CreatedAt() should be parsed, not zero")
	}

	// The essential mouseover caption must appear as visible text in the body,
	// distinct from the alt attribute, wrapped in the comic-caption paragraph.
	caption := "The late Triassic rifting was caused by a dinosaur"
	if !strings.Contains(first.Content(), `<p class="comic-caption">`) {
		t.Errorf("Content() missing caption paragraph: %q", first.Content())
	}
	if !strings.Contains(first.Content(), caption) {
		t.Errorf("Content() missing caption text: %q", first.Content())
	}
}

func TestProviderRegistration(t *testing.T) {
	info, err := providers.DefaultRegistry.Get("xkcd")
	if err != nil {
		t.Fatalf("provider not registered: %v", err)
	}
	if info.Name != "xkcd" {
		t.Errorf("Name = %q, want xkcd", info.Name)
	}
	if info.Preview == nil || info.Preview.TemplateName != "xkcd-atom" {
		t.Errorf("Preview.TemplateName = %+v, want xkcd-atom", info.Preview)
	}
}

func TestParsePubDate(t *testing.T) {
	if _, err := parsePubDate("Fri, 14 Aug 2026 04:00:00 -0000"); err != nil {
		t.Errorf("parsePubDate() error = %v", err)
	}
	if _, err := parsePubDate(""); err == nil {
		t.Error("parsePubDate(\"\") error = nil, want error")
	}
}
