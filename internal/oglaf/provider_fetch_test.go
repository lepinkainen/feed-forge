package oglaf

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/database"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// newProviderWithDB returns a *Provider with an in-memory content DB and the
// oglaf schema already initialised.
func newProviderWithDB(t *testing.T, feedURL string) *Provider {
	t.Helper()
	db := newTestDB(t)
	return &Provider{
		BaseProvider: &providers.BaseProvider{ContentDB: db},
		FeedURL:      feedURL,
	}
}

func TestItemFeedItemAccessors(t *testing.T) {
	published := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	item := &Item{RSSItem: &RSSItem{
		GUID:        "guid-1",
		Link:        "https://www.oglaf.com/episode/",
		Title:       "Episode Title",
		Description: "body",
		PublishedAt: published,
		ImageURL:    "https://media.oglaf.com/img.jpg",
	}}

	if got := item.Title(); got != "Episode Title" {
		t.Errorf("Title() = %q", got)
	}
	if got := item.Link(); got != "https://www.oglaf.com/episode/" {
		t.Errorf("Link() = %q", got)
	}
	if got := item.CommentsLink(); got != item.Link() {
		t.Errorf("CommentsLink() = %q, want Link()", got)
	}
	if got := item.Author(); got != "Oglaf" {
		t.Errorf("Author() = %q", got)
	}
	if got := item.Score(); got != 0 {
		t.Errorf("Score() = %d", got)
	}
	if got := item.CommentCount(); got != 0 {
		t.Errorf("CommentCount() = %d", got)
	}
	if !item.CreatedAt().Equal(published) {
		t.Errorf("CreatedAt() = %v, want %v", item.CreatedAt(), published)
	}
	cats := item.Categories()
	if len(cats) != 2 || cats[0] != "comics" || cats[1] != "oglaf" {
		t.Errorf("Categories() = %v", cats)
	}
	if got := item.ImageURL(); got != "https://media.oglaf.com/img.jpg" {
		t.Errorf("ImageURL() = %q", got)
	}
	if got := item.Content(); got != "body" {
		t.Errorf("Content() = %q", got)
	}
}

func TestExtractFullComicURLStripMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><img id="strip" src="https://media.oglaf.com/comic/page.jpg"></body></html>`))
	}))
	t.Cleanup(srv.Close)

	got, err := extractFullComicURL(srv.URL)
	if err != nil {
		t.Fatalf("extractFullComicURL: %v", err)
	}
	if got != "https://media.oglaf.com/comic/page.jpg" {
		t.Errorf("got %q", got)
	}
}

func TestExtractFullComicURLProtocolRelative(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><img id="strip" src="//media.oglaf.com/c/p.jpg"></html>`))
	}))
	t.Cleanup(srv.Close)

	got, err := extractFullComicURL(srv.URL)
	if err != nil {
		t.Fatalf("extractFullComicURL: %v", err)
	}
	if got != "https://media.oglaf.com/c/p.jpg" {
		t.Errorf("got %q, want https-prefixed", got)
	}
}

func TestExtractFullComicURLRootRelative(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><img id="strip" src="/comic/abs.jpg"></html>`))
	}))
	t.Cleanup(srv.Close)

	got, err := extractFullComicURL(srv.URL)
	if err != nil {
		t.Fatalf("extractFullComicURL: %v", err)
	}
	if got != "https://media.oglaf.com/comic/abs.jpg" {
		t.Errorf("got %q", got)
	}
}

func TestExtractFullComicURLMediaFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No id="strip" — the fallback mediaRegex should pick this up.
		_, _ = w.Write([]byte(`<html><img src="//media.oglaf.com/comic/fallback.jpg"></html>`))
	}))
	t.Cleanup(srv.Close)

	got, err := extractFullComicURL(srv.URL)
	if err != nil {
		t.Fatalf("extractFullComicURL: %v", err)
	}
	if got != "https://media.oglaf.com/comic/fallback.jpg" {
		t.Errorf("got %q", got)
	}
}

func TestExtractFullComicURLNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>no comic here</body></html>`))
	}))
	t.Cleanup(srv.Close)

	if _, err := extractFullComicURL(srv.URL); err == nil {
		t.Fatal("extractFullComicURL() err = nil, want missing-image error")
	}
}

func TestExtractFullComicURLRequestError(t *testing.T) {
	if _, err := extractFullComicURL("http://127.0.0.1:0/bad"); err == nil {
		t.Fatal("expected error on bad URL")
	}
}

