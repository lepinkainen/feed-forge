package youtube

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoverFeedURLFromHTML(t *testing.T) {
	const body = `<!doctype html><html><head>
<link rel="alternate" media="handheld" href="https://m.youtube.com/@Taskmaster">
<link rel="alternate" type="application/rss+xml" title="RSS" href="https://www.youtube.com/feeds/videos.xml?channel_id=UCT5C7yaO3RVuOgwP8JVAujQ">
</head><body></body></html>`

	got, err := discoverFeedURLFromHTML(strings.NewReader(body))
	if err != nil {
		t.Fatalf("discoverFeedURLFromHTML: %v", err)
	}
	const want = "https://www.youtube.com/feeds/videos.xml?channel_id=UCT5C7yaO3RVuOgwP8JVAujQ"
	if got != want {
		t.Errorf("feed URL = %q, want %q", got, want)
	}
}

func TestDiscoverFeedURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<link rel="alternate" type="application/rss+xml" href="https://www.youtube.com/feeds/videos.xml?channel_id=UC123">`))
	}))
	defer srv.Close()

	got, err := DiscoverFeedURL(srv.URL)
	if err != nil {
		t.Fatalf("DiscoverFeedURL: %v", err)
	}
	if got != "https://www.youtube.com/feeds/videos.xml?channel_id=UC123" {
		t.Errorf("feed URL = %q", got)
	}
}

func TestDiscoverFeedURLFromHTMLMissing(t *testing.T) {
	if _, err := discoverFeedURLFromHTML(strings.NewReader(`<html></html>`)); err == nil {
		t.Fatal("expected missing RSS link error")
	}
}
