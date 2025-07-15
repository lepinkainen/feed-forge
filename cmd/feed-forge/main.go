package main

import (
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/lepinkainen/feed-forge/internal/config"
	"github.com/lepinkainen/feed-forge/internal/hackernews"
	"github.com/lepinkainen/feed-forge/internal/pkg/providers"
	"github.com/lepinkainen/feed-forge/internal/reddit"
)

// CLI structure
var CLI struct {
	Config string `help:"Configuration file path" default:"config.yaml"`
	Debug  bool   `help:"Enable debug logging" default:"false"`

	Reddit struct {
		Outfile     string `help:"Output file path" short:"o" default:"reddit.xml"`
		MinScore    int    `help:"Minimum post score" default:"50"`
		MinComments int    `help:"Minimum comment count" default:"10"`
		Reauth      bool   `help:"Force re-authentication with Reddit." default:"false"`
	} `cmd:"" help:"Generate RSS feed from Reddit."`

	HackerNews struct {
		Outfile   string `help:"Output file path" short:"o" default:"hackernews.xml"`
		MinPoints int    `help:"Minimum points threshold" default:"50"`
		Limit     int    `help:"Maximum number of items" default:"30"`
	} `cmd:"hacker-news" help:"Generate RSS feed from Hacker News."`
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
	case "reddit":
		slog.Debug("Generating Reddit feed...")

		// Override config with CLI flags if provided
		minScore := CLI.Reddit.MinScore
		minComments := CLI.Reddit.MinComments

		// Create Reddit provider
		provider = reddit.NewRedditProvider(minScore, minComments, cfg)

		if err := provider.GenerateFeed(CLI.Reddit.Outfile, CLI.Reddit.Reauth); err != nil {
			slog.Error("Failed to generate Reddit feed", "error", err)
			os.Exit(1)
		}

	case "hacker-news":
		slog.Debug("Generating Hacker News feed...")

		// Override config with CLI flags if provided
		minPoints := CLI.HackerNews.MinPoints
		limit := CLI.HackerNews.Limit

		// Load category mapper (for now, pass nil - will be improved later)
		provider = hackernews.NewHackerNewsProvider(minPoints, limit, nil)

		if err := provider.GenerateFeed(CLI.HackerNews.Outfile, false); err != nil {
			slog.Error("Failed to generate Hacker News feed", "error", err)
			os.Exit(1)
		}

	default:
		panic(ctx.Command())
	}
}
