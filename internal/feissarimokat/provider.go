package feissarimokat

import (
	"fmt"
	"log/slog"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// Provider implements the FeedProvider interface for Feissarimokat
type Provider struct {
	*providers.BaseProvider
}

// Config holds Feissarimokat provider configuration for the factory
type Config struct {
	providers.GenerateConfig `yaml:",inline"`
}

// NewProvider creates a new Feissarimokat provider
func NewProvider() providers.FeedProvider {
	base, err := providers.NewBaseProvider(providers.DatabaseConfig{
		ContentDBName: "",
		UseContentDB:  false,
	})
	if err != nil {
		slog.Error("Failed to initialize Feissarimokat base provider", "error", err)
		return nil
	}

	return &Provider{
		BaseProvider: base,
	}
}

// factory creates a Feissarimokat provider from configuration
func factory(config any) (providers.FeedProvider, error) {
	_, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type for feissarimokat provider: expected *feissarimokat.Config")
	}

	provider := NewProvider()
	if provider == nil {
		return nil, fmt.Errorf("failed to create feissarimokat provider")
	}

	return provider, nil
}

func init() {
	providers.MustRegister("feissarimokat", &providers.ProviderInfo{
		Name:        "feissarimokat",
		Description: "Generate RSS feeds from Feissarimokat comics",
		Version:     "1.0.0",
		Factory:     factory,
		ConfigFactory: func() any {
			return &Config{}
		},
		Preview: &providers.PreviewInfo{
			ProviderName: "Feissarimokat",
			TemplateName: "feissarimokat-atom",
			FeedTitle:    "Feissarimokat",
			FeedLink:     "https://www.feissarimokat.com/",
			Description:  "Feissarimokat comics with embedded images",
			Author:       "Feissarimokat",
			FeedID:       "https://www.feissarimokat.com/",
		},
	})
}

// FetchItems implements the FeedProvider interface
func (p *Provider) FetchItems(limit int) ([]providers.FeedItem, error) {
	slog.Debug("Fetching Feissarimokat items")

	rssItems, err := fetchRSSFeed()
	if err != nil {
		return nil, err
	}

	items := processItems(rssItems)

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	return convertToFeedItems(items), nil
}

// GenerateFeed implements the FeedProvider interface
func (p *Provider) GenerateFeed(outfile string, _ bool) error {
	slog.Debug("Generating Feissarimokat feed")

	feedItems, err := p.FetchItems(0)
	if err != nil {
		return err
	}

	if dirErr := filesystem.EnsureDirectoryExists(outfile); dirErr != nil {
		return dirErr
	}

	feedConfig := feed.Config{
		Title:       "Feissarimokat",
		Link:        "https://www.feissarimokat.com/",
		Description: "Feissarimokat comics with embedded images",
		Author:      "Feissarimokat",
		ID:          "https://www.feissarimokat.com/",
	}

	if err := feed.SaveAtomFeedToFileWithEmbeddedTemplate(feedItems, "feissarimokat-atom", outfile, feedConfig, nil); err != nil {
		return err
	}

	feed.LogFeedGeneration(len(feedItems), outfile)
	return nil
}

// convertToFeedItems wraps items with the FeedItem interface
func convertToFeedItems(items []Item) []providers.FeedItem {
	feedItems := make([]providers.FeedItem, len(items))
	for i := range items {
		feedItems[i] = &items[i]
	}
	return feedItems
}
