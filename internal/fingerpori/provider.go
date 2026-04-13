package fingerpori

import (
	"fmt"
	"log/slog"

	"github.com/lepinkainen/feed-forge/pkg/feedmeta"
	"github.com/lepinkainen/feed-forge/pkg/providerfeed"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

var previewInfo = &providers.PreviewInfo{
	Config: feedmeta.Config{
		Title:       "Fingerpori Comics",
		Link:        "https://www.hs.fi/fingerpori/",
		Description: "Daily Fingerpori comics from Helsingin Sanomat",
		Author:      "Pertti Jarla",
		ID:          "https://www.hs.fi/fingerpori/",
	},
	ProviderName: "Fingerpori",
	TemplateName: "fingerpori-atom",
}

// Provider implements the FeedProvider interface for Fingerpori comics
type Provider struct {
	*providers.BaseProvider
	Limit int
}

// Config holds Fingerpori provider configuration for the factory
type Config struct {
	providers.GenerateConfig `yaml:",inline"`
	Limit                    int `yaml:"limit"`
}

// NewProvider creates a new Fingerpori provider
func NewProvider(limit int) (providers.FeedProvider, error) {
	// Fingerpori doesn't need a database (no caching required)
	base, err := providers.NewBaseProvider(providers.DatabaseConfig{
		ContentDBName: "",
		UseContentDB:  false,
	})
	if err != nil {
		slog.Error("Failed to initialize Fingerpori base provider", "error", err)
		return nil, fmt.Errorf("initialize fingerpori base provider: %w", err)
	}

	provider := &Provider{
		BaseProvider: base,
		Limit:        limit,
	}
	provider.SetGenerateFeedFunc(providerfeed.BuildGenerator(provider.FetchItems, previewInfo, nil, nil))

	return provider, nil
}

// factory creates a Fingerpori provider from configuration
func factory(config any) (providers.FeedProvider, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type for fingerpori provider: expected *fingerpori.Config")
	}

	provider, err := NewProvider(cfg.Limit)
	if err != nil {
		return nil, fmt.Errorf("create fingerpori provider: %w", err)
	}

	return provider, nil
}

func init() {
	providers.MustRegister("fingerpori", &providers.ProviderInfo{
		Name:        "fingerpori",
		Description: "Generate RSS feeds from Fingerpori comics",
		Version:     "1.0.0",
		Factory:     factory,
		ConfigFactory: func() any {
			return &Config{
				Limit: 100,
			}
		},
		Preview: previewInfo,
	})
}

// FetchItems implements the FeedProvider interface
func (p *Provider) FetchItems(limit int) ([]providers.FeedItem, error) {
	slog.Debug("Fetching Fingerpori items")

	// Fetch items from the API
	items, err := fetchItems()
	if err != nil {
		return nil, err
	}

	// Process items to add computed fields
	items = processItems(items)

	// Use provided limit or fall back to provider's default
	itemLimit := limit
	if itemLimit == 0 {
		itemLimit = p.Limit
	}

	// Apply limit if specified
	if itemLimit > 0 && len(items) > itemLimit {
		items = items[:itemLimit]
	}

	// Convert to FeedItem interface
	return convertToFeedItems(items), nil
}

// convertToFeedItems wraps Fingerpori items with the FeedItem interface
func convertToFeedItems(items []Item) []providers.FeedItem {
	feedItems := make([]providers.FeedItem, len(items))
	for i := range items {
		feedItems[i] = &items[i]
	}
	return feedItems
}
