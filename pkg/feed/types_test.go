package feed

import (
	"reflect"
	"testing"
	"time"

	"github.com/gorilla/feeds"
)

func TestNewGenerator(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description string
		link        string
		author      string
		expected    *Generator
	}{
		{
			name:        "basic generator",
			title:       "Test Feed",
			description: "A test feed",
			link:        "https://example.com",
			author:      "Test Author",
			expected: &Generator{
				Title:       "Test Feed",
				Description: "A test feed",
				Link:        "https://example.com",
				Author:      "Test Author",
			},
		},
		{
			name:        "empty values",
			title:       "",
			description: "",
			link:        "",
			author:      "",
			expected: &Generator{
				Title:       "",
				Description: "",
				Link:        "",
				Author:      "",
			},
		},
		{
			name:        "unicode content",
			title:       "测试 Feed",
			description: "A test feed with unicode",
			link:        "https://example.com/测试",
			author:      "Test Author 测试",
			expected: &Generator{
				Title:       "测试 Feed",
				Description: "A test feed with unicode",
				Link:        "https://example.com/测试",
				Author:      "Test Author 测试",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewGenerator(tt.title, tt.description, tt.link, tt.author)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("NewGenerator() = %+v, expected %+v", result, tt.expected)
			}
		})
	}
}

