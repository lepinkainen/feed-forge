package feed

import (
	"html"
)

// EscapeXML escapes XML special characters while avoiding double-encoding of existing HTML entities
func EscapeXML(s string) string {
	// First unescape any existing HTML entities to avoid double-encoding
	s = html.UnescapeString(s)
	// Then apply proper HTML escaping
	return html.EscapeString(s)
}
