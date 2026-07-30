package lobsters

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

// listingURLs returns the lobste.rs JSON listing URLs to fetch for the given
// tags. With no tags it returns the default "hottest" listing; otherwise one
// per-tag listing URL for each non-empty, trimmed tag.
func listingURLs(tags []string) []string {
	if len(tags) == 0 {
		return []string{"https://lobste.rs/hottest.json"}
	}

	urls := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		urls = append(urls, fmt.Sprintf("https://lobste.rs/t/%s.json", trimmed))
	}
	if len(urls) == 0 {
		return []string{"https://lobste.rs/hottest.json"}
	}
	return urls
}

// fetchStories retrieves and parses a lobste.rs JSON story listing.
func fetchStories(listingURL string) ([]*Item, error) {
	slog.Debug("Fetching lobsters listing", "url", listingURL)

	client := api.NewGenericClient()
	resp, err := client.Get(listingURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch lobsters feed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read lobsters response: %w", err)
	}

	var stories []story
	if err := json.Unmarshal(body, &stories); err != nil {
		return nil, fmt.Errorf("parse lobsters json: %w", err)
	}

	items := make([]*Item, 0, len(stories))
	for _, s := range stories {
		createdAt, err := time.Parse(time.RFC3339, s.CreatedAt)
		if err != nil {
			slog.Debug("Failed to parse lobsters created_at, skipping item", "short_id", s.ShortID, "created_at", s.CreatedAt, "error", err)
			continue
		}
		items = append(items, &Item{story: s, createdAt: createdAt})
	}

	slog.Debug("Parsed lobsters listing", "stories", len(items))
	return items, nil
}
