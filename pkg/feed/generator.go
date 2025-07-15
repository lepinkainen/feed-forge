package feed

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gorilla/feeds"
	"github.com/lepinkainen/feed-forge/internal/pkg/providers"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
)

// Generate creates a feed from the provided items
func (g *Generator) Generate(items []Item, feedType FeedType) (*feeds.Feed, error) {
	if feedType != RSS && feedType != Atom {
		return nil, fmt.Errorf("unsupported feed type: %s", feedType)
	}

	now := time.Now()
	feed := &feeds.Feed{
		Title:       g.Title,
		Link:        &feeds.Link{Href: g.Link},
		Description: g.Description,
		Author:      &feeds.Author{Name: g.Author},
		Created:     now,
		Updated:     now,
	}

	// Convert items to feed items
	for _, item := range items {
		feedItem := &feeds.Item{
			Title:       item.Title,
			Link:        &feeds.Link{Href: item.Link},
			Description: item.Description,
			Author:      &feeds.Author{Name: item.Author},
			Created:     item.Created,
			Id:          item.ID,
		}

		feed.Items = append(feed.Items, feedItem)
	}

	slog.Debug("Generated feed", "type", feedType, "items", len(feed.Items))
	return feed, nil
}

// GenerateFromFeedItems creates a feed from items implementing the FeedItem interface
func (g *Generator) GenerateFromFeedItems(items []providers.FeedItem, feedType FeedType) (*feeds.Feed, error) {
	if feedType != RSS && feedType != Atom {
		return nil, fmt.Errorf("unsupported feed type: %s", feedType)
	}

	now := time.Now()
	feed := &feeds.Feed{
		Title:       g.Title,
		Link:        &feeds.Link{Href: g.Link},
		Description: g.Description,
		Author:      &feeds.Author{Name: g.Author},
		Created:     now,
		Updated:     now,
	}

	// Convert FeedItem interface items to feed items
	for _, item := range items {
		feedItem := &feeds.Item{
			Title:       item.Title(),
			Link:        &feeds.Link{Href: item.Link()},
			Description: fmt.Sprintf("Score: %d | Comments: %d", item.Score(), item.CommentCount()),
			Author:      &feeds.Author{Name: item.Author()},
			Created:     item.CreatedAt(),
			Id:          item.CommentsLink(),
		}

		feed.Items = append(feed.Items, feedItem)
	}

	slog.Debug("Generated feed from FeedItems", "type", feedType, "items", len(feed.Items))
	return feed, nil
}

// GenerateEnhancedFromFeedItems creates an enhanced feed with categories from items implementing the FeedItem interface
func (g *Generator) GenerateEnhancedFromFeedItems(items []providers.FeedItem, feedType FeedType) (*feeds.Feed, map[string][]string, error) {
	if feedType != RSS && feedType != Atom {
		return nil, nil, fmt.Errorf("unsupported feed type: %s", feedType)
	}

	now := time.Now()
	feed := &feeds.Feed{
		Title:       g.Title,
		Link:        &feeds.Link{Href: g.Link},
		Description: g.Description,
		Author:      &feeds.Author{Name: g.Author},
		Created:     now,
		Updated:     now,
	}

	// Track categories for each item
	itemCategories := make(map[string][]string)

	// Convert FeedItem interface items to feed items
	for _, item := range items {
		feedItem := &feeds.Item{
			Title:       item.Title(),
			Link:        &feeds.Link{Href: item.Link()},
			Description: fmt.Sprintf("Score: %d | Comments: %d", item.Score(), item.CommentCount()),
			Author:      &feeds.Author{Name: item.Author()},
			Created:     item.CreatedAt(),
			Id:          item.CommentsLink(),
		}

		// Store categories for this item
		itemCategories[item.CommentsLink()] = item.Categories()

		feed.Items = append(feed.Items, feedItem)
	}

	slog.Debug("Generated enhanced feed from FeedItems", "type", feedType, "items", len(feed.Items))
	return feed, itemCategories, nil
}

// GenerateWithOpenGraph creates a feed with OpenGraph metadata from items implementing the FeedItem interface
func (g *Generator) GenerateWithOpenGraph(items []providers.FeedItem, feedType FeedType, ogFetcher *opengraph.Fetcher) (*feeds.Feed, map[string][]string, error) {
	if feedType != RSS && feedType != Atom {
		return nil, nil, fmt.Errorf("unsupported feed type: %s", feedType)
	}

	now := time.Now()
	feed := &feeds.Feed{
		Title:       g.Title,
		Link:        &feeds.Link{Href: g.Link},
		Description: g.Description,
		Author:      &feeds.Author{Name: g.Author},
		Created:     now,
		Updated:     now,
	}

	// Track categories for each item
	itemCategories := make(map[string][]string)

	// Collect URLs for OpenGraph fetching
	var urlsToFetch []string
	for _, item := range items {
		if item.Link() != "" {
			urlsToFetch = append(urlsToFetch, item.Link())
		}
	}

	// Fetch OpenGraph data concurrently
	var ogDataMap map[string]*opengraph.Data
	if ogFetcher != nil && len(urlsToFetch) > 0 {
		slog.Debug("Fetching OpenGraph data concurrently", "urlCount", len(urlsToFetch))
		ogDataMap = ogFetcher.FetchConcurrent(urlsToFetch)
		slog.Debug("Completed concurrent OpenGraph fetching")
	}

	// Convert FeedItem interface items to feed items with OpenGraph data
	for _, item := range items {
		description := g.buildEnhancedDescription(item, ogDataMap)

		feedItem := &feeds.Item{
			Title:       item.Title(),
			Link:        &feeds.Link{Href: item.Link()},
			Description: description,
			Author:      &feeds.Author{Name: item.Author()},
			Created:     item.CreatedAt(),
			Id:          item.CommentsLink(),
		}

		// Store categories for this item
		itemCategories[item.CommentsLink()] = item.Categories()

		feed.Items = append(feed.Items, feedItem)
	}

	slog.Debug("Generated OpenGraph-enhanced feed from FeedItems", "type", feedType, "items", len(feed.Items))
	return feed, itemCategories, nil
}

