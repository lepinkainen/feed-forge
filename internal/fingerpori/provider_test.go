package fingerpori

import (
	"testing"
	"time"
)

func TestFingerporiItem_FeedItemInterface(t *testing.T) {
	// Create a test item
	item := &FingerporiItem{
		ItemID:      12345,
		Href:        "/fingerpori/cart-1234567890",
		DisplayDate: "2024-01-15T00:00:00.000+02:00",
		ItemTitle:   "Test Comic",
		Picture: Picture{
			ID:           67890,
			Width:        800,
			Height:       600,
			URL:          "/path/to/abc123def/image.jpg",
			SquareURL:    "/path/to/abc123def/square.jpg",
			Photographer: "Pertti Jarla",
		},
		Tags:              []string{"comics", "humor"},
		ParsedDate:        time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		ProcessedImageURL: "https://images.sanoma-sndp.fi/abc123def/normal/1440.jpg",
		ContentHTML:       `<img src="https://images.sanoma-sndp.fi/abc123def/normal/1440.jpg" alt="Test Comic">`,
	}

	// Test FeedItem interface methods
	if item.Title() == "" {
		t.Error("Title() should not be empty")
	}

	if item.Link() != "https://www.hs.fi/fingerpori/cart-1234567890" {
		t.Errorf("Link() = %s, want https://www.hs.fi/fingerpori/cart-1234567890", item.Link())
	}

	if item.CommentsLink() != item.Link() {
		t.Error("CommentsLink() should equal Link()")
	}

	if item.Author() != "Pertti Jarla" {
		t.Errorf("Author() = %s, want Pertti Jarla", item.Author())
	}

	if item.Score() != 0 {
		t.Errorf("Score() = %d, want 0", item.Score())
	}

	if item.CommentCount() != 0 {
		t.Errorf("CommentCount() = %d, want 0", item.CommentCount())
	}

	if item.CreatedAt().IsZero() {
		t.Error("CreatedAt() should not be zero")
	}

	if len(item.Categories()) != 2 {
		t.Errorf("Categories() length = %d, want 2", len(item.Categories()))
	}

	if item.ImageURL() != "https://images.sanoma-sndp.fi/abc123def/normal/1440.jpg" {
		t.Errorf("ImageURL() = %s, want https://images.sanoma-sndp.fi/abc123def/normal/1440.jpg", item.ImageURL())
	}

	if item.Content() == "" {
		t.Error("Content() should not be empty")
	}
}

func TestExtractImageID(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		want  string
	}{
		{
			name: "Valid URL with image ID",
			url:  "/path/to/abc123def/image.jpg",
			want: "abc123def",
		},
		{
			name: "Short URL",
			url:  "/short",
			want: "",
		},
		{
			name: "Empty URL",
			url:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractImageID(tt.url); got != tt.want {
				t.Errorf("extractImageID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessItems(t *testing.T) {
	items := []FingerporiItem{
		{
			ItemID:      1,
			DisplayDate: "2024-01-15T00:00:00.000+02:00",
			ItemTitle:   "Test Comic 1",
			Picture: Picture{
				URL:          "/path/to/img123/image.jpg",
				Photographer: "Pertti Jarla",
			},
		},
	}

	processed := processItems(items)

	if len(processed) != 1 {
		t.Fatalf("processItems() returned %d items, want 1", len(processed))
	}

	item := processed[0]

	if item.ParsedDate.IsZero() {
		t.Error("ParsedDate should be set")
	}

	if item.ProcessedImageURL == "" {
		t.Error("ProcessedImageURL should be set")
	}

	if item.ContentHTML == "" {
		t.Error("ContentHTML should be set")
	}
}
