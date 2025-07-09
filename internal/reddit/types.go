package reddit

import (
	"time"

	"golang.org/x/oauth2"
)

// Config struct to hold application settings and tokens
type Config struct {
	ClientID      string    `json:"client_id"`
	ClientSecret  string    `json:"client_secret"` // This will be empty for "installed app" type
	RedirectURI   string    `json:"redirect_uri"`
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token"`
	ExpiresAt     time.Time `json:"expires_at"`
	ScoreFilter   int       `json:"score_filter"`
	CommentFilter int       `json:"comment_filter"`
	FeedType      string    `json:"feed_type"`     // "rss" or "atom"
	EnhancedAtom  bool      `json:"enhanced_atom"` // Use enhanced Atom features
	OutputPath    string    `json:"output_path"`
}

// RedditPost represents a simplified Reddit post structure for our needs
type RedditPost struct {
	Data struct {
		Title       string  `json:"title"`
		URL         string  `json:"url"`
		Permalink   string  `json:"permalink"`
		CreatedUTC  float64 `json:"created_utc"`
		Score       int     `json:"score"`
		NumComments int     `json:"num_comments"`
		Author      string  `json:"author"`
		Subreddit   string  `json:"subreddit"`
	} `json:"data"`
}

// RedditListing represents the structure of the Reddit API response for listings
type RedditListing struct {
	Data struct {
		Children []RedditPost `json:"children"`
		After    string       `json:"after"`
	} `json:"data"`
}

// Global constants
const (
	ConfigFileName = "reddit_config.json"
)

// Global variables
var (
	OAuth2Config *oauth2.Config
	Token        *oauth2.Token
	GlobalConfig Config
)
