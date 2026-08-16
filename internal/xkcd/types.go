package xkcd

import (
	"encoding/xml"
	"time"
)

// RSS is the top-level xkcd RSS document.
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

// Channel is the RSS channel element.
type Channel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []RSSItem `xml:"item"`
}

// RSSItem is a single xkcd comic entry. The comic image and its essential
// mouseover caption both live inside the HTML-encoded Description.
type RSSItem struct {
	ItemTitle   string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// Item wraps an RSSItem with the values parsed out of its Description and
// implements providers.FeedItem.
type Item struct {
	RSSItem
	imageURL  string
	caption   string // the img title/alt text; essential to the comic
	content   string // ready-to-render HTML: image plus visible caption
	createdAt time.Time
}

// Title returns the comic title.
func (i *Item) Title() string { return i.ItemTitle }

// Link returns the comic page URL.
func (i *Item) Link() string { return i.RSSItem.Link }

// CommentsLink returns the same URL as Link (xkcd has no separate discussion).
func (i *Item) CommentsLink() string { return i.RSSItem.Link }

// Author returns the site name.
func (i *Item) Author() string { return "xkcd" }

// Score returns 0 (not applicable to xkcd).
func (i *Item) Score() int { return 0 }

// CommentCount returns 0 (not applicable to xkcd).
func (i *Item) CommentCount() int { return 0 }

// CreatedAt returns the comic publication date.
func (i *Item) CreatedAt() time.Time { return i.createdAt }

// Categories returns tags for the item.
func (i *Item) Categories() []string { return []string{"comics", "xkcd"} }

// ImageURL returns the comic image URL, or empty if none was found.
func (i *Item) ImageURL() string { return i.imageURL }

// Content returns the comic image followed by its caption as visible text.
func (i *Item) Content() string { return i.content }
