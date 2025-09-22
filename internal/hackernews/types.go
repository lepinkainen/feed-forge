package hackernews

import (
	"fmt"
	"time"
)

// Item represents a single Hacker News story with metadata
type Item struct {
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

// Title returns the title of the Hacker News item
func (h *Item) Title() string {
	return h.ItemTitle
}

// Link returns the URL of the Hacker News item
func (h *Item) Link() string {
	return h.ItemLink
}

// CommentsLink returns the URL to the comments page
func (h *Item) CommentsLink() string {
	return h.ItemCommentsLink
}

// Author returns the author of the Hacker News item
func (h *Item) Author() string {
	return h.ItemAuthor
}

// Score returns the points/score of the Hacker News item
func (h *Item) Score() int {
	return h.Points
}

// CommentCount returns the number of comments on the item
func (h *Item) CommentCount() int {
	return h.ItemCommentCount
}

// CreatedAt returns the creation time of the item
func (h *Item) CreatedAt() time.Time {
	return h.ItemCreatedAt
}

// Categories returns the categories assigned to the item
func (h *Item) Categories() []string {
	return h.ItemCategories
}

// ImageURL returns the image URL for the item (empty for HN items)
func (h *Item) ImageURL() string {
	// HackerNews items typically don't have images
	return ""
}

// Content returns the body content of the item (empty for HN items)
func (h *Item) Content() string {
	// HackerNews items don't have body content, only titles and links
	return ""
}

// AuthorURI returns the Hacker News user profile URL
func (h *Item) AuthorURI() string {
	return fmt.Sprintf("https://news.ycombinator.com/user?id=%s", h.ItemAuthor)
}

// ItemDomain returns the domain extracted from the item link
func (h *Item) ItemDomain() string {
	return h.Domain
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
