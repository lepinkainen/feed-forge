package youtube

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

const (
	dearrowAPIBase = "https://sponsor.ajay.app"
)

// DeArrowTitle represents a single title submission from DeArrow
type DeArrowTitle struct {
	Title    string `json:"title"`
	Original bool   `json:"original"`
	Votes    int    `json:"votes"`
	Locked   bool   `json:"locked"`
	UUID     string `json:"UUID"`
}

// DeArrowBrandingResponse represents the response from the DeArrow branding API
type DeArrowBrandingResponse struct {
	Titles        []DeArrowTitle `json:"titles"`
	VideoDuration float64        `json:"videoDuration"`
	RandomTime    float64        `json:"randomTime"`
}

// DeArrowClient handles communication with the DeArrow API
type DeArrowClient struct {
	client *api.Client
}

// NewDeArrowClient creates a new DeArrow API client
func NewDeArrowClient() *DeArrowClient {
	// Use generic client with minimal configuration for DeArrow API
	baseClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	client := api.NewGenericClient(baseClient)

	return &DeArrowClient{
		client: client,
	}
}

// FetchTitle fetches the DeArrow title for a given YouTube video ID
// Returns the title, whether it's trusted (locked or votes >= 0), and any error
func (c *DeArrowClient) FetchTitle(videoID string) (title string, trusted bool, err error) {
	url := fmt.Sprintf("%s/api/branding?videoID=%s", dearrowAPIBase, videoID)

	slog.Debug("Fetching DeArrow title", "videoID", videoID, "url", url)

	resp, err := c.client.Get(url)
	if err != nil {
		return "", false, fmt.Errorf("failed to fetch DeArrow data: %w", err)
	}
	defer resp.Body.Close()

	// If no data found (404), return empty title
	if resp.StatusCode == http.StatusNotFound {
		slog.Debug("No DeArrow title found for video", "videoID", videoID)
		return "", false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("DeArrow API returned status %d", resp.StatusCode)
	}

	var brandingResp DeArrowBrandingResponse
	if err := json.NewDecoder(resp.Body).Decode(&brandingResp); err != nil {
		return "", false, fmt.Errorf("failed to decode DeArrow response: %w", err)
	}

	// Check if we have any titles
	if len(brandingResp.Titles) == 0 {
		slog.Debug("No titles available in DeArrow response", "videoID", videoID)
		return "", false, nil
	}

	// Get the first title (they're ordered by quality)
	firstTitle := brandingResp.Titles[0]

	// Skip original titles (we want the crowdsourced ones)
	if firstTitle.Original {
		slog.Debug("First title is original, skipping", "videoID", videoID)
		return "", false, nil
	}

	// Check if the title is trusted (locked or has positive/zero votes)
	trusted = firstTitle.Locked || firstTitle.Votes >= 0

	slog.Debug("DeArrow title fetched",
		"videoID", videoID,
		"title", firstTitle.Title,
		"votes", firstTitle.Votes,
		"locked", firstTitle.Locked,
		"trusted", trusted)

	return firstTitle.Title, trusted, nil
}
