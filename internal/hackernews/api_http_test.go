package hackernews

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}

func TestFetchItemsWithHTTPServer(t *testing.T) {
	oldSearchURL := algoliaSearchURL
	oldItemURLFmt := algoliaItemURLFmt
	defer func() {
		algoliaSearchURL = oldSearchURL
		algoliaItemURLFmt = oldItemURLFmt
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search_by_date" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("tags"); got != "front_page" {
			t.Fatalf("tags query = %q, want front_page", got)
		}
		if got := r.URL.Query().Get("hitsPerPage"); got != "100" {
			t.Fatalf("hitsPerPage query = %q, want 100", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"hits": [
				{
					"objectID": "123",
					"title": "Good story",
					"url": "https://example.com/good",
					"author": "alice",
					"points": 42,
					"num_comments": 7,
					"created_at": "2024-01-02T15:04:05Z"
				},
				{
					"objectID": "456",
					"title": "Bad timestamp story",
					"url": "https://example.com/bad-time",
					"author": "bob",
					"points": 12,
					"num_comments": 3,
					"created_at": "not-a-timestamp"
				}
			]
		}`))
	}))
	defer server.Close()

	algoliaSearchURL = server.URL + "/api/v1/search_by_date?tags=front_page&hitsPerPage=100"
	algoliaItemURLFmt = server.URL + "/api/v1/items/%s"

	before := time.Now()
	items := fetchItems()
	after := time.Now()

	if len(items) != 2 {
		t.Fatalf("len(fetchItems()) = %d, want 2", len(items))
	}
	if items[0].ItemID != "123" || items[0].ItemTitle != "Good story" || items[0].ItemLink != "https://example.com/good" {
		t.Fatalf("first item = %#v", items[0])
	}
	if items[0].ItemCommentsLink != "https://news.ycombinator.com/item?id=123" {
		t.Fatalf("first comments link = %q", items[0].ItemCommentsLink)
	}
	wantCreatedAt := time.Date(2024, time.January, 2, 15, 4, 5, 0, time.UTC)
	if !items[0].ItemCreatedAt.Equal(wantCreatedAt) {
		t.Fatalf("first created_at = %v, want %v", items[0].ItemCreatedAt, wantCreatedAt)
	}
	if items[1].ItemID != "456" || items[1].ItemCommentsLink != "https://news.ycombinator.com/item?id=456" {
		t.Fatalf("second item = %#v", items[1])
	}
	if items[1].ItemCreatedAt.Before(before.Add(-time.Second)) || items[1].ItemCreatedAt.After(after.Add(time.Second)) {
		t.Fatalf("fallback created_at = %v, want near now between %v and %v", items[1].ItemCreatedAt, before, after)
	}
}

func TestFetchItemsWithLiveSnapshotFixture(t *testing.T) {
	oldSearchURL := algoliaSearchURL
	defer func() { algoliaSearchURL = oldSearchURL }()

	fixture := mustReadFixture(t, "frontpage_live_snapshot.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	algoliaSearchURL = server.URL
	items := fetchItems()
	if len(items) < 20 {
		t.Fatalf("len(fetchItems()) = %d, want at least 20 items from live snapshot", len(items))
	}
	if items[0].ItemID == "" || items[0].ItemTitle == "" || items[0].ItemCommentsLink == "" {
		t.Fatalf("first parsed item missing fields: %#v", items[0])
	}
	if !strings.HasPrefix(items[0].ItemCommentsLink, "https://news.ycombinator.com/item?id=") {
		t.Fatalf("first comments link = %q, want HN item URL", items[0].ItemCommentsLink)
	}
	if items[0].ItemCreatedAt.IsZero() {
		t.Fatalf("first parsed item has zero ItemCreatedAt: %#v", items[0])
	}
}

func TestFetchItemsReturnsNilOnDecodeError(t *testing.T) {
	oldSearchURL := algoliaSearchURL
	defer func() { algoliaSearchURL = oldSearchURL }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":`))
	}))
	defer server.Close()

	algoliaSearchURL = server.URL
	if items := fetchItems(); items != nil {
		t.Fatalf("fetchItems() = %#v, want nil on malformed JSON", items)
	}
}

func TestFetchItemStatsWithLiveSnapshotFixture(t *testing.T) {
	oldItemURLFmt := algoliaItemURLFmt
	defer func() { algoliaItemURLFmt = oldItemURLFmt }()

	fixture := mustReadFixture(t, "item_live_snapshot.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	algoliaItemURLFmt = server.URL + "/%s"
	client := api.NewEnhancedClient(&api.EnhancedClientConfig{BaseClient: &http.Client{Timeout: 2 * time.Second}})

	stats := fetchItemStats(client, "ignored")
	if stats.err != nil || stats.isDeadItem {
		t.Fatalf("fetchItemStats(live fixture) = %#v", stats)
	}
	if stats.points <= 0 {
		t.Fatalf("fetchItemStats(live fixture) = %#v, want positive points", stats)
	}
}

func TestFetchItemStatsWithHTTPServer(t *testing.T) {
	oldItemURLFmt := algoliaItemURLFmt
	defer func() { algoliaItemURLFmt = oldItemURLFmt }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/items/live":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"points": 99, "num_comments": 12}`))
		case "/api/v1/items/dead404":
			http.NotFound(w, r)
		case "/api/v1/items/dead410":
			w.WriteHeader(http.StatusGone)
		case "/api/v1/items/badjson":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"points":`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	algoliaItemURLFmt = server.URL + "/api/v1/items/%s"
	client := api.NewEnhancedClient(&api.EnhancedClientConfig{BaseClient: &http.Client{Timeout: 2 * time.Second}})

	success := fetchItemStats(client, "live")
	if success.err != nil || success.isDeadItem || success.points != 99 || success.commentCount != 12 {
		t.Fatalf("fetchItemStats(live) = %#v", success)
	}

	dead404 := fetchItemStats(client, "dead404")
	if dead404.err != nil || !dead404.isDeadItem {
		t.Fatalf("fetchItemStats(dead404) = %#v, want dead item without error", dead404)
	}

	dead410 := fetchItemStats(client, "dead410")
	if dead410.err != nil || !dead410.isDeadItem {
		t.Fatalf("fetchItemStats(dead410) = %#v, want dead item without error", dead410)
	}

	badJSON := fetchItemStats(client, "badjson")
	if badJSON.err == nil || badJSON.isDeadItem {
		t.Fatalf("fetchItemStats(badjson) = %#v, want non-dead error", badJSON)
	}
	if !strings.Contains(badJSON.err.Error(), "failed to decode JSON") {
		t.Fatalf("fetchItemStats(badjson) error = %v, want decode context", badJSON.err)
	}
}
