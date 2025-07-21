package redditjson

import (
	"testing"
)

func TestCleanRedditHTML(t *testing.T) {
	fg := &FeedGenerator{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove SC_OFF and SC_ON comments",
			input:    `<!-- SC_OFF --><div class="md"><p>Test content</p></div><!-- SC_ON -->`,
			expected: `<div class="md"><p>Test content</p></div>`,
		},
		{
			name:     "Handle content without comments",
			input:    `<div class="md"><p>Clean content</p></div>`,
			expected: `<div class="md"><p>Clean content</p></div>`,
		},
		{
			name:     "Handle empty content",
			input:    ``,
			expected: ``,
		},
		{
			name:     "Handle only comments",
			input:    `<!-- SC_OFF --><!-- SC_ON -->`,
			expected: ``,
		},
		{
			name:     "Handle content with whitespace around comments",
			input:    `   <!-- SC_OFF -->   <p>Content</p>   <!-- SC_ON -->   `,
			expected: `<p>Content</p>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fg.cleanRedditHTML(tt.input)
			if result != tt.expected {
				t.Errorf("cleanRedditHTML() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildEnhancedContentNoDoubleEscaping(t *testing.T) {
	fg := &FeedGenerator{}

	post := RedditPost{
		Data: struct {
			Title        string       `json:"title"`
			URL          string       `json:"url"`
			Permalink    string       `json:"permalink"`
			CreatedUTC   float64      `json:"created_utc"`
			Score        int          `json:"score"`
			NumComments  int          `json:"num_comments"`
			Author       string       `json:"author"`
			Subreddit    string       `json:"subreddit"`
			SelfText     string       `json:"selftext"`
			SelfTextHTML string       `json:"selftext_html"`
			Thumbnail    string       `json:"thumbnail"`
			Preview      *PreviewData `json:"preview,omitempty"`
		}{
			Title:        "Test Post",
			Author:       "testuser",
			Subreddit:    "testsubreddit",
			URL:          "https://example.com",
			Permalink:    "/r/testsubreddit/comments/123/test_post/",
			Score:        100,
			NumComments:  50,
			SelfTextHTML: `<!-- SC_OFF --><div class="md"><p>Test with &quot;quotes&quot; and &amp; entities.</p></div><!-- SC_ON -->`,
		},
	}

	content := fg.buildEnhancedContent(post, nil)

	// Verify that Reddit comments are removed
	if contains := containsString(content, "<!-- SC_OFF -->"); contains {
		t.Errorf("buildEnhancedContent() should not contain <!-- SC_OFF -->")
	}
	if contains := containsString(content, "<!-- SC_ON -->"); contains {
		t.Errorf("buildEnhancedContent() should not contain <!-- SC_ON -->")
	}

	// Verify that HTML entities are properly decoded (since we're using CDATA now)
	if contains := containsString(content, `"quotes"`); !contains {
		t.Errorf("buildEnhancedContent() should decode &quot; entities to actual quotes")
	}
	if contains := containsString(content, "& entities"); !contains {
		t.Errorf("buildEnhancedContent() should decode &amp; entities to actual ampersands")
	}
	if contains := containsString(content, "&amp;quot;"); contains {
		t.Errorf("buildEnhancedContent() should not double-escape to &amp;quot;")
	}

	// Verify submission metadata is properly escaped
	if contains := containsString(content, "/u/testuser"); !contains {
		t.Errorf("buildEnhancedContent() should contain author link")
	}
	if contains := containsString(content, "/r/testsubreddit"); !contains {
		t.Errorf("buildEnhancedContent() should contain subreddit link")
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findString(s, substr) >= 0
}

func findString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
