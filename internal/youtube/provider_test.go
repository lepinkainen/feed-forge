package youtube

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns:media="http://search.yahoo.com/mrss/" xmlns="http://www.w3.org/2005/Atom">
  <title>Taskmaster</title>
  <yt:channelId>UCT5C7yaO3RVuOgwP8JVAujQ</yt:channelId>
  <entry>
    <id>yt:video:short123</id>
    <yt:videoId>short123</yt:videoId>
    <yt:channelId>UCT5C7yaO3RVuOgwP8JVAujQ</yt:channelId>
    <title>Short clip</title>
    <link rel="alternate" href="https://www.youtube.com/shorts/short123"/>
    <author><name>Taskmaster</name><uri>https://www.youtube.com/channel/UCT5C7yaO3RVuOgwP8JVAujQ</uri></author>
    <published>2026-05-18T12:15:01+00:00</published>
    <updated>2026-05-18T12:15:02+00:00</updated>
    <media:group>
      <media:title>Short clip</media:title>
      <media:thumbnail url="https://i3.ytimg.com/vi/short123/hqdefault.jpg" width="480" height="360"/>
      <media:description>short desc</media:description>
      <media:community><media:statistics views="100"/></media:community>
    </media:group>
  </entry>
  <entry>
    <id>yt:video:watch123</id>
    <yt:videoId>watch123</yt:videoId>
    <yt:channelId>UCT5C7yaO3RVuOgwP8JVAujQ</yt:channelId>
    <title>Full episode</title>
    <link rel="alternate" href="https://www.youtube.com/watch?v=watch123"/>
    <author><name>Taskmaster</name><uri>https://www.youtube.com/channel/UCT5C7yaO3RVuOgwP8JVAujQ</uri></author>
    <published>2026-05-17T18:00:33+00:00</published>
    <updated>2026-05-17T18:00:34+00:00</updated>
    <media:group>
      <media:title>Full episode</media:title>
      <media:thumbnail url="https://i3.ytimg.com/vi/watch123/hqdefault.jpg" width="480" height="360"/>
      <media:description>full desc</media:description>
      <media:community><media:statistics views="2000"/></media:community>
    </media:group>
  </entry>
</feed>`

func TestFetchItemsFiltersShortsByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(sampleFeed))
	}))
	defer srv.Close()

	provider := &Provider{FeedURLs: []string{srv.URL}, Limit: 30, IncludeShorts: false}
	items, err := provider.FetchItems(0)
	if err != nil {
		t.Fatalf("FetchItems: %v", err)
	}

	if got, want := len(items), 1; got != want {
		t.Fatalf("item count = %d, want %d", got, want)
	}
	if items[0].Title() != "Full episode" {
		t.Errorf("remaining item title = %q", items[0].Title())
	}
	if items[0].Score() != 2000 {
		t.Errorf("views = %d, want 2000", items[0].Score())
	}
}

func TestFetchItemsCanIncludeShorts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleFeed))
	}))
	defer srv.Close()

	provider := &Provider{FeedURLs: []string{srv.URL}, IncludeShorts: true}
	items, err := provider.FetchItems(0)
	if err != nil {
		t.Fatalf("FetchItems: %v", err)
	}

	if got, want := len(items), 2; got != want {
		t.Fatalf("item count = %d, want %d", got, want)
	}
	if items[0].Title() != "Short clip" {
		t.Errorf("newest item = %q, want Short clip", items[0].Title())
	}
}

func TestFetchItemsContinuesWhenSomeFeedsFail(t *testing.T) {
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleFeed))
	}))
	defer okSrv.Close()
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer failSrv.Close()

	provider := &Provider{FeedURLs: []string{okSrv.URL, failSrv.URL}, IncludeShorts: true}
	items, err := provider.FetchItems(0)
	if err != nil {
		t.Fatalf("FetchItems: %v", err)
	}
	if got, want := len(items), 2; got != want {
		t.Fatalf("item count = %d, want %d", got, want)
	}
}

func TestFetchItemsSucceedsWhenFeedYieldsZeroItemsAfterFilter(t *testing.T) {
	const shortsOnlyFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns:media="http://search.yahoo.com/mrss/" xmlns="http://www.w3.org/2005/Atom">
  <title>Shorts</title>
  <entry>
    <id>yt:video:short999</id>
    <yt:videoId>short999</yt:videoId>
    <title>Short only</title>
    <link rel="alternate" href="https://www.youtube.com/shorts/short999"/>
    <published>2026-05-18T12:15:01+00:00</published>
    <updated>2026-05-18T12:15:02+00:00</updated>
  </entry>
</feed>`
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(shortsOnlyFeed))
	}))
	defer okSrv.Close()
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer failSrv.Close()

	provider := &Provider{FeedURLs: []string{okSrv.URL, failSrv.URL}, IncludeShorts: false}
	items, err := provider.FetchItems(0)
	if err != nil {
		t.Fatalf("FetchItems should not error when at least one feed succeeded: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("item count = %d, want 0", len(items))
	}
}

func TestFetchItemsErrorsWhenAllFeedsFail(t *testing.T) {
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer failSrv.Close()

	provider := &Provider{FeedURLs: []string{failSrv.URL, failSrv.URL}}
	if _, err := provider.FetchItems(0); err == nil {
		t.Fatal("FetchItems should error when every feed fails")
	}
}

func TestNormalizeFeedURLs(t *testing.T) {
	got := normalizeFeedURLs(
		"https://www.youtube.com/feeds/videos.xml?channel_id=UC1",
		[]string{"https://www.youtube.com/feeds/videos.xml?channel_id=UC2", "https://www.youtube.com/feeds/videos.xml?channel_id=UC1"},
		[]string{"UC3"},
	)

	want := []string{
		"https://www.youtube.com/feeds/videos.xml?channel_id=UC1",
		"https://www.youtube.com/feeds/videos.xml?channel_id=UC2",
		"https://www.youtube.com/feeds/videos.xml?channel_id=UC3",
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestIsShortURL(t *testing.T) {
	cases := map[string]bool{
		"https://www.youtube.com/shorts/BWkuskQFz20":  true,
		"https://youtu.be/BWkuskQFz20":                false,
		"https://www.youtube.com/watch?v=YpBNXnfJLLM": false,
	}
	for raw, want := range cases {
		if got := isShortURL(raw); got != want {
			t.Errorf("isShortURL(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestFactoryRejectsMissingFeeds(t *testing.T) {
	if _, err := factory(&Config{}); err == nil {
		t.Fatal("factory should require at least one feed-url or channel-id")
	}
	if _, err := factory("bad config"); err == nil {
		t.Fatal("factory should reject wrong config type")
	}
}
