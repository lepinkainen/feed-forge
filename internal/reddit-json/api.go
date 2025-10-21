// Package redditjson provides a provider for fetching Reddit JSON feeds.
package redditjson

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

// RedditAPI handles Reddit JSON feed interactions using enhanced HTTP client
type RedditAPI struct {
	client  *api.EnhancedClient
	feedURL string
}

// NewRedditAPI creates a new Reddit API client for JSON feed access
func NewRedditAPI(feedURL string) *RedditAPI {
	enhancedClient := api.NewGenericClient()
	enhancedClient.SetUserAgent("feed-forge/1.0 (by /u/feedforge)")

	return &RedditAPI{
		client:  enhancedClient,
		feedURL: feedURL,
	}
}

// FetchRedditHomepage fetches posts from the user's JSON feed
func (r *RedditAPI) FetchRedditHomepage() ([]RedditPost, error) {
	var listing RedditListing

	// User-Agent is already set on the client
	err := r.client.GetAndDecode(r.feedURL, &listing, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Reddit JSON feed: %w", err)
	}

	slog.Debug("Successfully fetched Reddit JSON feed posts", "count", len(listing.Data.Children))
	return listing.Data.Children, nil
}

// FetchConcurrentHomepage fetches posts (single page for JSON feed)
func (r *RedditAPI) FetchConcurrentHomepage(_ int) ([]RedditPost, error) {
	// JSON feed is a single page, so just return the main fetch
	return r.FetchRedditHomepage()
}

// FilterPosts applies score and comment count filters to a list of Reddit posts
func FilterPosts(posts []RedditPost, minScore, minComments int) []RedditPost {
	var filtered []RedditPost
	for _, post := range posts {
		if post.Data.Score >= minScore && post.Data.NumComments >= minComments {
			filtered = append(filtered, post)
		}
	}

	slog.Debug("Filtered posts", "original", len(posts), "filtered", len(filtered), "minScore", minScore, "minComments", minComments)
	return filtered
}

// ValidateAPIResponse validates the structure of Reddit API responses
func ValidateAPIResponse(listing *RedditListing) error {
	if listing == nil {
		return fmt.Errorf("nil listing received")
	}

	if listing.Data.Children == nil {
		return fmt.Errorf("nil children in listing")
	}

	return nil
}

// UpdateStats updates API call statistics (placeholder for future implementation)
func UpdateStats(endpoint string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}

	slog.Debug("API call completed",
		"endpoint", endpoint,
		"duration", duration,
		"status", status,
	)
}
