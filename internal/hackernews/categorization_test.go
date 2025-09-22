package hackernews

import (
	"path/filepath"
	"testing"

	"github.com/lepinkainen/feed-forge/pkg/testutil"
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
		name       string
		title      string
		domain     string
		url        string
		mapper     *CategoryMapper
		goldenFile string
	}{
		{
			name:       "Show HN post",
			title:      "Show HN: My awesome project",
			domain:     "example.com",
			url:        "https://example.com/project",
			mapper:     mapper,
			goldenFile: "show_hn.json",
		},
		{
			name:       "Ask HN post",
			title:      "Ask HN: How do you handle burnout?",
			domain:     "news.ycombinator.com",
			url:        "https://news.ycombinator.com/item?id=123",
			mapper:     mapper,
			goldenFile: "ask_hn.json",
		},
		{
			name:       "GitHub project",
			title:      "New open source tool for developers",
			domain:     "github.com",
			url:        "https://github.com/user/repo",
			mapper:     mapper,
			goldenFile: "github_project.json",
		},
		{
			name:       "PDF document",
			title:      "Research paper on AI (PDF)",
			domain:     "example.com",
			url:        "https://example.com/paper.pdf",
			mapper:     mapper,
			goldenFile: "pdf_document.json",
		},
		{
			name:       "Video content",
			title:      "Introduction to Machine Learning video",
			domain:     "youtube.com",
			url:        "https://youtube.com/watch?v=123",
			mapper:     mapper,
			goldenFile: "video_content.json",
		},
		{
			name:       "Book mention",
			title:      "This book changed my perspective on programming",
			domain:     "example.com",
			url:        "https://example.com/book-review",
			mapper:     mapper,
			goldenFile: "book_mention.json",
		},
		{
			name:       "Ebook mention",
			title:      "Free ebook on data structures",
			domain:     "example.com",
			url:        "https://example.com/ebook",
			mapper:     mapper,
			goldenFile: "ebook_mention.json",
		},
		{
			name:       "No special categorization",
			title:      "Regular news article",
			domain:     "example.com",
			url:        "https://example.com/news",
			mapper:     mapper,
			goldenFile: "no_special_categorization.json",
		},
		{
			name:       "Empty domain",
			title:      "Some title",
			domain:     "",
			url:        "https://example.com",
			mapper:     mapper,
			goldenFile: "empty_domain.json",
		},
		{
			name:       "Nil mapper",
			title:      "Some title",
			domain:     "github.com",
			url:        "https://github.com/user/repo",
			mapper:     nil,
			goldenFile: "nil_mapper.json",
		},
		{
			name:       "Case insensitive Show HN",
			title:      "SHOW HN: My Project",
			domain:     "example.com",
			url:        "https://example.com",
			mapper:     mapper,
			goldenFile: "case_insensitive_show_hn.json",
		},
		{
			name:       "Case insensitive Ask HN",
			title:      "ASK HN: Question here",
			domain:     "example.com",
			url:        "https://example.com",
			mapper:     mapper,
			goldenFile: "case_insensitive_ask_hn.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categorizeContent(tt.title, tt.domain, tt.url, tt.mapper)
			goldenPath := filepath.Join("testdata", "categorization", tt.goldenFile)
			testutil.CompareGoldenSlice(t, goldenPath, result)
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
