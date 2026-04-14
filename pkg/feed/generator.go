// Package feed provides template-based Atom feed generation and feed helpers.
package feed

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/feedmeta"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// Config contains metadata for feed generation.
type Config = feedmeta.Config

// GenerateAtomFeed creates an Atom RSS feed using template-based generation.
// This is the unified function that replaces provider-specific generation logic.
func GenerateAtomFeed(items []providers.FeedItem, templateName, templatePath string, config Config, ogDB *opengraph.Database) (string, error) {
	return GenerateAtomFeedWithContext(context.Background(), items, templateName, templatePath, config, ogDB)
}

// GenerateAtomFeedWithContext creates an Atom RSS feed using template-based generation.
func GenerateAtomFeedWithContext(ctx context.Context, items []providers.FeedItem, templateName, templatePath string, config Config, ogDB *opengraph.Database) (string, error) {
	return generateAtomFeed(ctx, items, templateName, config, ogDB, func(generator *TemplateGenerator) error {
		return generator.LoadTemplate(templateName, templatePath)
	})
}

// SaveAtomFeedToFile generates and saves an Atom feed to a file.
func SaveAtomFeedToFile(items []providers.FeedItem, templateName, templatePath, outputPath string, config Config, ogDB *opengraph.Database) error {
	return SaveAtomFeedToFileWithContext(context.Background(), items, templateName, templatePath, outputPath, config, ogDB)
}

// SaveAtomFeedToFileWithContext generates and saves an Atom feed to a file.
func SaveAtomFeedToFileWithContext(ctx context.Context, items []providers.FeedItem, templateName, templatePath, outputPath string, config Config, ogDB *opengraph.Database) error {
	slog.Debug("Generating and saving Atom feed", "outputPath", outputPath, "itemCount", len(items))

	atomContent, err := GenerateAtomFeedWithContext(ctx, items, templateName, templatePath, config, ogDB)
	if err != nil {
		slog.Error("Failed to generate Atom feed", "error", err)
		return err
	}

	return os.WriteFile(outputPath, []byte(atomContent), 0o600)
}

// GenerateAtomFeedWithEmbeddedTemplate creates an Atom RSS feed using embedded templates with local override.
func GenerateAtomFeedWithEmbeddedTemplate(items []providers.FeedItem, templateName string, config Config, ogDB *opengraph.Database) (string, error) {
	return GenerateAtomFeedWithEmbeddedTemplateWithContext(context.Background(), items, templateName, config, ogDB)
}

// GenerateAtomFeedWithEmbeddedTemplateWithContext creates an Atom RSS feed using embedded templates with local override.
func GenerateAtomFeedWithEmbeddedTemplateWithContext(ctx context.Context, items []providers.FeedItem, templateName string, config Config, ogDB *opengraph.Database) (string, error) {
	return generateAtomFeed(ctx, items, templateName, config, ogDB, func(generator *TemplateGenerator) error {
		return generator.LoadTemplateWithFallback(templateName)
	})
}

// SaveAtomFeedToFileWithEmbeddedTemplate generates and saves an Atom feed using embedded templates with local override.
func SaveAtomFeedToFileWithEmbeddedTemplate(items []providers.FeedItem, templateName, outputPath string, config Config, ogDB *opengraph.Database) error {
	return SaveAtomFeedToFileWithEmbeddedTemplateWithContext(context.Background(), items, templateName, outputPath, config, ogDB)
}

// SaveAtomFeedToFileWithEmbeddedTemplateWithContext generates and saves an Atom feed using embedded templates with local override.
func SaveAtomFeedToFileWithEmbeddedTemplateWithContext(ctx context.Context, items []providers.FeedItem, templateName, outputPath string, config Config, ogDB *opengraph.Database) error {
	slog.Debug("Generating and saving Atom feed with embedded template", "outputPath", outputPath, "itemCount", len(items))

	atomContent, err := GenerateAtomFeedWithEmbeddedTemplateWithContext(ctx, items, templateName, config, ogDB)
	if err != nil {
		slog.Error("Failed to generate Atom feed", "error", err)
		return err
	}

	return os.WriteFile(outputPath, []byte(atomContent), 0o600)
}

func generateAtomFeed(ctx context.Context, items []providers.FeedItem, templateName string, config Config, ogDB *opengraph.Database, loadTemplate func(*TemplateGenerator) error) (string, error) {
	slog.Debug("Generating Atom feed", "templateName", templateName, "itemCount", len(items))

	templateGenerator := NewTemplateGenerator()
	if err := loadTemplate(templateGenerator); err != nil {
		slog.Error("Failed to load template", "templateName", templateName, "error", err)
		return "", err
	}

	urls := externalItemURLs(items)

	var ogData map[string]*opengraph.Data
	if ogDB != nil {
		ogFetcher := createOGFetcher(ogDB, config)
		slog.Debug("Fetching OpenGraph data", "url_count", len(urls))
		ogData = ogFetcher.FetchConcurrentWithContext(ctx, urls)
	}

	templateData := createGenericFeedData(items, config, ogData)

	var atomContent strings.Builder
	if err := templateGenerator.GenerateFromTemplate(templateName, templateData, &atomContent); err != nil {
		slog.Error("Failed to generate template feed", "templateName", templateName, "error", err)
		return "", err
	}

	result := atomContent.String()
	slog.Debug("Atom feed generated successfully", "templateName", templateName, "feedSize", len(result))
	return result, nil
}

func externalItemURLs(items []providers.FeedItem) []string {
	urls := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		link := item.Link()
		if link == "" || link == item.CommentsLink() {
			continue
		}
		if _, dup := seen[link]; dup {
			continue
		}
		seen[link] = struct{}{}
		urls = append(urls, link)
	}
	return urls
}

// createOGFetcher creates an OpenGraph fetcher, optionally with proxy support.
func createOGFetcher(ogDB *opengraph.Database, config Config) *opengraph.Fetcher {
	if config.ProxyURL != "" && config.ProxySecret != "" {
		return opengraph.NewFetcherWithProxy(ogDB, &opengraph.ProxyConfig{
			URL:    config.ProxyURL,
			Secret: config.ProxySecret,
		})
	}
	return opengraph.NewFetcher(ogDB)
}

// createGenericFeedData converts FeedItems to template data structure.
// This replaces the provider-specific CreateRedditFeedData and CreateHackerNewsFeedData functions.
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
			ImageURL:     item.ImageURL(),
		}

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
