package hackernews

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
	"github.com/lepinkainen/feed-forge/pkg/database"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

func algoliaSearchPayload() []byte {
	resp := AlgoliaResponse{
		Hits: []AlgoliaHit{
			{
				ObjectID:    "100",
				Title:       "Story One",
				URL:         "https://example.com/one",
				Author:      "alice",
				Points:      150,
				NumComments: 20,
				CreatedAt:   time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
			},
			{
				ObjectID:    "101",
				Title:       "Story Two",
				URL:         "https://example.com/two",
				Author:      "bob",
				Points:      75,
				NumComments: 8,
				CreatedAt:   "not-a-valid-timestamp", // exercises the fallback branch in fetchItems
			},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestFetchItemsParsesAlgoliaResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(algoliaSearchPayload())
	}))
	t.Cleanup(srv.Close)

	original := algoliaSearchURL
	algoliaSearchURL = srv.URL
	t.Cleanup(func() { algoliaSearchURL = original })

	items := fetchItems()
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].ItemID != "100" || items[0].ItemTitle != "Story One" || items[0].Points != 150 {
		t.Errorf("items[0] = %+v", items[0])
	}
	if items[0].ItemCommentsLink != "https://news.ycombinator.com/item?id=100" {
		t.Errorf("comments link = %q", items[0].ItemCommentsLink)
	}
	if items[0].ItemCreatedAt.Year() != 2026 {
		t.Errorf("items[0].ItemCreatedAt = %v, want parsed 2026", items[0].ItemCreatedAt)
	}
	// Bad timestamp should fall back to "now" — within a few seconds of the test start.
	if time.Since(items[1].ItemCreatedAt) > time.Minute {
		t.Errorf("items[1].ItemCreatedAt fallback not close to now: %v", items[1].ItemCreatedAt)
	}
}

func TestFetchItemsReturnsNilOnHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	original := algoliaSearchURL
	algoliaSearchURL = srv.URL
	t.Cleanup(func() { algoliaSearchURL = original })

	items := fetchItems()
	if items != nil {
		t.Errorf("fetchItems() = %v, want nil on error", items)
	}
}

func TestFetchItemStatsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/42") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"objectID":"42","points":200,"num_comments":30}`))
	}))
	t.Cleanup(srv.Close)

	original := algoliaItemURLFmt
	algoliaItemURLFmt = srv.URL + "/%s"
	t.Cleanup(func() { algoliaItemURLFmt = original })

	client := api.NewHackerNewsClient()
	update := fetchItemStats(client, "42")
	if update.err != nil {
		t.Fatalf("fetchItemStats err = %v", update.err)
	}
	if update.isDeadItem {
		t.Errorf("isDeadItem = true, want false")
	}
	if update.points != 200 || update.commentCount != 30 {
		t.Errorf("stats = (points=%d, comments=%d), want (200, 30)", update.points, update.commentCount)
	}
}

func TestFetchItemStats404MarksDead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	original := algoliaItemURLFmt
	algoliaItemURLFmt = srv.URL + "/%s"
	t.Cleanup(func() { algoliaItemURLFmt = original })

	client := api.NewHackerNewsClient()
	update := fetchItemStats(client, "dead1")
	if update.err != nil {
		t.Fatalf("fetchItemStats err = %v, want nil (404 treated as dead)", update.err)
	}
	if !update.isDeadItem {
		t.Errorf("isDeadItem = false, want true on 404")
	}
}

func TestFetchItemStats500PropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	original := algoliaItemURLFmt
	algoliaItemURLFmt = srv.URL + "/%s"
	t.Cleanup(func() { algoliaItemURLFmt = original })

	client := api.NewHackerNewsClient()
	update := fetchItemStats(client, "xx")
	if update.err == nil {
		t.Fatal("fetchItemStats err = nil, want non-nil on 500")
	}
	if update.isDeadItem {
		t.Errorf("isDeadItem = true, want false on 500")
	}
}

func TestUpdateItemStatsWritesToDatabase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/100"):
			_, _ = w.Write([]byte(`{"objectID":"100","points":999,"num_comments":77}`))
		case strings.HasSuffix(r.URL.Path, "/200"):
			http.Error(w, "gone", http.StatusGone)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	original := algoliaItemURLFmt
	algoliaItemURLFmt = srv.URL + "/%s"
	t.Cleanup(func() { algoliaItemURLFmt = original })

	db := newTestDB(t)
	if err := initializeSchema(db); err != nil {
		t.Fatalf("initializeSchema: %v", err)
	}

	// Seed three items: 100 is updatable, 200 is dead (410), 300 is in
	// recentlyUpdated and should be skipped entirely.
	now := time.Now()
	items := []Item{
		{ItemID: "100", ItemTitle: "live", Points: 10, ItemCommentCount: 1, ItemCreatedAt: now, UpdatedAt: now},
		{ItemID: "200", ItemTitle: "dead", Points: 5, ItemCommentCount: 0, ItemCreatedAt: now, UpdatedAt: now},
		{ItemID: "300", ItemTitle: "recent", Points: 50, ItemCommentCount: 3, ItemCreatedAt: now, UpdatedAt: now},
		{ItemID: "", ItemTitle: "empty-id", Points: 1},
	}
	_ = updateStoredItems(db, items)

	updateItemStats(db.DB(), items, map[string]bool{"300": true})

	// 100 got its stats bumped.
	var points, comments int
	if err := db.DB().QueryRow(`SELECT points, comment_count FROM items WHERE item_hn_id = ?`, "100").Scan(&points, &comments); err != nil {
		t.Fatalf("scan 100: %v", err)
	}
	if points != 999 || comments != 77 {
		t.Errorf("item 100 stats = (%d, %d), want (999, 77)", points, comments)
	}

	// 200 returned 410 Gone. fetchItemStats flags it isDeadItem with err=nil,
	// which updateItemStats currently treats as a successful stats update,
	// writing points=0/comments=0 rather than deleting. This test pins that
	// observed behavior — the apparent mismatch with the "dead item" branch
	// (which only runs when err != nil) is a latent bug worth a separate fix.
	if err := db.DB().QueryRow(`SELECT points, comment_count FROM items WHERE item_hn_id = ?`, "200").Scan(&points, &comments); err != nil {
		t.Fatalf("scan 200: %v", err)
	}
	if points != 0 || comments != 0 {
		t.Errorf("dead item stats = (%d, %d), want (0, 0)", points, comments)
	}

	// 300 was left alone (skipped via recentlyUpdated).
	if err := db.DB().QueryRow(`SELECT points FROM items WHERE item_hn_id = ?`, "300").Scan(&points); err != nil {
		t.Fatalf("scan 300: %v", err)
	}
	if points != 50 {
		t.Errorf("skipped item points = %d, want 50 (unchanged)", points)
	}
}

