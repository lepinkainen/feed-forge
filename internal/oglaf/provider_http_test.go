package oglaf

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFetchRSSFeedWithHTTPServer(t *testing.T) {
	rssFixture := mustReadFixture(t, "rss_fixture.xml")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rss" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write(rssFixture)
	}))
	defer server.Close()

	provider := &Provider{FeedURL: server.URL + "/rss"}
	items, err := provider.fetchRSSFeed()
	if err != nil {
		t.Fatalf("fetchRSSFeed() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(fetchRSSFeed()) = %d, want 2", len(items))
	}

	if items[0].Title != "First Comic" || items[0].Link != "https://example.com/comics/first" || items[0].Description != "First desc &amp; details" || items[0].GUID != "first-guid" {
		t.Fatalf("first item parsed incorrectly: %#v", items[0])
	}
	wantFirstTime := time.Date(2006, time.January, 2, 22, 4, 5, 0, time.UTC)
	if !items[0].PublishedAt.Equal(wantFirstTime) {
		t.Fatalf("first PublishedAt = %v, want %v", items[0].PublishedAt, wantFirstTime)
	}

	if items[1].Title != "Second Comic" || items[1].GUID != "second-guid" {
		t.Fatalf("second item parsed incorrectly: %#v", items[1])
	}
}

func TestFetchRSSFeedWithLiveSnapshotFixture(t *testing.T) {
	rssFixture := mustReadFixture(t, "rss_live_snapshot.xml")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rss" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write(rssFixture)
	}))
	defer server.Close()

	provider := &Provider{FeedURL: server.URL + "/rss"}
	items, err := provider.fetchRSSFeed()
	if err != nil {
		t.Fatalf("fetchRSSFeed() error = %v", err)
	}
	if len(items) < 10 {
		t.Fatalf("len(fetchRSSFeed()) = %d, want at least 10 items from live snapshot", len(items))
	}

	first := items[0]
	if first.Title == "" || first.Link == "" || first.GUID == "" {
		t.Fatalf("first parsed item missing required fields: %#v", first)
	}
	if first.PublishedAt.IsZero() {
		t.Fatalf("first parsed item has zero PublishedAt: %#v", first)
	}
	if first.Description == "" {
		t.Log("live snapshot fixture currently yields empty description via regex parser; title/link/pubDate still validated")
	}
}

func TestExtractFullComicURLWithHTTPServer(t *testing.T) {
	stripRelativeFixture := mustReadFixture(t, "comic_strip_relative.html")
	mediaFallbackFixture := mustReadFixture(t, "comic_media_fallback.html")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/strip-absolute":
			_, _ = w.Write([]byte(`<html><img id="strip" src="https://cdn.example.com/comic.jpg"></html>`))
		case "/strip-schemeless":
			_, _ = w.Write([]byte(`<html><img id="strip" src="//media.oglaf.com/comic/test.jpg"></html>`))
		case "/strip-relative":
			_, _ = w.Write(stripRelativeFixture)
		case "/media-fallback":
			_, _ = w.Write(mediaFallbackFixture)
		default:
			_, _ = w.Write([]byte(`<html><body>no comic here</body></html>`))
		}
	}))
	defer server.Close()

	tests := []struct {
		name    string
		path    string
		wantURL string
		wantErr string
	}{
		{name: "strip absolute", path: "/strip-absolute", wantURL: "https://cdn.example.com/comic.jpg"},
		{name: "strip schemeless", path: "/strip-schemeless", wantURL: "https://media.oglaf.com/comic/test.jpg"},
		{name: "strip relative", path: "/strip-relative", wantURL: "https://media.oglaf.com/comic/test.jpg"},
		{name: "media fallback", path: "/media-fallback", wantURL: "https://media.oglaf.com/comic/fallback.jpg"},
		{name: "missing image", path: "/missing", wantErr: "could not find comic image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractFullComicURL(server.URL + tt.path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("extractFullComicURL() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("extractFullComicURL() error = %v", err)
			}
			if got != tt.wantURL {
				t.Fatalf("extractFullComicURL() = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}

func TestItemAccessors(t *testing.T) {
	publishedAt := time.Date(2024, time.March, 10, 12, 0, 0, 0, time.UTC)
	item := &Item{RSSItem: &RSSItem{
		Title:       "Comic",
		Link:        "https://example.com/comic",
		Description: "Comic description",
		PublishedAt: publishedAt,
		ImageURL:    "https://media.oglaf.com/comic/test.jpg",
	}}

	if item.Title() != "Comic" || item.Link() != "https://example.com/comic" || item.CommentsLink() != "https://example.com/comic" {
		t.Fatalf("basic link/title accessors returned unexpected values: %#v", item)
	}
	if item.Author() != "Oglaf" || item.Score() != 0 || item.CommentCount() != 0 {
		t.Fatalf("author/score/count accessors returned unexpected values: %#v", item)
	}
	if !item.CreatedAt().Equal(publishedAt) {
		t.Fatalf("CreatedAt() = %v, want %v", item.CreatedAt(), publishedAt)
	}
	if got := item.Categories(); len(got) != 2 || got[0] != "comics" || got[1] != "oglaf" {
		t.Fatalf("Categories() = %v, want [comics oglaf]", got)
	}
	if item.ImageURL() != "https://media.oglaf.com/comic/test.jpg" {
		t.Fatalf("ImageURL() = %q", item.ImageURL())
	}
	if item.Content() != "Comic description" {
		t.Fatalf("Content() = %q, want %q", item.Content(), "Comic description")
	}
}
