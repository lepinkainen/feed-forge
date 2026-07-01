package bulletin

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	readability "github.com/go-shiori/go-readability"
)

// minExtractedLen is the threshold below which extraction is considered a
// failure and the caller should fall back to the feed-provided summary.
const minExtractedLen = 200

// extractText runs go-readability over the fetched page HTML and returns the
// plain-text article body. Returns an error if the page yields too little text
// (gutted page, paywall, extraction miss) so callers can fall back.
func extractText(pageURL string, htmlBytes []byte) (string, error) {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return "", fmt.Errorf("parse url %q: %w", pageURL, err)
	}

	article, err := readability.FromReader(bytes.NewReader(htmlBytes), parsed)
	if err != nil {
		return "", fmt.Errorf("readability %q: %w", pageURL, err)
	}

	text := strings.TrimSpace(article.TextContent)
	if len(text) < minExtractedLen {
		return "", fmt.Errorf("extracted text too short (%d bytes) for %q", len(text), pageURL)
	}
	return text, nil
}
