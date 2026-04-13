package hackernews

import (
	"fmt"
	"log/slog"
	"regexp"

	"github.com/lepinkainen/feed-forge/pkg/feedmeta"
	"github.com/lepinkainen/feed-forge/pkg/providerfeed"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

var (
	domainRegex = regexp.MustCompile(`^https?://([^/]+)`)
	previewInfo = &providers.PreviewInfo{
		Config: feedmeta.Config{
			Title:       "Hacker News Top Stories",
			Link:        "https://news.ycombinator.com/",
			Description: "High-quality Hacker News stories, updated regularly",
			Author:      "Feed Forge",
			ID:          "https://news.ycombinator.com/",
		},
		ProviderName: "Hacker News",
		TemplateName: "hackernews-atom",
	}
)

// Provider implements the FeedProvider interface for Hacker News
type Provider struct {
	*providers.BaseProvider
	MinPoints      int
	Limit          int
	CategoryMapper *CategoryMapper
}

// Config holds HackerNews provider configuration for the factory
type Config struct {
	providers.GenerateConfig `yaml:",inline"`
	MinPoints                int `yaml:"min-points"`
	Limit                    int `yaml:"limit"`
}

// NewProvider creates a new HackerNews provider
func NewProvider(minPoints, limit int, categoryMapper *CategoryMapper) (providers.FeedProvider, error) {
	// Initialize CategoryMapper if not provided
	if categoryMapper == nil {
		categoryMapper = LoadConfig("") // Use default configuration
	}

	base, err := providers.NewBaseProvider(providers.DatabaseConfig{
		ContentDBName: "hackernews.db",
		UseContentDB:  true,
	})
	if err != nil {
		slog.Error("Failed to initialize Hacker News base provider", "error", err)
		return nil, fmt.Errorf("initialize hackernews base provider: %w", err)
	}

	provider := &Provider{
		BaseProvider:   base,
		MinPoints:      minPoints,
		Limit:          limit,
		CategoryMapper: categoryMapper,
	}
	provider.SetGenerateFeedFunc(providerfeed.BuildGenerator(provider.FetchItems, previewInfo, nil, provider.OgDB))

	return provider, nil
}

// factory creates a HackerNews provider from configuration
func factory(config any) (providers.FeedProvider, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type for hackernews provider: expected *hackernews.Config")
	}

	provider, err := NewProvider(cfg.MinPoints, cfg.Limit, nil)
	if err != nil {
		return nil, fmt.Errorf("create hackernews provider: %w", err)
	}

	return provider, nil
}

func init() {
	providers.MustRegister("hackernews", &providers.ProviderInfo{
		Name:        "hackernews",
		Description: "Generate RSS feeds from Hacker News top stories",
		Version:     "1.0.0",
		Factory:     factory,
		ConfigFactory: func() any {
			return &Config{
				MinPoints: 50,
				Limit:     30,
			}
		},
		Preview: previewInfo,
	})
}

// FetchItems implements the FeedProvider interface
func (p *Provider) FetchItems(limit int) ([]providers.FeedItem, error) {
	contentDB := p.ContentDB

	// Fetch current front page items
	newItems := fetchItems()

	// Initialize database schema
	if err := initializeSchema(contentDB); err != nil {
		return nil, err
	}

	// Update database with new items and get list of updated item IDs
	recentlyUpdated := updateStoredItems(contentDB, newItems)

	// Use provided limit or fall back to provider's default
	itemLimit := limit
	if itemLimit == 0 {
		itemLimit = p.Limit
	}

	// Get all items from database
	allItems, err := getAllItems(contentDB, itemLimit, p.MinPoints)
	if err != nil {
		return nil, err
	}

	// Update item stats with current data from Algolia, skipping recently updated items
	updateItemStats(contentDB.DB(), allItems, recentlyUpdated)

	// Re-fetch items to get updated stats
	allItems, err = getAllItems(contentDB, itemLimit, p.MinPoints)
	if err != nil {
		return nil, err
	}

	// Process items to add HackerNews-specific categorization
	preprocessedItems := preprocessItems(allItems, p.MinPoints, p.CategoryMapper)

	// Convert to FeedItem interface
	return convertToFeedItems(preprocessedItems), nil
}

// preprocessItems applies HackerNews-specific categorization and metadata
func preprocessItems(items []Item, minPoints int, categoryMapper *CategoryMapper) []Item {

	for i := range items {
		item := &items[i]

		// Extract domain from the article link
		domain := ""
		if matches := domainRegex.FindStringSubmatch(item.ItemLink); len(matches) > 1 {
			domain = matches[1]
		}

		// Generate HackerNews-specific categories
		categories := categorizeContent(item.ItemTitle, domain, item.ItemLink, categoryMapper)
		pointCategory := categorizeByPoints(item.Points, minPoints)
		categories = append(categories, pointCategory)

		// Populate the item's Domain and Categories fields for the FeedItem interface
		item.Domain = domain
		item.ItemCategories = categories
	}

	return items
}

// convertToFeedItems wraps preprocessed Hacker News items with the FeedItem interface.
func convertToFeedItems(items []Item) []providers.FeedItem {
	feedItems := make([]providers.FeedItem, len(items))
	for i := range items {
		feedItems[i] = &items[i]
	}
	return feedItems
}
