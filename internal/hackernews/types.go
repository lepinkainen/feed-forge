package hackernews

import "time"

// HackerNewsItem represents a single Hacker News story with metadata
type HackerNewsItem struct {
	ItemID       string
	Title        string
	Link         string
	CommentsLink string
	Points       int
	CommentCount int
	Author       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AlgoliaResponse represents the response structure from Algolia API
type AlgoliaResponse struct {
	Hits []AlgoliaHit `json:"hits"`
}

// AlgoliaHit represents a single hit from Algolia search results
type AlgoliaHit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	CreatedAt   string `json:"created_at"`
}

// statsUpdate represents the result of updating an item's statistics
type statsUpdate struct {
	itemID       string
	points       int
	commentCount int
	err          error
	isDeadItem   bool
}
