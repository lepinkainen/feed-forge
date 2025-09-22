package redditjson

import (
	"fmt"
	"html"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// RedditPost represents a simplified Reddit post structure for our needs
type RedditPost struct {
	Data struct {
		Title        string       `json:"title"`
		URL          string       `json:"url"`
		Permalink    string       `json:"permalink"`
		CreatedUTC   float64      `json:"created_utc"`
		Score        int          `json:"score"`
		NumComments  int          `json:"num_comments"`
		Author       string       `json:"author"`
		Subreddit    string       `json:"subreddit"`
		SelfText     string       `json:"selftext"`
		SelfTextHTML string       `json:"selftext_html"`
		Thumbnail    string       `json:"thumbnail"`
		Preview      *PreviewData `json:"preview,omitempty"`
	} `json:"data"`
}

// PreviewData represents Reddit's preview image data structure
type PreviewData struct {
	Images []PreviewImage `json:"images"`
}

// PreviewImage represents a single preview image with different resolutions
type PreviewImage struct {
	Source      ImageSource   `json:"source"`
	Resolutions []ImageSource `json:"resolutions"`
}

// ImageSource represents an image URL with dimensions
type ImageSource struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// FeedItem interface implementation for RedditPost
func (r *RedditPost) Title() string {
	return r.Data.Title
}

func (r *RedditPost) Link() string {
	return r.Data.URL
}

func (r *RedditPost) CommentsLink() string {
	return "https://www.reddit.com" + r.Data.Permalink
}

func (r *RedditPost) Author() string {
	return r.Data.Author
}

func (r *RedditPost) Score() int {
	return r.Data.Score
}

func (r *RedditPost) CommentCount() int {
	return r.Data.NumComments
}

func (r *RedditPost) CreatedAt() time.Time {
	return time.Unix(int64(r.Data.CreatedUTC), 0)
}

func (r *RedditPost) Categories() []string {
	// Return subreddit in r/ format for enhanced Atom generation
	if r.Data.Subreddit != "" {
		return []string{fmt.Sprintf("r/%s", r.Data.Subreddit)}
	}
	return []string{}
}

// ImageURL returns the best available image URL for the post
func (r *RedditPost) ImageURL() string {
	// Prefer preview image if available (higher quality)
	if r.Data.Preview != nil && len(r.Data.Preview.Images) > 0 {
		source := r.Data.Preview.Images[0].Source
		if source.URL != "" {
			return source.URL
		}
	}

	// Fall back to thumbnail if available
	if r.Data.Thumbnail != "" && r.Data.Thumbnail != "default" && r.Data.Thumbnail != "self" && r.Data.Thumbnail != "nsfw" {
		return r.Data.Thumbnail
	}

	return ""
}

// Content returns the cleaned selftext content for the post
func (r *RedditPost) Content() string {
	if r.Data.SelfTextHTML != "" && r.Data.SelfTextHTML != "null" {
		return cleanRedditHTML(r.Data.SelfTextHTML)
	}
	return ""
}

// cleanRedditHTML removes Reddit-specific HTML comments and decodes HTML entities
func cleanRedditHTML(htmlContent string) string {
	// Fix double-encoded ampersands that come from Reddit's API
	htmlContent = strings.ReplaceAll(htmlContent, "&amp;amp;", "&amp;")

	// First, decode HTML entities (Reddit sends HTML-encoded content)
	htmlContent = html.UnescapeString(htmlContent)

	// Remove Reddit-specific HTML comments
	htmlContent = strings.ReplaceAll(htmlContent, "<!-- SC_OFF -->", "")
	htmlContent = strings.ReplaceAll(htmlContent, "<!-- SC_ON -->", "")

	// Remove any extra whitespace that might result from comment removal
	htmlContent = strings.TrimSpace(htmlContent)

	return htmlContent
}

// RedditListing represents the structure of the Reddit API response for listings
type RedditListing struct {
	Data struct {
		Children []RedditPost `json:"children"`
		After    string       `json:"after"`
	} `json:"data"`
}

// Global variables
var (
	OAuth2Config *oauth2.Config
	Token        *oauth2.Token
)
