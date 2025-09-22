package feed

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// mockFeedItem implements the FeedItem interface for testing
type mockFeedItem struct {
	title        string
	link         string
	commentsLink string
	author       string
	score        int
	commentCount int
	createdAt    time.Time
	categories   []string
	imageURL     string
	content      string
}

func (m *mockFeedItem) Title() string        { return m.title }
func (m *mockFeedItem) Link() string         { return m.link }
func (m *mockFeedItem) CommentsLink() string { return m.commentsLink }
func (m *mockFeedItem) Author() string       { return m.author }
func (m *mockFeedItem) Score() int           { return m.score }
func (m *mockFeedItem) CommentCount() int    { return m.commentCount }
func (m *mockFeedItem) CreatedAt() time.Time { return m.createdAt }
func (m *mockFeedItem) Categories() []string { return m.categories }
func (m *mockFeedItem) ImageURL() string     { return m.imageURL }
func (m *mockFeedItem) Content() string      { return m.content }

func TestHackerNewsTemplateGeneration(t *testing.T) {
	// Skip this test in CI or if templates directory doesn't exist
	if _, err := os.Stat("../../templates/hackernews-atom.tmpl"); os.IsNotExist(err) {
		t.Skip("Templates directory not found, skipping integration test")
	}

	// Create template generator
	tg := NewTemplateGenerator()

	// Load Hacker News template
	err := tg.LoadTemplate("hackernews-atom", "../../templates/hackernews-atom.tmpl")
	if err != nil {
		t.Fatalf("Failed to load Hacker News template: %v", err)
	}

	// Create mock Hacker News items
	items := []providers.FeedItem{
		&mockFeedItem{
			title:        "Template-based feed generation for HN",
			link:         "https://example.com/article",
			commentsLink: "https://news.ycombinator.com/item?id=123456",
			author:       "testuser",
			score:        150,
			commentCount: 42,
			createdAt:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			categories:   []string{"example.com", "High Score 100+"},
		},
		&mockFeedItem{
			title:        "Another HN story",
			link:         "https://golang.org/templates",
			commentsLink: "https://news.ycombinator.com/item?id=789012",
			author:       "gopher",
			score:        89,
			commentCount: 23,
			createdAt:    time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
			categories:   []string{"golang.org", "High Score 20+"},
		},
	}

	// Create OpenGraph data
	ogData := map[string]*opengraph.Data{
		"https://example.com/article": {
			Title:       "Template-based Feed Generation Guide",
			Description: "A comprehensive guide to using Go templates for RSS/Atom feed generation",
			Image:       "https://example.com/image1.jpg",
			SiteName:    "Example.com",
		},
	}

	// Generate template-based feed
	templateData := tg.CreateHackerNewsFeedData(items, ogData)
	var templateOutput strings.Builder
	err = tg.GenerateFromTemplate("hackernews-atom", templateData, &templateOutput)
	if err != nil {
		t.Fatalf("Failed to generate template feed: %v", err)
	}

	templateResult := templateOutput.String()

	// Basic checks that output is valid XML
	if !strings.Contains(templateResult, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Error("Template output missing XML declaration")
	}

	// Check that output contains required feed elements
	expectedElements := []string{
		"<feed",
		"<title>Hacker News Top Stories</title>",
		"<entry>",
		`<category term="points:150" label="Points: 150" scheme="hackernews-metadata"/>`,
		`<category term="comments:42" label="Comments: 42" scheme="hackernews-metadata"/>`,
		"Template-based feed generation for HN",
		"testuser",
	}

	for _, element := range expectedElements {
		if !strings.Contains(templateResult, element) {
			t.Errorf("Template output missing expected element: %s", element)
		}
	}

	// Check that template output includes domain information
	if !strings.Contains(templateResult, `<category term="domain:example.com" label="Domain: example.com" scheme="hackernews-metadata"/>`) {
		t.Error("Template output missing domain metadata")
	}

	// Check that template output uses standard namespaces only
	if !strings.Contains(templateResult, `xmlns="http://www.w3.org/2005/Atom"`) {
		t.Error("Template output missing standard Atom namespace")
	}

	// Basic validation that output generates reasonable feed length
	if len(templateResult) < 500 {
		t.Error("Template output seems too short")
	}

	t.Logf("Template output length: %d characters", len(templateResult))

	// Should have proper score display in content
	if !strings.Contains(templateResult, "<strong>Score:</strong> 150") {
		t.Error("Template output missing score display in content")
	}

	// Should include OpenGraph preview for external links
	if !strings.Contains(templateResult, "Template-based Feed Generation Guide") {
		t.Error("Template output missing OpenGraph preview")
	}
}
