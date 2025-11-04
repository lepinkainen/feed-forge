// Package fingerpori provides a provider for fetching Fingerpori comics from HS.fi.
package fingerpori

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
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

	// Create enhanced HTTP client with rate limiting and retry support
	client := api.NewGenericClient()

	// Fetch and decode the JSON data using enhanced client
	var items []Item
	err := client.GetAndDecode(FingerporiAPIURL, &items, nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching Fingerpori data: %w", err)
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
