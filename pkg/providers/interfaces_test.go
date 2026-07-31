package providers

import (
	"errors"
	"testing"
	"time"
)

// Mock implementations for testing
type mockFeedProvider struct {
	generateFeedFunc func(outfile string) error
	fetchItemsFunc   func(limit int) ([]FeedItem, error)
}

func (m *mockFeedProvider) GenerateFeed(outfile string) error {
	if m.generateFeedFunc != nil {
		return m.generateFeedFunc(outfile)
	}
	return nil
}

func (m *mockFeedProvider) FetchItems(limit int) ([]FeedItem, error) {
	if m.fetchItemsFunc != nil {
		return m.fetchItemsFunc(limit)
	}
	return []FeedItem{}, nil
}

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

func TestFeedItemInterface(t *testing.T) {
	now := time.Now()
	item := &mockFeedItem{
		title:        "Test Title",
		link:         "https://example.com",
		commentsLink: "https://example.com/comments",
		author:       "Test Author",
		score:        100,
		commentCount: 25,
		createdAt:    now,
		categories:   []string{"tech", "news"},
	}

	// Test all interface methods
	if item.Title() != "Test Title" {
		t.Errorf("Title() = %v, want %v", item.Title(), "Test Title")
	}

	if item.Link() != "https://example.com" {
		t.Errorf("Link() = %v, want %v", item.Link(), "https://example.com")
	}

	if item.CommentsLink() != "https://example.com/comments" {
		t.Errorf("CommentsLink() = %v, want %v", item.CommentsLink(), "https://example.com/comments")
	}

	if item.Author() != "Test Author" {
		t.Errorf("Author() = %v, want %v", item.Author(), "Test Author")
	}

	if item.Score() != 100 {
		t.Errorf("Score() = %v, want %v", item.Score(), 100)
	}

	if item.CommentCount() != 25 {
		t.Errorf("CommentCount() = %v, want %v", item.CommentCount(), 25)
	}

	if !item.CreatedAt().Equal(now) {
		t.Errorf("CreatedAt() = %v, want %v", item.CreatedAt(), now)
	}

	categories := item.Categories()
	if len(categories) != 2 || categories[0] != "tech" || categories[1] != "news" {
		t.Errorf("Categories() = %v, want %v", categories, []string{"tech", "news"})
	}
}

func TestFeedProviderInterface(t *testing.T) {
	provider := &mockFeedProvider{
		generateFeedFunc: func(outfile string) error {
			if outfile == "" {
				return errors.New("outfile cannot be empty")
			}
			return nil
		},
	}

	// Test successful call
	err := provider.GenerateFeed("output.xml")
	if err != nil {
		t.Errorf("GenerateFeed() with valid params should not error, got %v", err)
	}

	// Test error case
	err = provider.GenerateFeed("")
	if err == nil {
		t.Errorf("GenerateFeed() with empty outfile should error")
	}
}
