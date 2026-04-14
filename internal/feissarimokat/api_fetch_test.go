package feissarimokat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Feissarimokat</title>
    <link>https://www.feissarimokat.com/</link>
    <description>Comics</description>
    <item>
      <title>Post One</title>
      <description>Body one</description>
      <link>%s/post-one/</link>
    </item>
    <item>
      <title>Post Two</title>
      <description>Body two</description>
      <link>%s/post-two/</link>
    </item>
  </channel>
</rss>`

// postOneHTML references an absolute image URL and a relative one, exercising
// both branches of the URL normalisation in scrapeImages.
const postOneHTML = `<html><body>
<div class="postbody">
  <p>intro</p>
  <img src="https://static.feissarimokat.com/img/abs.jpg" alt="abs">
  <img src="/img/rel.jpg" alt="rel">
</div>
</body></html>`

const postTwoHTML = `<html><body>
<div class="navbar">nothing here</div>
<p>no postbody div</p>
</body></html>`

func TestFetchRSSFeedParsesItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>x</title><link>x</link><description>x</description>
<item><title>A</title><description>d1</description><link>https://example.test/a</link></item>
<item><title>B</title><description>d2</description><link>https://example.test/b</link></item>
</channel></rss>`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	originalFeedURL := FeedURL
	FeedURL = srv.URL
	t.Cleanup(func() { FeedURL = originalFeedURL })

	items, err := fetchRSSFeed()
	if err != nil {
		t.Fatalf("fetchRSSFeed() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].ItemTitle != "A" || items[0].ItemLink != "https://example.test/a" {
		t.Errorf("unexpected first item: %+v", items[0])
	}
}

func TestFetchRSSFeedHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	original := FeedURL
	FeedURL = srv.URL
	t.Cleanup(func() { FeedURL = original })

	if _, err := fetchRSSFeed(); err == nil {
		t.Fatal("fetchRSSFeed() error = nil, want non-nil on 500")
	}
}

func TestFetchRSSFeedBadXML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("not xml at all")); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	original := FeedURL
	FeedURL = srv.URL
	t.Cleanup(func() { FeedURL = original })

	if _, err := fetchRSSFeed(); err == nil {
		t.Fatal("fetchRSSFeed() error = nil, want parse error")
	}
}

func TestScrapeImagesExtractsAndResolves(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(postOneHTML)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	imgs, err := scrapeImages(srv.URL)
	if err != nil {
		t.Fatalf("scrapeImages() error = %v", err)
	}
	if len(imgs) != 2 {
		t.Fatalf("got %d images, want 2: %v", len(imgs), imgs)
	}
	if imgs[0] != "https://static.feissarimokat.com/img/abs.jpg" {
		t.Errorf("images[0] = %q, want absolute URL unchanged", imgs[0])
	}
	if !strings.HasPrefix(imgs[1], ImageBaseURL) || !strings.HasSuffix(imgs[1], "/img/rel.jpg") {
		t.Errorf("images[1] = %q, want ImageBaseURL-prefixed", imgs[1])
	}
}

func TestScrapeImagesMissingPostbodyReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(postTwoHTML)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	imgs, err := scrapeImages(srv.URL)
	if err != nil {
		t.Fatalf("scrapeImages() error = %v", err)
	}
	if imgs != nil {
		t.Errorf("scrapeImages() = %v, want nil when postbody missing", imgs)
	}
}

func TestScrapeImagesRequestError(t *testing.T) {
	if _, err := scrapeImages("http://127.0.0.1:0/does-not-exist"); err == nil {
		t.Fatal("scrapeImages() error = nil, want error on bad URL")
	}
}

