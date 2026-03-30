package feed

import (
	"fmt"
	"html"
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

// xmlEscape escapes XML special characters while avoiding double-encoding
func xmlEscape(s string) string {
	// First unescape any existing HTML entities to avoid double-encoding
	s = html.UnescapeString(s)
	// Then apply proper HTML escaping
	return html.EscapeString(s)
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
	return sprintf("Score: %d | Comments: %d", score, comments)
}

// truncateText truncates text to a maximum length
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// sprintf is a helper for template string formatting
func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
