package hackernews

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/lepinkainen/feed-forge/pkg/database"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// dbMutex protects concurrent access to OpenGraph database operations
var dbMutex sync.Mutex

// initializeSchema initializes the database schema using shared utilities
func initializeSchema(db *database.Database) error {
	// Create items table if it doesn't exist
	createItemsTable := `
	CREATE TABLE IF NOT EXISTS items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,      -- Internal row ID (optional, but common)
		item_hn_id TEXT NOT NULL UNIQUE,        -- Hacker News Item ID, for deduplication
		title TEXT NOT NULL,
		link TEXT NOT NULL,                     -- The actual article URL
		comments_link TEXT,
		points INTEGER DEFAULT 0,
		comment_count INTEGER DEFAULT 0,
		author TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	if err := db.ExecuteSchema(createItemsTable); err != nil {
		return fmt.Errorf("failed to create items table: %w", err)
	}

	slog.Debug("Database schema initialized successfully")
	return nil
}

// updateStoredItems updates the database with new items, returns map of updated item IDs
func updateStoredItems(db *database.Database, newItems []HackerNewsItem) map[string]bool {
	slog.Debug("Updating stored items", "itemCount", len(newItems))
	updatedItems := make(map[string]bool)

	for _, item := range newItems {
		// The 'item.CreatedAt' should be the original submission time of the HN post.
		// The 'item.UpdatedAt' should be when it was last seen/modified by your scraper.
		result, err := db.DB().Exec(`
			INSERT INTO items (item_hn_id, title, link, comments_link, points, comment_count, author, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(item_hn_id) DO UPDATE SET
				title = excluded.title,
				link = excluded.link, 
				comments_link = excluded.comments_link,
				points = excluded.points,
				comment_count = excluded.comment_count,
				author = excluded.author,
				updated_at = excluded.updated_at`, // Note: created_at is not updated on conflict
			item.ItemID, item.ItemTitle, item.ItemLink, item.ItemCommentsLink, item.Points, item.ItemCommentCount, item.ItemAuthor, item.ItemCreatedAt, item.UpdatedAt)

		if err != nil {
			slog.Error("Error updating item", "error", err, "hn_id", item.ItemID)
			continue
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			slog.Debug("Processed item (added/updated in DB)", "title", item.ItemTitle, "hn_id", item.ItemID)
			updatedItems[item.ItemID] = true
		}
	}

	return updatedItems
}

// getAllItems retrieves items from database with minimum points threshold
func getAllItems(db *database.Database, limit int, minPoints int) ([]HackerNewsItem, error) {
	slog.Debug("Querying database for items", "limit", limit, "minPoints", minPoints)
	rows, err := db.DB().Query("SELECT item_hn_id, title, link, comments_link, points, comment_count, author, created_at, updated_at FROM items WHERE points > ? ORDER BY created_at DESC LIMIT ?", minPoints, limit)
	if err != nil {
		slog.Error("Failed to query database", "error", err)
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var items []HackerNewsItem
	for rows.Next() {
		var item HackerNewsItem
		err := rows.Scan(&item.ItemID, &item.ItemTitle, &item.ItemLink, &item.ItemCommentsLink, &item.Points, &item.ItemCommentCount, &item.ItemAuthor, &item.ItemCreatedAt, &item.UpdatedAt)
		if err != nil {
			slog.Error("Error scanning row", "error", err)
			continue
		}
		items = append(items, item)
	}

	slog.Debug("Retrieved items from database", "count", len(items))
	return items, nil
}
