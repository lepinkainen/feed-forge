package feed

import (
	"time"
)

// Generator handles RSS/Atom feed generation
type Generator struct {
	Title       string
	Description string
	Link        string
	Author      string
}

// NewGenerator creates a new feed generator
func NewGenerator(title, description, link, author string) *Generator {
	return &Generator{
		Title:       title,
		Description: description,
		Link:        link,
		Author:      author,
	}
}

// Item represents a feed item
type Item struct {
	Title       string
	Link        string
	Description string
	Author      string
	Created     time.Time
	ID          string
	Categories  []string
}

// Metadata contains metadata about a generated feed
type Metadata struct {
	Title       string
	Description string
	ItemCount   int
	Created     time.Time
	Updated     time.Time
	OldestItem  time.Time
	NewestItem  time.Time
}

// FeedType represents the type of feed to generate
type FeedType string

const (
	RSS  FeedType = "rss"
	Atom FeedType = "atom"
)
