package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/lepinkainen/feed-forge/internal/config"
	"golang.org/x/oauth2"
)

// RedditAPI handles Reddit API interactions
type RedditAPI struct {
	client      *http.Client
	userAgent   string
	rateLimiter *RateLimiter
}

// RateLimiter implements simple rate limiting for API calls
type RateLimiter struct {
	mu       sync.Mutex
	lastCall time.Time
	minDelay time.Duration
}

// NewRateLimiter creates a new rate limiter with minimum delay between calls
func NewRateLimiter(minDelay time.Duration) *RateLimiter {
	return &RateLimiter{
		minDelay: minDelay,
	}
}

// Wait blocks until it's safe to make another API call
func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	elapsed := time.Since(rl.lastCall)
	if elapsed < rl.minDelay {
		time.Sleep(rl.minDelay - elapsed)
	}
	rl.lastCall = time.Now()
}

// NewRedditAPI creates a new Reddit API client
func NewRedditAPI(client *http.Client) *RedditAPI {
	return &RedditAPI{
		client:      client,
		userAgent:   "FeedForge/1.0 by theshrike79",
		rateLimiter: NewRateLimiter(1 * time.Second), // 1 second minimum between calls
	}
}

// FetchRedditHomepage fetches posts from the authenticated user's homepage with retry logic
func (api *RedditAPI) FetchRedditHomepage() ([]RedditPost, error) {
	const maxRetries = 3
	var posts []RedditPost
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 2 * time.Second
			slog.Warn("Retrying Reddit API call", "attempt", attempt+1, "backoff", backoff)
			time.Sleep(backoff)
		}

		posts, err = api.fetchHomepageWithRateLimit()
		if err == nil {
			break
		}

		// If it's a rate limit error, wait longer
		if isRateLimitError(err) {
			slog.Warn("Rate limited by Reddit API", "attempt", attempt+1)
			time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
			continue
		}

		// For other errors, log and continue retrying
		slog.Warn("Reddit API request failed", "attempt", attempt+1, "error", err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch Reddit homepage after %d attempts: %w", maxRetries, err)
	}

	slog.Debug("Successfully fetched Reddit homepage posts", "count", len(posts))
	return posts, nil
}

// fetchHomepageWithRateLimit fetches homepage posts with rate limiting
func (api *RedditAPI) fetchHomepageWithRateLimit() ([]RedditPost, error) {
	api.rateLimiter.Wait()

	// Reddit API endpoint for user's front page. Limit to 100 posts for a good sample.
	// For a logged-in user, this is usually accessed via /hot or /best without a subreddit prefix.
	// Let's use /best as it's often the default sorted homepage.
	apiURL := "https://oauth.reddit.com/best?limit=100" // User's personalized "best" feed

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", api.userAgent)

	resp, err := api.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Reddit API returned non-OK status: %s", resp.Status)
	}

	var listing RedditListing
	err = json.NewDecoder(resp.Body).Decode(&listing)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Reddit API response: %w", err)
	}

	return listing.Data.Children, nil
}

// FetchConcurrentHomepage fetches multiple pages of homepage posts concurrently
func (api *RedditAPI) FetchConcurrentHomepage(pageCount int) ([]RedditPost, error) {
	if pageCount <= 0 {
		pageCount = 1
	}

	type result struct {
		posts []RedditPost
		err   error
	}

	results := make(chan result, pageCount)
	var wg sync.WaitGroup

	// First page
	wg.Add(1)
	go func() {
		defer wg.Done()
		posts, err := api.fetchHomepageWithRateLimit()
		results <- result{posts: posts, err: err}
	}()

	// Additional pages would require pagination logic
	// For now, just fetch the first page

	wg.Wait()
	close(results)

	var allPosts []RedditPost
	for res := range results {
		if res.err != nil {
			return nil, res.err
		}
		allPosts = append(allPosts, res.posts...)
	}

	return allPosts, nil
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

// isRateLimitError checks if an error is due to rate limiting
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	// Check for OAuth2 retrieve error with 429 status
	if oe, ok := err.(*oauth2.RetrieveError); ok {
		return oe.Response.StatusCode == http.StatusTooManyRequests
	}

	return false
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

	return oauthConfig.Client(ctx, token)
}
