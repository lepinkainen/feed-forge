// Package main provides the CLI entry point for feed-forge.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/preview"
	"github.com/lepinkainen/feed-forge/pkg/providers"

	// Import providers to trigger init() self-registration
	"github.com/lepinkainen/feed-forge/internal/fingerpori"
	"github.com/lepinkainen/feed-forge/internal/hackernews"
	redditjson "github.com/lepinkainen/feed-forge/internal/reddit-json"
)

// CLI structure
var CLI struct {
	Config string `help:"Configuration file path" default:"config.yaml"`
	Debug  bool   `help:"Enable debug logging" default:"false"`

	Reddit struct {
		Outfile     string `help:"Output file path" short:"o" default:"reddit.xml"`
		MinScore    int    `help:"Minimum post score" default:"50"`
		MinComments int    `help:"Minimum comment count" default:"10"`
		FeedID      string `help:"Reddit feed ID"`
		Username    string `help:"Reddit username"`
	} `cmd:"reddit" help:"Generate RSS feed from Reddit."`

	HackerNews struct {
		Outfile   string `help:"Output file path" short:"o" default:"hackernews.xml"`
		MinPoints int    `help:"Minimum points threshold" default:"50"`
		Limit     int    `help:"Maximum number of items" default:"30"`
	} `cmd:"hacker-news" help:"Generate RSS feed from Hacker News."`

	Fingerpori struct {
		Outfile string `help:"Output file path" short:"o" default:"fingerpori.xml"`
		Limit   int    `help:"Maximum number of items" default:"100"`
	} `cmd:"fingerpori" help:"Generate RSS feed from Fingerpori comics."`

	Preview struct {
		Reddit struct {
			MinScore    int    `help:"Minimum post score" default:"50"`
			MinComments int    `help:"Minimum comment count" default:"10"`
			FeedID      string `help:"Reddit feed ID"`
			Username    string `help:"Reddit username"`
			Limit       int    `help:"Maximum number of items to fetch" default:"30"`
			Index       int    `help:"Output XML for specific item index (0-based) to stdout" default:"-1"`
		} `cmd:"reddit" help:"Preview Reddit feed items."`

		HackerNews struct {
			MinPoints int `help:"Minimum points threshold" default:"50"`
			Limit     int `help:"Maximum number of items" default:"30"`
			Index     int `help:"Output XML for specific item index (0-based) to stdout" default:"-1"`
		} `cmd:"hacker-news" help:"Preview Hacker News feed items."`

		Fingerpori struct {
			Limit int `help:"Maximum number of items" default:"10"`
			Index int `help:"Output XML for specific item index (0-based) to stdout" default:"-1"`
		} `cmd:"fingerpori" help:"Preview Fingerpori feed items."`
	} `cmd:"preview" help:"Preview feed items interactively."`
}

func main() {
	// Parse CLI with Kong YAML configuration file loading
	ctx := kong.Parse(&CLI,
		kong.Configuration(kongyaml.Loader, "config.yaml", "~/.feed-forge/config.yaml"),
	)

	// Configure logging level based on debug flag
	if CLI.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	} else {
		slog.SetLogLoggerLevel(slog.LevelWarn)
	}

	switch ctx.Command() {
	case "hacker-news":
		generateFeed("hacker-news", &hackernews.Config{
			MinPoints: CLI.HackerNews.MinPoints,
			Limit:     CLI.HackerNews.Limit,
		}, CLI.HackerNews.Outfile)

	case "reddit":
		// Validate required parameters
		if CLI.Reddit.FeedID == "" || CLI.Reddit.Username == "" {
			slog.Error("Reddit feed requires both feed_id and username to be set via CLI flags or config file")
			os.Exit(1)
		}
		generateFeed("reddit", &redditjson.Config{
			MinScore:    CLI.Reddit.MinScore,
			MinComments: CLI.Reddit.MinComments,
			FeedID:      CLI.Reddit.FeedID,
			Username:    CLI.Reddit.Username,
		}, CLI.Reddit.Outfile)

	case "fingerpori":
		generateFeed("fingerpori", &fingerpori.Config{
			Limit: CLI.Fingerpori.Limit,
		}, CLI.Fingerpori.Outfile)

	case "preview reddit":
		// Validate required parameters
		if CLI.Preview.Reddit.FeedID == "" || CLI.Preview.Reddit.Username == "" {
			slog.Error("Reddit feed requires both feed_id and username to be set via CLI flags or config file")
			os.Exit(1)
		}
		previewFeed("reddit", &redditjson.Config{
			MinScore:    CLI.Preview.Reddit.MinScore,
			MinComments: CLI.Preview.Reddit.MinComments,
			FeedID:      CLI.Preview.Reddit.FeedID,
			Username:    CLI.Preview.Reddit.Username,
		}, "Reddit", CLI.Preview.Reddit.Limit, CLI.Preview.Reddit.Index)

	case "preview hacker-news":
		previewFeed("hacker-news", &hackernews.Config{
			MinPoints: CLI.Preview.HackerNews.MinPoints,
			Limit:     CLI.Preview.HackerNews.Limit,
		}, "Hacker News", CLI.Preview.HackerNews.Limit, CLI.Preview.HackerNews.Index)

	case "preview fingerpori":
		previewFeed("fingerpori", &fingerpori.Config{
			Limit: CLI.Preview.Fingerpori.Limit,
		}, "Fingerpori", CLI.Preview.Fingerpori.Limit, CLI.Preview.Fingerpori.Index)

	default:
		panic(ctx.Command())
	}
}

// generateFeed is a helper function to create and run a feed provider
func generateFeed(providerName string, config any, outfile string) {
	slog.Debug("Generating feed", "provider", providerName)

	// Create provider using registry
	provider, err := providers.DefaultRegistry.CreateProvider(providerName, config)
	if err != nil {
		slog.Error("Failed to create provider", "provider", providerName, "error", err)
		os.Exit(1)
	}

	// Generate feed
	if err := provider.GenerateFeed(outfile, false); err != nil {
		slog.Error("Failed to generate feed", "provider", providerName, "error", err)
		os.Exit(1)
	}
}

// previewFeed is a helper function to preview feed items
func previewFeed(providerName string, config any, displayName string, limit int, index int) {
	slog.Debug("Previewing feed", "provider", providerName)

	// Create provider using registry
	provider, err := providers.DefaultRegistry.CreateProvider(providerName, config)
	if err != nil {
		slog.Error("Failed to create provider", "provider", providerName, "error", err)
		os.Exit(1)
	}

	// Fetch items
	items, err := provider.FetchItems(limit)
	if err != nil {
		slog.Error("Failed to fetch items", "provider", providerName, "error", err)
		os.Exit(1)
	}

	// Get feed configuration from provider metadata
	metadata := provider.Metadata()
	feedConfig := feed.Config{
		Title:       metadata.Title,
		Link:        metadata.Link,
		Description: metadata.Description,
		Author:      metadata.Author,
		ID:          metadata.ID,
	}

	// If index is specified, output XML directly to stdout
	if index >= 0 {
		if index >= len(items) {
			slog.Error("Index out of range", "index", index, "total", len(items))
			os.Exit(1)
		}
		xml := preview.FormatXMLItem(items[index], metadata.TemplateName, feedConfig)
		fmt.Println(xml)
		return
	}

	// Run preview TUI with template
	if err := preview.Run(items, displayName, metadata.TemplateName, feedConfig); err != nil {
		slog.Error("Preview failed", "error", err)
		os.Exit(1)
	}
}
