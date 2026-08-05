package lemmy

import (
	"encoding/xml"
	"html"
	"strings"
	"time"
)

// rssFeed mirrors the top-level <rss> element of a Lemmy feed. Lemmy serves
// RSS 2.0 with the Dublin Core and Media RSS extensions.
type rssFeed struct {
	XMLName xml.Name  `xml:"rss"`
	Items   []rssItem `xml:"channel>item"`
}

// rssItem captures the subset of <item> we parse. Score, comment count, author
// name, and the external article URL are not separate elements: Lemmy embeds
// them in the HTML of <description>, so parseItem scrapes them out.
type rssItem struct {
	Title        string       `xml:"title"`
	Link         string       `xml:"link"`
	Description  string       `xml:"description"`
	Guid         string       `xml:"guid"`
	Comments     string       `xml:"comments"`
	PubDate      string       `xml:"pubDate"`
	Category     rssCategory  `xml:"category"`
	Enclosure    rssEnclosure `xml:"enclosure"`
	MediaContent rssMedia     `xml:"http://search.yahoo.com/mrss/ content"`
	Creator      string       `xml:"http://purl.org/dc/elements/1.1/ creator"`
}

type rssCategory struct {
	Domain string `xml:"domain,attr"`
	Value  string `xml:",chardata"`
}

type rssEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

type rssMedia struct {
	Medium string `xml:"medium,attr"`
	URL    string `xml:"url,attr"`
}

// Item wraps a parsed rssItem and implements providers.FeedItem.
type Item struct {
	item         rssItem
	instanceHost string
	community    string
	author       string
	authorURI    string
	externalLink string
	cleanContent string
	score        int
	commentCount int
	createdAt    time.Time
}

// Title returns the post title. Lemmy wraps titles in CDATA but still encodes
// HTML entities inside them, so we unescape to recover the original characters.
func (i *Item) Title() string {
	return html.UnescapeString(i.item.Title)
}

// Link returns the external article URL for link posts, or the Lemmy post page
// for text posts. Used as the OpenGraph fetch target by the shared generator.
func (i *Item) Link() string {
	if i.externalLink != "" {
		return i.externalLink
	}
	return i.CommentsLink()
}

// CommentsLink returns the Lemmy post page, which is what feed readers should
// open by default. Matches the Reddit and Tildes providers' behavior.
func (i *Item) CommentsLink() string {
	if i.item.Guid != "" {
		return i.item.Guid
	}
	return i.item.Link
}

// Author returns the poster's username, qualified with "@host" when the account
// lives on a remote instance (e.g. "someone@piefed.world").
func (i *Item) Author() string {
	return i.author
}

// AuthorURI is duck-typed by pkg/feed/generator.go and populates the Atom
// <author><uri> element.
func (i *Item) AuthorURI() string {
	return i.authorURI
}

// Score returns the point count scraped from the item description.
func (i *Item) Score() int {
	return i.score
}

// CommentCount returns the comment count scraped from the item description.
func (i *Item) CommentCount() int {
	return i.commentCount
}

// CreatedAt returns the <pubDate> parsed at the RSS boundary.
func (i *Item) CreatedAt() time.Time {
	return i.createdAt
}

// Categories returns the community name (e.g. "memes") as a single-element
// slice, or nil when the community could not be determined.
func (i *Item) Categories() []string {
	if i.community == "" {
		return nil
	}
	return []string{i.community}
}

// ImageURL prefers the <media:content> thumbnail, falling back to an image
// <enclosure>. Non-image enclosures (link posts enclose the article URL with
// type text/html) are ignored.
func (i *Item) ImageURL() string {
	if i.item.MediaContent.URL != "" {
		return i.item.MediaContent.URL
	}
	if strings.HasPrefix(i.item.Enclosure.Type, "image/") {
		return i.item.Enclosure.URL
	}
	return ""
}

// Content returns the post body with the "submitted by … / N points | N
// comments" header and the leading external-link wrapper stripped. Pure link
// and image posts have no body and return "".
func (i *Item) Content() string {
	return i.cleanContent
}
