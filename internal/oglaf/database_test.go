package oglaf

import (
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/database"
)

func TestDatabaseSchema(t *testing.T) {
	// Create a temporary database for testing
	db, err := database.NewDatabase(database.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Test schema initialization
	if err := initializeSchema(db); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Test saving an RSS item
	item := &RSSItem{
		GUID:        "test-guid",
		Link:        "https://www.oglaf.com/test/",
		Title:       "Test Comic",
		Description: "Test description",
		PubDate:     time.Now().Format(time.RFC1123Z),
		ImageURL:    "",
	}

	if err := saveRSSItem(db, item); err != nil {
		t.Fatalf("Failed to save RSS item: %v", err)
	}

	// Test retrieving the RSS item
	retrieved, err := getRSSItemByLink(db, item.Link)
	if err != nil {
		t.Fatalf("Failed to retrieve RSS item: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected to find RSS item but got nil")
	}

	if retrieved.GUID != item.GUID || retrieved.Title != item.Title {
		t.Errorf("Retrieved item doesn't match saved item")
	}

	// Test marking image as extracted
	imageURL := "https://media.oglaf.com/comic/test.jpg"
	if err := markImageExtracted(db, item.Link, imageURL); err != nil {
		t.Fatalf("Failed to mark image as extracted: %v", err)
	}

	// Test getting unprocessed comics (should be empty)
	unprocessed, err := getUnprocessedComics(db, 10)
	if err != nil {
		t.Fatalf("Failed to get unprocessed comics: %v", err)
	}
	if len(unprocessed) != 0 {
		t.Errorf("Expected 0 unprocessed comics, got %d", len(unprocessed))
	}

	// Test getting processed comics (should have 1)
	processed, err := getProcessedComics(db, 10)
	if err != nil {
		t.Fatalf("Failed to get processed comics: %v", err)
	}
	if len(processed) != 1 {
		t.Errorf("Expected 1 processed comic, got %d", len(processed))
	}
}

func TestNewRSSItemsDetection(t *testing.T) {
	// Create a temporary database for testing
	db, err := database.NewDatabase(database.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Initialize schema
	if err := initializeSchema(db); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Create test items
	existingItem := &RSSItem{
		GUID:        "existing-guid",
		Link:        "https://www.oglaf.com/existing/",
		Title:       "Existing Comic",
		Description: "Existing description",
		PubDate:     time.Now().Add(-1 * time.Hour).Format(time.RFC1123Z),
	}

	newItem := &RSSItem{
		GUID:        "new-guid",
		Link:        "https://www.oglaf.com/new/",
		Title:       "New Comic",
		Description: "New description",
		PubDate:     time.Now().Format(time.RFC1123Z),
	}

	allItems := []*RSSItem{existingItem, newItem}

	// Save existing item
	if err := saveRSSItem(db, existingItem); err != nil {
		t.Fatalf("Failed to save existing item: %v", err)
	}

	// Test new item detection
	newItems, err := getNewRSSItems(db, allItems)
	if err != nil {
		t.Fatalf("Failed to detect new items: %v", err)
	}

	if len(newItems) != 1 {
		t.Errorf("Expected 1 new item, got %d", len(newItems))
	}

	if len(newItems) > 0 && newItems[0].Link != newItem.Link {
		t.Errorf("Expected new item with link %s, got %s", newItem.Link, newItems[0].Link)
	}
}
