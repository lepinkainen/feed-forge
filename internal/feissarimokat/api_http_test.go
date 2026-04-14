package feissarimokat

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchRSSFeedWithHTTPServer(t *testing.T) {
	oldFeedURL := FeedURL
	defer func() { FeedURL = oldFeedURL }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/feed.rss" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
		<rss><channel>
			<title>Feissarit</title>
			<item>
				<title>Post One</title>
				<description><![CDATA[First desc]]></description>
				<link>https://example.com/post/1</link>
			</item>
			<item>
				<title>Post Two</title>
				<description>Second desc</description>
				<link>https://example.com/post/2</link>
			</item>
		</channel></rss>`))
	}))
	defer server.Close()

	FeedURL = server.URL + "/feed.rss"
	items, err := fetchRSSFeed()
	if err != nil {
		t.Fatalf("fetchRSSFeed() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(fetchRSSFeed()) = %d, want 2", len(items))
	}
	if items[0].ItemTitle != "Post One" || items[0].Description != "First desc" || items[0].ItemLink != "https://example.com/post/1" {
		t.Fatalf("first parsed item = %#v", items[0])
	}
	if items[1].ItemTitle != "Post Two" || items[1].ItemLink != "https://example.com/post/2" {
		t.Fatalf("second parsed item = %#v", items[1])
	}
}

func TestFetchRSSFeedReturnsParseErrorForInvalidXML(t *testing.T) {
	oldFeedURL := FeedURL
	defer func() { FeedURL = oldFeedURL }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<rss><channel><item></rss>`))
	}))
	defer server.Close()

	FeedURL = server.URL
	items, err := fetchRSSFeed()
	if err == nil || !strings.Contains(err.Error(), "error parsing RSS XML") {
		t.Fatalf("fetchRSSFeed() = (%v, %v), want XML parse error", items, err)
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

func TestFetchRSSFeedWithLiveSnapshotFixture(t *testing.T) {
	oldFeedURL := FeedURL
	defer func() { FeedURL = oldFeedURL }()

	feedFixture := mustReadFixture(t, "feed_live_snapshot.rss")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write(feedFixture)
	}))
	defer server.Close()

	FeedURL = server.URL
	items, err := fetchRSSFeed()
	if err != nil {
		t.Fatalf("fetchRSSFeed() error = %v", err)
	}
	if len(items) < 10 {
		t.Fatalf("len(fetchRSSFeed()) = %d, want at least 10 items from live snapshot", len(items))
	}
	if items[0].ItemTitle == "" || items[0].ItemLink == "" || items[0].Description == "" {
		t.Fatalf("first parsed item missing fields: %#v", items[0])
	}
	if !strings.Contains(items[0].ItemLink, "https://www.feissarimokat.com/") {
		t.Fatalf("first parsed item link = %q, want feissarimokat URL", items[0].ItemLink)
	}
}

func TestScrapeImagesWithHTTPServer(t *testing.T) {
	oldImageBaseURL := ImageBaseURL
	defer func() { ImageBaseURL = oldImageBaseURL }()
	ImageBaseURL = "https://img.test"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/post":
			_, _ = w.Write([]byte(`<html><body>
				<div class="postbody">
					<p>caption</p>
					<img src="/img/one.jpg">
					<img src="https://cdn.example.com/two.jpg">
				</div>
				<img src="/outside.jpg">
			</body></html>`))
		case "/no-postbody":
			_, _ = w.Write([]byte(`<html><body><div class="content"><img src="/ignored.jpg"></div></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	images, err := scrapeImages(server.URL + "/post")
	if err != nil {
		t.Fatalf("scrapeImages(/post) error = %v", err)
	}
	want := []string{"https://img.test/img/one.jpg", "https://cdn.example.com/two.jpg"}
	if len(images) != len(want) {
		t.Fatalf("len(scrapeImages(/post)) = %d, want %d (%v)", len(images), len(want), images)
	}
	for i := range want {
		if images[i] != want[i] {
			t.Fatalf("scrapeImages(/post)[%d] = %q, want %q", i, images[i], want[i])
		}
	}

	missing, err := scrapeImages(server.URL + "/no-postbody")
	if err != nil {
		t.Fatalf("scrapeImages(/no-postbody) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("scrapeImages(/no-postbody) = %v, want nil", missing)
	}
}

func TestScrapeImagesWithLiveSnapshotFixture(t *testing.T) {
	oldImageBaseURL := ImageBaseURL
	defer func() { ImageBaseURL = oldImageBaseURL }()

	postFixture := mustReadFixture(t, "post_live_snapshot.html")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(postFixture)
	}))
	defer server.Close()

	ImageBaseURL = "https://static.feissarimokat.com"
	images, err := scrapeImages(server.URL)
	if err != nil {
		t.Fatalf("scrapeImages() error = %v", err)
	}
	if len(images) == 0 {
		t.Fatal("scrapeImages() returned no images for live snapshot fixture")
	}
	for i, image := range images {
		if !strings.HasPrefix(image, "https://static.feissarimokat.com/") {
			t.Fatalf("images[%d] = %q, want static.feissarimokat.com URL", i, image)
		}
	}
}

func TestProcessItemsBuildsContentAndSkipsScrapeFailures(t *testing.T) {
	oldImageBaseURL := ImageBaseURL
	defer func() { ImageBaseURL = oldImageBaseURL }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte(`<div class="postbody"><img src="/img/ok.jpg"></div>`))
		case "/quoted":
			_, _ = w.Write([]byte(`<div class="postbody"><img src="/img/test.jpg"></div>`))
		case "/broken":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	ImageBaseURL = server.URL

	items := processItems([]RSSItem{
		{ItemTitle: "Okay", Description: "Desc", ItemLink: server.URL + "/ok"},
		{ItemTitle: `Title "quoted" <b>tag</b>`, Description: "Escaped", ItemLink: server.URL + "/quoted"},
		{ItemTitle: "Broken", Description: "Skip", ItemLink: server.URL + "/broken"},
	})

	if len(items) != 2 {
		t.Fatalf("len(processItems()) = %d, want 2", len(items))
	}
	if items[0].ContentHTML != "Desc\n<img src=\""+server.URL+"/img/ok.jpg\" alt=\"Okay\">" {
		t.Fatalf("first ContentHTML = %q", items[0].ContentHTML)
	}
	if items[0].ImageURL() != server.URL+"/img/ok.jpg" {
		t.Fatalf("first ImageURL() = %q", items[0].ImageURL())
	}
	if !strings.Contains(items[1].ContentHTML, "Escaped\n<img src=\""+server.URL+"/img/test.jpg\"") {
		t.Fatalf("second ContentHTML = %q, want embedded image", items[1].ContentHTML)
	}
	if !strings.Contains(items[1].ContentHTML, `alt="Title &#34;quoted&#34; &lt;b&gt;tag&lt;/b&gt;"`) {
		t.Fatalf("second ContentHTML = %q, want escaped alt text", items[1].ContentHTML)
	}
}
