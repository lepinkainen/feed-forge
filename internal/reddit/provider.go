package reddit

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lepinkainen/feed-forge/internal/config"
	"github.com/lepinkainen/feed-forge/internal/pkg/providers"
	"github.com/lepinkainen/feed-forge/pkg/database"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
)

// RedditProvider implements the FeedProvider interface for Reddit
type RedditProvider struct {
	MinScore    int
	MinComments int
	Config      *config.Config
}

// NewRedditProvider creates a new Reddit provider
func NewRedditProvider(minScore, minComments int, config *config.Config) providers.FeedProvider {
	return &RedditProvider{
		MinScore:    minScore,
		MinComments: minComments,
		Config:      config,
	}
}

// GenerateFeed implements the FeedProvider interface
func (p *RedditProvider) GenerateFeed(outfile string, reauth bool) error {
	// If reauth is requested, clear the refresh token
	if reauth {
		p.Config.Reddit.RefreshToken = ""
	}
	// Initialize OpenGraph database
	ogDBPath, err := database.GetDefaultPath("opengraph.db")
	if err != nil {
		return err
	}

	ogDB, err := opengraph.NewDatabase(ogDBPath)
	if err != nil {
		return err
	}
	defer ogDB.Close()

	// Clean up expired entries
	if err := ogDB.CleanupExpiredEntries(); err != nil {
		slog.Warn("Failed to cleanup expired entries", "error", err)
	}

	// Authenticate and get the token
	token, err := handleAuthentication(p.Config)
	if err != nil {
		return err
	}

	// Create authenticated HTTP client
	ctx := context.Background()
	client := getOAuthConfig(p.Config).Client(ctx, token)

	// Create Reddit API client
	redditAPI := NewRedditAPI(client)

	// Fetch Reddit homepage posts
	posts, err := redditAPI.FetchRedditHomepage()
	if err != nil {
		return err
	}

	// Filter posts
	filteredPosts := FilterPosts(posts, p.MinScore, p.MinComments)

	// Create OpenGraph fetcher
	ogFetcher := opengraph.NewFetcher(ogDB)

	// Create feed generator
	feedGenerator := NewFeedGenerator(ogFetcher)

	// Ensure output directory exists
	outDir := filepath.Dir(outfile)
	err = os.MkdirAll(outDir, 0755)
	if err != nil {
		return err
	}

	// Generate enhanced Atom feed (hardcoded to always use atom with enhanced features)
	if err := feedGenerator.SaveCustomAtomFeedToFile(filteredPosts, outfile); err != nil {
		return err
	}

	slog.Info("RSS feed saved", "count", len(filteredPosts), "filename", outfile)
	return nil
}
