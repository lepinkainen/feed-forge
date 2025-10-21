// Package main provides the CLI entry point for feed-forge.
package main

import (
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/lepinkainen/feed-forge/internal/config"
	"github.com/lepinkainen/feed-forge/internal/fingerpori"
	"github.com/lepinkainen/feed-forge/internal/hackernews"
	redditjson "github.com/lepinkainen/feed-forge/internal/reddit-json"
	"github.com/lepinkainen/feed-forge/pkg/providers"
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
}

func main() {
	ctx := kong.Parse(&CLI)

	// Configure logging level based on debug flag
	if CLI.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	} else {
		slog.SetLogLoggerLevel(slog.LevelWarn)
	}

	// Load configuration
	cfg, err := config.LoadConfig(CLI.Config)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	var provider providers.FeedProvider

	switch ctx.Command() {
	case "hacker-news":
		slog.Debug("Generating Hacker News feed...")

		// Override config with CLI flags if provided
		minPoints := CLI.HackerNews.MinPoints
		limit := CLI.HackerNews.Limit

		// Load category mapper (for now, pass nil - will be improved later)
		provider = hackernews.NewProvider(minPoints, limit, nil)
		if provider == nil {
			slog.Error("Failed to create Hacker News provider")
			os.Exit(1)
		}

		if err := provider.GenerateFeed(CLI.HackerNews.Outfile, false); err != nil {
			slog.Error("Failed to generate Hacker News feed", "error", err)
			os.Exit(1)
		}

	case "reddit":
		slog.Debug("Generating Reddit feed...")

		// Override config with CLI flags if provided
		minScore := CLI.Reddit.MinScore
		minComments := CLI.Reddit.MinComments
		feedID := CLI.Reddit.FeedID
		username := CLI.Reddit.Username

		// Use config values as fallback if CLI flags are empty
		if feedID == "" {
			feedID = cfg.Reddit.FeedID
		}
		if username == "" {
			username = cfg.Reddit.Username
		}

		// Validate required parameters
		if feedID == "" || username == "" {
			slog.Error("Reddit feed requires both feed_id and username to be set via CLI flags or config file")
			os.Exit(1)
		}

		// Create Reddit provider
		provider = redditjson.NewRedditProvider(minScore, minComments, feedID, username, cfg)
		if provider == nil {
			slog.Error("Failed to create Reddit provider")
			os.Exit(1)
		}

		if err := provider.GenerateFeed(CLI.Reddit.Outfile, false); err != nil {
			slog.Error("Failed to generate Reddit feed", "error", err)
			os.Exit(1)
		}

	case "fingerpori":
		slog.Debug("Generating Fingerpori feed...")

		// Override config with CLI flags if provided
		limit := CLI.Fingerpori.Limit

		// Create Fingerpori provider
		provider = fingerpori.NewProvider(limit)
		if provider == nil {
			slog.Error("Failed to create Fingerpori provider")
			os.Exit(1)
		}

		if err := provider.GenerateFeed(CLI.Fingerpori.Outfile, false); err != nil {
			slog.Error("Failed to generate Fingerpori feed", "error", err)
			os.Exit(1)
		}

	default:
		panic(ctx.Command())
	}
}
