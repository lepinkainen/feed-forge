package feed

import (
	"reflect"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

type minimalFeedItem struct {
	title        string
	link         string
	commentsLink string
	author       string
	score        int
	comments     int
	createdAt    time.Time
	categories   []string
	imageURL     string
	content      string
	authorURI    string
	subreddit    string
	domain       string
}

func (m minimalFeedItem) Title() string        { return m.title }
func (m minimalFeedItem) Link() string         { return m.link }
func (m minimalFeedItem) CommentsLink() string { return m.commentsLink }
func (m minimalFeedItem) Author() string       { return m.author }
func (m minimalFeedItem) Score() int           { return m.score }
func (m minimalFeedItem) CommentCount() int    { return m.comments }
func (m minimalFeedItem) CreatedAt() time.Time { return m.createdAt }
func (m minimalFeedItem) Categories() []string { return m.categories }
func (m minimalFeedItem) ImageURL() string     { return m.imageURL }
func (m minimalFeedItem) Content() string      { return m.content }
func (m minimalFeedItem) AuthorURI() string    { return m.authorURI }
func (m minimalFeedItem) Subreddit() string    { return m.subreddit }
func (m minimalFeedItem) ItemDomain() string   { return m.domain }

var _ providers.FeedItem = minimalFeedItem{}

func fetcherHasProxy(fetcher any) bool {
	v := reflect.ValueOf(fetcher)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	field := v.FieldByName("proxy")
	return field.IsValid() && !field.IsNil()
}

func TestCreateOGFetcher_UsesProxyOnlyWhenConfigComplete(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   bool
	}{
		{name: "no proxy config", config: Config{}, want: false},
		{name: "missing secret", config: Config{ProxyURL: "https://proxy.example"}, want: false},
		{name: "complete proxy", config: Config{ProxyURL: "https://proxy.example", ProxySecret: "secret"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := createOGFetcher(nil, tt.config)
			if got := fetcherHasProxy(fetcher); got != tt.want {
				t.Fatalf("createOGFetcher() proxy=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateGenericFeedData_PreservesOptionalFields(t *testing.T) {
	createdAt := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	items := []providers.FeedItem{minimalFeedItem{
		title:        "Post",
		link:         "https://example.com/post",
		commentsLink: "https://example.com/comments",
		author:       "alice",
		score:        99,
		comments:     12,
		createdAt:    createdAt,
		categories:   []string{"r/test", "featured"},
		imageURL:     "https://example.com/image.jpg",
		content:      "Body",
		authorURI:    "https://example.com/users/alice",
		subreddit:    "test",
		domain:       "example.com",
	}}
	ogData := map[string]*opengraph.Data{"https://example.com/post": {Title: "OG title"}}
	config := Config{Title: "Feed", Link: "https://feed.example", Description: "Desc", Author: "Forge", ID: "feed-id"}

	data := createGenericFeedData(items, config, ogData)
	if data.FeedTitle != "Feed" || data.FeedLink != "https://feed.example" || data.FeedAuthor != "Forge" || data.FeedID != "feed-id" {
		t.Fatalf("unexpected feed metadata: %#v", data)
	}
	if len(data.Items) != 1 {
		t.Fatalf("len(data.Items) = %d, want 1", len(data.Items))
	}

	item := data.Items[0]
	if item.AuthorURI != "https://example.com/users/alice" || item.Subreddit != "test" || item.Domain != "example.com" {
		t.Fatalf("optional fields not preserved: %#v", item)
	}
	if item.Summary != "Score: 99 | Comments: 12" {
		t.Fatalf("Summary = %q", item.Summary)
	}
	if item.Updated != createdAt.Format(time.RFC3339) || item.Published != createdAt.Format(time.RFC3339) {
		t.Fatalf("timestamps not formatted from CreatedAt: %#v", item)
	}
	if data.OpenGraphData["https://example.com/post"].Title != "OG title" {
		t.Fatalf("OpenGraphData not attached: %#v", data.OpenGraphData)
	}
}
