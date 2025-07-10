package hackernews

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lepinkainen/feed-forge/internal/pkg/providers"
	"github.com/lepinkainen/feed-forge/pkg/database"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
)

// HackerNewsProvider implements the FeedProvider interface for Hacker News
type HackerNewsProvider struct {
	MinPoints      int
	Limit          int
	CategoryMapper *CategoryMapper
}

// NewHackerNewsProvider creates a new HackerNews provider
func NewHackerNewsProvider(minPoints, limit int, categoryMapper *CategoryMapper) providers.FeedProvider {
	// Initialize CategoryMapper if not provided
	if categoryMapper == nil {
		categoryMapper = LoadConfig("", "") // Use default configuration
	}

	return &HackerNewsProvider{
		MinPoints:      minPoints,
		Limit:          limit,
		CategoryMapper: categoryMapper,
	}
}

// GenerateFeed implements the FeedProvider interface
func (p *HackerNewsProvider) GenerateFeed(outfile string, reauth bool) error {
	// Initialize content database
	dbPath, err := database.GetDefaultPath("hackernews.db")
	if err != nil {
		return err
	}

	contentDB, err := database.NewDatabase(database.Config{Path: dbPath})
	if err != nil {
		return err
	}
	defer contentDB.Close()

	// Initialize OpenGraph database
	ogDBPath, err := database.GetDefaultPath("opengraph.db")
	if err != nil {
		return err
	}

	ogDB, err := opengraph.NewDatabase(ogDBPath)
	if err != nil {
		return err
	}
	defer ogDB.Close()

	// Clean up expired OpenGraph cache entries
	if err := ogDB.CleanupExpiredEntries(); err != nil {
		slog.Warn("Failed to cleanup expired OpenGraph cache", "error", err)
	}

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
	outDir := filepath.Dir(outfile)
	err = os.MkdirAll(outDir, 0755)
	if err != nil {
		return err
	}

	// Generate and save the feed
	rss := generateRSSFeed(contentDB.DB(), ogDB, allItems, p.MinPoints, p.CategoryMapper)
	err = os.WriteFile(outfile, []byte(rss), 0644)
	if err != nil {
		return err
	}

	slog.Info("RSS feed saved", "count", len(allItems), "filename", outfile)
	return nil
}
