package tildes

import (
	"encoding/xml"
	"fmt"
	"html"
	"time"
)

// atomFeed mirrors the top-level <feed> element of a Tildes group Atom feed.
type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}

// atomEntry captures the subset of <entry> we parse. In a Tildes feed the
// <id> is always the topic page, while <link rel="alternate"> points to the
// external article for link posts or back to the topic for text posts —
// that split is what drives Item.Link vs Item.CommentsLink.
type atomEntry struct {
	Title   string     `xml:"title"`
	ID      string     `xml:"id"`
	Links   []atomLink `xml:"link"`
	Content string     `xml:"content"`
	Author  atomAuthor `xml:"author"`
	Updated time.Time  `xml:"updated"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

// alternateHref returns the href of the first link with rel="alternate" (or
// the empty rel, which Atom treats as alternate). Falls back to the entry ID
// when no alternate link is present.
func (e *atomEntry) alternateHref() string {
	for _, l := range e.Links {
		if l.Rel == "alternate" || l.Rel == "" {
			return l.Href
		}
	}
	return e.ID
}

// Item wraps a parsed atomEntry and implements providers.FeedItem.
type Item struct {
	entry        atomEntry
	group        string
	cleanContent string
	votes        int
	commentCount int
}

// Title returns the entry title with a "[~group] " prefix. Tildes wraps titles
// in CDATA but still encodes HTML entities inside (e.g. &#34;), so we run them
// through html.UnescapeString to recover the original characters.
func (i *Item) Title() string {
	title := html.UnescapeString(i.entry.Title)
	if i.group != "" {
		return fmt.Sprintf("[%s] %s", i.group, title)
	}
	return title
}

// Link returns the <link rel="alternate"> href — the external article for
// link posts, or the Tildes topic URL for text posts. Used as the OpenGraph
// fetch target by the shared feed generator.
func (i *Item) Link() string {
	return i.entry.alternateHref()
}

// CommentsLink returns the Tildes topic page (the <id>), which is what feed
// readers should open by default. Matches the Reddit provider's behavior.
func (i *Item) CommentsLink() string {
	return i.entry.ID
}

// Author returns the Tildes username from <author><name>.
func (i *Item) Author() string {
	return i.entry.Author.Name
}

// AuthorURI is duck-typed by pkg/feed/generator.go and populates the Atom
// <author><uri> element.
func (i *Item) AuthorURI() string {
	if i.entry.Author.Name == "" {
		return ""
	}
	return "https://tildes.net/user/" + i.entry.Author.Name
}

// Score returns the parsed vote count from the Tildes entry footer.
func (i *Item) Score() int {
	return i.votes
}

// CommentCount returns the parsed comment count from the Tildes entry footer.
func (i *Item) CommentCount() int {
	return i.commentCount
}

// CreatedAt returns the entry's <updated> timestamp.
func (i *Item) CreatedAt() time.Time {
	return i.entry.Updated
}

// Categories returns the group name (e.g. "~tech") as a single-element slice.
func (i *Item) Categories() []string {
	if i.group == "" {
		return nil
	}
	return []string{i.group}
}

// ImageURL is empty — the Tildes Atom feed does not expose thumbnails.
func (i *Item) ImageURL() string {
	return ""
}

// Content returns the entry body with the trailing Tildes footer stripped.
func (i *Item) Content() string {
	return i.cleanContent
}
