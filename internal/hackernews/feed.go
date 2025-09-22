package hackernews

import (
	"database/sql"
	"log/slog"
	"regexp"
	"strings"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// generateRSSFeed creates an Atom RSS feed using template-based generation
func generateRSSFeed(db *sql.DB, ogDB *opengraph.Database, items []HackerNewsItem, minPoints int, categoryMapper *CategoryMapper) (string, error) {
	slog.Debug("Generating RSS feed using template-based generation", "itemCount", len(items))

	// Create template generator
	templateGenerator := feed.NewTemplateGenerator()

	// Load Hacker News template
	err := templateGenerator.LoadTemplate("hackernews-atom", "templates/hackernews-atom.tmpl")
	if err != nil {
		slog.Error("Failed to load Hacker News Atom template", "error", err)
		return "", err
	}

	// Process items to add HackerNews-specific categorization
	preprocessedItems := preprocessHackerNewsItems(items, minPoints, categoryMapper)

	// Convert to FeedItem interface
	feedItems := make([]providers.FeedItem, len(preprocessedItems))
	for i, item := range preprocessedItems {
		feedItems[i] = &item
	}

	// Collect URLs for OpenGraph fetching
	urls := make([]string, 0, len(feedItems))
	for _, item := range feedItems {
		if item.Link() != "" && item.Link() != item.CommentsLink() {
			urls = append(urls, item.Link())
		}
	}

	// Fetch OpenGraph data concurrently
	var ogData map[string]*opengraph.Data
	if ogDB != nil {
		ogFetcher := opengraph.NewFetcher(ogDB)
		slog.Debug("Fetching OpenGraph data for template feed", "url_count", len(urls))
		ogData = ogFetcher.FetchConcurrent(urls)
	}

	// Create template data
	templateData := templateGenerator.CreateHackerNewsFeedData(feedItems, ogData)

	// Generate using template
	var atomContent strings.Builder
	err = templateGenerator.GenerateFromTemplate("hackernews-atom", templateData, &atomContent)
	if err != nil {
		slog.Error("Failed to generate template feed", "error", err)
		return "", err
	}

	slog.Debug("Template-based Atom feed generated successfully", "feedSize", len(atomContent.String()))
	return atomContent.String(), nil
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
