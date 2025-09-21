package feed

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// Mock implementations for testing
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

// Test media thumbnail in full feed generation
func TestGenerateEnhancedAtomWithConfig_MediaThumbnail(t *testing.T) {
	g := NewGenerator("Test Feed", "https://example.com", "test@example.com", "Test Author")
	config := DefaultEnhancedAtomConfig()
	config.Title = "Test Enhanced Feed"

	items := []providers.FeedItem{
		&mockFeedItem{
			title:        "Post with Image",
			link:         "https://example.com/post",
			commentsLink: "https://example.com/post/comments",
			author:       "testuser",
			score:        100,
			commentCount: 20,
			createdAt:    time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
			categories:   []string{"test"},
			imageURL:     "https://i.redd.it/test123.jpg",
		},
		&mockFeedItem{
			title:        "Post without Image",
			link:         "https://example.com/post2",
			commentsLink: "https://example.com/post2/comments",
			author:       "testuser2",
			score:        50,
			commentCount: 10,
			createdAt:    time.Date(2023, 1, 16, 10, 30, 0, 0, time.UTC),
			categories:   []string{"test"},
			imageURL:     "", // No image
		},
	}

	feed, err := g.GenerateEnhancedAtomWithConfig(items, config, nil)
	if err != nil {
		t.Errorf("GenerateEnhancedAtomWithConfig() error = %v", err)
		return
	}

	// Test that media namespace is included
	if !strings.Contains(feed, `xmlns:media="http://search.yahoo.com/mrss/"`) {
		t.Errorf("Generated feed missing media namespace")
	}

	// Test that media thumbnail is generated for post with image
	if !strings.Contains(feed, `<media:thumbnail url="https://i.redd.it/test123.jpg"/>`) {
		t.Errorf("Generated feed missing media thumbnail for post with image")
	}

	// Verify the post without image doesn't have a media thumbnail
	// We can't easily test negative cases in the full feed, but we can count occurrences
	thumbnailCount := strings.Count(feed, `<media:thumbnail`)
	if thumbnailCount != 1 {
		t.Errorf("Expected 1 media thumbnail, found %d", thumbnailCount)
	}
}

