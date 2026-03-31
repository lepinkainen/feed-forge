package feissarimokat

import (
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

const (
	// FeedURL is the RSS feed endpoint for feissarimokat.com
	FeedURL = "https://static.feissarimokat.com/dynamic/latest/posts.rss"

	// ImageBaseURL is used to convert relative image URLs to absolute
	ImageBaseURL = "https://static.feissarimokat.com"
)

// Compiled regexes for HTML scraping
var (
	postbodyRegex = regexp.MustCompile(`(?s)<div[^>]*class="postbody"[^>]*>(.*?)</div>`)
	imgSrcRegex   = regexp.MustCompile(`<img[^>]*src="([^"]+)"`)
)

// fetchRSSFeed fetches and parses the feissarimokat RSS feed
func fetchRSSFeed() ([]RSSItem, error) {
	slog.Debug("Fetching Feissarimokat RSS feed", "url", FeedURL)

	client := api.NewGenericClient()
	resp, err := client.Get(FeedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching RSS feed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading RSS response: %w", err)
	}

	var rss RSS
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("error parsing RSS XML: %w", err)
	}

	slog.Debug("Successfully fetched Feissarimokat items", "count", len(rss.Channel.Items))
	return rss.Channel.Items, nil
}

// scrapeImages fetches a post page and extracts images from div.postbody
func scrapeImages(pageURL string) ([]string, error) {
	client := api.NewGenericClient()
	resp, err := client.Get(pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading page: %w", err)
	}

	// Find the postbody div content
	postbodyMatch := postbodyRegex.FindSubmatch(body)
	if postbodyMatch == nil {
		return nil, nil // No postbody div found, not an error
	}

	// Extract all image src attributes within the postbody
	imgMatches := imgSrcRegex.FindAllSubmatch(postbodyMatch[1], -1)
	var images []string
	for _, match := range imgMatches {
		src := string(match[1])
		// Convert relative URLs to absolute
		if !strings.HasPrefix(src, "http") {
			src = ImageBaseURL + src
		}
		images = append(images, src)
	}

	return images, nil
}

// processItems scrapes images for each RSS item and builds content HTML
func processItems(rssItems []RSSItem) []Item {
	var items []Item

	for _, rssItem := range rssItems {
		images, err := scrapeImages(rssItem.ItemLink)
		if err != nil {
			slog.Warn("Failed to scrape images", "url", rssItem.ItemLink, "error", err)
			continue
		}

		// Build HTML content with description and embedded images
		var html strings.Builder
		html.WriteString(rssItem.Description)
		for _, img := range images {
			fmt.Fprintf(&html, "\n<img src=%q alt=%q>", img, rssItem.ItemTitle)
		}

		items = append(items, Item{
			RSSItem:     rssItem,
			Images:      images,
			ContentHTML: html.String(),
		})
	}

	return items
}
