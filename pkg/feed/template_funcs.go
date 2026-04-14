package feed

import (
	"fmt"
	"strings"
	"text/template"
	"time"
)

// TemplateFuncs returns a map of template helper functions
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"xmlEscape":   xmlEscape,
		"formatTime":  formatTime,
		"formatDate":  formatDate,
		"formatScore": formatScore,
		"joinStrings": strings.Join,
		"contains":    strings.Contains,
		"hasPrefix":   strings.HasPrefix,
		"truncate":    truncateText,
	}
}

// xmlEscape escapes XML special characters and strips invalid XML 1.0 code points.
func xmlEscape(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		if !isValidXMLRune(r) {
			continue
		}

		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&#39;")
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

func isValidXMLRune(r rune) bool {
	switch {
	case r == 0x9, r == 0xA, r == 0xD:
		return true
	case r >= 0x20 && r <= 0xD7FF:
		return true
	case r >= 0xE000 && r <= 0xFFFD:
		return true
	case r >= 0x10000 && r <= 0x10FFFF:
		return true
	default:
		return false
	}
}

// formatTime formats time in RFC3339 format for Atom feeds
func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// formatDate parses an RFC3339 timestamp and returns a human-readable date
func formatDate(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("2 January 2006")
}

// formatScore formats score and comment count for display
func formatScore(score, comments int) string {
	return fmt.Sprintf("Score: %d | Comments: %d", score, comments)
}

// truncateText truncates text to a maximum length
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
