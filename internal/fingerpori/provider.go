package fingerpori

import (
	"log/slog"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// Provider implements the FeedProvider interface for Fingerpori comics
type Provider struct {
	*providers.BaseProvider
	Limit int
}

// NewProvider creates a new Fingerpori provider
func NewProvider(limit int) providers.FeedProvider {
	// Fingerpori doesn't need a database (no caching required)
	base, err := providers.NewBaseProvider(providers.DatabaseConfig{
		ContentDBName: "",
		UseContentDB:  false,
	})
	if err != nil {
		slog.Error("Failed to initialize Fingerpori base provider", "error", err)
		return nil
	}

	return &Provider{
		BaseProvider: base,
		Limit:        limit,
	}
}

// GenerateFeed implements the FeedProvider interface
func (p *Provider) GenerateFeed(outfile string, reauth bool) error {
	slog.Debug("Generating Fingerpori feed")

	// Fetch items from the API
	items, err := fetchItems()
	if err != nil {
		return err
	}

	// Process items to add computed fields
	items = processItems(items)

	// Apply limit if specified
	if p.Limit > 0 && len(items) > p.Limit {
		items = items[:p.Limit]
	}

	// Ensure output directory exists
	if dirErr := filesystem.EnsureDirectoryExists(outfile); dirErr != nil {
		return dirErr
	}

	// Convert to FeedItem interface
	feedItems := convertToFeedItems(items)

	// Define feed configuration
	feedConfig := feed.Config{
		Title:       "Fingerpori Comics",
		Link:        "https://www.hs.fi/fingerpori/",
		Description: "Daily Fingerpori comics from Helsingin Sanomat",
		Author:      "Pertti Jarla",
		ID:          "https://www.hs.fi/fingerpori/",
	}

	// Generate and save the feed using embedded templates
	// Note: We don't use OpenGraph DB for comic images
	if err := feed.SaveAtomFeedToFileWithEmbeddedTemplate(feedItems, "fingerpori-atom", outfile, feedConfig, nil); err != nil {
		return err
	}

	feed.LogFeedGeneration(len(items), outfile)
	return nil
}

// convertToFeedItems wraps Fingerpori items with the FeedItem interface
func convertToFeedItems(items []Item) []providers.FeedItem {
	feedItems := make([]providers.FeedItem, len(items))
	for i := range items {
		feedItems[i] = &items[i]
	}
	return feedItems
}
