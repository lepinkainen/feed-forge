package youtube

import (
	"net/url"
	"strings"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/atom"
)

// atomFeed mirrors the subset of YouTube's channel Atom feed used by Feed Forge.
// Embedding atom.Feed requires the root <feed> to be in the Atom namespace,
// which YouTube's feed declares.
type atomFeed struct {
	Title     string     `xml:"title"`
	ChannelID string     `xml:"channelId"`
	Author    atomAuthor `xml:"author"`
	Links     []atomLink `xml:"link"`
	atom.Feed[atomEntry]
}

type atomEntry struct {
	ID        string     `xml:"id"`
	VideoID   string     `xml:"videoId"`
	ChannelID string     `xml:"channelId"`
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	Author    atomAuthor `xml:"author"`
	Published atom.Time  `xml:"published"`
	Updated   atom.Time  `xml:"updated"`
	Media     mediaGroup `xml:"group"`
}

type atomLink = atom.Link

type atomAuthor = atom.Person

type mediaGroup struct {
	Title       string         `xml:"title"`
	Content     mediaContent   `xml:"content"`
	Thumbnail   mediaThumbnail `xml:"thumbnail"`
	Description string         `xml:"description"`
	Community   mediaCommunity `xml:"community"`
}

type mediaContent struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
}

type mediaThumbnail struct {
	URL    string `xml:"url,attr"`
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
}

type mediaCommunity struct {
	Statistics mediaStatistics `xml:"statistics"`
}

type mediaStatistics struct {
	Views int `xml:"views,attr"`
}

func (e *atomEntry) alternateHref() string {
	if href := atom.AlternateHref(e.Links); href != "" {
		return href
	}
	if e.VideoID == "" {
		return ""
	}
	return "https://www.youtube.com/watch?v=" + e.VideoID
}

func (e *atomEntry) isShort() bool {
	return isShortURL(e.alternateHref())
}

func isShortURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.Contains(raw, "/shorts/")
	}
	return strings.HasPrefix(u.Path, "/shorts/")
}

// Item wraps a YouTube Atom entry and implements providers.FeedItem.
type Item struct {
	entry        atomEntry
	channelTitle string
}

// Title returns the video title.
func (i *Item) Title() string {
	return i.entry.Title
}

// Link returns the YouTube video URL.
func (i *Item) Link() string {
	return i.entry.alternateHref()
}

// CommentsLink returns the YouTube video URL as the canonical entry link.
func (i *Item) CommentsLink() string {
	return i.Link()
}

// Author returns the channel author name.
func (i *Item) Author() string {
	if i.entry.Author.Name != "" {
		return i.entry.Author.Name
	}
	return i.channelTitle
}

// AuthorURI returns the YouTube channel URL.
func (i *Item) AuthorURI() string {
	return i.entry.Author.URI
}

// Score returns the view count exposed by the YouTube feed.
func (i *Item) Score() int {
	return i.entry.Media.Community.Statistics.Views
}

// CommentCount returns 0 because YouTube Atom feeds do not expose comments.
func (i *Item) CommentCount() int {
	return 0
}

// CreatedAt returns the video's published time, falling back to updated time.
func (i *Item) CreatedAt() time.Time {
	if !i.entry.Published.IsZero() {
		return i.entry.Published.Time
	}
	return i.entry.Updated.Time
}

// Categories returns YouTube plus channel title categories.
func (i *Item) Categories() []string {
	cats := []string{"youtube"}
	if i.channelTitle != "" {
		cats = append(cats, i.channelTitle)
	}
	return cats
}

// ImageURL returns the video thumbnail URL.
func (i *Item) ImageURL() string {
	return i.entry.Media.Thumbnail.URL
}

// Content returns the video description.
func (i *Item) Content() string {
	return i.entry.Media.Description
}
