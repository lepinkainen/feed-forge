package xkcd

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
	"github.com/lepinkainen/feed-forge/pkg/httpcache"
)

// FeedURL is the xkcd RSS endpoint. It is a var so tests can point it at an
// httptest server.
var FeedURL = "https://xkcd.com/rss.xml"

// The comic image lives in an HTML-encoded <img> inside the RSS <description>.
// The title attribute carries the essential mouseover caption; alt duplicates it.
var (
	imgSrcRegex   = regexp.MustCompile(`<img[^>]*\bsrc="([^"]+)"`)
	imgTitleRegex = regexp.MustCompile(`<img[^>]*\btitle="([^"]*)"`)
	imgAltRegex   = regexp.MustCompile(`<img[^>]*\balt="([^"]*)"`)
)

// fetchItems fetches and parses the xkcd RSS feed into Items. A nil store skips
// conditional-GET caching.
func fetchItems(store *httpcache.Store) ([]Item, error) {
	slog.Debug("Fetching xkcd RSS feed", "url", FeedURL)

	client := api.NewGenericClient()
	body, err := httpcache.CachedGet(context.Background(), client, store, FeedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch xkcd feed: %w", err)
	}

	var rss RSS
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("parse xkcd rss: %w", err)
	}

	items := make([]Item, 0, len(rss.Channel.Items))
	for _, raw := range rss.Channel.Items {
		items = append(items, buildItem(raw))
	}

	slog.Debug("Fetched xkcd items", "count", len(items))
	return items, nil
}

// buildItem parses one RSS item, pulling the image URL and caption out of the
// description and rendering content with the caption as visible text.
func buildItem(raw RSSItem) Item {
	imageURL := firstSubmatch(imgSrcRegex, raw.Description)
	caption := firstSubmatch(imgTitleRegex, raw.Description)
	if caption == "" {
		caption = firstSubmatch(imgAltRegex, raw.Description)
	}

	createdAt, err := parsePubDate(raw.PubDate)
	if err != nil {
		slog.Warn("Unparseable xkcd pubDate; using zero time", "link", raw.Link, "pubDate", raw.PubDate, "error", err)
	}

	return Item{
		RSSItem:   raw,
		imageURL:  imageURL,
		caption:   caption,
		content:   renderContent(imageURL, caption, raw.ItemTitle),
		createdAt: createdAt,
	}
}

// renderContent builds the entry body: the comic image followed by the caption
// as a visible paragraph, since xkcd hides it in the img title attribute.
func renderContent(imageURL, caption, title string) string {
	if imageURL == "" {
		return html.EscapeString(caption)
	}
	img := `<img src="` + html.EscapeString(imageURL) + `" alt="` + html.EscapeString(title) + `" />`
	if caption == "" {
		return img
	}
	return img + "\n<p class=\"comic-caption\">" + html.EscapeString(caption) + "</p>"
}

// parsePubDate parses xkcd's RFC1123Z pubDate, e.g. "Fri, 14 Aug 2026 04:00:00 -0000".
func parsePubDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty pubDate")
	}
	if t, err := time.Parse(time.RFC1123Z, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC1123, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognized pubDate format: %q", s)
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
