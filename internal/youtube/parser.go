package youtube

import (
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// YouTubeFeed represents the root of a YouTube Atom feed
type YouTubeFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Xmlns   string      `xml:"xmlns,attr"`
	Title   string      `xml:"title"`
	Link    []Link      `xml:"link"`
	Author  Author      `xml:"author"`
	ID      string      `xml:"id"`
	Entries []FeedEntry `xml:"entry"`
}

// Link represents a link element in the feed
type Link struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

// Author represents the author/channel of the feed
type Author struct {
	Name string `xml:"name"`
	URI  string `xml:"uri"`
}

// FeedEntry represents a single video entry in the YouTube feed
type FeedEntry struct {
	VideoID   string    `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
	ChannelID string    `xml:"http://www.youtube.com/xml/schemas/2015 channelId"`
	Title     string    `xml:"title"`
	Link      Link      `xml:"link"`
	Author    Author    `xml:"author"`
	Published time.Time `xml:"published"`
	Updated   time.Time `xml:"updated"`

	// Media group contains additional metadata
	MediaGroup MediaGroup `xml:"http://search.yahoo.com/mrss/ group"`
}

// MediaGroup contains media-specific metadata
type MediaGroup struct {
	Title       string         `xml:"http://search.yahoo.com/mrss/ title"`
	Content     MediaContent   `xml:"http://search.yahoo.com/mrss/ content"`
	Thumbnail   MediaThumbnail `xml:"http://search.yahoo.com/mrss/ thumbnail"`
	Description string         `xml:"http://search.yahoo.com/mrss/ description"`
	Community   MediaCommunity `xml:"http://search.yahoo.com/mrss/ community"`
}

// MediaContent represents the media content element
type MediaContent struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
}

// MediaThumbnail represents the thumbnail element
type MediaThumbnail struct {
	URL    string `xml:"url,attr"`
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
}

// MediaCommunity contains community statistics
type MediaCommunity struct {
	StarRating StarRating `xml:"http://search.yahoo.com/mrss/ starRating"`
	Statistics Statistics `xml:"http://search.yahoo.com/mrss/ statistics"`
}

// StarRating represents the star rating element
type StarRating struct {
	Count   int     `xml:"count,attr"`
	Average float64 `xml:"average,attr"`
	Min     int     `xml:"min,attr"`
	Max     int     `xml:"max,attr"`
}

// Statistics represents view statistics
type Statistics struct {
	Views int `xml:"views,attr"`
}

// FetchYouTubeFeed fetches and parses a YouTube RSS feed from the given URL
func FetchYouTubeFeed(feedURL string) (*YouTubeFeed, error) {
	slog.Debug("Fetching YouTube feed", "url", feedURL)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(feedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch YouTube feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("YouTube feed returned status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse XML
	var feed YouTubeFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse YouTube feed XML: %w", err)
	}

	slog.Debug("YouTube feed parsed successfully",
		"title", feed.Title,
		"entries", len(feed.Entries),
		"author", feed.Author.Name)

	return &feed, nil
}

// ConstructFeedURL constructs a YouTube RSS feed URL from a channel ID or playlist ID
func ConstructFeedURL(feedType, id string) (string, error) {
	switch strings.ToLower(feedType) {
	case "channel":
		return fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", id), nil
	case "playlist":
		return fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?playlist_id=%s", id), nil
	default:
		return "", fmt.Errorf("invalid feed type: %s (must be 'channel' or 'playlist')", feedType)
	}
}
