package feed

import (
	"log/slog"

	"github.com/lepinkainen/feed-forge/pkg/opengraph"
)

// EnhancedFeedGenerator provides common feed generation utilities for providers
type EnhancedFeedGenerator struct {
	ogFetcher *opengraph.Fetcher
}

// NewEnhancedFeedGenerator creates a new enhanced feed generator
func NewEnhancedFeedGenerator(ogDB *opengraph.Database) *EnhancedFeedGenerator {
	return &EnhancedFeedGenerator{
		ogFetcher: opengraph.NewFetcher(ogDB),
	}
}

// GetOpenGraphFetcher returns the OpenGraph fetcher for use by providers
func (efg *EnhancedFeedGenerator) GetOpenGraphFetcher() *opengraph.Fetcher {
	return efg.ogFetcher
}

// LogFeedGeneration logs the completion of feed generation
func LogFeedGeneration(itemCount int, filename string) {
	slog.Info("RSS feed saved", "count", itemCount, "filename", filename)
}
