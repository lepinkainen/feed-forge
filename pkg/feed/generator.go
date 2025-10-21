// Package feed provides template-based Atom feed generation and feed helpers.
package feed

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// Config contains metadata for feed generation
type Config struct {
	Title       string
	Link        string
	Description string
	Author      string
	ID          string
}

// GenerateAtomFeed creates an Atom RSS feed using template-based generation
// This is the unified function that replaces provider-specific generation logic
func GenerateAtomFeed(items []providers.FeedItem, templateName, templatePath string, config Config, ogDB *opengraph.Database) (string, error) {
	slog.Debug("Generating Atom feed using unified generator", "templateName", templateName, "itemCount", len(items))

	// Create template generator
	templateGenerator := NewTemplateGenerator()

	// Load template (old API for backward compatibility)
	err := templateGenerator.LoadTemplate(templateName, templatePath)
	if err != nil {
		slog.Error("Failed to load template", "templateName", templateName, "path", templatePath, "error", err)
		return "", err
	}

	// Collect URLs for OpenGraph fetching
	urls := make([]string, 0, len(items))
	for _, item := range items {
		if item.Link() != "" && item.Link() != item.CommentsLink() {
			urls = append(urls, item.Link())
		}
	}

	// Fetch OpenGraph data concurrently
	var ogData map[string]*opengraph.Data
	if ogDB != nil {
		ogFetcher := opengraph.NewFetcher(ogDB)
		slog.Debug("Fetching OpenGraph data for unified feed", "url_count", len(urls))
		ogData = ogFetcher.FetchConcurrent(urls)
	}

	// Create template data using generic function
	templateData := createGenericFeedData(items, config, ogData)

	// Generate using template
	var atomContent strings.Builder
	err = templateGenerator.GenerateFromTemplate(templateName, templateData, &atomContent)
	if err != nil {
		slog.Error("Failed to generate template feed", "error", err)
		return "", err
	}

	slog.Debug("Unified Atom feed generated successfully", "feedSize", len(atomContent.String()))
	return atomContent.String(), nil
}

// SaveAtomFeedToFile generates and saves an Atom feed to a file
func SaveAtomFeedToFile(items []providers.FeedItem, templateName, templatePath, outputPath string, config Config, ogDB *opengraph.Database) error {
	slog.Debug("Generating and saving Atom feed", "outputPath", outputPath, "itemCount", len(items))

	atomContent, err := GenerateAtomFeed(items, templateName, templatePath, config, ogDB)
	if err != nil {
		slog.Error("Failed to generate Atom feed", "error", err)
		return err
	}

	return os.WriteFile(outputPath, []byte(atomContent), 0o644)
}

// GenerateAtomFeedWithEmbeddedTemplate creates an Atom RSS feed using embedded templates with local override
func GenerateAtomFeedWithEmbeddedTemplate(items []providers.FeedItem, templateName string, config Config, ogDB *opengraph.Database) (string, error) {
	slog.Debug("Generating Atom feed with embedded template", "templateName", templateName, "itemCount", len(items))

	// Create template generator
	templateGenerator := NewTemplateGenerator()

	// Load template with fallback to embedded
	err := templateGenerator.LoadTemplateWithFallback(templateName)
	if err != nil {
		slog.Error("Failed to load template", "templateName", templateName, "error", err)
		return "", err
	}

	// Collect URLs for OpenGraph fetching
	urls := make([]string, 0, len(items))
	for _, item := range items {
		if item.Link() != "" && item.Link() != item.CommentsLink() {
			urls = append(urls, item.Link())
		}
	}

	// Fetch OpenGraph data concurrently
	var ogData map[string]*opengraph.Data
	if ogDB != nil {
		ogFetcher := opengraph.NewFetcher(ogDB)
		slog.Debug("Fetching OpenGraph data for unified feed", "url_count", len(urls))
		ogData = ogFetcher.FetchConcurrent(urls)
	}

	// Create template data using generic function
	templateData := createGenericFeedData(items, config, ogData)

	// Generate using template
	var atomContent strings.Builder
	err = templateGenerator.GenerateFromTemplate(templateName, templateData, &atomContent)
	if err != nil {
		slog.Error("Failed to generate template feed", "error", err)
		return "", err
	}

	slog.Debug("Unified Atom feed generated successfully", "feedSize", len(atomContent.String()))
	return atomContent.String(), nil
}

// SaveAtomFeedToFileWithEmbeddedTemplate generates and saves an Atom feed using embedded templates with local override
func SaveAtomFeedToFileWithEmbeddedTemplate(items []providers.FeedItem, templateName, outputPath string, config Config, ogDB *opengraph.Database) error {
	slog.Debug("Generating and saving Atom feed with embedded template", "outputPath", outputPath, "itemCount", len(items))

	atomContent, err := GenerateAtomFeedWithEmbeddedTemplate(items, templateName, config, ogDB)
	if err != nil {
		slog.Error("Failed to generate Atom feed", "error", err)
		return err
	}

	return os.WriteFile(outputPath, []byte(atomContent), 0o644)
}

// createGenericFeedData converts FeedItems to template data structure
// This replaces the provider-specific CreateRedditFeedData and CreateHackerNewsFeedData functions
func createGenericFeedData(items []providers.FeedItem, config Config, ogData map[string]*opengraph.Data) *TemplateData {
	now := time.Now()

	data := &TemplateData{
		FeedTitle:       config.Title,
		FeedLink:        config.Link,
		FeedDescription: config.Description,
		FeedAuthor:      config.Author,
		FeedID:          config.ID,
		Updated:         now.Format(time.RFC3339),
		Generator:       "Feed Forge",
		OpenGraphData:   ogData,
		Items:           make([]TemplateItem, len(items)),
	}

	for i, item := range items {
		templateItem := TemplateItem{
			Title:        item.Title(),
			Link:         item.Link(),
			CommentsLink: item.CommentsLink(),
			ID:           item.CommentsLink(),
			Updated:      item.CreatedAt().Format(time.RFC3339),
			Published:    item.CreatedAt().Format(time.RFC3339),
			Author:       item.Author(),
			Categories:   item.Categories(),
			Score:        item.Score(),
			Comments:     item.CommentCount(),
			Content:      item.Content(),
			Summary:      fmt.Sprintf("Score: %d | Comments: %d", item.Score(), item.CommentCount()),
		}

		// Extract provider-specific fields through type assertions
		if authorURI, ok := item.(interface{ AuthorURI() string }); ok {
			templateItem.AuthorURI = authorURI.AuthorURI()
		}
		if subreddit, ok := item.(interface{ Subreddit() string }); ok {
			templateItem.Subreddit = subreddit.Subreddit()
		}
		if domain, ok := item.(interface{ ItemDomain() string }); ok {
			templateItem.Domain = domain.ItemDomain()
		}

		data.Items[i] = templateItem
	}

	return data
}