// buildEnhancedDescription creates a rich description with OpenGraph data
func (g *Generator) buildEnhancedDescription(item providers.FeedItem, ogDataMap map[string]*opengraph.Data) string {
	// Build base description
	description := fmt.Sprintf("Score: %d | Comments: %d", item.Score(), item.CommentCount())

	// Add categories
	if categories := item.Categories(); len(categories) > 0 {
		description += fmt.Sprintf(" | Categories: %s", strings.Join(categories, ", "))
	}

	// Add OpenGraph preview if available
	if ogDataMap != nil && item.Link() != "" {
		if ogData, exists := ogDataMap[item.Link()]; exists && ogData != nil {
			description += g.formatOpenGraphPreview(ogData)
		}
	}

	return description
}

// formatOpenGraphPreview formats OpenGraph data as HTML for inclusion in feed descriptions
func (g *Generator) formatOpenGraphPreview(og *opengraph.Data) string {
	if og == nil {
		return ""
	}

	var preview string
	preview += "\n\n" + `<div style="margin-top: 16px; padding: 12px; background: #f9f9f9; border-radius: 6px; border-left: 3px solid #007acc;">`
	preview += `<h4 style="margin: 0 0 8px 0; color: #007acc; font-size: 14px;">📄 Article Preview</h4>`

	if og.Title != "" {
		preview += fmt.Sprintf(`<p style="margin: 0 0 6px 0; font-weight: bold; color: #333;">%s</p>`, EscapeXML(og.Title))
	}

	if og.Description != "" {
		preview += fmt.Sprintf(`<p style="margin: 0 0 6px 0; color: #666; line-height: 1.4; font-size: 13px;">%s</p>`, EscapeXML(og.Description))
	}

	if og.Image != "" {
		preview += fmt.Sprintf(`<img src="%s" alt="Article image" style="max-width: 100%%; height: auto; border-radius: 4px; margin-top: 8px;" loading="lazy">`, EscapeXML(og.Image))
	}

	preview += `</div>`
	return preview
}

// SaveToFile saves the generated feed to a specified file
func (g *Generator) SaveToFile(feed *feeds.Feed, feedType FeedType, outputPath string) error {
	// Ensure output directory exists
	if err := filesystem.EnsureDirectoryExists(outputPath); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	switch feedType {
	case RSS:
		err = feed.WriteRss(file)
	case Atom:
		err = feed.WriteAtom(file)
	default:
		return fmt.Errorf("unsupported feed type: %s", feedType)
	}

	if err != nil {
		return fmt.Errorf("failed to write %s feed: %w", feedType, err)
	}

	slog.Info("Feed saved successfully", "type", feedType, "path", outputPath)
	return nil
}

// ValidateFeed validates the generated feed structure
func (g *Generator) ValidateFeed(feed *feeds.Feed) error {
	if feed == nil {
		return fmt.Errorf("feed is nil")
	}

	if feed.Title == "" {
		return fmt.Errorf("feed title is empty")
	}

	if feed.Link == nil || feed.Link.Href == "" {
		return fmt.Errorf("feed link is empty")
	}

	if feed.Description == "" {
		return fmt.Errorf("feed description is empty")
	}

	if len(feed.Items) == 0 {
		return fmt.Errorf("feed has no items")
	}

	// Validate feed items
	for i, item := range feed.Items {
		if err := ValidateFeedItem(item); err != nil {
			return fmt.Errorf("item %d validation failed: %w", i, err)
		}
	}

	return nil
}

// ValidateFeedItem validates individual feed items (standalone function)
func ValidateFeedItem(item *feeds.Item) error {
	if item.Title == "" {
		return fmt.Errorf("item title is empty")
	}

	if item.Link == nil || item.Link.Href == "" {
		return fmt.Errorf("item link is empty")
	}

	if item.Id == "" {
		return fmt.Errorf("item ID is empty")
	}

	return nil
}

// GetMetadata returns metadata about the generated feed
func (g *Generator) GetMetadata(feed *feeds.Feed) *Metadata {
	if feed == nil {
		return nil
	}

	metadata := &Metadata{
		Title:       feed.Title,
		Description: feed.Description,
		ItemCount:   len(feed.Items),
		Created:     feed.Created,
		Updated:     feed.Updated,
	}

	if len(feed.Items) > 0 {
		// Find oldest and newest items
		oldest := feed.Items[0].Created
		newest := feed.Items[0].Created

		for _, item := range feed.Items {
			if item.Created.Before(oldest) {
				oldest = item.Created
			}
			if item.Created.After(newest) {
				newest = item.Created
			}
		}

		metadata.OldestItem = oldest
		metadata.NewestItem = newest
	}

	return metadata
}

// EscapeXML escapes XML special characters
func EscapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// TruncateString truncates a string to a maximum length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
