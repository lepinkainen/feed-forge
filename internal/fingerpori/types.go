package fingerpori

import (
	"fmt"
	"time"
)

// Item represents a single Fingerpori comic item from the HS.fi JSON API
type Item struct {
	ItemID            int64          `json:"id"`
	Href              string         `json:"href"`
	DisplayDate       string         `json:"displayDate"`
	ItemTitle         string         `json:"title"`
	Picture           Picture        `json:"picture"`
	PaidType          string         `json:"paidType"`
	Category          string         `json:"category"`
	SectionTheme      string         `json:"sectionTheme"`
	InfoRowContent    map[string]any `json:"infoRowContent"`
	Tags              []string       `json:"tags"`
	ParsedDate        time.Time      `json:"-"`
	ProcessedImageURL string         `json:"-"`
	ContentHTML       string         `json:"-"`
}

// Picture represents the comic image metadata
type Picture struct {
	ID           int64  `json:"id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	URL          string `json:"url"`
	SquareURL    string `json:"squareUrl"`
	Photographer string `json:"photographer"`
}

// Title returns the title of the comic
func (f *Item) Title() string {
	return fmt.Sprintf("%s - %s", f.ItemTitle, f.ParsedDate.Format("2006-01-02"))
}

// Link returns the URL to the comic page on HS.fi
func (f *Item) Link() string {
	return fmt.Sprintf("https://www.hs.fi%s", f.Href)
}

// CommentsLink returns the same as Link (no separate comments for Fingerpori)
func (f *Item) CommentsLink() string {
	return f.Link()
}

// Author returns the photographer/author name
func (f *Item) Author() string {
	return f.Picture.Photographer
}

// Score returns 0 (not applicable for Fingerpori comics)
func (f *Item) Score() int {
	return 0
}

// CommentCount returns 0 (not applicable for Fingerpori comics)
func (f *Item) CommentCount() int {
	return 0
}

// CreatedAt returns the publication date of the comic
func (f *Item) CreatedAt() time.Time {
	return f.ParsedDate
}

// Categories returns tags associated with the comic
func (f *Item) Categories() []string {
	return f.Tags
}

// ImageURL returns the processed high-resolution image URL
func (f *Item) ImageURL() string {
	return f.ProcessedImageURL
}

// Content returns the HTML content with the comic image
func (f *Item) Content() string {
	return f.ContentHTML
}
