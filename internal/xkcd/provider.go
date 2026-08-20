package xkcd

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/lepinkainen/feed-forge/pkg/feedmeta"
	"github.com/lepinkainen/feed-forge/pkg/httpcache"
	"github.com/lepinkainen/feed-forge/pkg/providerfeed"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

var previewInfo = &providers.PreviewInfo{
	Config: feedmeta.Config{
		Title:       "xkcd",
		Link:        "https://xkcd.com/",
		Description: "xkcd comics with the mouseover caption as visible text",
		Author:      "xkcd",
		ID:          "https://xkcd.com/rss.xml",
	},
	ProviderName: "xkcd",
	TemplateName: "xkcd-atom",
}

// Provider implements providers.FeedProvider for the xkcd RSS feed.
type Provider struct {
	*providers.BaseProvider
}

// Config is the YAML/CLI configuration for the xkcd provider.
type Config struct {
	providers.GenerateConfig `yaml:",inline"`
}

func factory(config any) (providers.FeedProvider, error) {
	if _, ok := config.(*Config); !ok {
		return nil, fmt.Errorf("invalid config type for xkcd provider: expected *xkcd.Config")
	}
	return NewProvider()
}

func init() {
	providers.MustRegister("xkcd", &providers.ProviderInfo{
		Name:        "xkcd",
		Description: "Generate RSS feeds from xkcd comics",
		Version:     "1.0.0",
		Factory:     factory,
		ConfigFactory: func() any {
			return &Config{
				GenerateConfig: providers.GenerateConfig{Interval: "6h"},
			}
		},
		Preview: previewInfo,
	})
}

// NewProvider creates a stateless xkcd provider.
func NewProvider() (providers.FeedProvider, error) {
	base, err := providers.NewBaseProvider(providers.DatabaseConfig{
		UseContentDB: false, // xkcd feed is small and re-fetched each run
	})
	if err != nil {
		slog.Error("Failed to create base provider for xkcd", "error", err)
		return nil, fmt.Errorf("initialize xkcd base provider: %w", err)
	}

	p := &Provider{BaseProvider: base}
	p.SetGenerateFeedFunc(providerfeed.BuildGenerator(p.FetchItems, previewInfo, nil, nil))
	return p, nil
}

func (p *Provider) httpCacheStore() *httpcache.Store {
	if p == nil || p.BaseProvider == nil {
		return nil
	}
	return p.HTTPCache
}

// FetchItems fetches the xkcd RSS feed, newest first, honoring limit (0 = all).
func (p *Provider) FetchItems(limit int) ([]providers.FeedItem, error) {
	items, err := fetchItems(p.httpCacheStore())
	if err != nil {
		return nil, err
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt().After(items[j].CreatedAt())
	})

	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	feedItems := make([]providers.FeedItem, len(items))
	for i := range items {
		feedItems[i] = &items[i]
	}
	return feedItems, nil
}
