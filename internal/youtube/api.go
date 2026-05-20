package youtube

import (
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

const channelFeedURLFormat = "https://www.youtube.com/feeds/videos.xml?channel_id=%s"

func fetchAtomFeed(feedURL string) (*atomFeed, error) {
	slog.Debug("Fetching YouTube Atom feed", "url", feedURL)

	client := api.NewGenericClient()
	resp, err := client.Get(feedURL, map[string]string{"Accept": "application/atom+xml, application/xml;q=0.9, */*;q=0.8"})
	if err != nil {
		return nil, fmt.Errorf("fetch youtube feed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read youtube response: %w", err)
	}

	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse youtube atom: %w", err)
	}

	slog.Debug("Parsed YouTube Atom feed", "url", feedURL, "entries", len(feed.Entries))
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
