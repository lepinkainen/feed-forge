package youtube

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
	"github.com/lepinkainen/feed-forge/pkg/httpcache"
)

const channelFeedURLFormat = "https://www.youtube.com/feeds/videos.xml?channel_id=%s"

// maxStaleAge bounds how long a cached feed copy may paper over upstream
// failures. Feeds are checked at most daily, so two days of consecutive
// failures means the feed is genuinely broken and should surface an error.
const maxStaleAge = 48 * time.Hour

// fetchAtomFeed fetches and parses a YouTube channel Atom feed. The last good
// response is cached in store; when YouTube returns an error (notably the
// intermittent 404 it serves the legacy feed endpoint from datacenter IPs), the
// cached copy is parsed instead so the run degrades gracefully rather than
// failing. Once the cached copy is older than maxStaleAge the error surfaces.
func fetchAtomFeed(store *httpcache.Store, feedURL string) (*atomFeed, error) {
	slog.Debug("Fetching YouTube Atom feed", "url", feedURL)

	client := api.NewGenericClient()
	headers := map[string]string{"Accept": "application/atom+xml, application/xml;q=0.9, */*;q=0.8"}
	body, stale, err := httpcache.CachedGetWithStale(context.Background(), client, store, feedURL, headers, maxStaleAge)
	if err != nil {
		if !stale || len(body) == 0 {
			return nil, fmt.Errorf("fetch youtube feed: %w", err)
		}
		// Transient upstream failure absorbed by cache: log below Warn so
		// cron output stays quiet (its stderr is forwarded to Discord).
		slog.Info("YouTube feed fetch failed; serving cached copy", "url", feedURL, "error", err)
	}

	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse youtube atom: %w", err)
	}

	slog.Debug("Parsed YouTube Atom feed", "url", feedURL, "entries", len(feed.Entries), "stale", stale)
	return &feed, nil
}

func channelFeedURL(channelID string) string {
	return fmt.Sprintf(channelFeedURLFormat, strings.TrimSpace(channelID))
}

func normalizeFeedURLs(feedURL string, feedURLs, channelIDs []string) []string {
	seen := make(map[string]struct{}, len(feedURLs)+len(channelIDs)+1)
	out := make([]string, 0, len(feedURLs)+len(channelIDs)+1)
	add := func(raw string) {
		normalized := strings.TrimSpace(raw)
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	add(feedURL)
	for _, candidate := range feedURLs {
		add(candidate)
	}
	for _, channelID := range channelIDs {
		id := strings.TrimSpace(channelID)
		if id != "" {
			add(channelFeedURL(id))
		}
	}
	return out
}

func isYouTubeFeedURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.Contains(u.Host, "youtube.com") && u.Path == "/feeds/videos.xml"
}
