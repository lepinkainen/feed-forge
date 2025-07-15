package feed

import (
	"log/slog"
	"net/http"

	"github.com/lepinkainen/feed-forge/pkg/opengraph"
)

// EnhancedFeedGenerator provides common feed generation utilities for providers
type EnhancedFeedGenerator struct {
	OGFetcher *opengraph.Fetcher
}

// NewEnhancedFeedGenerator creates a new enhanced feed generator
func NewEnhancedFeedGenerator(ogDB *opengraph.Database) *EnhancedFeedGenerator {
	return &EnhancedFeedGenerator{
		OGFetcher: opengraph.NewFetcher(ogDB),
	}
}

// NewEnhancedFeedGeneratorWithRedditClient creates a new enhanced feed generator with an authenticated Reddit client
func NewEnhancedFeedGeneratorWithRedditClient(ogDB *opengraph.Database, redditClient *http.Client) *EnhancedFeedGenerator {
	return &EnhancedFeedGenerator{
		OGFetcher: opengraph.NewFetcherWithRedditClient(ogDB, redditClient),
	}
}

// LogFeedGeneration logs the completion of feed generation
func LogFeedGeneration(itemCount int, filename string) {
	slog.Debug("RSS feed saved", "count", itemCount, "filename", filename)
}