// Test media thumbnail generation
func TestGenerateMediaThumbnail(t *testing.T) {
	g := NewGenerator("Test Feed", "https://example.com", "test@example.com", "Test Author")

	tests := []struct {
		name     string
		item     *mockFeedItem
		expected string
	}{
		{
			name: "With image URL",
			item: &mockFeedItem{
				title:    "Test Post",
				link:     "https://example.com",
				imageURL: "https://example.com/image.jpg",
			},
			expected: `<media:thumbnail url="https://example.com/image.jpg"/>`,
		},
		{
			name: "Without image URL",
			item: &mockFeedItem{
				title:    "Test Post",
				link:     "https://example.com",
				imageURL: "",
			},
			expected: "",
		},
		{
			name: "With special characters in URL",
			item: &mockFeedItem{
				title:    "Test Post",
				link:     "https://example.com",
				imageURL: "https://example.com/image with spaces & special chars.jpg",
			},
			expected: `<media:thumbnail url="https://example.com/image with spaces &amp; special chars.jpg"/>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result strings.Builder
			g.generateMediaThumbnail(&result, tt.item)

			if result.String() != tt.expected {
				t.Errorf("generateMediaThumbnail() = %v, want %v", result.String(), tt.expected)
			}
		})
	}
}

// For testing OpenGraph data without a real fetcher
func createMockOpenGraphData() map[string]*opengraph.Data {
	return map[string]*opengraph.Data{
		"https://example.com": {
			Title:       "Example Article",
			Description: "This is an example article",
			Image:       "https://example.com/image.jpg",
			SiteName:    "Example Site",
		},
		"https://news.ycombinator.com/item?id=123": {
			Title:       "HN Article",
			Description: "A Hacker News article",
			Image:       "https://hn.example.com/hn.png",
			SiteName:    "Hacker News",
		},
	}
}

func TestDefaultEnhancedAtomConfig(t *testing.T) {
	config := DefaultEnhancedAtomConfig()

	if config == nil {
		t.Errorf("DefaultEnhancedAtomConfig() returned nil")
		return
	}

	// Test default values
	if config.Title != "Enhanced Feed" {
		t.Errorf("DefaultEnhancedAtomConfig().Title = %v, want %v", config.Title, "Enhanced Feed")
	}

	if config.Generator != "Feed Forge" {
		t.Errorf("DefaultEnhancedAtomConfig().Generator = %v, want %v", config.Generator, "Feed Forge")
	}

	if !config.MultipleLinks {
		t.Errorf("DefaultEnhancedAtomConfig().MultipleLinks should be true")
	}

	if !config.EnhancedContent {
		t.Errorf("DefaultEnhancedAtomConfig().EnhancedContent should be true")
	}

	if !config.OpenGraphIntegration {
		t.Errorf("DefaultEnhancedAtomConfig().OpenGraphIntegration should be true")
	}

	if config.CustomMetadata {
		t.Errorf("DefaultEnhancedAtomConfig().CustomMetadata should be false by default")
	}
}

func TestRedditEnhancedAtomConfig(t *testing.T) {
	config := RedditEnhancedAtomConfig()

	if config == nil {
		t.Errorf("RedditEnhancedAtomConfig() returned nil")
		return
	}

	// Test Reddit-specific values
	if config.CustomNamespace != "reddit" {
		t.Errorf("RedditEnhancedAtomConfig().CustomNamespace = %v, want %v", config.CustomNamespace, "reddit")
	}

	if config.CustomNamespaceURI != "http://reddit.com/atom/ns" {
		t.Errorf("RedditEnhancedAtomConfig().CustomNamespaceURI = %v, want expected URI", config.CustomNamespaceURI)
	}

	if config.Title != "Reddit Homepage" {
		t.Errorf("RedditEnhancedAtomConfig().Title = %v, want %v", config.Title, "Reddit Homepage")
	}

	if !config.CustomMetadata {
		t.Errorf("RedditEnhancedAtomConfig().CustomMetadata should be true")
	}

	// Should inherit default behavior
	if !config.MultipleLinks {
		t.Errorf("RedditEnhancedAtomConfig().MultipleLinks should be true")
	}
}

func TestHackerNewsEnhancedAtomConfig(t *testing.T) {
	config := HackerNewsEnhancedAtomConfig()

	if config == nil {
		t.Errorf("HackerNewsEnhancedAtomConfig() returned nil")
		return
	}

	// Test Hacker News-specific values
	if config.CustomNamespace != "hn" {
		t.Errorf("HackerNewsEnhancedAtomConfig().CustomNamespace = %v, want %v", config.CustomNamespace, "hn")
	}

	if config.CustomNamespaceURI != "http://news.ycombinator.com/atom/ns" {
		t.Errorf("HackerNewsEnhancedAtomConfig().CustomNamespaceURI = %v, want expected URI", config.CustomNamespaceURI)
	}

	if config.Title != "Hacker News Top Stories" {
		t.Errorf("HackerNewsEnhancedAtomConfig().Title = %v, want %v", config.Title, "Hacker News Top Stories")
	}

	if !config.CustomMetadata {
		t.Errorf("HackerNewsEnhancedAtomConfig().CustomMetadata should be true")
	}
}

func TestGenerateEnhancedAtomWithConfig_Basic(t *testing.T) {
	generator := NewGenerator("Test Feed", "https://test.com", "test-id", "Test Author")
	config := DefaultEnhancedAtomConfig()
	config.Title = "Test Enhanced Feed"

	now := time.Now()
	items := []providers.FeedItem{
		&mockFeedItem{
			title:        "Test Article",
			link:         "https://example.com",
			commentsLink: "https://example.com/comments",
			author:       "Test Author",
			score:        100,
			commentCount: 25,
			createdAt:    now,
			categories:   []string{"tech", "news"},
		},
	}

	feed, err := generator.GenerateEnhancedAtomWithConfig(items, config, nil)

	if err != nil {
		t.Errorf("GenerateEnhancedAtomWithConfig() error = %v", err)
		return
	}

	// Test basic XML structure
	if !strings.Contains(feed, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Errorf("Generated feed missing XML declaration")
	}

	if !strings.Contains(feed, `<feed xmlns="http://www.w3.org/2005/Atom" xmlns:media="http://search.yahoo.com/mrss/">`) {
		t.Errorf("Generated feed missing Atom namespace")
	}

	// Test feed metadata
	if !strings.Contains(feed, "<title>Test Enhanced Feed</title>") {
		t.Errorf("Generated feed missing title")
	}

	// Test entry content
	if !strings.Contains(feed, "<title>Test Article</title>") {
		t.Errorf("Generated feed missing entry title")
	}

	if !strings.Contains(feed, `<entry>`) || !strings.Contains(feed, `</entry>`) {
		t.Errorf("Generated feed missing entry tags")
	}

	if !strings.Contains(feed, `</feed>`) {
		t.Errorf("Generated feed missing closing feed tag")
	}
}

func TestGenerateEnhancedAtomWithConfig_CustomNamespace(t *testing.T) {
	generator := NewGenerator("Test Feed", "https://test.com", "test-id", "Test Author")
	config := RedditEnhancedAtomConfig()

	items := []providers.FeedItem{
		&mockFeedItem{
			title:        "Reddit Post",
			link:         "https://reddit.com/r/test/post",
			commentsLink: "https://reddit.com/r/test/post/comments",
			author:       "reddit_user",
			score:        150,
			commentCount: 42,
			createdAt:    time.Now(),
			categories:   []string{"r/test"},
		},
	}

	feed, err := generator.GenerateEnhancedAtomWithConfig(items, config, nil)

	if err != nil {
		t.Errorf("GenerateEnhancedAtomWithConfig() error = %v", err)
		return
	}

	// Test standard namespace (no custom namespaces)
	expectedNamespace := `<feed xmlns="http://www.w3.org/2005/Atom" xmlns:media="http://search.yahoo.com/mrss/">`
	if !strings.Contains(feed, expectedNamespace) {
		t.Errorf("Generated feed missing standard namespace declaration")
	}

	// Test metadata as standard categories
	if !strings.Contains(feed, `<category term="score:150" label="Score: 150" scheme="reddit-metadata"/>`) {
		t.Errorf("Generated feed missing Reddit score metadata")
	}

	if !strings.Contains(feed, `<category term="comments:42" label="Comments: 42" scheme="reddit-metadata"/>`) {
		t.Errorf("Generated feed missing Reddit comments metadata")
	}

	if !strings.Contains(feed, `<category term="subreddit:test" label="Subreddit: r/test" scheme="reddit-metadata"/>`) {
		t.Errorf("Generated feed missing Reddit subreddit metadata")
	}
}

func TestGenerateEnhancedAtomWithConfig_WithOpenGraph(t *testing.T) {
	// Test without a real fetcher (OpenGraph integration is tested in unit tests for individual functions)
	generator := NewGenerator("Test Feed", "https://test.com", "test-id", "Test Author")
	config := DefaultEnhancedAtomConfig()
	config.OpenGraphIntegration = false // Disable to avoid nil fetcher issues

	items := []providers.FeedItem{
		&mockFeedItem{
			title:        "Article without OpenGraph",
			link:         "https://example.com",
			commentsLink: "https://example.com/comments",
			author:       "Test Author",
			score:        100,
			commentCount: 25,
			createdAt:    time.Now(),
			categories:   []string{"tech"},
		},
	}

	feed, err := generator.GenerateEnhancedAtomWithConfig(items, config, nil)

	if err != nil {
		t.Errorf("GenerateEnhancedAtomWithConfig() without OpenGraph error = %v", err)
		return
	}

	// Should still generate valid feed
	if !strings.Contains(feed, "<title>Article without OpenGraph</title>") {
		t.Errorf("Generated feed missing entry title")
	}

	// Should have enhanced content even without OpenGraph (check in content element)
	if !strings.Contains(feed, "Score: 100") {
		t.Errorf("Generated feed missing enhanced content: %s", feed)
	}
}

func TestGenerateMultipleLinks(t *testing.T) {
	generator := NewGenerator("Test", "https://test.com", "test", "Test")
	var atom strings.Builder

	item := &mockFeedItem{
		link:         "https://example.com",
		commentsLink: "https://example.com/comments",
	}

	generator.generateMultipleLinks(&atom, item, false)
	result := atom.String()

	// Should have both main link and comments link
	if !strings.Contains(result, `<link rel="alternate" type="text/html" href="https://example.com"/>`) {
		t.Errorf("generateMultipleLinks() missing main link")
	}

	if !strings.Contains(result, `<link rel="replies" type="text/html" href="https://example.com/comments" title="Comments"/>`) {
		t.Errorf("generateMultipleLinks() missing comments link")
	}
}

func TestGenerateMultipleLinks_SameLinks(t *testing.T) {
	generator := NewGenerator("Test", "https://test.com", "test", "Test")
	var atom strings.Builder

	item := &mockFeedItem{
		link:         "https://example.com",
		commentsLink: "https://example.com", // Same as main link
	}

	generator.generateMultipleLinks(&atom, item, false)
	result := atom.String()

	// Should only have main link when they're the same
	linkCount := strings.Count(result, `<link`)
	if linkCount != 1 {
		t.Errorf("generateMultipleLinks() with same links should generate 1 link, got %d", linkCount)
	}
}

func TestGenerateExtendedAuthor(t *testing.T) {
	generator := NewGenerator("Test", "https://test.com", "test", "Test")

	tests := []struct {
		name          string
		item          *mockFeedItem
		expectedURI   string
		shouldHaveURI bool
	}{
		{
			name: "Reddit author",
			item: &mockFeedItem{
				author:       "reddit_user",
				commentsLink: "https://reddit.com/r/test/comments/123",
			},
			expectedURI:   "https://www.reddit.com/user/reddit_user",
			shouldHaveURI: true,
		},
		{
			name: "Hacker News author",
			item: &mockFeedItem{
				author:       "hn_user",
				commentsLink: "https://news.ycombinator.com/item?id=123",
			},
			expectedURI:   "https://news.ycombinator.com/user?id=hn_user",
			shouldHaveURI: true,
		},
		{
			name: "Unknown platform",
			item: &mockFeedItem{
				author:       "unknown_user",
				commentsLink: "https://unknown.com/post/123",
			},
			expectedURI:   "",
			shouldHaveURI: false,
		},
		{
			name: "Empty author",
			item: &mockFeedItem{
				author:       "",
				commentsLink: "https://reddit.com/r/test/comments/123",
			},
			expectedURI:   "",
			shouldHaveURI: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var atom strings.Builder
			generator.generateExtendedAuthor(&atom, tt.item)
			result := atom.String()

			if tt.item.Author() == "" {
				if result != "" {
					t.Errorf("generateExtendedAuthor() with empty author should produce no output, got %q", result)
				}
				return
			}

			if tt.shouldHaveURI {
				if !strings.Contains(result, fmt.Sprintf("<uri>%s</uri>", tt.expectedURI)) {
					t.Errorf("generateExtendedAuthor() missing expected URI %s in %q", tt.expectedURI, result)
				}
			} else {
				if strings.Contains(result, "<uri>") {
					t.Errorf("generateExtendedAuthor() should not have URI for unknown platform")
				}
			}

			if !strings.Contains(result, fmt.Sprintf("<name>%s</name>", tt.item.Author())) {
				t.Errorf("generateExtendedAuthor() missing author name")
			}
		})
	}
}

func TestGenerateCustomMetadata_Reddit(t *testing.T) {
	generator := NewGenerator("Test", "https://test.com", "test", "Test")
	var atom strings.Builder

	item := &mockFeedItem{
		score:        150,
		commentCount: 42,
		categories:   []string{"r/golang", "tech"},
	}

	generator.generateCustomMetadata(&atom, item, "reddit")
	result := atom.String()

	// Test Reddit metadata as standard categories
	if !strings.Contains(result, `<category term="score:150" label="Score: 150" scheme="reddit-metadata"/>`) {
		t.Errorf("generateCustomMetadata() missing Reddit score")
	}

	if !strings.Contains(result, `<category term="comments:42" label="Comments: 42" scheme="reddit-metadata"/>`) {
		t.Errorf("generateCustomMetadata() missing Reddit comments")
	}

	if !strings.Contains(result, `<category term="subreddit:golang" label="Subreddit: r/golang" scheme="reddit-metadata"/>`) {
		t.Errorf("generateCustomMetadata() missing Reddit subreddit")
	}
}

func TestGenerateCustomMetadata_HackerNews(t *testing.T) {
	generator := NewGenerator("Test", "https://test.com", "test", "Test")
	var atom strings.Builder

	item := &mockFeedItem{
		score:        200,
		commentCount: 85,
		categories:   []string{"tech", "github.com"},
	}

	generator.generateCustomMetadata(&atom, item, "hn")
	result := atom.String()

	// Test Hacker News metadata as standard categories
	if !strings.Contains(result, `<category term="points:200" label="Points: 200" scheme="hackernews-metadata"/>`) {
		t.Errorf("generateCustomMetadata() missing HN points")
	}

	if !strings.Contains(result, `<category term="comments:85" label="Comments: 85" scheme="hackernews-metadata"/>`) {
		t.Errorf("generateCustomMetadata() missing HN comments")
	}

	if !strings.Contains(result, `<category term="domain:github.com" label="Domain: github.com" scheme="hackernews-metadata"/>`) {
		t.Errorf("generateCustomMetadata() missing HN domain")
	}
}

func TestGenerateEnclosures(t *testing.T) {
	generator := NewGenerator("Test", "https://test.com", "test", "Test")
	var atom strings.Builder

	item := &mockFeedItem{
		link: "https://example.com",
	}

	ogDataMap := createMockOpenGraphData()

	generator.generateEnclosures(&atom, item, ogDataMap)
	result := atom.String()

	if !strings.Contains(result, `<link rel="enclosure" type="image/jpeg" href="https://example.com/image.jpg"/>`) {
		t.Errorf("generateEnclosures() missing image enclosure")
	}
}

func TestGenerateEnclosures_NoImage(t *testing.T) {
	generator := NewGenerator("Test", "https://test.com", "test", "Test")
	var atom strings.Builder

	item := &mockFeedItem{
		link: "https://example.com",
	}

	ogDataMap := map[string]*opengraph.Data{
		"https://example.com": {
			// No image field
			Title: "Example",
		},
	}

	generator.generateEnclosures(&atom, item, ogDataMap)
	result := atom.String()

	if strings.Contains(result, `<link rel="enclosure"`) {
		t.Errorf("generateEnclosures() should not generate enclosure without image")
	}
}

func TestBuildProviderEnhancedContent(t *testing.T) {
	generator := NewGenerator("Test", "https://test.com", "test", "Test")

	item := &mockFeedItem{
		score:        100,
		commentCount: 25,
		categories:   []string{"tech", "news"},
		link:         "https://example.com",
	}

	ogDataMap := createMockOpenGraphData()

	content := generator.buildProviderEnhancedContent(item, ogDataMap)

	// Test base metadata
	if !strings.Contains(content, "<strong>Score:</strong> 100") {
		t.Errorf("buildProviderEnhancedContent() missing score")
	}

	if !strings.Contains(content, "<strong>Comments:</strong> 25") {
		t.Errorf("buildProviderEnhancedContent() missing comments")
	}

	// Test OpenGraph preview
	if !strings.Contains(content, "🔗 Link Preview") {
		t.Errorf("buildProviderEnhancedContent() missing OpenGraph preview header")
	}

	if !strings.Contains(content, "Example Article") {
		t.Errorf("buildProviderEnhancedContent() missing OpenGraph title")
	}

	if !strings.Contains(content, "This is an example article") {
		t.Errorf("buildProviderEnhancedContent() missing OpenGraph description")
	}

	if !strings.Contains(content, `<img src="https://example.com/image.jpg"`) {
		t.Errorf("buildProviderEnhancedContent() missing OpenGraph image")
	}

	if !strings.Contains(content, "Source: Example Site") {
		t.Errorf("buildProviderEnhancedContent() missing OpenGraph site name")
	}
}

func TestBuildProviderEnhancedContent_NoOpenGraph(t *testing.T) {
	generator := NewGenerator("Test", "https://test.com", "test", "Test")

	item := &mockFeedItem{
		score:        100,
		commentCount: 25,
		categories:   []string{"tech"},
		link:         "https://example.com",
	}

	content := generator.buildProviderEnhancedContent(item, nil)

	// Should still have basic metadata
	if !strings.Contains(content, "<strong>Score:</strong> 100") {
		t.Errorf("buildProviderEnhancedContent() missing score without OpenGraph")
	}

	// Should not have OpenGraph preview
	if strings.Contains(content, "🔗 Link Preview") {
		t.Errorf("buildProviderEnhancedContent() should not have OpenGraph preview without data")
	}
}
