package httpcache

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

func TestStoreSaveGetRoundTrip(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "http_cache.db"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if got, ok := store.Get("https://example.com/missing"); ok || got.ETag != "" || got.LastModified != "" {
		t.Fatalf("Get(missing) = (%#v, %v), want zero,false", got, ok)
	}

	want := api.CacheValidators{ETag: `"abc"`, LastModified: "Mon, 01 Jan 2024 00:00:00 GMT"}
	if err := store.Save("https://example.com/feed", want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, ok := store.Get("https://example.com/feed")
	if !ok {
		t.Fatal("Get(saved) ok = false, want true")
	}
	if got != want {
		t.Fatalf("Get(saved) = %#v, want %#v", got, want)
	}
}

func TestStoreEmptyValidatorsDisableGet(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "http_cache.db"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	url := "https://example.com/feed"
	if err := store.Save(url, api.CacheValidators{ETag: `"abc"`}); err != nil {
		t.Fatalf("Save(initial) error = %v", err)
	}
	if err := store.Save(url, api.CacheValidators{}); err != nil {
		t.Fatalf("Save(empty) error = %v", err)
	}

	if got, ok := store.Get(url); ok || got.ETag != "" || got.LastModified != "" {
		t.Fatalf("Get(empty validators) = (%#v, %v), want zero,false", got, ok)
	}
}

func TestCachedGetSavesValidatorsAndReturnsNotModified(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "http_cache.db"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 2 {
			if got := r.Header.Get("If-None-Match"); got != `"v1"` {
				t.Fatalf("If-None-Match = %q, want %q", got, `"v1"`)
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte("fresh"))
	}))
	defer server.Close()

	client := api.NewEnhancedClient(&api.EnhancedClientConfig{
		RetryPolicy: &api.RetryPolicy{MaxAttempts: 1, RetryableErrors: []int{}},
		RateLimiter: api.NewNoOpRateLimiter(),
	})

	body, err := CachedGet(t.Context(), client, store, server.URL, nil)
	if err != nil {
		t.Fatalf("CachedGet(first) error = %v", err)
	}
	if string(body) != "fresh" {
		t.Fatalf("body = %q, want fresh", body)
	}

	_, err = CachedGet(t.Context(), client, store, server.URL, nil)
	if !errors.Is(err, ErrNotModified) {
		t.Fatalf("CachedGet(second) error = %v, want ErrNotModified", err)
	}
}

func TestCachedGetServerErrorUsesClientRetry(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := api.NewEnhancedClient(&api.EnhancedClientConfig{
		RetryPolicy: &api.RetryPolicy{MaxAttempts: 2, InitialBackoff: time.Millisecond, RetryableErrors: []int{http.StatusInternalServerError}},
		RateLimiter: api.NewNoOpRateLimiter(),
	})

	_, err := CachedGet(t.Context(), client, nil, server.URL, nil)
	if err == nil {
		t.Fatal("CachedGet(500) error = nil, want error")
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
}

func TestCachedGetNilStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("If-None-Match"); got != "" {
			t.Fatalf("If-None-Match = %q, want empty", got)
		}
		if got := r.Header.Get("If-Modified-Since"); got != "" {
			t.Fatalf("If-Modified-Since = %q, want empty", got)
		}
		w.Header().Set("ETag", `"nil-store"`)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := api.NewEnhancedClient(&api.EnhancedClientConfig{
		RetryPolicy: &api.RetryPolicy{MaxAttempts: 1, RetryableErrors: []int{}},
		RateLimiter: api.NewNoOpRateLimiter(),
	})

	body, err := CachedGet(t.Context(), client, nil, server.URL, nil)
	if err != nil {
		t.Fatalf("CachedGet(nil store) error = %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
}

func TestCachedGetContextCancellation(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "http_cache.db"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	serverHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHit = true
		_, _ = w.Write([]byte("unexpected"))
	}))
	defer server.Close()

	client := api.NewEnhancedClient(&api.EnhancedClientConfig{
		RetryPolicy: &api.RetryPolicy{MaxAttempts: 1, RetryableErrors: []int{}},
		RateLimiter: api.NewNoOpRateLimiter(),
	})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = CachedGet(ctx, client, store, server.URL, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CachedGet(canceled) error = %v, want context.Canceled", err)
	}
	if serverHit {
		t.Fatal("server was hit after context cancellation")
	}
}

func TestCachedGetWithStaleServesCachedBodyOnError(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "http_cache.db"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			_, _ = w.Write([]byte("fresh feed"))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := api.NewEnhancedClient(&api.EnhancedClientConfig{
		RetryPolicy: &api.RetryPolicy{MaxAttempts: 1, RetryableErrors: []int{}},
		RateLimiter: api.NewNoOpRateLimiter(),
	})

	body, stale, err := CachedGetWithStale(t.Context(), client, store, server.URL, nil, 0)
	if err != nil {
		t.Fatalf("CachedGetWithStale(first) error = %v", err)
	}
	if stale || string(body) != "fresh feed" {
		t.Fatalf("first = (%q, stale=%v), want (\"fresh feed\", false)", body, stale)
	}

	body, stale, err = CachedGetWithStale(t.Context(), client, store, server.URL, nil, 0)
	if err == nil {
		t.Fatal("CachedGetWithStale(404) error = nil, want underlying error")
	}
	if !stale || string(body) != "fresh feed" {
		t.Fatalf("second = (%q, stale=%v), want cached \"fresh feed\", stale=true", body, stale)
	}
}

