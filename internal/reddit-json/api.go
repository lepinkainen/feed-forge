// Package redditjson provides a provider for fetching Reddit JSON feeds.
package redditjson

import (
	"fmt"
	"log/slog"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

// RedditAPI handles Reddit JSON feed interactions using enhanced HTTP client
type RedditAPI struct {
	client  *api.EnhancedClient
	feedURL string
}

// NewRedditAPI creates a new Reddit API client for JSON feed access
func NewRedditAPI(feedURL string) *RedditAPI {
	enhancedClient := api.NewRedditClient(nil)
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
