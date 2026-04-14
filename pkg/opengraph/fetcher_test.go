package opengraph

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/testutil"
	"golang.org/x/net/html"
)

func newTestOGDB(t *testing.T) *Database {
	t.Helper()
	db, err := NewDatabase(filepath.Join(t.TempDir(), "opengraph.db"))
	if err != nil {
		t.Fatalf("NewDatabase() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestNewFetcherVariants(t *testing.T) {
	db := newTestOGDB(t)

	plain := NewFetcher(db)
	if plain == nil || plain.db != db || plain.proxy != nil {
		t.Fatalf("NewFetcher() = %#v", plain)
	}
	if plain.client == nil || plain.semaphore == nil || plain.lastFetch == nil {
		t.Fatalf("NewFetcher() did not initialize internals: %#v", plain)
	}

	proxy := &ProxyConfig{URL: "https://proxy.example", Secret: "secret"}
	proxied := NewFetcherWithProxy(db, proxy)
	if proxied == nil || proxied.proxy != proxy {
		t.Fatalf("NewFetcherWithProxy() = %#v", proxied)
	}
}

func TestFetchData_InvalidAndBlockedURLs(t *testing.T) {
	fetcher := NewFetcher(nil)

	if data, err := fetcher.FetchData("://bad"); err == nil || data != nil {
		t.Fatalf("FetchData(invalid) = (%#v, %v), want error", data, err)
	}

	if data, err := fetcher.FetchData("http://127.0.0.1:8080"); err == nil || data != nil {
		t.Fatalf("FetchData(loopback) = (%#v, %v), want error", data, err)
	}

	if data, err := fetcher.FetchData("https://twitter.com/example/status/1"); err != nil || data != nil {
		t.Fatalf("FetchData(blocked) = (%#v, %v), want (nil, nil)", data, err)
	}

	if fetcher.isBlockedURL("https://evil.example/?next=twitter.com") {
		t.Fatal("isBlockedURL(query containing blocked hostname) = true, want false")
	}

	proxied := NewFetcherWithProxy(nil, &ProxyConfig{URL: "https://proxy.example", Secret: "secret"})
	if !proxied.isBlockedURL("https://i.redd.it/image.png") {
		t.Fatal("isBlockedURL(i.redd.it) = false, want true")
	}
	if proxied.isBlockedURL("https://www.reddit.com/r/golang/comments/1") {
		t.Fatal("isBlockedURL(reddit page with proxy) = true, want false")
	}
}

func TestFetchData_UsesCacheAndRecentFailure(t *testing.T) {
	db := newTestOGDB(t)
	fetcher := NewFetcher(db)
	now := time.Now().UTC()

	cachedInput := &Data{
		URL:         "https://example.com/cached",
		Title:       "Cached title",
		Description: "Cached description",
		FetchedAt:   now,
		ExpiresAt:   now.Add(time.Hour),
	}
	if err := db.SaveCachedData(cachedInput, true); err != nil {
		t.Fatalf("SaveCachedData(success) error = %v", err)
	}

	cached, err := fetcher.FetchData(cachedInput.URL)
	if err != nil {
		t.Fatalf("FetchData(cached) error = %v", err)
	}
	if cached == nil || cached.Title != "Cached title" {
		t.Fatalf("FetchData(cached) = %#v", cached)
	}

	failedURL := "https://example.com/failed"
	if err := db.SaveCachedData(&Data{URL: failedURL, FetchedAt: now, ExpiresAt: now.Add(time.Hour)}, false); err != nil {
		t.Fatalf("SaveCachedData(failure) error = %v", err)
	}

	data, err := fetcher.FetchData(failedURL)
	if err != nil || data != nil {
		t.Fatalf("FetchData(recent failure) = (%#v, %v), want (nil, nil)", data, err)
	}
}

func TestFetchFreshDataAndFetchDataSuccess(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head>
			<title>Fallback title</title>
			<meta property="og:title" content="  OG Title  ">
			<meta property="og:description" content="OG Description">
			<meta property="og:image" content="/img.png">
			<meta property="og:site_name" content=" Example Site ">
		</head><body></body></html>`))
	}))
	defer server.Close()

	targetURL := "http://example.invalid/post"
	db := newTestOGDB(t)
	fetcher := NewFetcher(db)
	fetcher.resolver = testutil.StubResolver{Lookup: func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}}
	fetcher.client.Transport = rewriteHostTransport(server)

	fresh, err := fetcher.fetchFreshData(context.Background(), targetURL)
	if err != nil {
		t.Fatalf("fetchFreshData() error = %v", err)
	}
	if fresh == nil || fresh.Title != "Fallback title" || fresh.Description != "OG Description" || fresh.Image != "/img.png" {
		t.Fatalf("fetchFreshData() = %#v", fresh)
	}

	data, err := fetcher.FetchData(targetURL)
	if err != nil {
		t.Fatalf("FetchData() error = %v", err)
	}
	if data == nil {
		t.Fatal("FetchData() = nil, want data")
	}
	if data.Image != "http://example.invalid/img.png" {
		t.Fatalf("resolved image = %q, want %q", data.Image, "http://example.invalid/img.png")
	}
	if data.SiteName != "Example Site" {
		t.Fatalf("SiteName = %q, want %q", data.SiteName, "Example Site")
	}
	if hits.Load() < 2 {
		t.Fatalf("server hits = %d, want at least 2", hits.Load())
	}

	cached, err := fetcher.FetchData(targetURL)
	if err != nil {
		t.Fatalf("FetchData(cached after save) error = %v", err)
	}
	if cached == nil || cached.Title != data.Title {
		t.Fatalf("FetchData(cached after save) = %#v", cached)
	}
}

func TestFetchFreshData_GzipAndProxy(t *testing.T) {
	var proxyHits atomic.Int32
	var sawSecret atomic.Bool
	var sawTarget atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		if r.Header.Get("X-Proxy-Secret") == "secret" {
			sawSecret.Store(true)
		}
		if strings.Contains(r.Header.Get("X-Target-URL"), "reddit.com") {
			sawTarget.Store(true)
		}

		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, _ = gz.Write([]byte(`<!doctype html><html><head><meta name="twitter:title" content="Proxy Title"><meta name="twitter:image" content="https://example.com/image.jpg"></head></html>`))
		_ = gz.Close()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(buf.Bytes())
	}))
	defer proxy.Close()

	fetcher := NewFetcherWithProxy(nil, &ProxyConfig{URL: proxy.URL, Secret: "secret"})
	data, err := fetcher.fetchFreshData(context.Background(), "https://www.reddit.com/r/golang/comments/1")
	if err != nil {
		t.Fatalf("fetchFreshData(proxy) error = %v", err)
	}
	if data == nil || data.Title != "Proxy Title" || data.Image != "https://example.com/image.jpg" {
		t.Fatalf("fetchFreshData(proxy) = %#v", data)
	}
	if proxyHits.Load() != 1 || !sawSecret.Load() || !sawTarget.Load() {
		t.Fatalf("proxy behavior hits=%d secret=%v target=%v", proxyHits.Load(), sawSecret.Load(), sawTarget.Load())
	}
}

func TestFetchData_CachesFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	targetURL := "http://example.invalid/failure"
	db := newTestOGDB(t)
	fetcher := NewFetcher(db)
	fetcher.resolver = testutil.StubResolver{Lookup: func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}}
	fetcher.client.Transport = rewriteHostTransport(server)
	data, err := fetcher.FetchData(targetURL)
	if err == nil || data != nil {
		t.Fatalf("FetchData(failure) = (%#v, %v), want error", data, err)
	}

	hasFailure, err := db.HasRecentFailure(targetURL)
	if err != nil {
		t.Fatalf("HasRecentFailure() error = %v", err)
	}
	if !hasFailure {
		t.Fatal("HasRecentFailure() = false, want true")
	}
}

func TestExtractOpenGraphTagsAndProcessMetaTag(t *testing.T) {
	fetcher := NewFetcher(nil)
	doc, err := html.Parse(strings.NewReader(`<!doctype html><html><head>
		<title> Page Title </title>
		<meta name="description" content="fallback description">
		<meta property="og:image" content="/fallback.png">
		<meta property="og:site_name" content="Site">
	</head></html>`))
	if err != nil {
		t.Fatalf("html.Parse() error = %v", err)
	}

	data := &Data{}
	fetcher.extractOpenGraphTags(doc, data)
	if data.Title != "Page Title" || data.Description != "fallback description" || data.Image != "/fallback.png" || data.SiteName != "Site" {
		t.Fatalf("extractOpenGraphTags() = %#v", data)
	}

	node, err := html.Parse(strings.NewReader(`<html><head><meta property="og:title" content="Preferred"></head></html>`))
	if err != nil {
		t.Fatalf("html.Parse(meta) error = %v", err)
	}
	var metaNode *html.Node
	var findMeta func(*html.Node)
	findMeta = func(n *html.Node) {
		if n == nil || metaNode != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "meta" {
			metaNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findMeta(c)
		}
	}
	findMeta(node)
	if metaNode == nil {
		t.Fatal("failed to find meta node")
	}
	data = &Data{}
	fetcher.processMetaTag(metaNode, data)
	if data.Title != "Preferred" {
		t.Fatalf("processMetaTag() title = %q, want Preferred", data.Title)
	}
}

func TestCleanupDataAndConvertToUTF8AndURLHelpers(t *testing.T) {
	fetcher := NewFetcher(nil)
	data := &Data{
		Title:       "  " + strings.Repeat("T", 205) + "\x00  ",
		Description: "  " + strings.Repeat("D", 505) + "\x00  ",
		Image:       "://bad",
		SiteName:    " Site\x00 ",
	}
	fetcher.cleanupData(data, "https://example.com/post")
	if !strings.HasSuffix(data.Title, "...") || strings.Contains(data.Title, "\x00") || len(data.Title) > 205 {
		t.Fatalf("cleanupData() title = %q", data.Title)
	}
	if !strings.HasSuffix(data.Description, "...") || strings.Contains(data.Description, "\x00") || len(data.Description) > 505 {
		t.Fatalf("cleanupData() description = %q", data.Description)
	}
	if data.Image != "" {
		t.Fatalf("cleanupData() image = %q, want empty", data.Image)
	}
	if data.SiteName != "Site" {
		t.Fatalf("cleanupData() site name = %q, want Site", data.SiteName)
	}

	converted, err := fetcher.convertToUTF8([]byte("hello"), "text/html; charset=not-real")
	if err != nil || converted != "hello" {
		t.Fatalf("convertToUTF8() = (%q, %v), want (hello, nil)", converted, err)
	}

	if !isProxiableRedditURL("https://www.reddit.com/r/golang") || isProxiableRedditURL("https://example.com") {
		t.Fatal("isProxiableRedditURL() unexpected result")
	}
	if !fetcher.isBlockedURL("https://x.com/user/status/1") || fetcher.isBlockedURL("https://example.com") {
		t.Fatal("isBlockedURL() unexpected result")
	}
}

func rewriteHostTransport(server *httptest.Server) *http.Transport {
	serverAddr := server.Listener.Addr().String()

	return &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, serverAddr)
		},
	}
}

func TestSafeDialContextRejectsDirectAndResolvedPrivateIPs(t *testing.T) {
	baseDialer := &net.Dialer{}

	directDial := safeDialContext(nil, nil, baseDialer)
	if _, err := directDial(context.Background(), "tcp", "8.8.8.8:80"); err == nil {
		t.Fatal("safeDialContext(direct public ip) error = nil, want rejection")
	}

	resolver := testutil.StubResolver{Lookup: func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}}, nil
	}}
	resolvedDial := safeDialContext(resolver, nil, baseDialer)
	if _, err := resolvedDial(context.Background(), "tcp", "example.com:80"); err == nil {
		t.Fatal("safeDialContext(resolved private ip) error = nil, want rejection")
	}

	errorResolver := testutil.StubResolver{Lookup: func(context.Context, string) ([]net.IPAddr, error) {
		return nil, errors.New("dns failure")
	}}
	failedDial := safeDialContext(errorResolver, nil, baseDialer)
	if _, err := failedDial(context.Background(), "tcp", "example.com:80"); err == nil {
		t.Fatal("safeDialContext(dns failure) error = nil, want rejection")
	}
}

func TestFetchConcurrentWithContext_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fetcher := NewFetcher(nil)
	fetcher.resolver = testutil.StubResolver{Lookup: func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}}

	start := time.Now()
	results := fetcher.FetchConcurrentWithContext(ctx, []string{"http://example.invalid/one", "http://example.invalid/two"})
	if len(results) != 0 {
		t.Fatalf("len(FetchConcurrentWithContext(cancelled)) = %d, want 0", len(results))
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("FetchConcurrentWithContext(cancelled) took %v, want prompt return", elapsed)
	}
}

func TestFetchConcurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><meta property="og:title" content="` + r.URL.Path + `"></head></html>`))
	}))
	defer server.Close()

	fetcher := NewFetcher(nil)
	fetcher.resolver = testutil.StubResolver{Lookup: func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}}
	fetcher.client.Transport = rewriteHostTransport(server)
	oneURL := "http://example.invalid/one"
	twoURL := "http://example.invalid/two"
	results := fetcher.FetchConcurrent([]string{"", oneURL, twoURL})
	if len(results) != 2 {
		t.Fatalf("len(FetchConcurrent()) = %d, want 2", len(results))
	}
	if results[oneURL].Title != "/one" || results[twoURL].Title != "/two" {
		t.Fatalf("FetchConcurrent() = %#v", results)
	}

	if empty := fetcher.FetchConcurrent(nil); len(empty) != 0 {
		t.Fatalf("FetchConcurrent(nil) len = %d, want 0", len(empty))
	}
}
