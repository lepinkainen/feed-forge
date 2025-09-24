package hackernews

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
)

// fetchItems retrieves current front page items from Algolia API
func fetchItems() []Item {
	slog.Debug("Fetching Hacker News items from Algolia API")

	var algoliaResp AlgoliaResponse
	client := api.NewHackerNewsClient() // Use enhanced client with rate limiting
	err := client.GetAndDecode("https://hn.algolia.com/api/v1/search_by_date?tags=front_page&hitsPerPage=100", &algoliaResp, nil)
	if err != nil {
		slog.Error("Failed to fetch or decode Hacker News items", "error", err)
		return nil
	}

	var items []Item
	now := time.Now()
	slog.Debug("Processing Algolia response", "hitCount", len(algoliaResp.Hits))

	for _, hit := range algoliaResp.Hits {
		// Parse the ISO 8601 timestamp
		createdAt, err := time.Parse(time.RFC3339, hit.CreatedAt)
		if err != nil {
			slog.Warn("Failed to parse timestamp, using current time", "error", err, "timestamp", hit.CreatedAt)
			createdAt = now
		}

		// Generate comments link from object ID
		commentsLink := fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID)

		// Use numeric fields directly
		points := hit.Points
		commentCount := hit.NumComments

		slog.Debug("Found item",
			"title", hit.Title,
			"link", hit.URL,
			"commentsLink", commentsLink,
			"points", points,
			"comments", commentCount,
			"author", hit.Author,
			"createdAt", createdAt)

		items = append(items, Item{
			ItemID:           hit.ObjectID,
			ItemTitle:        hit.Title,
			ItemLink:         hit.URL,
			ItemCommentsLink: commentsLink,
			Points:           points,
			ItemCommentCount: commentCount,
			ItemAuthor:       hit.Author,
			ItemCreatedAt:    createdAt,
			UpdatedAt:        now,
		})
	}

	slog.Debug("Finished processing items", "totalItems", len(items))
	return items
}

// updateItemStats updates item statistics using concurrent API calls to Algolia
func updateItemStats(db *sql.DB, items []Item, recentlyUpdated map[string]bool) {
	slog.Debug("Updating item stats", "itemCount", len(items))
	skippedCount := 0

	// Filter items that need updating
	var itemsToUpdate []Item
	for _, item := range items {
		// Skip items with empty ItemID
		if item.ItemID == "" {
			slog.Warn("Skipping item with empty ItemID", "title", item.ItemTitle)
			continue
		}

		// Skip items that were just updated in updateStoredItems
		if recentlyUpdated[item.ItemID] {
			slog.Debug("Skipping recently updated item", "hn_id", item.ItemID)
			skippedCount++
			continue
		}

		itemsToUpdate = append(itemsToUpdate, item)
	}

	if len(itemsToUpdate) == 0 {
		if skippedCount > 0 {
			slog.Debug("Skipped recently updated items", "count", skippedCount)
		}
		return
	}

	// Create worker pool for concurrent API calls
	const numWorkers = 10
	workChan := make(chan Item, len(itemsToUpdate))
	resultChan := make(chan statsUpdate, len(itemsToUpdate))
	var wg sync.WaitGroup

	// Start workers
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range workChan {
				update := fetchItemStats(item.ItemID)
				resultChan <- update
			}
		}()
	}

	// Send work to workers
	for _, item := range itemsToUpdate {
		workChan <- item
	}
	close(workChan)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results and update database
	updatedCount := 0
	deletedCount := 0
	for update := range resultChan {
		if update.err != nil {
			if update.isDeadItem {
				// Delete the dead item from database
				_, err := db.Exec(`DELETE FROM items WHERE item_hn_id = ?`, update.itemID)
				if err != nil {
					slog.Warn("Failed to delete dead item from database", "error", err, "hn_id", update.itemID)
				} else {
					slog.Debug("Deleted dead item from database", "hn_id", update.itemID)
					deletedCount++
				}
			} else {
				slog.Warn("Failed to fetch item stats from Algolia", "error", update.err, "hn_id", update.itemID)
			}
			continue
		}

		// Update database with current stats
		_, err := db.Exec(`
			UPDATE items SET 
				points = ?, 
				comment_count = ?, 
				updated_at = ?
			WHERE item_hn_id = ?`,
			update.points, update.commentCount, time.Now(), update.itemID)

		if err != nil {
			slog.Warn("Failed to update item stats in database", "error", err, "hn_id", update.itemID)
			continue
		}

		slog.Debug("Updated item stats", "hn_id", update.itemID, "points", update.points, "comments", update.commentCount)
		updatedCount++
	}

	slog.Debug("Completed stats update", "updated", updatedCount, "deleted", deletedCount, "skipped", skippedCount)
}

// fetchItemStats retrieves current statistics for a single item from Algolia API
func fetchItemStats(itemID string) statsUpdate {
	// Fetch current stats from Algolia API using enhanced client
	url := fmt.Sprintf("https://hn.algolia.com/api/v1/items/%s", itemID)
	client := api.NewHackerNewsClient() // Use enhanced client with rate limiting and retries
	var algoliaItem AlgoliaHit
	err := client.GetAndDecode(url, &algoliaItem, nil)
	if err != nil {
		// Check if this is a 404 Not Found or 410 Gone error, indicating the item has been deleted
		var httpErr *api.HTTPError
		if errors.As(err, &httpErr) && (httpErr.StatusCode == 404 || httpErr.StatusCode == 410) {
			slog.Debug("Item not found (404/410), marking as dead", "hn_id", itemID, "status", httpErr.StatusCode)
			return statsUpdate{itemID: itemID, isDeadItem: true, err: nil}
		}
		return statsUpdate{itemID: itemID, err: fmt.Errorf("failed to decode JSON: %w", err)}
	}

	return statsUpdate{
		itemID:       itemID,
		points:       algoliaItem.Points,
		commentCount: algoliaItem.NumComments,
		err:          nil,
	}
}
