package hackernews

import (
	"reflect"
	"testing"
	"time"
)

func TestCategorizeContent(t *testing.T) {
	// Create a simple test category mapper
	config := &DomainConfig{
		CategoryDomains: map[string][]string{
			"Development": {"github.com"},
			"Video":       {"youtube.com"},
			"Research":    {"arxiv.org"},
			"News":        {"techcrunch.com"},
		},
	}
	mapper := NewCategoryMapper(config)

	tests := []struct {
		name     string
		title    string
		domain   string
		url      string
		mapper   *CategoryMapper
		expected []string
	}{
		{
			name:     "Show HN post",
			title:    "Show HN: My awesome project",
			domain:   "example.com",
			url:      "https://example.com/project",
			mapper:   mapper,
			expected: []string{"example.com", "Show HN"},
		},
		{
			name:     "Ask HN post",
			title:    "Ask HN: How do you handle burnout?",
			domain:   "news.ycombinator.com",
			url:      "https://news.ycombinator.com/item?id=123",
			mapper:   mapper,
			expected: []string{"news.ycombinator.com", "Ask HN"},
		},
		{
			name:     "GitHub project",
			title:    "New open source tool for developers",
			domain:   "github.com",
			url:      "https://github.com/user/repo",
			mapper:   mapper,
			expected: []string{"github.com", "Development"},
		},
		{
			name:     "PDF document",
			title:    "Research paper on AI (PDF)",
			domain:   "example.com",
			url:      "https://example.com/paper.pdf",
			mapper:   mapper,
			expected: []string{"example.com", "PDF"},
		},
		{
			name:     "Video content",
			title:    "Introduction to Machine Learning video",
			domain:   "youtube.com",
			url:      "https://youtube.com/watch?v=123",
			mapper:   mapper,
			expected: []string{"youtube.com", "Video", "Video"},
		},
		{
			name:     "Book mention",
			title:    "This book changed my perspective on programming",
			domain:   "example.com",
			url:      "https://example.com/book-review",
			mapper:   mapper,
			expected: []string{"example.com", "Book"},
		},
		{
			name:     "Ebook mention",
			title:    "Free ebook on data structures",
			domain:   "example.com",
			url:      "https://example.com/ebook",
			mapper:   mapper,
			expected: []string{"example.com", "Book"},
		},
		{
			name:     "No special categorization",
			title:    "Regular news article",
			domain:   "example.com",
			url:      "https://example.com/news",
			mapper:   mapper,
			expected: []string{"example.com"},
		},
		{
			name:     "Empty domain",
			title:    "Some title",
			domain:   "",
			url:      "https://example.com",
			mapper:   mapper,
			expected: []string{},
		},
		{
			name:     "Nil mapper",
			title:    "Some title",
			domain:   "github.com",
			url:      "https://github.com/user/repo",
			mapper:   nil,
			expected: []string{"github.com"},
		},
		{
			name:     "Case insensitive Show HN",
			title:    "SHOW HN: My Project",
			domain:   "example.com",
			url:      "https://example.com",
			mapper:   mapper,
			expected: []string{"example.com", "Show HN"},
		},
		{
			name:     "Case insensitive Ask HN",
			title:    "ASK HN: Question here",
			domain:   "example.com",
			url:      "https://example.com",
			mapper:   mapper,
			expected: []string{"example.com", "Ask HN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categorizeContent(tt.title, tt.domain, tt.url, tt.mapper)

			// Handle nil vs empty slice comparison
			if len(result) == 0 && len(tt.expected) == 0 {
				return // Both are effectively empty
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("categorizeContent(%q, %q, %q, mapper) = %v, expected %v",
					tt.title, tt.domain, tt.url, result, tt.expected)
			}
		})
	}
}

func TestCategorizeByPoints(t *testing.T) {
	tests := []struct {
		name      string
		points    int
		minPoints int
		expected  string
	}{
		{
			name:      "viral content 500+",
			points:    750,
			minPoints: 50,
			expected:  "Viral 500+",
		},
		{
			name:      "hot content 200+",
			points:    350,
			minPoints: 50,
			expected:  "Hot 200+",
		},
		{
			name:      "high score 100+",
			points:    150,
			minPoints: 50,
			expected:  "High Score 100+",
		},
		{
			name:      "high score double min",
			points:    120,
			minPoints: 50,
			expected:  "High Score 100+",
		},
		{
			name:      "popular at min threshold",
			points:    50,
			minPoints: 50,
			expected:  "Popular 50+",
		},
		{
			name:      "popular above min threshold",
			points:    75,
			minPoints: 50,
			expected:  "Popular 50+",
		},
		{
			name:      "rising below min threshold",
			points:    25,
			minPoints: 50,
			expected:  "Rising",
		},
		{
			name:      "zero points",
			points:    0,
			minPoints: 10,
			expected:  "Rising",
		},
		{
			name:      "negative points",
			points:    -5,
			minPoints: 10,
			expected:  "Rising",
		},
		{
			name:      "exactly at double min threshold",
			points:    100,
			minPoints: 50,
			expected:  "High Score 100+",
		},
		{
			name:      "edge case: min threshold 1",
			points:    3,
			minPoints: 1,
			expected:  "High Score 2+",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categorizeByPoints(tt.points, tt.minPoints)
			if result != tt.expected {
				t.Errorf("categorizeByPoints(%d, %d) = %q, expected %q",
					tt.points, tt.minPoints, result, tt.expected)
			}
		})
	}
}

func TestCalculatePostAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		createdAt time.Time
		expected  string
	}{
		{
			name:      "just now",
			createdAt: now.Add(-30 * time.Second),
			expected:  "just now",
		},
		{
			name:      "5 minutes ago",
			createdAt: now.Add(-5 * time.Minute),
			expected:  "5 minutes ago",
		},
		{
			name:      "1 hour ago",
			createdAt: now.Add(-1 * time.Hour),
			expected:  "1 hours ago",
		},
		{
			name:      "3 hours ago",
			createdAt: now.Add(-3 * time.Hour),
			expected:  "3 hours ago",
		},
		{
			name:      "1 day ago",
			createdAt: now.Add(-24 * time.Hour),
			expected:  "1 days ago",
		},
		{
			name:      "3 days ago",
			createdAt: now.Add(-3 * 24 * time.Hour),
			expected:  "3 days ago",
		},
		{
			name:      "1 week ago",
			createdAt: now.Add(-7 * 24 * time.Hour),
			expected:  "1 weeks ago",
		},
		{
			name:      "2 weeks ago",
			createdAt: now.Add(-14 * 24 * time.Hour),
			expected:  "2 weeks ago",
		},
		{
			name:      "1 month ago",
			createdAt: now.Add(-30 * 24 * time.Hour),
			expected:  "4 weeks ago",
		},
		{
			name:      "exactly 1 minute",
			createdAt: now.Add(-1 * time.Minute),
			expected:  "1 minutes ago",
		},
		{
			name:      "exactly 1 hour",
			createdAt: now.Add(-1 * time.Hour),
			expected:  "1 hours ago",
		},
		{
			name:      "exactly 1 day",
			createdAt: now.Add(-24 * time.Hour),
			expected:  "1 days ago",
		},
		{
			name:      "exactly 1 week",
			createdAt: now.Add(-7 * 24 * time.Hour),
			expected:  "1 weeks ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePostAge(tt.createdAt)
			if result != tt.expected {
				t.Errorf("calculatePostAge(%v) = %q, expected %q",
					tt.createdAt, result, tt.expected)
			}
		})
	}
}
