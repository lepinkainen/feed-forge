package preview

import (
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/testutil"
)

type mockFeedItem struct {
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
}

func (m mockFeedItem) Title() string        { return m.title }
func (m mockFeedItem) Link() string         { return m.link }
func (m mockFeedItem) CommentsLink() string { return m.commentsLink }
func (m mockFeedItem) Author() string       { return m.author }
func (m mockFeedItem) Score() int           { return m.score }
func (m mockFeedItem) CommentCount() int    { return m.comments }
func (m mockFeedItem) CreatedAt() time.Time { return m.createdAt }
func (m mockFeedItem) Categories() []string { return m.categories }
func (m mockFeedItem) ImageURL() string     { return m.imageURL }
func (m mockFeedItem) Content() string      { return m.content }
func (m mockFeedItem) AuthorURI() string    { return "https://example.com/authors/" + m.author }
func (m mockFeedItem) Subreddit() string    { return "" }
func (m mockFeedItem) ItemDomain() string   { return "example.com" }

func TestFormatCompactListItem_Golden(t *testing.T) {
	item := mockFeedItem{
		title:     "A very long title that should be truncated before it exceeds the maximum width allowed in compact preview mode",
		score:     123,
		comments:  45,
		createdAt: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	got := FormatCompactListItem(2, item)
	testutil.CompareGolden(t, filepath.Join("testdata", "compact.txt.golden"), got)
}

func TestFormatDetailedItem_Golden(t *testing.T) {
	item := mockFeedItem{
		title:        "Preview title",
		link:         "https://example.com/post",
		commentsLink: "https://example.com/post/comments",
		author:       "alice",
		score:        123,
		comments:     45,
		categories:   []string{"cats", "dogs"},
		imageURL:     "https://example.com/image.jpg",
		content:      "This content should be wrapped nicely across multiple lines so the preview output stays readable in terminal snapshots.",
	}

	got := FormatDetailedItem(item)
	testutil.CompareGolden(t, filepath.Join("testdata", "detailed.txt.golden"), got)
}

func TestFormatXMLItem_Golden(t *testing.T) {
	oldOverride := feed.GetTemplateOverrideFS()
	oldFallback := feed.GetTemplateFallbackFS()
	t.Cleanup(func() {
		feed.SetTemplateOverrideFS(oldOverride)
		feed.SetTemplateFallbackFS(oldFallback)
	})

	feed.SetTemplateOverrideFS(fstest.MapFS{
		"preview.tmpl": &fstest.MapFile{Data: []byte(`<feed>{{range .Items}}<entry><title>{{xmlEscape .Title}}</title><author>{{.Author}}</author><summary>{{.Summary}}</summary></entry>{{end}}</feed>`)},
	})
	feed.SetTemplateFallbackFS(fstest.MapFS{})

	item := mockFeedItem{
		title:     "Hello & Goodbye",
		author:    "alice",
		score:     123,
		comments:  45,
		createdAt: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	got := FormatXMLItem(item, "preview", feed.Config{Title: "Preview"})
	testutil.CompareGolden(t, filepath.Join("testdata", "xml.txt.golden"), got)
}

func TestWrapText_DefaultWidthAndWhitespace(t *testing.T) {
	got := wrapText("  hello    world  ", 0)
	if got != "hello world" {
		t.Fatalf("wrapText() = %q", got)
	}
}

func TestWrapXMLContent_WrapsLongLines(t *testing.T) {
	got := wrapXMLContent("<tag>one two three four five six seven eight nine ten</tag>", 20)
	if !strings.Contains(got, "\n") {
		t.Fatalf("wrapXMLContent() did not wrap output: %q", got)
	}
}

func TestFormatTimeAgo(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{name: "seconds", t: now.Add(-30 * time.Second), want: "just now"},
		{name: "one minute", t: now.Add(-1 * time.Minute), want: "1 minute ago"},
		{name: "hours", t: now.Add(-2 * time.Hour), want: "2 hours ago"},
		{name: "days", t: now.Add(-48 * time.Hour), want: "2 days ago"},
		{name: "old date", t: now.Add(-10 * 24 * time.Hour), want: now.Add(-10 * 24 * time.Hour).Format("2006-01-02")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTimeAgo(tt.t); got != tt.want {
				t.Fatalf("formatTimeAgo() = %q, want %q", got, tt.want)
			}
		})
	}
}