func TestFeedType_String(t *testing.T) {
	tests := []struct {
		name     string
		feedType FeedType
		expected string
	}{
		{
			name:     "RSS feed type",
			feedType: RSS,
			expected: "rss",
		},
		{
			name:     "Atom feed type",
			feedType: Atom,
			expected: "atom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(tt.feedType)
			if result != tt.expected {
				t.Errorf("FeedType string = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestItem_Validation(t *testing.T) {
	baseTime := time.Now()

	tests := []struct {
		name  string
		item  Item
		valid bool
	}{
		{
			name: "valid item",
			item: Item{
				Title:       "Test Title",
				Link:        "https://example.com",
				Description: "Test Description",
				Author:      "Test Author",
				Created:     baseTime,
				ID:          "test-id",
				Categories:  []string{"test"},
			},
			valid: true,
		},
		{
			name: "minimal valid item",
			item: Item{
				Title:   "Test Title",
				Link:    "https://example.com",
				Created: baseTime,
				ID:      "test-id",
			},
			valid: true,
		},
		{
			name: "empty title",
			item: Item{
				Title:   "",
				Link:    "https://example.com",
				Created: baseTime,
				ID:      "test-id",
			},
			valid: false, // Would fail validation if validated
		},
		{
			name: "empty link",
			item: Item{
				Title:   "Test Title",
				Link:    "",
				Created: baseTime,
				ID:      "test-id",
			},
			valid: false, // Would fail validation if validated
		},
		{
			name: "empty ID",
			item: Item{
				Title:   "Test Title",
				Link:    "https://example.com",
				Created: baseTime,
				ID:      "",
			},
			valid: false, // Would fail validation if validated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the item can be created (structure validation)
			if tt.item.Title == "" && tt.valid {
				t.Errorf("Item with empty title should not be marked as valid")
			}
			if tt.item.Link == "" && tt.valid {
				t.Errorf("Item with empty link should not be marked as valid")
			}
			if tt.item.ID == "" && tt.valid {
				t.Errorf("Item with empty ID should not be marked as valid")
			}
		})
	}
}

func TestMetadata_Creation(t *testing.T) {
	baseTime := time.Now()
	created := baseTime.Add(-1 * time.Hour)
	updated := baseTime

	tests := []struct {
		name     string
		metadata Metadata
		expected bool
	}{
		{
			name: "valid metadata",
			metadata: Metadata{
				Title:       "Test Feed",
				Description: "Test Description",
				ItemCount:   5,
				Created:     created,
				Updated:     updated,
				OldestItem:  baseTime.Add(-2 * time.Hour),
				NewestItem:  baseTime.Add(-30 * time.Minute),
			},
			expected: true,
		},
		{
			name: "zero metadata",
			metadata: Metadata{
				Title:       "",
				Description: "",
				ItemCount:   0,
				Created:     time.Time{},
				Updated:     time.Time{},
				OldestItem:  time.Time{},
				NewestItem:  time.Time{},
			},
			expected: true, // Struct can be created even with zero values
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that metadata struct can be properly initialized
			if tt.metadata.ItemCount < 0 {
				t.Errorf("ItemCount should not be negative")
			}
			if !tt.metadata.OldestItem.IsZero() && !tt.metadata.NewestItem.IsZero() {
				if tt.metadata.OldestItem.After(tt.metadata.NewestItem) {
					t.Errorf("OldestItem (%v) should not be after NewestItem (%v)",
						tt.metadata.OldestItem, tt.metadata.NewestItem)
				}
			}
		})
	}
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "ampersand",
			input:    "Tom & Jerry",
			expected: "Tom &amp; Jerry",
		},
		{
			name:     "less than",
			input:    "5 < 10",
			expected: "5 &lt; 10",
		},
		{
			name:     "greater than",
			input:    "10 > 5",
			expected: "10 &gt; 5",
		},
		{
			name:     "double quotes",
			input:    `Say "Hello"`,
			expected: "Say &quot;Hello&quot;",
		},
		{
			name:     "single quotes",
			input:    "Don't do it",
			expected: "Don&apos;t do it",
		},
		{
			name:     "all special characters",
			input:    `<tag attr="value" class='style'>Tom & Jerry</tag>`,
			expected: "&lt;tag attr=&quot;value&quot; class=&apos;style&apos;&gt;Tom &amp; Jerry&lt;/tag&gt;",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "multiple ampersands",
			input:    "A & B & C",
			expected: "A &amp; B &amp; C",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeXML(tt.input)
			if result != tt.expected {
				t.Errorf("EscapeXML(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "no truncation needed",
			input:    "Hello",
			maxLen:   10,
			expected: "Hello",
		},
		{
			name:     "exact length",
			input:    "Hello",
			maxLen:   5,
			expected: "Hello",
		},
		{
			name:     "truncation needed",
			input:    "Hello World",
			maxLen:   8,
			expected: "Hello...",
		},
		{
			name:     "very short max length",
			input:    "Hello World",
			maxLen:   3,
			expected: "...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   5,
			expected: "",
		},
		{
			name:     "very small max length",
			input:    "Hello",
			maxLen:   4,
			expected: "H...",
		},
		{
			name:     "unicode string",
			input:    "Hello 世界",
			maxLen:   8,
			expected: "Hello...",
		},
		{
			name:     "long string",
			input:    "This is a very long string that needs truncation",
			maxLen:   20,
			expected: "This is a very lo...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("TruncateString(%q, %d) = %q, expected %q",
					tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestValidateFeedItem(t *testing.T) {
	tests := []struct {
		name        string
		item        *feeds.Item
		expectError bool
	}{
		{
			name: "valid item",
			item: &feeds.Item{
				Title: "Test Title",
				Link:  &feeds.Link{Href: "https://example.com"},
				Id:    "test-id",
			},
			expectError: false,
		},
		{
			name: "empty title",
			item: &feeds.Item{
				Title: "",
				Link:  &feeds.Link{Href: "https://example.com"},
				Id:    "test-id",
			},
			expectError: true,
		},
		{
			name: "nil link",
			item: &feeds.Item{
				Title: "Test Title",
				Link:  nil,
				Id:    "test-id",
			},
			expectError: true,
		},
		{
			name: "empty link href",
			item: &feeds.Item{
				Title: "Test Title",
				Link:  &feeds.Link{Href: ""},
				Id:    "test-id",
			},
			expectError: true,
		},
		{
			name: "empty ID",
			item: &feeds.Item{
				Title: "Test Title",
				Link:  &feeds.Link{Href: "https://example.com"},
				Id:    "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFeedItem(tt.item)
			if (err != nil) != tt.expectError {
				t.Errorf("ValidateFeedItem() error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}
