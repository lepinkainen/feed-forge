package reddit

import (
	"context"

	"github.com/lepinkainen/feed-forge/internal/config"
	"github.com/lepinkainen/feed-forge/internal/pkg/providers"
	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
)

// RedditProvider implements the FeedProvider interface for Reddit
type RedditProvider struct {
	*providers.BaseProvider
	MinScore    int
	MinComments int
	Config      *config.Config
}

// NewRedditProvider creates a new Reddit provider
func NewRedditProvider(minScore, minComments int, config *config.Config) providers.FeedProvider {
	base, err := providers.NewBaseProvider(providers.DatabaseConfig{
		ContentDBName: "", // Reddit doesn't use content DB
		UseContentDB:  false,
	})
	if err != nil {
		// TODO: Handle error properly - for now return nil
		return nil
	}

	return &RedditProvider{
		BaseProvider: base,
		MinScore:     minScore,
		MinComments:  minComments,
		Config:       config,
	}
}

// GenerateFeed implements the FeedProvider interface
func (p *RedditProvider) GenerateFeed(outfile string, reauth bool) error {
	// If reauth is requested, clear the refresh token
	if reauth {
		p.Config.Reddit.RefreshToken = ""
	}

	// Clean up expired entries using base provider
	if err := p.CleanupExpired(); err != nil {
		// Non-fatal error, just warn
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

	// Create enhanced feed generator
	feedHelper := feed.NewEnhancedFeedGenerator(p.OgDB)
	feedGenerator := NewFeedGenerator(feedHelper.OGFetcher)

	// Ensure output directory exists
	if err := filesystem.EnsureDirectoryExists(outfile); err != nil {
		return err
	}

	// Generate enhanced Atom feed (hardcoded to always use atom with enhanced features)
	if err := feedGenerator.SaveCustomAtomFeedToFile(filteredPosts, outfile); err != nil {
		return err
	}

	feed.LogFeedGeneration(len(filteredPosts), outfile)
	return nil
}
