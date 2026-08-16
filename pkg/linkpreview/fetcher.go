package linkpreview

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/urlutils"
	"golang.org/x/sync/singleflight"
)

var errNotModified = errors.New("opengraph not modified")

// ProxyConfig configures a proxy for fetching URLs from blocked domains
type ProxyConfig struct {
	URL    string // Proxy endpoint URL
	Secret string // Shared secret for authentication
}

// Fetcher handles OpenGraph metadata fetching with rate limiting and caching
type Fetcher struct {
	client      *http.Client
	resolver    urlutils.LookupIPAddrsResolver
	db          *Database
	proxy       *ProxyConfig
	domainMutex sync.Mutex
	lastFetch   map[string]time.Time
	semaphore   chan struct{}
	fetchGroup  singleflight.Group
}

// NewFetcher creates a new OpenGraph fetcher
func NewFetcher(db *Database) *Fetcher {
	return newFetcher(db, nil)
}

// NewFetcherWithProxy creates a new OpenGraph fetcher that routes reddit URLs through a proxy
func NewFetcherWithProxy(db *Database, proxy *ProxyConfig) *Fetcher {
	return newFetcher(db, proxy)
}

func newFetcher(db *Database, proxy *ProxyConfig) *Fetcher {
	resolver := net.DefaultResolver
	transport := newSafeFetchTransport(resolver, allowedDialHosts(proxy), nil)

	return &Fetcher{
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
		resolver:  resolver,
		db:        db,
		proxy:     proxy,
		lastFetch: make(map[string]time.Time),
		semaphore: make(chan struct{}, 5), // Max 5 concurrent fetches
	}
}

// FetchData fetches OpenGraph data from a URL with caching.
func (f *Fetcher) FetchData(targetURL string) (*Data, error) {
	return f.FetchDataWithContext(context.Background(), targetURL)
}

// precheckURL decides whether targetURL is worth fetching at all. It returns
// skip for a URL that has no OpenGraph data to offer, which callers report as
// "no data" rather than as an error, and an error for a URL that must not be
// fetched.
func (f *Fetcher) precheckURL(ctx context.Context, targetURL string) (skip bool, err error) {
	// Rejected first, and without resolving anything: a relative URL, a
	// non-HTTP scheme, localhost, or a literal IP is an error whether or not it
	// looks like a media file, so an unsafe URL cannot hide behind a .jpg.
	if !urlutils.IsFetchableURLSyntax(targetURL) {
		return false, fmt.Errorf("invalid or disallowed fetch URL: %s", targetURL)
	}
	// Then the extension check, still ahead of the resolving fetchability test:
	// for a feed full of image links this skips the DNS lookups too, not just
	// the HTTP requests.
	if isNonPageURL(targetURL) {
		slog.Debug("Skipping URL that cannot carry OpenGraph tags", "url", targetURL)
		return true, nil
	}
	if !urlutils.IsFetchableURLWithContext(ctx, f.resolver, targetURL) {
		return false, fmt.Errorf("invalid or disallowed fetch URL: %s", targetURL)
	}
	if f.isBlockedURL(targetURL) {
		slog.Debug("Skipping blocked URL", "url", targetURL)
		return true, nil
	}
	return false, nil
}

// FetchDataWithContext fetches OpenGraph data from a URL with caching.
func (f *Fetcher) FetchDataWithContext(ctx context.Context, targetURL string) (*Data, error) {
	skipFetch, err := f.precheckURL(ctx, targetURL)
	if err != nil {
		return nil, err
	}
	if skipFetch {
		return nil, nil
	}

	cached, expired, skip := f.lookupCachedData(targetURL)
	if cached != nil {
		return cached, nil
	}
	if skip {
		return nil, nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	data, err := f.fetchWithExpiredHint(fetchCtx, targetURL, expired)
	if errors.Is(err, errNotModified) && expired != nil {
		return f.refreshExpired(expired, targetURL), nil
	}

	fetchSuccess := err == nil && data != nil
	if err != nil {
		slog.Debug("Failed to fetch OpenGraph data", "url", targetURL, "error", err)
		if data == nil {
			data = newFailurePlaceholder(targetURL)
		}
	} else if data != nil {
		// cleanupData already ran inside the singleflight function in
		// doFetchConditional; the returned pointer is shared between waiters.
		slog.Debug("Successfully fetched OpenGraph data", "url", targetURL, "title", data.Title)
	}

	if f.db != nil && data != nil {
		if cacheErr := f.db.SaveCachedData(data, fetchSuccess); cacheErr != nil {
			slog.Warn("Failed to cache OpenGraph data", "url", targetURL, "error", cacheErr)
		}
	}

	if fetchSuccess {
		return data, nil
	}
	return nil, err
}
