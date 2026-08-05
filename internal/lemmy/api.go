package lemmy

import (
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

var (
	// submittedByRegex matches the "submitted by <user> to <community>" opening
	// of every Lemmy item description. Groups: author URL, author name,
	// community URL, community name.
	submittedByRegex = regexp.MustCompile(`^\s*submitted by <a href="([^"]*)">([^<]*)</a> to <a href="([^"]*)">([^<]*)</a>`)

	// pointsCommentsRegex matches the "N points | <a …>M comments</a>" counts
	// line. Lemmy renders it as HTML text, not as feed extension elements.
	pointsCommentsRegex = regexp.MustCompile(`(\d+)\s+points\s*\|\s*<a href="[^"]*">(\d+)\s+comments</a>`)

	// leadingAnchorRegex matches the bare link or image anchor that Lemmy puts
	// directly after the counts line for link and image posts. Group 1 is the
	// post's URL field.
	leadingAnchorRegex = regexp.MustCompile(`(?s)^\s*<a href="([^"]*)">(?:\s*<img[^>]*>\s*|[^<]*)</a>\s*`)

	// leadingBreakRegex matches the <br> separators that follow the counts line.
	leadingBreakRegex = regexp.MustCompile(`^(?:\s|<br\s*/?>)+`)

	// pubDateLayouts are tried in order against <pubDate>. Lemmy emits an
	// RFC1123Z date with an unpadded day ("Mon, 3 Aug 2026 12:54:24 +0000"),
	// which the stdlib RFC1123Z layout rejects, so the unpadded variants come
	// first.
	pubDateLayouts = []string{
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
	}
)

// buildFrontFeedURL constructs the authenticated Lemmy front page feed URL.
//
// The route is "/feeds/{type}/{name}.xml", so the JWT goes in the name
// position: /feeds/front/{jwt}.xml. The Lemmy contributor API documentation
// lists this as "/feeds/front.xml/{jwt}", which 404s — see
// crates/routes/src/feeds.rs in the Lemmy source for the real shape.
//
// See api.SanitizeURLForLog for why the resulting URL must never be logged
// verbatim.
func buildFrontFeedURL(instance, jwt, sort string) string {
	base := strings.TrimSuffix(strings.TrimSpace(instance), "/")
	feedURL := fmt.Sprintf("%s/feeds/front/%s.xml", base, url.PathEscape(jwt))
	if sort != "" {
		feedURL += "?sort=" + url.QueryEscape(sort)
	}
	return feedURL
}

// fetchFeed retrieves and parses a Lemmy RSS feed. The URL is deliberately kept
// out of the log record because it carries the account JWT.
func fetchFeed(feedURL, instance, sort string) ([]rssItem, error) {
	slog.Debug("Fetching Lemmy front page feed", "instance", instance, "sort", sort)

	client := api.NewGenericClient()
	resp, err := client.Get(feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch lemmy feed for %s: %w", instance, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read lemmy response: %w", err)
	}

	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse lemmy rss: %w", err)
	}

	slog.Debug("Parsed Lemmy feed", "instance", instance, "items", len(feed.Items))
	return feed.Items, nil
}

// parseItem maps one RSS item to an Item, scraping the score, comment count,
// author, community, external link, and body out of the description HTML.
func parseItem(raw rssItem, instanceHost string) *Item {
	item := &Item{
		item:         raw,
		instanceHost: instanceHost,
		createdAt:    parsePubDate(raw.PubDate),
	}

	body := raw.Description

	if m := submittedByRegex.FindStringSubmatchIndex(body); m != nil {
		item.authorURI = body[m[2]:m[3]]
		item.community = strings.TrimSpace(body[m[8]:m[9]])
		body = body[m[1]:]
	}
	item.author = authorName(item.authorURI, instanceHost)

	// The <category> element is the authoritative community name; the header
	// anchor is only a fallback for feeds that omit it.
	if raw.Category.Value != "" {
		item.community = strings.TrimSpace(raw.Category.Value)
	}

	if m := pointsCommentsRegex.FindStringSubmatchIndex(body); m != nil {
		item.score, _ = strconv.Atoi(body[m[2]:m[3]])
		item.commentCount, _ = strconv.Atoi(body[m[4]:m[5]])
		body = body[m[1]:]
	}

	body = leadingBreakRegex.ReplaceAllString(body, "")

	if m := leadingAnchorRegex.FindStringSubmatch(body); m != nil {
		if href := strings.TrimSpace(m[1]); href != "" && href != item.CommentsLink() {
			item.externalLink = href
		}
		body = body[len(m[0]):]
	}

	item.cleanContent = strings.TrimSpace(body)
	return item
}

// authorName derives a display name from the <dc:creator> profile URL. Remote
// accounts are qualified with their instance host, so a federated poster reads
// as "someone@piefed.world" rather than colliding with a local username.
func authorName(profileURL, instanceHost string) string {
	if profileURL == "" {
		return ""
	}

	parsed, err := url.Parse(profileURL)
	if err != nil {
		return ""
	}

	name := strings.Trim(parsed.Path, "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if name == "" {
		return ""
	}

	if parsed.Host != "" && instanceHost != "" && !strings.EqualFold(parsed.Host, instanceHost) {
		return name + "@" + parsed.Host
	}
	return name
}

// parsePubDate converts the RSS <pubDate> string into a time.Time at the feed
// boundary, so nothing downstream handles a raw upstream date string.
func parsePubDate(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}
	for _, layout := range pubDateLayouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed
		}
	}
	slog.Debug("Failed to parse Lemmy pubDate", "pub_date", trimmed)
	return time.Time{}
}

// normalizeCommunity reduces a community reference to its bare lower-case name,
// so configuration can be written as "memes", "!memes", or
// "!memes@lemmy.world" and still match the feed's category value.
func normalizeCommunity(name string) string {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	trimmed = strings.TrimPrefix(trimmed, "!")
	trimmed = strings.TrimPrefix(trimmed, "~")
	trimmed = strings.TrimPrefix(trimmed, "c/")
	if idx := strings.Index(trimmed, "@"); idx > 0 {
		trimmed = trimmed[:idx]
	}
	return trimmed
}

// normalizeCommunitySet builds a lookup set of communities to ignore. An empty
// result means every community is allowed.
func normalizeCommunitySet(names []string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		if normalized := normalizeCommunity(name); normalized != "" {
			set[normalized] = struct{}{}
		}
	}
	return set
}
