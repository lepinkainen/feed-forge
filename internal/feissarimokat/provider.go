package feissarimokat

import (
	"fmt"
	"log/slog"

	"github.com/lepinkainen/feed-forge/pkg/feedmeta"
	"github.com/lepinkainen/feed-forge/pkg/providerfeed"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

var previewInfo = &providers.PreviewInfo{
	Config: feedmeta.Config{
		Title:       "Feissarimokat",
		Link:        "https://www.feissarimokat.com/",
		Description: "Feissarimokat comics with embedded images",
		Author:      "Feissarimokat",
		ID:          "https://www.feissarimokat.com/",
	},
	ProviderName: "Feissarimokat",
	TemplateName: "feissarimokat-atom",
}

// Provider implements the FeedProvider interface for Feissarimokat
type Provider struct {
	*providers.BaseProvider
}

// Config holds Feissarimokat provider configuration for the factory
type Config struct {
	providers.GenerateConfig `yaml:",inline"`
}

// NewProvider creates a new Feissarimokat provider
func NewProvider() (providers.FeedProvider, error) {
	base, err := providers.NewBaseProvider(providers.DatabaseConfig{
		ContentDBName: "",
		UseContentDB:  false,
	})
	if err != nil {
		slog.Error("Failed to initialize Feissarimokat base provider", "error", err)
		return nil, fmt.Errorf("initialize feissarimokat base provider: %w", err)
	}

	provider := &Provider{
		BaseProvider: base,
	}
	provider.SetGenerateFeedFunc(providerfeed.BuildGenerator(provider.FetchItems, previewInfo, nil, nil))

	return provider, nil
}

// factory creates a Feissarimokat provider from configuration
func factory(config any) (providers.FeedProvider, error) {
	_, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type for feissarimokat provider: expected *feissarimokat.Config")
	}

	provider, err := NewProvider()
	if err != nil {
		return nil, fmt.Errorf("create feissarimokat provider: %w", err)
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
		Preview: previewInfo,
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

// convertToFeedItems wraps items with the FeedItem interface
func convertToFeedItems(items []Item) []providers.FeedItem {
	feedItems := make([]providers.FeedItem, len(items))
	for i := range items {
		feedItems[i] = &items[i]
	}
	return feedItems
}