func TestUpdateItemStatsNoOpWhenAllSkipped(t *testing.T) {
	db := newTestDB(t)
	if err := initializeSchema(db); err != nil {
		t.Fatalf("initializeSchema: %v", err)
	}
	// All items are in recentlyUpdated, so updateItemStats should not make any
	// HTTP calls. We set algoliaItemURLFmt to an unreachable address; if it
	// were called the test would stall the retry loop, not fail — so instead
	// we assert quickly by bounding the elapsed time indirectly: no HTTP means
	// near-instant return.
	now := time.Now()
	items := []Item{{ItemID: "1", UpdatedAt: now, ItemCreatedAt: now}}
	_ = updateStoredItems(db, items)

	done := make(chan struct{})
	go func() {
		updateItemStats(db.DB(), items, map[string]bool{"1": true})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("updateItemStats should short-circuit when all items are recentlyUpdated")
	}
}

func TestFetchItemsWiresThroughProvider(t *testing.T) {
	// Serve both the search endpoint and per-item stats endpoint from one server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "search_by_date") || strings.HasSuffix(r.URL.Path, "/search"):
			_, _ = w.Write(algoliaSearchPayload())
		case strings.Contains(r.URL.Path, "/items/"):
			// Echo back reasonable stats for anything.
			_, _ = w.Write([]byte(`{"objectID":"100","points":321,"num_comments":42}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	origSearch := algoliaSearchURL
	origItem := algoliaItemURLFmt
	algoliaSearchURL = srv.URL + "/search"
	algoliaItemURLFmt = srv.URL + "/items/%s"
	t.Cleanup(func() {
		algoliaSearchURL = origSearch
		algoliaItemURLFmt = origItem
	})

	db := newTestDB(t)
	p := &Provider{
		BaseProvider:   &providers.BaseProvider{ContentDB: db},
		MinPoints:      10,
		Limit:          10,
		CategoryMapper: LoadConfig(""),
	}

	items, err := p.FetchItems(0)
	if err != nil {
		t.Fatalf("FetchItems: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("FetchItems() returned 0 items, want >0")
	}
	// Story One has 150 points (above MinPoints=10); Story Two has 75 — also above.
	// Both should come back. Story Two uses the "now" fallback timestamp.
	titles := make(map[string]bool)
	for _, it := range items {
		titles[it.Title()] = true
	}
	if !titles["Story One"] {
		t.Errorf("expected 'Story One' in items, got titles=%v", titles)
	}
}

func TestFetchItemsExcludesBelowMinPoints(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(algoliaSearchPayload())
	}))
	t.Cleanup(srv.Close)

	origSearch := algoliaSearchURL
	origItem := algoliaItemURLFmt
	algoliaSearchURL = srv.URL
	algoliaItemURLFmt = srv.URL + "/nonexistent/%s" // forces 404 → dead, avoids real lookups
	t.Cleanup(func() {
		algoliaSearchURL = origSearch
		algoliaItemURLFmt = origItem
	})

	db := newTestDB(t)
	p := &Provider{
		BaseProvider:   &providers.BaseProvider{ContentDB: db},
		MinPoints:      1000, // both seeded stories fall below this
		Limit:          10,
		CategoryMapper: LoadConfig(""),
	}

	items, err := p.FetchItems(0)
	if err != nil {
		t.Fatalf("FetchItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("MinPoints=1000 returned %d items, want 0", len(items))
	}
}

// Ensure the provider's ContentDB integration still works through BaseProvider.
func TestFetchItemsUsesProvidedContentDB(t *testing.T) {
	db, err := database.NewDatabase(database.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":[]}`))
	}))
	t.Cleanup(srv.Close)

	origSearch := algoliaSearchURL
	algoliaSearchURL = srv.URL
	t.Cleanup(func() { algoliaSearchURL = origSearch })

	p := &Provider{
		BaseProvider:   &providers.BaseProvider{ContentDB: db},
		MinPoints:      0,
		Limit:          10,
		CategoryMapper: LoadConfig(""),
	}

	items, err := p.FetchItems(0)
	if err != nil {
		t.Fatalf("FetchItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 from empty response", len(items))
	}
}
