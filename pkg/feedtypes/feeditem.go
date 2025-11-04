// Package feedtypes provides shared type definitions used by feed generation.
package feedtypes

import "time"

// FeedItem defines the essential fields for any feed entry.
type FeedItem interface {
	Title() string
	Link() string
	CommentsLink() string
	Author() string
	Score() int
	CommentCount() int
	CreatedAt() time.Time
	Categories() []string
	ImageURL() string
	Content() string
}
