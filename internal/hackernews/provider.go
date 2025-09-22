package hackernews

import (
	"log/slog"
	"os"

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
		categoryMapper = LoadConfig("", "") // Use default configuration
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

	// Generate and save the feed
	rss, err := generateRSSFeed(contentDB.DB(), ogDB, allItems, p.MinPoints, p.CategoryMapper)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outfile, []byte(rss), 0o644); err != nil {
		return err
	}

	feed.LogFeedGeneration(len(allItems), outfile)
	return nil
}
