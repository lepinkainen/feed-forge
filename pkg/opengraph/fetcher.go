package opengraph

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
	urlMutexes  sync.Map
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

// FetchDataWithContext fetches OpenGraph data from a URL with caching.
func (f *Fetcher) FetchDataWithContext(ctx context.Context, targetURL string) (*Data, error) {
	if !urlutils.IsFetchableURLWithContext(ctx, f.resolver, targetURL) {
		return nil, fmt.Errorf("invalid or disallowed fetch URL: %s", targetURL)
	}
	if f.isBlockedURL(targetURL) {
		slog.Debug("Skipping blocked URL", "url", targetURL)
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
		cleanupData(data, targetURL)
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
