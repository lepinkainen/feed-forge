package feissarimokat

import (
	"encoding/xml"
	"time"
)

// RSS represents the top-level RSS XML structure from feissarimokat.com
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

// Channel represents the RSS channel element
type Channel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []RSSItem `xml:"item"`
}

// RSSItem represents a single item in the RSS feed
type RSSItem struct {
	ItemTitle   string `xml:"title"`
	Description string `xml:"description"`
	ItemLink    string `xml:"link"`
}

// Item wraps an RSSItem with scraped image data and implements providers.FeedItem
type Item struct {
	RSSItem
	Images      []string
	ContentHTML string
}

// Title returns the title of the post
func (i *Item) Title() string {
	return i.ItemTitle
}

// Link returns the URL to the post
func (i *Item) Link() string {
	return i.ItemLink
}

// CommentsLink returns the same as Link (no separate comments)
func (i *Item) CommentsLink() string {
	return i.ItemLink
}

// Author returns the site name
func (i *Item) Author() string {
	return "Feissarimokat"
}

// Score returns 0 (not applicable)
func (i *Item) Score() int {
	return 0
}

// CommentCount returns 0 (not applicable)
func (i *Item) CommentCount() int {
	return 0
}

// CreatedAt returns current time (the source RSS has no pubDate per item)
func (i *Item) CreatedAt() time.Time {
	return time.Now()
}

// Categories returns tags for the item
func (i *Item) Categories() []string {
	return []string{"comics", "feissarimokat"}
}

// ImageURL returns the first scraped image URL, or empty
func (i *Item) ImageURL() string {
	if len(i.Images) > 0 {
		return i.Images[0]
	}
	return ""
}

// Content returns the HTML content with embedded images
func (i *Item) Content() string {
	return i.ContentHTML
}
