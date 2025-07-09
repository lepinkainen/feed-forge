package feed

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/feeds"
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

	slog.Info("Generated feed", "type", feedType, "items", len(feed.Items))
	return feed, nil
}

// SaveToFile saves the generated feed to a specified file
func (g *Generator) SaveToFile(feed *feeds.Feed, feedType FeedType, outputPath string) error {
	// Ensure output directory exists
	outDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
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
		if err := g.validateFeedItem(item); err != nil {
			return fmt.Errorf("item %d validation failed: %w", i, err)
		}
	}

	return nil
}

// validateFeedItem validates individual feed items
func (g *Generator) validateFeedItem(item *feeds.Item) error {
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
