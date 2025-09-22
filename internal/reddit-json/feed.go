package redditjson

import (
	"log/slog"
	"os"
	"strings"

	feedpkg "github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// generateRedditTemplateFeed creates an Atom RSS feed using template-based generation
func generateRedditTemplateFeed(posts []RedditPost, ogFetcher *opengraph.Fetcher) (string, error) {
	slog.Debug("Generating RSS feed using template-based generation", "itemCount", len(posts))

	// Create template generator
	templateGenerator := feedpkg.NewTemplateGenerator()

	// Load Reddit template
	err := templateGenerator.LoadTemplate("reddit-atom", "templates/reddit-atom.tmpl")
	if err != nil {
		slog.Error("Failed to load Reddit Atom template", "error", err)
		return "", err
	}

	// Convert to FeedItem interface
	feedItems := make([]providers.FeedItem, len(posts))
	for i, post := range posts {
		feedItems[i] = &post
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
	if ogFetcher != nil {
		slog.Debug("Fetching OpenGraph data for template feed", "url_count", len(urls))
		ogData = ogFetcher.FetchConcurrent(urls)
	}

	// Create template data
	templateData := templateGenerator.CreateRedditFeedData(feedItems, ogData)

	// Generate using template
	var atomContent strings.Builder
	err = templateGenerator.GenerateFromTemplate("reddit-atom", templateData, &atomContent)
	if err != nil {
		slog.Error("Failed to generate template feed", "error", err)
		return "", err
	}

	slog.Debug("Template-based Atom feed generated successfully", "feedSize", len(atomContent.String()))
	return atomContent.String(), nil
}

// SaveRedditFeedToFile saves a Reddit Atom feed using template-based generation
func SaveRedditFeedToFile(posts []RedditPost, outputPath string, ogFetcher *opengraph.Fetcher) error {
	slog.Debug("Generating Reddit Atom feed", "outputPath", outputPath, "postCount", len(posts))

	atomContent, err := generateRedditTemplateFeed(posts, ogFetcher)
	if err != nil {
		slog.Error("Failed to generate Atom feed", "error", err)
		return err
	}

	return os.WriteFile(outputPath, []byte(atomContent), 0o644)
}
