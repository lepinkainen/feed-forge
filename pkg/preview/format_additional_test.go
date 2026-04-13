package preview

import (
	"strings"
	"testing"
	"time"
)

func TestFormatDetailedItem_OmitsEmptyOptionalFields(t *testing.T) {
	item := mockFeedItem{
		title:     "Only Title",
		link:      "https://example.com/post",
		score:     1,
		comments:  2,
		createdAt: time.Time{},
	}

	got := FormatDetailedItem(item)
	for _, unwanted := range []string{"Comments: https://", "Author:", "Posted:", "Categories:", "Image:", "Content:"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("FormatDetailedItem() unexpectedly contained %q:\n%s", unwanted, got)
		}
	}
}

func TestFormatDetailedItem_TruncatesLongContent(t *testing.T) {
	item := mockFeedItem{
		title:    "Long content",
		link:     "https://example.com/post",
		content:  strings.Repeat("word ", 260),
		score:    10,
		comments: 5,
	}

	got := FormatDetailedItem(item)
	if !strings.Contains(got, "Content:\n") {
		t.Fatalf("FormatDetailedItem() missing content section:\n%s", got)
	}
	if !strings.Contains(got, "...") {
		t.Fatalf("FormatDetailedItem() should truncate long content:\n%s", got)
	}
}

func TestWrapXMLContent_LeavesShortLinesUnchanged(t *testing.T) {
	input := "<entry>short</entry>"
	got := wrapXMLContent(input, 80)
	if got != input+"\n" {
		t.Fatalf("wrapXMLContent() = %q, want %q", got, input+"\n")
	}
}

func TestFormatTimeAgo_EdgeCases(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{name: "plural minutes", t: now.Add(-5 * time.Minute), want: "5 minutes ago"},
		{name: "one hour", t: now.Add(-1 * time.Hour), want: "1 hour ago"},
		{name: "one day", t: now.Add(-24 * time.Hour), want: "1 day ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTimeAgo(tt.t); got != tt.want {
				t.Fatalf("formatTimeAgo() = %q, want %q", got, tt.want)
			}
		})
	}
}
