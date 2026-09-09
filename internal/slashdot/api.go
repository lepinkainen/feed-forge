package slashdot

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/lepinkainen/feed-forge/pkg/atom"

	"github.com/lepinkainen/feed-forge/pkg/api"
	"github.com/lepinkainen/feed-forge/pkg/httpcache"
)

// Slashdot appends sharing buttons and a redundant story link to summaries.
// The match tolerates other attributes on the <div> and the optional <p>.
var footerRegex = regexp.MustCompile(`(?s)\s*(?:<p[^>]*>\s*)?<div[^>]*\bclass="share_submission".*$`)

var (
	// Slashdot strips the story body to plain text before it reaches the feed;
	// paragraph breaks survive only as blank lines.
	paragraphBreakRegex = regexp.MustCompile(`\n[ \t\r]*\n\s*`)
	blockTagRegex       = regexp.MustCompile(`(?i)<(p|br|div|ul|ol|blockquote|pre|h[1-6])\b`)
)

// paragraphs wraps a tag-stripped body in <p> elements at its blank lines so
// readers keep the paragraph structure. Bodies that already carry block-level
// markup pass through unchanged.
func paragraphs(body string) string {
	if body == "" || blockTagRegex.MatchString(body) {
		return body
	}
	parts := paragraphBreakRegex.Split(body, -1)
	var sb strings.Builder
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		sb.WriteString("<p>")
		sb.WriteString(part)
		sb.WriteString("</p>")
	}
	return sb.String()
}

func fetchAtomFeed(feedURL string, store *httpcache.Store) ([]atomEntry, error) {
	client := api.NewGenericClient()
	// Cache the body as well as validators: preview needs items on a 304, and
	// generation may need to create a missing output file from that same body.
	// Upstream failures still surface; we do not opt into serving stale data.
	body, _, err := httpcache.CachedGetWithStale(context.Background(), client, store, feedURL, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("fetch slashdot feed: %w", err)
	}
	feed, err := atom.Decode[atomFeed](body)
	if err != nil {
		return nil, fmt.Errorf("parse slashdot atom: %w", err)
	}
	entries := make([]atomEntry, 0, len(feed.Entries))
	for _, raw := range feed.Entries {
		entry, ok := parseEntry(raw)
		if !ok {
			slog.Warn("Skipping Slashdot entry with invalid timestamp", "id", raw.ID)
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// parseEntry derives the item fields from the decoded entry. It reports false
// when the entry has no usable timestamp, because the feed cannot order it.
func parseEntry(entry atomEntry) (atomEntry, bool) {
	if entry.Updated.IsZero() {
		return atomEntry{}, false
	}
	var err error
	entry.comments, err = parseCommentCount(entry.Comments)
	if err != nil {
		slog.Warn("Invalid Slashdot comment count; using zero", "id", entry.ID, "comments", entry.Comments, "error", err)
	}
	// Atom titles are plain text. encoding/xml has already decoded XML entities.
	entry.Title = strings.TrimSpace(entry.Title)
	entry.content = paragraphs(strings.TrimSpace(footerRegex.ReplaceAllString(entry.Summary.HTML(), "")))
	href := atom.AlternateHref(entry.Links)
	if href == "" {
		href = entry.ID
	}
	entry.link = cleanStoryURL(href)
	return entry, true
}

var groupedCountRegex = regexp.MustCompile(`^\d{1,3}(,\d{3})+$`)

func parseCommentCount(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	if groupedCountRegex.MatchString(value) {
		value = strings.ReplaceAll(value, ",", "")
	}
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse comment count: %w", err)
	}
	if count < 0 {
		return 0, fmt.Errorf("negative comment count %d", count)
	}
	return count, nil
}

func cleanStoryURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := u.Query()
	for key := range query {
		if strings.HasPrefix(key, "utm_") {
			query.Del(key)
		}
	}
	u.RawQuery = query.Encode()
	return u.String()
}
