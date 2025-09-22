package redditjson

import (
	"testing"
)

func TestCleanRedditHTML(t *testing.T) {

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
			result := cleanRedditHTML(tt.input)
			if result != tt.expected {
				t.Errorf("cleanRedditHTML() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestBuildEnhancedContentNoDoubleEscaping was removed as buildEnhancedContent
// is no longer used after removing the enhanced generation system