func TestProcessItemsBuildsContent(t *testing.T) {
	// One server serves RSS-linked pages: /a has postbody images, /b has none,
	// /fail returns 500 (should be skipped with a warning, not a panic).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			_, _ = w.Write([]byte(postOneHTML))
		case "/b":
			_, _ = w.Write([]byte(postTwoHTML))
		case "/fail":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	rssItems := []RSSItem{
		{ItemTitle: "A", Description: "desc-a", ItemLink: srv.URL + "/a"},
		{ItemTitle: "B", Description: "desc-b", ItemLink: srv.URL + "/b"},
		{ItemTitle: "F", Description: "desc-f", ItemLink: srv.URL + "/fail"},
	}

	items := processItems(rssItems)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (failing one should be skipped)", len(items))
	}
	if !strings.Contains(items[0].ContentHTML, "desc-a") {
		t.Errorf("item A missing description in ContentHTML: %q", items[0].ContentHTML)
	}
	if !strings.Contains(items[0].ContentHTML, "<img") || len(items[0].Images) != 2 {
		t.Errorf("item A should have 2 scraped images and <img> tags; images=%v content=%q", items[0].Images, items[0].ContentHTML)
	}
	if len(items[1].Images) != 0 {
		t.Errorf("item B should have no images, got %v", items[1].Images)
	}
	if items[1].ContentHTML != "desc-b" {
		t.Errorf("item B content = %q, want description only", items[1].ContentHTML)
	}
}

func TestFetchItemsAppliesLimitAndBuildsFeedItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".rss") {
			// RSS feed with item links pointing back to this same server.
			base := "http://" + r.Host
			_, _ = w.Write([]byte(strings.Replace(strings.Replace(sampleRSS, "%s", base, 1), "%s", base, 1)))
			return
		}
		// Any other path → serve postbody HTML.
		_, _ = w.Write([]byte(postOneHTML))
	}))
	t.Cleanup(srv.Close)

	original := FeedURL
	FeedURL = srv.URL + "/feed.rss"
	t.Cleanup(func() { FeedURL = original })

	p := &Provider{}
	feedItems, err := p.FetchItems(1)
	if err != nil {
		t.Fatalf("FetchItems() error = %v", err)
	}
	if len(feedItems) != 1 {
		t.Fatalf("limit=1 returned %d items, want 1", len(feedItems))
	}
	if feedItems[0].Title() != "Post One" {
		t.Errorf("first item title = %q, want Post One", feedItems[0].Title())
	}
	if feedItems[0].ImageURL() == "" {
		t.Errorf("first item should have an ImageURL after scraping")
	}
}

func TestFetchItemsNoLimitReturnsAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".rss") {
			base := "http://" + r.Host
			_, _ = w.Write([]byte(strings.Replace(strings.Replace(sampleRSS, "%s", base, 1), "%s", base, 1)))
			return
		}
		_, _ = w.Write([]byte(postTwoHTML))
	}))
	t.Cleanup(srv.Close)

	original := FeedURL
	FeedURL = srv.URL + "/feed.rss"
	t.Cleanup(func() { FeedURL = original })

	p := &Provider{}
	feedItems, err := p.FetchItems(0)
	if err != nil {
		t.Fatalf("FetchItems() error = %v", err)
	}
	if len(feedItems) != 2 {
		t.Fatalf("FetchItems(0) returned %d items, want 2", len(feedItems))
	}
}

func TestFetchItemsPropagatesFetchError(t *testing.T) {
	original := FeedURL
	FeedURL = "http://127.0.0.1:0/nope"
	t.Cleanup(func() { FeedURL = original })

	p := &Provider{}
	if _, err := p.FetchItems(0); err == nil {
		t.Fatal("FetchItems() error = nil, want propagated fetch error")
	}
}

func TestConvertToFeedItemsUniqueBackingArray(t *testing.T) {
	items := []Item{
		{RSSItem: RSSItem{ItemTitle: "One", ItemLink: "https://example.test/1"}},
		{RSSItem: RSSItem{ItemTitle: "Two", ItemLink: "https://example.test/2"}},
	}
	feedItems := convertToFeedItems(items)
	if len(feedItems) != len(items) {
		t.Fatalf("len = %d, want %d", len(feedItems), len(items))
	}
	if feedItems[0].Title() != "One" || feedItems[1].Title() != "Two" {
		t.Errorf("titles not preserved: %q, %q", feedItems[0].Title(), feedItems[1].Title())
	}
	// Mutating the source slice must be visible via the feed items (they share
	// backing memory). This is the same invariant the HN test checks.
	items[0].ItemTitle = "Mutated"
	if feedItems[0].Title() != "Mutated" {
		t.Errorf("feedItems[0].Title() = %q, want Mutated after source mutation", feedItems[0].Title())
	}
}
