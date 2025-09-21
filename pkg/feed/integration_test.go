package feed

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

func TestHackerNewsTemplateVsHardcoded(t *testing.T) {
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

	// Generate hardcoded feed for comparison using enhanced atom
	generator := NewGenerator(
		"Hacker News Top Stories",
		"High-quality Hacker News stories, updated regularly",
		"https://news.ycombinator.com/",
		"Feed Forge",
	)

	config := HackerNewsEnhancedAtomConfig()
	hardcodedResult, err := generator.GenerateEnhancedAtomWithConfig(items, config, nil)
	if err != nil {
		t.Fatalf("Failed to generate hardcoded feed: %v", err)
	}

	// Basic checks that both outputs are valid XML
	// Template output will have HTML-encoded XML declaration
	if !strings.Contains(templateResult, "&lt;?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Error("Template output missing XML declaration")
	}

	if !strings.Contains(hardcodedResult, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Error("Hardcoded output missing XML declaration")
	}

	// Check that both contain feed elements
	expectedElements := []string{
		"<feed",
		"<title>Hacker News Top Stories</title>",
		"<entry>",
		"<hn:points>150</hn:points>",
		"<hn:comments>42</hn:comments>",
		"Template-based feed generation for HN",
		"testuser",
	}

	for _, element := range expectedElements {
		if !strings.Contains(templateResult, element) {
			t.Errorf("Template output missing expected element: %s", element)
		}
		if !strings.Contains(hardcodedResult, element) {
			t.Errorf("Hardcoded output missing expected element: %s", element)
		}
	}

	// Check that template output includes domain information
	if !strings.Contains(templateResult, "<hn:domain>example.com</hn:domain>") {
		t.Error("Template output missing domain metadata")
	}

	// Check template-specific formatting
	if !strings.Contains(templateResult, "xmlns:hn=\"http://news.ycombinator.com/atom/ns\"") {
		t.Error("Template output missing HN namespace")
	}

	// Basic validation that both generate reasonable feed lengths
	if len(templateResult) < 500 {
		t.Error("Template output seems too short")
	}

	if len(hardcodedResult) < 500 {
		t.Error("Hardcoded output seems too short")
	}

	t.Logf("Template output length: %d characters", len(templateResult))
	t.Logf("Hardcoded output length: %d characters", len(hardcodedResult))

	// Both should have proper score display in content
	if !strings.Contains(templateResult, "<strong>Score:</strong> 150") {
		t.Error("Template output missing score display in content")
	}

	if !strings.Contains(hardcodedResult, "<strong>Score:</strong> 150") {
		t.Error("Hardcoded output missing score display in content")
	}
}
