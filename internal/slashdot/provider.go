// Package slashdot generates feeds from Slashdot's Atom syndication feed.
//
// The feed's story bodies are plain text: Slashdot strips inline links and
// markup before publishing, and paragraph breaks survive only as blank lines.
// parseEntry rebuilds the paragraphs. The links cannot be recovered without
// fetching every story page, and this provider does not scrape Slashdot pages.
// One feed request per interval is the load Slashdot asks feed readers for.
package slashdot

import (
	"fmt"
	"sort"

	"github.com/lepinkainen/feed-forge/pkg/feedmeta"
	"github.com/lepinkainen/feed-forge/pkg/providerfeed"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

const defaultFeedURL = "https://rss.slashdot.org/Slashdot/slashdotMainatom"

var previewInfo = &providers.PreviewInfo{
	Config: feedmeta.Config{
		Title: "Slashdot", Link: "https://slashdot.org/",
		Description: "News for nerds, stuff that matters",
		Author:      "Slashdot", ID: defaultFeedURL,
	},
	ProviderName: "Slashdot", TemplateName: "slashdot-atom",
}

// Config holds the Slashdot provider configuration.
type Config struct {
	providers.GenerateConfig `yaml:",inline"`
}

// Provider reads a Slashdot Atom feed without a content database.
type Provider struct {
	*providers.BaseProvider
	feedURL string
}

func init() {
	providers.MustRegister("slashdot", &providers.ProviderInfo{
		Name: "slashdot", Description: "Generate Atom feeds from Slashdot", Version: "1.0.0",
		Factory: factory, Preview: previewInfo,
		ConfigFactory: func() any {
			return &Config{GenerateConfig: providers.GenerateConfig{Interval: "30m"}}
		},
	})
}

func factory(config any) (providers.FeedProvider, error) {
	cfg, ok := config.(*Config)
	if !ok || cfg == nil {
		return nil, fmt.Errorf("invalid config type for slashdot provider: expected *slashdot.Config")
	}
	return NewProvider()
}

// NewProvider creates a Slashdot provider for the canonical front-page feed.
func NewProvider() (providers.FeedProvider, error) {
	base, err := providers.NewBaseProvider(providers.DatabaseConfig{UseContentDB: false})
	if err != nil {
		return nil, fmt.Errorf("initialize slashdot base provider: %w", err)
	}
	p := &Provider{BaseProvider: base, feedURL: defaultFeedURL}
	p.SetGenerateFeedFunc(providerfeed.BuildGenerator(p.FetchItems, previewInfo, nil, nil))
	return p, nil
}

// FetchItems returns stories newest first, applying limit after sorting.
func (p *Provider) FetchItems(limit int) ([]providers.FeedItem, error) {
	entries, err := fetchAtomFeed(p.feedURL, p.HTTPCacheStore())
	if err != nil {
		return nil, err
	}
	items := make([]providers.FeedItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, &Item{entry: entry})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt().After(items[j].CreatedAt()) })
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items, nil
}