// oglafRSSFixture returns an RSS body with three items whose links point at
// the given base URL path. Plain-text titles/descriptions match the regex
// parser's preferred path (the CDATA alternative in the existing regex does
// not match real Oglaf output).
func oglafRSSFixture(base string) string {
	return `<?xml version="1.0"?>
<rss version="2.0"><channel>
<title>Oglaf</title>
<link>` + base + `</link>
<description>comics</description>
<item>
<title>First</title>
<link>` + base + `/first/</link>
<description>body one</description>
<pubDate>Sun, 12 Apr 2026 00:00:00 +0000</pubDate>
<guid>` + base + `/first/</guid>
</item>
<item>
<title>Second</title>
<link>` + base + `/second/</link>
<description>body two</description>
<pubDate>Sun, 05 Apr 2026 00:00:00 +0000</pubDate>
<guid>` + base + `/second/</guid>
</item>
<item>
<title>Missing Date</title>
<link>` + base + `/nodate/</link>
<description>no date</description>
<guid>` + base + `/nodate/</guid>
</item>
</channel></rss>`
}

func TestFetchRSSFeedParsesValidItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(oglafRSSFixture("https://example.test")))
	}))
	t.Cleanup(srv.Close)

	p := newProviderWithDB(t, srv.URL)
	items, err := p.fetchRSSFeed()
	if err != nil {
		t.Fatalf("fetchRSSFeed: %v", err)
	}
	// The item without pubDate should be skipped, leaving 2.
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (skipping pubDate-less item)", len(items))
	}
	if items[0].Title != "First" || items[0].GUID != "https://example.test/first/" {
		t.Errorf("items[0] = %+v", items[0])
	}
	if items[0].PublishedAt.Year() != 2026 {
		t.Errorf("PublishedAt not parsed: %v", items[0].PublishedAt)
	}
}

func TestFetchRSSFeedHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	p := newProviderWithDB(t, srv.URL)
	if _, err := p.fetchRSSFeed(); err == nil {
		t.Fatal("fetchRSSFeed() err = nil, want non-nil on 500")
	}
}

func TestFetchRSSFeedIncrementalSavesNewItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(oglafRSSFixture("https://example.test")))
	}))
	t.Cleanup(srv.Close)

	p := newProviderWithDB(t, srv.URL)

	// First run: everything is new.
	newItems, err := p.fetchRSSFeedIncremental(p.ContentDB)
	if err != nil {
		t.Fatalf("fetchRSSFeedIncremental: %v", err)
	}
	if len(newItems) != 2 {
		t.Fatalf("first run: got %d new items, want 2", len(newItems))
	}

	// Second run against the same feed: nothing should be new (idempotent).
	newItems, err = p.fetchRSSFeedIncremental(p.ContentDB)
	if err != nil {
		t.Fatalf("fetchRSSFeedIncremental second: %v", err)
	}
	if len(newItems) != 0 {
		t.Errorf("second run: got %d new items, want 0 (already cached)", len(newItems))
	}
}

func TestFetchRSSFeedIncrementalPropagatesFetchError(t *testing.T) {
	p := newProviderWithDB(t, "http://127.0.0.1:0/nope")
	if _, err := p.fetchRSSFeedIncremental(p.ContentDB); err == nil {
		t.Fatal("fetchRSSFeedIncremental: err = nil, want fetch error")
	}
}

func TestProcessComicsIncrementalBackfillsAndReturnsProcessed(t *testing.T) {
	db := newTestDB(t)

	// Serve image pages for two comics; the third returns 500 to exercise the
	// markExtractionError branch.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/first":
			_, _ = w.Write([]byte(`<img id="strip" src="https://media.oglaf.com/comic/first.jpg">`))
		case "/second":
			_, _ = w.Write([]byte(`<img id="strip" src="https://media.oglaf.com/comic/second.jpg">`))
		case "/broken":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	// Seed three unprocessed comics pointing at our test server.
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	items := []*RSSItem{
		{GUID: "1", Link: srv.URL + "/first", Title: "First", Description: "d1", PublishedAt: base},
		{GUID: "2", Link: srv.URL + "/second", Title: "Second", Description: "d2", PublishedAt: base.Add(time.Hour)},
		{GUID: "3", Link: srv.URL + "/broken", Title: "Broken", Description: "d3", PublishedAt: base.Add(2 * time.Hour)},
	}
	for _, it := range items {
		if err := saveRSSItem(db, it); err != nil {
			t.Fatalf("saveRSSItem: %v", err)
		}
	}

	p := &Provider{BaseProvider: &providers.BaseProvider{ContentDB: db}, FeedURL: srv.URL}
	feedItems, err := p.processComicsIncremental(db)
	if err != nil {
		t.Fatalf("processComicsIncremental: %v", err)
	}

	// Two comics extracted successfully → 2 feed items returned.
	if len(feedItems) != 2 {
		t.Fatalf("got %d feed items, want 2 (broken one should be absent)", len(feedItems))
	}
	for _, fi := range feedItems {
		if !strings.Contains(fi.Content(), "<img") {
			t.Errorf("expected comicDescription HTML in Content(); got %q", fi.Content())
		}
	}
}

