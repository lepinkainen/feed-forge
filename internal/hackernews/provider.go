package hackernews

import (
	"os"

	"github.com/lepinkainen/feed-forge/internal/pkg/providers"
	"github.com/lepinkainen/feed-forge/pkg/database"
	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
)

// HackerNewsProvider implements the FeedProvider interface for Hacker News
type HackerNewsProvider struct {
	*providers.BaseProvider
	MinPoints      int
	Limit          int
	CategoryMapper *CategoryMapper
	databases      *database.ProviderDatabases
}

// NewHackerNewsProvider creates a new HackerNews provider
func NewHackerNewsProvider(minPoints, limit int, categoryMapper *CategoryMapper) providers.FeedProvider {
	// Initialize CategoryMapper if not provided
	if categoryMapper == nil {
		categoryMapper = LoadConfig("", "") // Use default configuration
	}

	// Initialize databases
	databases, err := database.InitializeProviderDatabases("hackernews.db", true)
	if err != nil {
		// TODO: Handle error properly - for now return nil
		return nil
	}

	base, err := providers.NewBaseProvider(providers.DatabaseConfig{
		ContentDBName: "hackernews.db",
		UseContentDB:  true,
	})
	if err != nil {
		databases.Close()
		return nil
	}

	return &HackerNewsProvider{
		BaseProvider:   base,
		MinPoints:      minPoints,
		Limit:          limit,
		CategoryMapper: categoryMapper,
		databases:      databases,
	}
}

// GenerateFeed implements the FeedProvider interface
func (p *HackerNewsProvider) GenerateFeed(outfile string, reauth bool) error {
	// Clean up expired entries using base provider
	if err := p.CleanupExpired(); err != nil {
		// Non-fatal error, just warn
	}

	// Get database connections
	contentDB := p.databases.ContentDB
	ogDB := p.databases.OpenGraphDB

	// Fetch current front page items
	newItems := fetchHackerNewsItems()

	// Initialize database schema
	if err := initializeSchema(contentDB); err != nil {
		return err
	}

	// Update database with new items and get list of updated item IDs
	recentlyUpdated := updateStoredItems(contentDB, newItems)

	// Get all items from database
	allItems := getAllItems(contentDB, p.Limit, p.MinPoints)

	// Update item stats with current data from Algolia, skipping recently updated items
	updateItemStats(contentDB.DB(), allItems, recentlyUpdated)

	// Re-fetch items to get updated stats for RSS generation
	allItems = getAllItems(contentDB, p.Limit, p.MinPoints)

	// Ensure output directory exists
	if err := filesystem.EnsureDirectoryExists(outfile); err != nil {
		return err
	}

	// Generate and save the feed
	rss := generateRSSFeed(contentDB.DB(), ogDB, allItems, p.MinPoints, p.CategoryMapper)
	if err := os.WriteFile(outfile, []byte(rss), 0644); err != nil {
		return err
	}

	feed.LogFeedGeneration(len(allItems), outfile)
	return nil
}
