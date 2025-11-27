package youtube

import (
	"time"
)

// VideoItem represents a YouTube video with optional DeArrow enrichment
type VideoItem struct {
	VideoID         string
	OriginalTitle   string
	EnrichedTitle   string // DeArrow title if available and trusted
	ChannelName     string
	ChannelURL      string
	VideoURL        string
	ThumbnailURL    string
	Description     string
	PublishedAt     time.Time
	Views           int
	AverageRating   float64
	RatingCount     int
	UseDeArrowTitle bool
}

// Title returns the enriched title if available and enabled, otherwise the original
func (v *VideoItem) Title() string {
	if v.UseDeArrowTitle && v.EnrichedTitle != "" {
		return v.EnrichedTitle
	}
	return v.OriginalTitle
}

// Link returns the YouTube video URL
func (v *VideoItem) Link() string {
	return v.VideoURL
}

// CommentsLink returns the YouTube video URL (same as Link for YouTube)
func (v *VideoItem) CommentsLink() string {
	return v.VideoURL
}

// Author returns the channel name
func (v *VideoItem) Author() string {
	return v.ChannelName
}

// Score returns the view count as the score
func (v *VideoItem) Score() int {
	return v.Views
}

// CommentCount returns 0 as YouTube RSS doesn't provide comment counts
func (v *VideoItem) CommentCount() int {
	return 0
}

// CreatedAt returns the published timestamp
func (v *VideoItem) CreatedAt() time.Time {
	return v.PublishedAt
}

// Categories returns YouTube-specific categories
func (v *VideoItem) Categories() []string {
	categories := []string{"youtube"}
	if v.UseDeArrowTitle && v.EnrichedTitle != "" {
		categories = append(categories, "dearrow")
	}
	return categories
}

// ImageURL returns the thumbnail URL
func (v *VideoItem) ImageURL() string {
	return v.ThumbnailURL
}

// Content returns the video description
func (v *VideoItem) Content() string {
	return v.Description
}
