package youtube

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

// DiscoverFeedURL fetches a YouTube channel page and returns its advertised RSS feed URL.
func DiscoverFeedURL(channelPageURL string) (string, error) {
	client := api.NewGenericClient()
	resp, err := client.Get(channelPageURL, map[string]string{
		"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	})
	if err != nil {
		return "", fmt.Errorf("fetch youtube channel page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	feedURL, err := discoverFeedURLFromHTML(resp.Body)
	if err != nil {
		return "", err
	}
	return feedURL, nil
}

func discoverFeedURLFromHTML(r io.Reader) (string, error) {
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if errors.Is(z.Err(), io.EOF) {
				return "", fmt.Errorf("youtube RSS feed link not found")
			}
			return "", fmt.Errorf("parse youtube channel page: %w", z.Err())
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if !strings.EqualFold(t.Data, "link") {
				continue
			}

			attrs := make(map[string]string, len(t.Attr))
			for _, attr := range t.Attr {
				attrs[strings.ToLower(attr.Key)] = attr.Val
			}
			if attrs["href"] == "" {
				continue
			}
			if !strings.EqualFold(attrs["type"], "application/rss+xml") {
				continue
			}
			if !hasRel(attrs["rel"], "alternate") {
				continue
			}
			return attrs["href"], nil
		}
	}
}

func hasRel(rel, want string) bool {
	for _, part := range strings.Fields(rel) {
		if strings.EqualFold(part, want) {
			return true
		}
	}
	return false
}
