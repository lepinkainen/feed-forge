package reddit

import (
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

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
