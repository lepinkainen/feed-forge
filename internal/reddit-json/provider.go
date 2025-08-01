package redditjson

import (
	"fmt"

	"github.com/lepinkainen/feed-forge/internal/config"
	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// constructFeedURL builds the Reddit JSON feed URL from feed ID and username
func constructFeedURL(feedID, username string) string {
	return fmt.Sprintf("https://www.reddit.com/.json?feed=%s&user=%s", feedID, username)
}

// RedditProvider implements the FeedProvider interface for Reddit JSON feeds
type RedditProvider struct {
	*providers.BaseProvider
	MinScore    int
	MinComments int
	FeedID      string
	Username    string
	Config      *config.Config
}

// NewRedditProvider creates a new Reddit JSON provider
func NewRedditProvider(minScore, minComments int, feedID, username string, config *config.Config) providers.FeedProvider {
	base, err := providers.NewBaseProvider(providers.DatabaseConfig{
		ContentDBName: "", // Reddit JSON doesn't use content DB
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
		FeedID:       feedID,
		Username:     username,
		Config:       config,
	}
}

// GenerateFeed implements the FeedProvider interface
func (p *RedditProvider) GenerateFeed(outfile string, reauth bool) error {
	// reauth parameter is ignored for JSON feeds (no authentication needed)

	// Clean up expired entries using base provider
	if err := p.CleanupExpired(); err != nil {
		// Non-fatal error, just warn
	}

	// Construct feed URL from config parameters
	feedURL := constructFeedURL(p.FeedID, p.Username)

	// Create Reddit API client with constructed URL
	redditAPI := NewRedditAPI(feedURL)

	// Fetch Reddit posts from JSON feed
	posts, err := redditAPI.FetchRedditHomepage()
	if err != nil {
		return err
	}

	// Filter posts
	filteredPosts := FilterPosts(posts, p.MinScore, p.MinComments)

	// Create enhanced feed generator (no authentication needed for JSON feed)
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
