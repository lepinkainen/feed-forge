package hackernews

import (
	"log/slog"
	"regexp"

	"github.com/lepinkainen/feed-forge/pkg/database"
	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// Provider implements the FeedProvider interface for Hacker News
type Provider struct {
	*providers.BaseProvider
	MinPoints      int
	Limit          int
	CategoryMapper *CategoryMapper
	databases      *database.ProviderDatabases
}

// NewProvider creates a new HackerNews provider
func NewProvider(minPoints, limit int, categoryMapper *CategoryMapper) providers.FeedProvider {
	// Initialize CategoryMapper if not provided
	if categoryMapper == nil {
		categoryMapper = LoadConfig("") // Use default configuration
	}

	// Initialize databases
	databases, err := database.InitializeProviderDatabases("hackernews.db", true)
	if err != nil {
		slog.Error("Failed to initialize Hacker News databases", "error", err)
		return nil
	}

	base, err := providers.NewBaseProvider(providers.DatabaseConfig{
		ContentDBName: "hackernews.db",
		UseContentDB:  true,
	})
	if err != nil {
		slog.Error("Failed to initialize Hacker News base provider", "error", err)
		if closeErr := databases.Close(); closeErr != nil {
			slog.Error("Failed to close databases", "error", closeErr)
		}
		return nil
	}

	return &Provider{
		BaseProvider:   base,
		MinPoints:      minPoints,
		Limit:          limit,
		CategoryMapper: categoryMapper,
		databases:      databases,
	}
}

// GenerateFeed implements the FeedProvider interface
func (p *Provider) GenerateFeed(outfile string, reauth bool) error {
	// Clean up expired entries using base provider
	if err := p.CleanupExpired(); err != nil {
		slog.Warn("Failed to cleanup expired entries", "error", err)
	}

	// Get database connections
	contentDB := p.databases.ContentDB
	ogDB := p.databases.OpenGraphDB

	// Fetch current front page items
	newItems := fetchItems()

	// Initialize database schema
	if err := initializeSchema(contentDB); err != nil {
		return err
	}

	// Update database with new items and get list of updated item IDs
	recentlyUpdated := updateStoredItems(contentDB, newItems)

	// Get all items from database
	allItems, err := getAllItems(contentDB, p.Limit, p.MinPoints)
	if err != nil {
		return err
	}

	// Update item stats with current data from Algolia, skipping recently updated items
	updateItemStats(contentDB.DB(), allItems, recentlyUpdated)

	// Re-fetch items to get updated stats for RSS generation
	allItems, err = getAllItems(contentDB, p.Limit, p.MinPoints)
	if err != nil {
		return err
	}

	// Ensure output directory exists
	if dirErr := filesystem.EnsureDirectoryExists(outfile); dirErr != nil {
		return dirErr
	}

	// Process items to add HackerNews-specific categorization
	preprocessedItems := preprocessItems(allItems, p.MinPoints, p.CategoryMapper)

	// Convert to FeedItem interface
	feedItems := make([]providers.FeedItem, len(preprocessedItems))
	for i, item := range preprocessedItems {
		feedItems[i] = &item
	}

	// Define feed configuration
	feedConfig := feed.Config{
		Title:       "Hacker News Top Stories",
		Link:        "https://news.ycombinator.com/",
		Description: "High-quality Hacker News stories, updated regularly",
		Author:      "Feed Forge",
		ID:          "https://news.ycombinator.com/",
	}

	// Generate and save the feed using embedded templates with local override
	if err := feed.SaveAtomFeedToFileWithEmbeddedTemplate(feedItems, "hackernews-atom", outfile, feedConfig, ogDB); err != nil {
		return err
	}

	feed.LogFeedGeneration(len(allItems), outfile)
	return nil
}

// preprocessItems applies HackerNews-specific categorization and metadata
func preprocessItems(items []Item, minPoints int, categoryMapper *CategoryMapper) []Item {
	domainRegex := regexp.MustCompile(`^https?://([^/]+)`)

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
