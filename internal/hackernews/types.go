package hackernews

import "time"

// HackerNewsItem represents a single Hacker News story with metadata
type HackerNewsItem struct {
	ItemID           string
	ItemTitle        string
	ItemLink         string
	ItemCommentsLink string
	Points           int
	ItemCommentCount int
	ItemAuthor       string
	ItemCreatedAt    time.Time
	UpdatedAt        time.Time
	Domain           string   // Domain extracted from Link
	ItemCategories   []string // Categories determined from title, domain, and points
}

// FeedItem interface implementation for HackerNewsItem
func (h *HackerNewsItem) Title() string {
	return h.ItemTitle
}

func (h *HackerNewsItem) Link() string {
	return h.ItemLink
}

func (h *HackerNewsItem) CommentsLink() string {
	return h.ItemCommentsLink
}

func (h *HackerNewsItem) Author() string {
	return h.ItemAuthor
}

func (h *HackerNewsItem) Score() int {
	return h.Points
}

func (h *HackerNewsItem) CommentCount() int {
	return h.ItemCommentCount
}

func (h *HackerNewsItem) CreatedAt() time.Time {
	return h.ItemCreatedAt
}

func (h *HackerNewsItem) Categories() []string {
	return h.ItemCategories
}

func (h *HackerNewsItem) ImageURL() string {
	// HackerNews items typically don't have images
	return ""
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
