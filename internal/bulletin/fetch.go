package bulletin

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/lepinkainen/feed-forge/pkg/api"
	"github.com/lepinkainen/feed-forge/pkg/httpcache"
)

// politeDelay is the pause between successive article-page fetches, providing
// basic per-run pacing since the shared generic client is not rate limited.
const politeDelay = 500 * time.Millisecond

// feedMaxStale bounds how long a cached feed body may be served if upstream
// fails; article pages are immutable once stored so they use the same bound.
const feedMaxStale = 24 * time.Hour

var tagStripper = regexp.MustCompile(`<[^>]*>`)

// Fetch polls every configured source feed, extracts full text for new entries,
// computes their SimHash, and stores them as unpublished items. Existing items
// (by URL) are skipped without re-fetching their article page.
func Fetch(ctx context.Context, cfg Config, dbPath string) error {
	cfg = cfg.withDefaults()
	if len(cfg.Feeds) == 0 {
		slog.Warn("bulletin: no source feeds configured")
		return nil
	}

	store, err := NewStore(dbPath) //nolint:contextcheck // connection bootstrap, not request-scoped I/O
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	cacheStore, err := httpcache.NewStore(httpcacheDBPath(dbPath)) //nolint:contextcheck // connection bootstrap, not request-scoped I/O
	if err != nil {
		return err
	}
	defer func() { _ = cacheStore.Close() }()

	client := api.NewGenericClient()
	client.SetUserAgent(userAgent)
	parser := gofeed.NewParser()

	var inserted, skipped int
	for _, src := range cfg.Feeds {
		if err := ctx.Err(); err != nil {
			return err
		}
		body, _, err := httpcache.CachedGetWithStale(ctx, client, cacheStore, src.URL, nil, feedMaxStale)
		if err != nil {
			if errors.Is(err, httpcache.ErrNotModified) {
				slog.Debug("bulletin: feed unchanged", "feed", src.URL)
				continue
			}
			slog.Warn("bulletin: fetch feed failed", "feed", src.URL, "error", err)
			continue
		}

		feed, err := parser.Parse(strings.NewReader(string(body)))
		if err != nil {
			slog.Warn("bulletin: parse feed failed", "feed", src.URL, "error", err)
			continue
		}

		ins, skp, err := ingestEntries(ctx, store, client, cacheStore, src, feed.Items)
		inserted += ins
		skipped += skp
		if err != nil {
			return err
		}
	}

	slog.Info("bulletin: fetch complete", "inserted", inserted, "skipped", skipped)
	return nil
}

// ingestEntries stores the new entries of one source feed, fetching each new
// entry's article page and pacing between page fetches. It returns the counts
// accumulated before any error.
func ingestEntries(ctx context.Context, store *Store, client *api.EnhancedClient, cacheStore *httpcache.Store, src FeedSource, entries []*gofeed.Item) (inserted, skipped int, err error) {
	for _, entry := range entries {
		link := entry.Link
		if link == "" {
			continue
		}
		has, err := store.HasItem(ctx, link)
		if err != nil {
			return inserted, skipped, err
		}
		if has {
			skipped++
			continue
		}

		text := fetchArticleText(ctx, client, cacheStore, link, entry)
		// Pace after every article fetch, including ones that yielded no
		// usable text: the network request already hit the origin.
		select {
		case <-ctx.Done():
			return inserted, skipped, ctx.Err()
		case <-time.After(politeDelay):
		}
		if text == "" {
			continue
		}

		ok, err := store.InsertItem(ctx, Item{
			FeedURL:   src.URL,
			FeedName:  src.Name,
			URL:       link,
			Title:     entry.Title,
			RawText:   text,
			SimHash:   SimHash(entry.Title + " " + text),
			FetchedAt: time.Now().UTC(),
		})
		if err != nil {
			return inserted, skipped, err
		}
		if ok {
			inserted++
		}
	}
	return inserted, skipped, nil
}

// fetchArticleText fetches and extracts the article body, falling back to the
// feed-provided content/description if extraction yields too little.
func fetchArticleText(ctx context.Context, client *api.EnhancedClient, cacheStore *httpcache.Store, link string, entry *gofeed.Item) string {
	body, _, err := httpcache.CachedGetWithStale(ctx, client, cacheStore, link, nil, feedMaxStale)
	// On a transient upstream failure CachedGetWithStale may still return a usable
	// stale cached body; prefer extracting that over the (often truncated) feed
	// summary. Only fall back to the feed content when there is no body at all.
	if err != nil && !errors.Is(err, httpcache.ErrNotModified) && len(body) == 0 {
		slog.Debug("bulletin: fetch article failed, using feed content", "url", link, "error", err)
		return feedFallbackText(entry)
	}

	text, err := extractText(link, body)
	if err != nil {
		slog.Debug("bulletin: extraction failed, using feed content", "url", link, "error", err)
		return feedFallbackText(entry)
	}
	return text
}

// feedFallbackText derives plain text from the feed entry itself when full-text
// extraction is unavailable.
func feedFallbackText(entry *gofeed.Item) string {
	raw := entry.Content
	if raw == "" {
		raw = entry.Description
	}
	return strings.TrimSpace(tagStripper.ReplaceAllString(raw, " "))
}