func TestCachedGetWithStaleRejectsCopyOlderThanMaxStale(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "http_cache.db"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			_, _ = w.Write([]byte("fresh feed"))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := api.NewEnhancedClient(&api.EnhancedClientConfig{
		RetryPolicy: &api.RetryPolicy{MaxAttempts: 1, RetryableErrors: []int{}},
		RateLimiter: api.NewNoOpRateLimiter(),
	})

	if _, _, err := CachedGetWithStale(t.Context(), client, store, server.URL, nil, 48*time.Hour); err != nil {
		t.Fatalf("CachedGetWithStale(first) error = %v", err)
	}

	// Within the window the cached copy is served despite the 404.
	body, stale, err := CachedGetWithStale(t.Context(), client, store, server.URL, nil, 48*time.Hour)
	if err == nil || !stale || string(body) != "fresh feed" {
		t.Fatalf("within window = (%q, stale=%v, err=%v), want cached body, stale=true, underlying error", body, stale, err)
	}

	// Age the cached copy past the window; the error must now surface.
	if _, err := store.db.Exec(`UPDATE http_validators SET updated_at = ?`, time.Now().UTC().Add(-72*time.Hour)); err != nil {
		t.Fatalf("age cache entry: %v", err)
	}

	body, stale, err = CachedGetWithStale(t.Context(), client, store, server.URL, nil, 48*time.Hour)
	if err == nil {
		t.Fatal("CachedGetWithStale(expired cache) error = nil, want error")
	}
	if stale || body != nil {
		t.Fatalf("expired = (%q, stale=%v), want (nil, false)", body, stale)
	}
}

func TestCachedGetWithStaleNotModifiedRefreshesTimestamp(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "http_cache.db"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch hits {
		case 1:
			w.Header().Set("ETag", `"v1"`)
			_, _ = w.Write([]byte("fresh feed"))
		case 2:
			w.WriteHeader(http.StatusNotModified)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := api.NewEnhancedClient(&api.EnhancedClientConfig{
		RetryPolicy: &api.RetryPolicy{MaxAttempts: 1, RetryableErrors: []int{}},
		RateLimiter: api.NewNoOpRateLimiter(),
	})

	if _, _, err := CachedGetWithStale(t.Context(), client, store, server.URL, nil, 48*time.Hour); err != nil {
		t.Fatalf("CachedGetWithStale(first) error = %v", err)
	}

	// Backdate the entry beyond the stale window, then hit the 304: the
	// timestamp must refresh because the cached copy was verified current.
	if _, err := store.db.Exec(`UPDATE http_validators SET updated_at = ?`, time.Now().UTC().Add(-72*time.Hour)); err != nil {
		t.Fatalf("age cache entry: %v", err)
	}

	body, stale, err := CachedGetWithStale(t.Context(), client, store, server.URL, nil, 48*time.Hour)
	if err != nil || stale || string(body) != "fresh feed" {
		t.Fatalf("304 = (%q, stale=%v, err=%v), want cached body, stale=false, nil error", body, stale, err)
	}

	// The 404 that follows must serve stale again — the 304 reset the clock.
	body, stale, err = CachedGetWithStale(t.Context(), client, store, server.URL, nil, 48*time.Hour)
	if err == nil || !stale || string(body) != "fresh feed" {
		t.Fatalf("after 304 = (%q, stale=%v, err=%v), want cached body, stale=true, underlying error", body, stale, err)
	}
}

func TestCachedGetWithStaleNoCacheReturnsError(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "http_cache.db"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := api.NewEnhancedClient(&api.EnhancedClientConfig{
		RetryPolicy: &api.RetryPolicy{MaxAttempts: 1, RetryableErrors: []int{}},
		RateLimiter: api.NewNoOpRateLimiter(),
	})

	body, stale, err := CachedGetWithStale(t.Context(), client, store, server.URL, nil, 0)
	if err == nil {
		t.Fatal("CachedGetWithStale(404, no cache) error = nil, want error")
	}
	if stale || body != nil {
		t.Fatalf("got (%q, stale=%v), want (nil, false)", body, stale)
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "http_cache.db"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	const goroutines = 20
	const iterations = 25

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)
	for i := range goroutines {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := range iterations {
				url := fmt.Sprintf("https://example.com/feed/%d", j%5)
				validators := api.CacheValidators{ETag: fmt.Sprintf(`"worker-%d-%d"`, worker, j)}
				if err := store.Save(url, validators); err != nil {
					errCh <- err
					return
				}
				if got, ok := store.Get(url); !ok || got.ETag == "" {
					errCh <- fmt.Errorf("Get(%s) = (%#v, %v), want validator", url, got, ok)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}
