package hackernews

import (
	"database/sql"
	"encoding/xml"
	"log/slog"
	"os"
	"regexp"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// Use shared Atom types from pkg/feed
type AtomCategory = feed.AtomCategory
type CustomAtomEntry = feed.CustomAtomEntry
type CustomAtomFeed = feed.CustomAtomFeed

// generateRSSFeed creates an Atom RSS feed from the provided items with OpenGraph data
func generateRSSFeed(db *sql.DB, ogDB *opengraph.Database, items []HackerNewsItem, minPoints int, categoryMapper *CategoryMapper) string {
	slog.Debug("Generating RSS feed using shared infrastructure", "itemCount", len(items))

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

	// Initialize OpenGraph fetcher and use shared generation with OpenGraph
	ogFetcher := opengraph.NewFetcher(ogDB)
	feedObj, itemCategories, err := generator.GenerateWithOpenGraph(feedItems, feed.Atom, ogFetcher)
	if err != nil {
		slog.Error("Failed to generate RSS feed", "error", err)
		os.Exit(1)
	}

	// Generate custom Atom feed with HackerNews-specific categories
	customAtomFeed := feed.ConvertToCustomAtom(feedObj, itemCategories)

	// Convert to XML
	xmlData, err := xml.MarshalIndent(customAtomFeed, "", "  ")
	if err != nil {
		slog.Error("Failed to generate RSS feed", "error", err)
		os.Exit(1)
	}

	// Add XML header
	rss := xml.Header + string(xmlData)

	slog.Debug("RSS feed generated successfully using shared infrastructure", "feedSize", len(rss))
	return rss
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
