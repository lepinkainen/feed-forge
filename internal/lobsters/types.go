package lobsters

import (
	"html"
	"time"
)

// story mirrors the subset of fields returned by lobste.rs's JSON story
// listing endpoints (e.g. https://lobste.rs/hottest.json). url is empty for
// text/Ask posts, which host their body in description instead.
type story struct {
	ShortID          string   `json:"short_id"`
	ShortIDURL       string   `json:"short_id_url"`
	CreatedAt        string   `json:"created_at"`
	Title            string   `json:"title"`
	URL              string   `json:"url"`
	Score            int      `json:"score"`
	Flags            int      `json:"flags"`
	CommentCount     int      `json:"comment_count"`
	Description      string   `json:"description"`
	DescriptionPlain string   `json:"description_plain"`
	SubmitterUser    string   `json:"submitter_user"`
	UserIsAuthor     bool     `json:"user_is_author"`
	Tags             []string `json:"tags"`
	CommentsURL      string   `json:"comments_url"`
}

// Item wraps a parsed lobste.rs story and implements providers.FeedItem.
type Item struct {
	story     story
	createdAt time.Time
}

// Title returns the story title, unescaping any HTML entities.
func (i *Item) Title() string {
	return html.UnescapeString(i.story.Title)
}

// Link returns the external article URL for link posts, falling back to the
// comments page for text/Ask posts (which have no url).
func (i *Item) Link() string {
	if i.story.URL == "" {
		return i.CommentsLink()
	}
	return i.story.URL
}

// CommentsLink returns the lobste.rs story/comments page.
func (i *Item) CommentsLink() string {
	return i.story.CommentsURL
}

// Author returns the submitting user's username.
func (i *Item) Author() string {
	return i.story.SubmitterUser
}

// AuthorURI is duck-typed by pkg/feed/generator.go and populates the Atom
// <author><uri> element. https://lobste.rs/u/<user> 301-redirects to the
// canonical https://lobste.rs/~<user>, so we link directly to the canonical
// form.
func (i *Item) AuthorURI() string {
	if i.story.SubmitterUser == "" {
		return ""
	}
	return "https://lobste.rs/~" + i.story.SubmitterUser
}

// Score returns the story's vote score.
func (i *Item) Score() int {
	return i.story.Score
}

// CommentCount returns the number of comments on the story.
func (i *Item) CommentCount() int {
	return i.story.CommentCount
}

// CreatedAt returns the parsed submission timestamp.
func (i *Item) CreatedAt() time.Time {
	return i.createdAt
}

// Categories returns the story's tags, or nil if it has none.
func (i *Item) Categories() []string {
	if len(i.story.Tags) == 0 {
		return nil
	}
	return i.story.Tags
}

// ImageURL is empty — lobste.rs does not expose story thumbnails.
func (i *Item) ImageURL() string {
	return ""
}

// Content returns the story's HTML description, empty for pure link posts.
func (i *Item) Content() string {
	return i.story.Description
}
