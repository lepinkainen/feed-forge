package hackernews

import (
	"database/sql"
	"log/slog"
	"os"
	"regexp"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// generateRSSFeed creates an Atom RSS feed from the provided items with OpenGraph data
func generateRSSFeed(db *sql.DB, ogDB *opengraph.Database, items []HackerNewsItem, minPoints int, categoryMapper *CategoryMapper) string {
	slog.Debug("Generating RSS feed using enhanced Atom infrastructure", "itemCount", len(items))

	// Create shared feed generator
	generator := feed.NewGenerator(
		"Hacker News Top Stories",
		"High-quality Hacker News stories, updated regularly",
		"https://news.ycombinator.com/",
		"Feed Forge",
	)

	// Process items to add HackerNews-specific categorization
	preprocessedItems := preprocessHackerNewsItems(items, minPoints, categoryMapper)

	// Convert to FeedItem interface
	feedItems := make([]providers.FeedItem, len(preprocessedItems))
	for i, item := range preprocessedItems {
		feedItems[i] = &item
	}

	// Use enhanced Atom generation with HackerNews-specific configuration
	config := feed.HackerNewsEnhancedAtomConfig()
	ogFetcher := opengraph.NewFetcher(ogDB)
	atomContent, err := generator.GenerateEnhancedAtomWithConfig(feedItems, config, ogFetcher)
	if err != nil {
		slog.Error("Failed to generate enhanced Atom feed", "error", err)
		os.Exit(1)
	}

	slog.Debug("Enhanced Atom feed generated successfully", "feedSize", len(atomContent))
	return atomContent
}

// preprocessHackerNewsItems applies HackerNews-specific categorization and metadata
func preprocessHackerNewsItems(items []HackerNewsItem, minPoints int, categoryMapper *CategoryMapper) []HackerNewsItem {
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
