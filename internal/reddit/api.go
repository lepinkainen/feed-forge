package reddit

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/lepinkainen/feed-forge/internal/config"
	"github.com/lepinkainen/feed-forge/pkg/api"
	"golang.org/x/oauth2"
)

// RedditAPI handles Reddit API interactions using enhanced HTTP client
type RedditAPI struct {
	client *api.EnhancedClient
}

// NewRedditAPI creates a new Reddit API client with enhanced functionality
func NewRedditAPI(baseClient *http.Client) *RedditAPI {
	enhancedClient := api.NewRedditClient(baseClient)
	return &RedditAPI{
		client: enhancedClient,
	}
}

// FetchRedditHomepage fetches posts from the authenticated user's homepage
// Rate limiting and retry logic are handled by the enhanced client
func (r *RedditAPI) FetchRedditHomepage() ([]RedditPost, error) {
	apiURL := "https://oauth.reddit.com/best?limit=100"
	var listing RedditListing

	err := r.client.GetAndDecode(apiURL, &listing, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Reddit homepage: %w", err)
	}

	slog.Debug("Successfully fetched Reddit homepage posts", "count", len(listing.Data.Children))
	return listing.Data.Children, nil
}

// FetchConcurrentHomepage fetches multiple pages of homepage posts concurrently
// Note: Reddit API pagination would require "after" parameter implementation
func (r *RedditAPI) FetchConcurrentHomepage(pageCount int) ([]RedditPost, error) {
	if pageCount <= 0 {
		pageCount = 1
	}

	// For now, just fetch the first page since pagination requires "after" parameter
	// The enhanced client handles rate limiting and retries automatically
	return r.FetchRedditHomepage()
}

// FilterPosts applies score and comment count filters to a list of Reddit posts
func FilterPosts(posts []RedditPost, minScore, minComments int) []RedditPost {
	var filtered []RedditPost
	for _, post := range posts {
		if post.Data.Score >= minScore && post.Data.NumComments >= minComments {
			filtered = append(filtered, post)
		}
	}

	slog.Debug("Filtered posts", "original", len(posts), "filtered", len(filtered), "minScore", minScore, "minComments", minComments)
	return filtered
}

// ValidateAPIResponse validates the structure of Reddit API responses
func ValidateAPIResponse(listing *RedditListing) error {
	if listing == nil {
		return fmt.Errorf("nil listing received")
	}

	if listing.Data.Children == nil {
		return fmt.Errorf("nil children in listing")
	}

	return nil
}

// UpdateStats updates API call statistics (placeholder for future implementation)
func UpdateStats(endpoint string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}

	slog.Debug("API call completed",
		"endpoint", endpoint,
		"duration", duration,
		"status", status,
	)
}

// CreateAuthenticatedClient creates an OAuth2 authenticated HTTP client
func CreateAuthenticatedClient(ctx context.Context, config *config.Config) *http.Client {
	// Create OAuth2 token from config
	token := &oauth2.Token{
		AccessToken:  config.Reddit.AccessToken,
		RefreshToken: config.Reddit.RefreshToken,
		Expiry:       config.Reddit.ExpiresAt,
	}

	// Create OAuth2 config
	oauthConfig := &oauth2.Config{
		ClientID:     config.Reddit.ClientID,
		ClientSecret: config.Reddit.ClientSecret,
		RedirectURL:  config.Reddit.RedirectURI,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.reddit.com/api/v1/authorize",
			TokenURL: "https://www.reddit.com/api/v1/access_token",
		},
		Scopes: []string{"read"},
	}

	oauthClient := oauthConfig.Client(ctx, token)
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: oauthClient.Transport,
	}
}
