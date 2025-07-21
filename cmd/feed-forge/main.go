package main

import (
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/lepinkainen/feed-forge/internal/config"
	"github.com/lepinkainen/feed-forge/internal/hackernews"
	redditjson "github.com/lepinkainen/feed-forge/internal/reddit-json"
	redditoauth "github.com/lepinkainen/feed-forge/internal/reddit-oauth"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// CLI structure
var CLI struct {
	Config string `help:"Configuration file path" default:"config.yaml"`
	Debug  bool   `help:"Enable debug logging" default:"false"`

	RedditOAuth struct {
		Outfile     string `help:"Output file path" short:"o" default:"reddit.xml"`
		MinScore    int    `help:"Minimum post score" default:"50"`
		MinComments int    `help:"Minimum comment count" default:"10"`
		Reauth      bool   `help:"Force re-authentication with Reddit." default:"false"`
	} `cmd:"reddit-oauth" help:"Generate RSS feed from Reddit using OAuth."`

	RedditJSON struct {
		Outfile     string `help:"Output file path" short:"o" default:"reddit.xml"`
		MinScore    int    `help:"Minimum post score" default:"50"`
		MinComments int    `help:"Minimum comment count" default:"10"`
		FeedID      string `help:"Reddit feed ID"`
		Username    string `help:"Reddit username"`
	} `cmd:"reddit-json" help:"Generate RSS feed from Reddit JSON feed."`

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
	case "reddit-oauth":
		slog.Debug("Generating Reddit feed...")

		// Override config with CLI flags if provided
		minScore := CLI.RedditOAuth.MinScore
		minComments := CLI.RedditOAuth.MinComments

		// Create Reddit provider
		provider = redditoauth.NewRedditProvider(minScore, minComments, cfg)
		if provider == nil {
			slog.Error("Failed to create Reddit provider")
			os.Exit(1)
		}

		if err := provider.GenerateFeed(CLI.RedditOAuth.Outfile, CLI.RedditOAuth.Reauth); err != nil {
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
		if provider == nil {
			slog.Error("Failed to create Hacker News provider")
			os.Exit(1)
		}

		if err := provider.GenerateFeed(CLI.HackerNews.Outfile, false); err != nil {
			slog.Error("Failed to generate Hacker News feed", "error", err)
			os.Exit(1)
		}

	case "reddit-json":
		slog.Debug("Generating Reddit JSON feed...")

		// Override config with CLI flags if provided
		minScore := CLI.RedditJSON.MinScore
		minComments := CLI.RedditJSON.MinComments
		feedID := CLI.RedditJSON.FeedID
		username := CLI.RedditJSON.Username

		// Use config values as fallback if CLI flags are empty
		if feedID == "" {
			feedID = cfg.RedditJSON.FeedID
		}
		if username == "" {
			username = cfg.RedditJSON.Username
		}

		// Validate required parameters
		if feedID == "" || username == "" {
			slog.Error("Reddit JSON feed requires both feed_id and username to be set via CLI flags or config file")
			os.Exit(1)
		}

		// Create Reddit JSON provider
		provider = redditjson.NewRedditProvider(minScore, minComments, feedID, username, cfg)
		if provider == nil {
			slog.Error("Failed to create Reddit JSON provider")
			os.Exit(1)
		}

		if err := provider.GenerateFeed(CLI.RedditJSON.Outfile, false); err != nil {
			slog.Error("Failed to generate Reddit JSON feed", "error", err)
			os.Exit(1)
		}

	default:
		panic(ctx.Command())
	}
}