func TestFetchItemsEndToEnd(t *testing.T) {
	// Single server serves RSS at /feed, then per-comic pages at their links.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/feed":
			_, _ = w.Write([]byte(oglafRSSFixture(srv.URL)))
		case strings.HasSuffix(r.URL.Path, "/first/"):
			_, _ = w.Write([]byte(`<img id="strip" src="https://media.oglaf.com/comic/first.jpg">`))
		case strings.HasSuffix(r.URL.Path, "/second/"):
			_, _ = w.Write([]byte(`<img id="strip" src="https://media.oglaf.com/comic/second.jpg">`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	db, err := database.NewDatabase(database.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	p := &Provider{
		BaseProvider: &providers.BaseProvider{ContentDB: db},
		FeedURL:      srv.URL + "/feed",
	}

	items, err := p.FetchItems(10)
	if err != nil {
		t.Fatalf("FetchItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d feed items, want 2", len(items))
	}

	// Apply limit.
	limited, err := p.FetchItems(1)
	if err != nil {
		t.Fatalf("FetchItems(limit=1): %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("limit=1 returned %d items, want 1", len(limited))
	}
}

func TestMarkExtractionErrorUpdatesStatus(t *testing.T) {
	db := newTestDB(t)
	item := &RSSItem{
		GUID:        "g",
		Link:        "https://www.oglaf.com/ep/",
		Title:       "Ep",
		Description: "d",
		PublishedAt: time.Now().UTC(),
	}
	if err := saveRSSItem(db, item); err != nil {
		t.Fatalf("saveRSSItem: %v", err)
	}

	if err := markExtractionError(db, item.Link, "boom"); err != nil {
		t.Fatalf("markExtractionError: %v", err)
	}

	var msg string
	var extracted bool
	row := db.DB().QueryRow(`SELECT image_extracted, extraction_error FROM oglaf_comic_status WHERE link = ?`, item.Link)
	if err := row.Scan(&extracted, &msg); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if extracted {
		t.Errorf("image_extracted = true, want false after extraction error")
	}
	if msg != "boom" {
		t.Errorf("extraction_error = %q, want %q", msg, "boom")
	}
}

func TestCleanupOldDataRemovesAgedAndOrphanRows(t *testing.T) {
	db := newTestDB(t)

	// Insert an item with an ancient created_at so cleanup removes it.
	// saveRSSItem stamps created_at to time.Now() so we update it manually.
	item := &RSSItem{
		GUID:        "old",
		Link:        "https://www.oglaf.com/old/",
		Title:       "Old",
		Description: "",
		PublishedAt: time.Now().UTC(),
	}
	if err := saveRSSItem(db, item); err != nil {
		t.Fatalf("saveRSSItem: %v", err)
	}
	ancient := time.Now().AddDate(-2, 0, 0)
	if _, err := db.DB().Exec(`UPDATE oglaf_rss_items SET created_at = ? WHERE link = ?`, ancient, item.Link); err != nil {
		t.Fatalf("age row: %v", err)
	}

	// Insert an orphan comic_status row (no matching rss_items row).
	if _, err := db.DB().Exec(`INSERT INTO oglaf_comic_status (link, image_extracted) VALUES (?, FALSE)`, "https://orphan/"); err != nil {
		t.Fatalf("insert orphan: %v", err)
	}

	if err := cleanupOldData(db); err != nil {
		t.Fatalf("cleanupOldData: %v", err)
	}

	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM oglaf_rss_items WHERE link = ?`, item.Link).Scan(&count); err != nil {
		t.Fatalf("count rss: %v", err)
	}
	if count != 0 {
		t.Errorf("aged RSS item still present: %d rows", count)
	}

	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM oglaf_comic_status WHERE link = ?`, "https://orphan/").Scan(&count); err != nil {
		t.Fatalf("count orphan: %v", err)
	}
	if count != 0 {
		t.Errorf("orphan comic_status still present: %d rows", count)
	}
}
