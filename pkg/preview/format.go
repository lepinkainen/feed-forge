// Package preview provides interactive feed item preview functionality using Bubble Tea TUI.
package preview

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// wrapText wraps text to the specified width, breaking at word boundaries when possible
func wrapText(text string, width int) string {
	if width <= 0 {
		width = 70
	}

	var result strings.Builder
	var line strings.Builder
	lineLen := 0

	words := strings.Fields(text)
	for i, word := range words {
		wordLen := len(word)

		// If adding this word would exceed width, start a new line
		if lineLen > 0 && lineLen+1+wordLen > width {
			result.WriteString(line.String())
			result.WriteString("\n")
			line.Reset()
			lineLen = 0
		}

		// Add space before word if not at start of line
		if lineLen > 0 {
			line.WriteString(" ")
			lineLen++
		}

		line.WriteString(word)
		lineLen += wordLen

		// Write the last line
		if i == len(words)-1 {
			result.WriteString(line.String())
		}
	}

	return result.String()
}

// FormatCompactListItem formats a single feed item in compact list format
// Example: "1. [1234↑ 56💬] 2025-10-21T13:33:58+03:00 - Post Title"
func FormatCompactListItem(index int, item providers.FeedItem) string {
	score := item.Score()
	comments := item.CommentCount()
	title := item.Title()
	dateISO := item.CreatedAt().Format(time.RFC3339)

	// Truncate title if too long (120 char width total)
	const maxTitleLength = 70
	if len(title) > maxTitleLength {
		title = title[:maxTitleLength-3] + "..."
	}

	return fmt.Sprintf("%2d. [%4d↑ %3d💬] %s  %s", index+1, score, comments, dateISO, title)
}

// FormatDetailedItem formats a single feed item with all metadata
func FormatDetailedItem(item providers.FeedItem) string {
	var b strings.Builder

	b.WriteString("═══════════════════════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("Title: %s\n", item.Title()))
	b.WriteString(fmt.Sprintf("Link: %s\n", item.Link()))

	if commentsLink := item.CommentsLink(); commentsLink != "" {
		b.WriteString(fmt.Sprintf("Comments: %s\n", commentsLink))
	}

	if author := item.Author(); author != "" {
		b.WriteString(fmt.Sprintf("Author: %s\n", author))
	}

	b.WriteString(fmt.Sprintf("Score: %d | Comments: %d\n", item.Score(), item.CommentCount()))

	if !item.CreatedAt().IsZero() {
		b.WriteString(fmt.Sprintf("Posted: %s\n", formatTimeAgo(item.CreatedAt())))
	}

	if categories := item.Categories(); len(categories) > 0 {
		b.WriteString(fmt.Sprintf("Categories: %s\n", strings.Join(categories, ", ")))
	}

	if imageURL := item.ImageURL(); imageURL != "" {
		b.WriteString(fmt.Sprintf("Image: %s\n", imageURL))
	}

	if content := item.Content(); content != "" {
		// Limit content preview
		const maxContentLength = 1000
		if len(content) > maxContentLength {
			content = content[:maxContentLength] + "..."
		}
		// Word-wrap the content for readability
		wrapped := wrapText(content, 70)
		b.WriteString(fmt.Sprintf("\nContent:\n%s\n", wrapped))
	}

	b.WriteString("═══════════════════════════════════════════════════════════════════════\n")

	return b.String()
}

// FormatXMLItem formats a single feed item as an Atom XML entry using the actual feed template
func FormatXMLItem(item providers.FeedItem, templateName string, config feed.Config) string {
	// Generate a full feed with just this one item using the real template
	items := []providers.FeedItem{item}

	feedXML, err := feed.GenerateAtomFeedWithEmbeddedTemplate(items, templateName, config, nil)
	if err != nil {
		return fmt.Sprintf("Error generating feed: %s", err)
	}

	// Extract the <entry>...</entry> section using regex
	entryRegex := regexp.MustCompile(`(?s)<entry>.*?</entry>`)
	match := entryRegex.FindString(feedXML)
	if match == "" {
		return "No entry found in generated feed"
	}

	// Word-wrap long lines for readability (but keep XML structure intact)
	return wrapXMLContent(match, 80)
}

// wrapXMLContent wraps only the content inside tags, not the tags themselves
func wrapXMLContent(xml string, width int) string {
	// Simple approach: just ensure lines don't exceed width by adding newlines
	// This preserves the XML structure while making it readable
	var result strings.Builder
	lines := strings.Split(xml, "\n")

	for _, line := range lines {
		if len(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// For very long lines (usually content), try to wrap at tag boundaries or spaces
		remaining := line
		for len(remaining) > width {
			breakPoint := width
			// Try to find a good break point (space, > or <)
			for i := width; i > width-20 && i > 0; i-- {
				if remaining[i] == ' ' || remaining[i] == '>' {
					breakPoint = i + 1
					break
				}
			}
			result.WriteString(remaining[:breakPoint])
			result.WriteString("\n")
			remaining = remaining[breakPoint:]
		}
		if remaining != "" {
			result.WriteString(remaining)
			result.WriteString("\n")
		}
	}

	return result.String()
}

// formatTimeAgo formats a time.Time as a human-readable "X ago" string
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < 7*24*time.Hour:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("2006-01-02")
	}
}
