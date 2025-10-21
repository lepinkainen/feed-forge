package fingerpori

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	// FingerporiAPIURL is the endpoint for fetching Fingerpori comics from HS.fi
	FingerporiAPIURL = "https://www.hs.fi/api/laneitems/39221/list/normal/290"

	// ImageBaseURL is the base URL for Sanoma's image CDN
	ImageBaseURL = "https://images.sanoma-sndp.fi/"

	// DefaultImageWidth is the width to use for comic images
	DefaultImageWidth = 1440
)

// fetchItems fetches Fingerpori comics from the HS.fi API
func fetchItems() ([]Item, error) {
	slog.Debug("Fetching Fingerpori items from API", "url", FingerporiAPIURL)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Fetch the JSON data
	resp, err := client.Get(FingerporiAPIURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching data: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("Error closing response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Parse the JSON data
	var items []Item
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	slog.Debug("Successfully fetched Fingerpori items", "count", len(items))
	return items, nil
}

// processItems processes raw API items and adds computed fields
func processItems(items []Item) []Item {
	now := time.Now()

	for i := range items {
		item := &items[i]

		// Parse the display date
		displayDate, err := time.Parse("2006-01-02T15:04:05.000-07:00", item.DisplayDate)
		if err != nil {
			slog.Warn("Error parsing date, using current time",
				"date", item.DisplayDate,
				"error", err)
			displayDate = now
		}
		item.ParsedDate = displayDate

		// Process the image URL
		// Extract image ID from the URL (format: /path/to/IMAGE_ID/...)
		imageID := extractImageID(item.Picture.URL)
		if imageID != "" {
			item.ProcessedImageURL = fmt.Sprintf("%s%s/normal/%d.jpg",
				ImageBaseURL, imageID, DefaultImageWidth)
		}

		// Generate HTML content with the image
		item.ContentHTML = fmt.Sprintf(`<img src=%q alt=%q>`,
			item.ProcessedImageURL, item.ItemTitle)
	}

	return items
}

// extractImageID extracts the image ID from a picture URL
// Example URL: "/path/to/abc123def/file.jpg" -> "abc123def"
func extractImageID(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 3 {
		return parts[3]
	}
	return ""
}
