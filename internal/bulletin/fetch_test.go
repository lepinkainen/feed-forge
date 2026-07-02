package bulletin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mmcdole/gofeed"
)

func TestFeedFallbackText(t *testing.T) {
	tests := []struct {
		name  string
		entry *gofeed.Item
		want  string
	}{
		{
			name:  "strips tags from content",
			entry: &gofeed.Item{Content: "<p>Hello <b>world</b></p>"},
			want:  "Hello  world",
		},
		{
			name:  "prefers content over description",
			entry: &gofeed.Item{Content: "from content", Description: "from description"},
			want:  "from content",
		},
		{
			name:  "falls back to description when content empty",
			entry: &gofeed.Item{Description: "<span>desc</span>"},
			want:  "desc",
		},
		{
			name:  "trims surrounding whitespace",
			entry: &gofeed.Item{Content: "   <div>text</div>   "},
			want:  "text",
		},
		{
			name:  "empty when both empty",
			entry: &gofeed.Item{},
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := feedFallbackText(tt.entry); got != tt.want {
				t.Errorf("feedFallbackText() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFetchEndToEnd drives Fetch against a local httptest server serving one RSS
// feed and its article page, exercising the fetch -> extract -> store path
// without any injection seam (Fetch is fully driven by cfg.Feeds + dbPath).
func TestFetchEndToEnd(t *testing.T) {
	const rssFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Test Feed</title><link>%[1]s</link>
<item><title>Story One</title><link>%[1]s/article</link><description>truncated feed summary</description></item>
</channel></rss>`

	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/feed", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, rssFeed, srv.URL)
	})
	mux.HandleFunc("/article", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(sampleArticleHTML))
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	dbPath := filepath.Join(t.TempDir(), "bulletin.db")
	cfg := Config{Feeds: []FeedSource{{URL: srv.URL + "/feed", Name: "Example News"}}}

	if err := Fetch(cfg, dbPath); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	items, err := store.UnpublishedItems(context.Background())
	if err != nil {
		t.Fatalf("UnpublishedItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 stored item, got %d", len(items))
	}
	it := items[0]
	if it.Title != "Story One" {
		t.Errorf("Title = %q, want Story One", it.Title)
	}
	if it.FeedName != "Example News" {
		t.Errorf("FeedName = %q, want Example News", it.FeedName)
	}
	// Full-text extraction must win over the truncated feed description.
	if !strings.Contains(it.RawText, "city council convened") {
		t.Errorf("RawText did not use extracted article body: %q", it.RawText)
	}
	if it.SimHash == 0 {
		t.Error("expected a non-zero SimHash for the stored item")
	}

	// A second run must skip the already-seen URL rather than re-inserting.
	if err := Fetch(cfg, dbPath); err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	items, err = store.UnpublishedItems(context.Background())
	if err != nil {
		t.Fatalf("UnpublishedItems after re-fetch: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("re-fetch should not add items; got %d", len(items))
	}
}
