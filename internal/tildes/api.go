package tildes

import (
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

var (
	votesRegex    = regexp.MustCompile(`Votes:\s*(\d+)`)
	commentsRegex = regexp.MustCompile(`Comments:\s*(\d+)`)
	// trailingFooterRegex matches the "<hr/> + <p>Link URL/Comments URL/Votes/Comments</p>…" tail
	// that every Tildes entry's content ends with. We anchor on the first occurrence of either
	// "<p>Link URL:" or "<p>Comments URL:" and strip from there to the end.
	trailingFooterRegex = regexp.MustCompile(`(?s)\s*(?:<hr\s*/?>\s*)?<p>(?:Link URL|Comments URL):.*$`)
)

// fetchAtomFeed retrieves and parses a Tildes group Atom feed.
func fetchAtomFeed(feedURL string) ([]atomEntry, error) {
	slog.Debug("Fetching Tildes Atom feed", "url", feedURL)

	client := api.NewGenericClient()
	resp, err := client.Get(feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch tildes feed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read tildes response: %w", err)
	}

	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse tildes atom: %w", err)
	}

	slog.Debug("Parsed Tildes Atom feed", "entries", len(feed.Entries))
	return feed.Entries, nil
}

// normalizeTopic trims whitespace and an optional leading "~" so callers can
// pass either "tech" or "~tech" and get the bare topic name back.
func normalizeTopic(topic string) string {
	return strings.TrimPrefix(strings.TrimSpace(topic), "~")
}

// normalizeTopics returns unique, non-empty topic names. The singular topic is
// kept for backwards-compatible configs; topics provides multi-group support.
func normalizeTopics(topic string, topics []string) []string {
	seen := make(map[string]struct{}, len(topics)+1)
	out := make([]string, 0, len(topics)+1)
	add := func(raw string) {
		name := normalizeTopic(raw)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}

	add(topic)
	for _, candidate := range topics {
		add(candidate)
	}
	return out
}

// buildFeedURL constructs the Tildes Atom feed URL for the given bare topic.
func buildFeedURL(topic string) string {
	return fmt.Sprintf("https://tildes.net/~%s/topics.atom", topic)
}

// parseVotesAndComments extracts the "Votes: N" and "Comments: N" counts that
// Tildes embeds as plain text in each entry's HTML content footer. Missing or
// unparseable counts return 0.
func parseVotesAndComments(contentHTML string) (votes, comments int) {
	if m := votesRegex.FindStringSubmatch(contentHTML); len(m) == 2 {
		votes, _ = strconv.Atoi(m[1])
	}
	if m := commentsRegex.FindStringSubmatch(contentHTML); len(m) == 2 {
		comments, _ = strconv.Atoi(m[1])
	}
	return votes, comments
}

// cleanContent strips the "Link URL / Comments URL / Votes / Comments"
// boilerplate footer that Tildes appends to every entry's <content>, leaving
// only the actual post body. For pure link posts (whose entire body is the
// footer) the result is an empty string.
func cleanContent(contentHTML string) string {
	cleaned := trailingFooterRegex.ReplaceAllString(contentHTML, "")
	return strings.TrimSpace(cleaned)
}
