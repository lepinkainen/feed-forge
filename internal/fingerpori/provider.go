package fingerpori

import (
	"fmt"
	"log/slog"

	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// Provider implements the FeedProvider interface for Fingerpori comics
type Provider struct {
	*providers.BaseProvider
	Limit int
}

// Config holds Fingerpori provider configuration for the factory
type Config struct {
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

// factory creates a Fingerpori provider from configuration
func factory(config any) (providers.FeedProvider, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type for fingerpori provider: expected *fingerpori.Config")
	}

	provider := NewProvider(cfg.Limit)
	if provider == nil {
		return nil, fmt.Errorf("failed to create fingerpori provider")
	}

	return provider, nil
}

func init() {
	providers.MustRegister("fingerpori", &providers.ProviderInfo{
		Name:        "fingerpori",
		Description: "Generate RSS feeds from Fingerpori comics",
		Version:     "1.0.0",
		Factory:     factory,
	})
}

// Metadata returns feed metadata for the Fingerpori provider
func (p *Provider) Metadata() providers.FeedMetadata {
	return providers.FeedMetadata{
		Title:        "Fingerpori Comics",
		Link:         "https://www.hs.fi/fingerpori/",
		Description:  "Daily Fingerpori comics from Helsingin Sanomat",
		Author:       "Pertti Jarla",
		ID:           "https://www.hs.fi/fingerpori/",
		TemplateName: "fingerpori-atom",
	}
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

	// Convert to FeedItem interface using generic helper
	return providers.ConvertToFeedItems(items), nil
}

// GenerateFeed implements the FeedProvider interface
func (p *Provider) GenerateFeed(outfile string, _ bool) error {
	slog.Debug("Generating Fingerpori feed")
	// Note: Fingerpori doesn't use OpenGraph DB (OgDB is nil)
	// Delegate to BaseProvider's common implementation
	return p.BaseProvider.GenerateFeed(p, outfile)
}
