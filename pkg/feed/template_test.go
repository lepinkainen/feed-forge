package feed

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

func TestTemplateGenerator_CreateRedditFeedData(t *testing.T) {
	tg := NewTemplateGenerator()

	items := []providers.FeedItem{
		&mockFeedItem{
			title:        "Test Reddit Post",
			link:         "https://example.com/article",
			commentsLink: "https://reddit.com/r/test/comments/123/test",
			author:       "testuser",
			score:        100,
			commentCount: 25,
			createdAt:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			categories:   []string{"r/test"},
		},
	}

	ogData := map[string]*opengraph.Data{
		"https://example.com/article": {
			Title:       "Test Article",
			Description: "Test description",
			Image:       "https://example.com/image.jpg",
			SiteName:    "Example",
		},
	}

	data := tg.CreateRedditFeedData(items, ogData)

	if data.FeedTitle != "Reddit Homepage" {
		t.Errorf("Expected feed title 'Reddit Homepage', got '%s'", data.FeedTitle)
	}

	if len(data.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(data.Items))
	}

	item := data.Items[0]
	if item.Title != "Test Reddit Post" {
		t.Errorf("Expected title 'Test Reddit Post', got '%s'", item.Title)
	}

	if item.Score != 100 {
		t.Errorf("Expected score 100, got %d", item.Score)
	}

	if item.Subreddit != "test" {
		t.Errorf("Expected subreddit 'test', got '%s'", item.Subreddit)
	}
}

func TestTemplateGenerator_CreateHackerNewsFeedData(t *testing.T) {
	tg := NewTemplateGenerator()

	items := []providers.FeedItem{
		&mockFeedItem{
			title:        "Test HN Post",
			link:         "https://example.com/article",
			commentsLink: "https://news.ycombinator.com/item?id=123456",
			author:       "testuser",
			score:        150,
			commentCount: 42,
			createdAt:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			categories:   []string{"example.com", "High Score 100+"},
		},
	}

	ogData := map[string]*opengraph.Data{
		"https://example.com/article": {
			Title:       "Test Article",
			Description: "Test description",
			Image:       "https://example.com/image.jpg",
			SiteName:    "Example",
		},
	}

	data := tg.CreateHackerNewsFeedData(items, ogData)

	if data.FeedTitle != "Hacker News Top Stories" {
		t.Errorf("Expected feed title 'Hacker News Top Stories', got '%s'", data.FeedTitle)
	}

	if len(data.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(data.Items))
	}

	item := data.Items[0]
	if item.Title != "Test HN Post" {
		t.Errorf("Expected title 'Test HN Post', got '%s'", item.Title)
	}

	if item.Score != 150 {
		t.Errorf("Expected score 150, got %d", item.Score)
	}

	if item.Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", item.Domain)
	}
}

func TestTemplateGenerator_LoadTemplate(t *testing.T) {
	tg := NewTemplateGenerator()

	// Create a simple test template content
	templateContent := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>{{.FeedTitle}}</title>
  {{range .Items}}
  <entry>
    <title>{{.Title | xmlEscape}}</title>
    <score>{{.Score}}</score>
  </entry>
  {{end}}
</feed>`

	// Write template to temporary file
	tmpFile := "/tmp/test-template.tmpl"
	err := os.WriteFile(tmpFile, []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test template: %v", err)
	}
	defer func() {
		// Clean up
		_ = os.Remove(tmpFile)
	}()

	// Test loading template
	err = tg.LoadTemplate("test-template", tmpFile)
	if err != nil {
		t.Fatalf("Failed to load template: %v", err)
	}

	// Test template execution
	data := &TemplateData{
		FeedTitle: "Test Feed",
		Items: []TemplateItem{
			{
				Title: "Test & Item",
				Score: 100,
			},
		},
	}

	var output strings.Builder
	err = tg.GenerateFromTemplate("test-template", data, &output)
	if err != nil {
		t.Fatalf("Failed to generate from template: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Test Feed") {
		t.Errorf("Expected output to contain 'Test Feed', got: %s", result)
	}

	if !strings.Contains(result, "Test &amp; Item") {
		t.Errorf("Expected XML-escaped title, got: %s", result)
	}

	if !strings.Contains(result, "<score>100</score>") {
		t.Errorf("Expected score in output, got: %s", result)
	}
}

func TestTemplateGenerator_GetAvailableTemplates(t *testing.T) {
	tg := NewTemplateGenerator()

	// Initially should have no templates
	templates := tg.GetAvailableTemplates()
	if len(templates) != 0 {
		t.Errorf("Expected 0 templates initially, got %d", len(templates))
	}

	// Load a template
	templateContent := `<title>{{.FeedTitle}}</title>`
	tmpFile := "/tmp/test-template.tmpl"
	err := os.WriteFile(tmpFile, []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test template: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile)
	}()

	err = tg.LoadTemplate("test", tmpFile)
	if err != nil {
		t.Fatalf("Failed to load template: %v", err)
	}

	// Should now have one template
	templates = tg.GetAvailableTemplates()
	if len(templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(templates))
	}

	if templates[0] != "test" {
		t.Errorf("Expected template name 'test', got '%s'", templates[0])
	}
}
