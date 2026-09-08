package slashdot

import (
	"time"

	"github.com/lepinkainen/feed-forge/pkg/atom"
)

type atomFeed = atom.Feed[atomEntry]

// atomEntry mirrors one Slashdot <entry>. Fallible scalars decode leniently
// (atom.Time) or as text (Comments) so malformed metadata affects only its
// own entry. The unexported fields are derived once by parseEntry.
type atomEntry struct {
	ID         string         `xml:"id"`
	Title      string         `xml:"title"`
	Links      []atomLink     `xml:"link"`
	Summary    atomText       `xml:"summary"`
	Updated    atom.Time      `xml:"updated"`
	Author     string         `xml:"author>name"`
	Categories []atomCategory `xml:"category"`
	Comments   string         `xml:"http://purl.org/rss/1.0/modules/slash/ comments"`

	link     string
	content  string
	comments int
}

type atomText = atom.Text

type atomLink = atom.Link

type atomCategory = atom.Category

// Item is a Slashdot story implementing providers.FeedItem.
type Item struct{ entry atomEntry }

// Title returns the story title.
func (i *Item) Title() string { return i.entry.Title }

// Link returns the Slashdot story URL without feed tracking parameters.
func (i *Item) Link() string { return i.entry.link }

// CommentsLink returns the story's discussion page.
func (i *Item) CommentsLink() string { return i.Link() }

// Author returns the story editor's name.
func (i *Item) Author() string { return i.entry.Author }

// Score returns zero because Slashdot exposes no story vote score.
func (i *Item) Score() int { return 0 }

// CommentCount returns the count supplied by Slashdot.
func (i *Item) CommentCount() int { return i.entry.comments }

// CreatedAt returns the upstream story timestamp.
func (i *Item) CreatedAt() time.Time { return i.entry.Updated.Time }

// Categories returns the story's topic names.
func (i *Item) Categories() []string {
	categories := make([]string, 0, len(i.entry.Categories))
	for _, category := range i.entry.Categories {
		categories = append(categories, category.Term)
	}
	return categories
}

// ImageURL returns empty because the feed has no story thumbnails.
func (i *Item) ImageURL() string { return "" }

// Content returns the summary HTML with Slashdot's sharing footer removed.
func (i *Item) Content() string { return i.entry.content }
