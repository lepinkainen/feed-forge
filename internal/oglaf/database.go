package oglaf

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/database"
)

// initializeSchema initializes the database schema for Oglaf provider
func initializeSchema(db *database.Database) error {
	// Create RSS items table
	createRSSItemsTable := `
	CREATE TABLE IF NOT EXISTS oglaf_rss_items (
		guid TEXT PRIMARY KEY,
		link TEXT UNIQUE NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		pub_date TEXT NOT NULL,
		image_url TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	if err := db.ExecuteSchema(createRSSItemsTable); err != nil {
		return fmt.Errorf("failed to create oglaf_rss_items table: %w", err)
	}

	// Create comic status table
	createComicStatusTable := `
	CREATE TABLE IF NOT EXISTS oglaf_comic_status (
		link TEXT PRIMARY KEY,
		image_extracted BOOLEAN DEFAULT FALSE,
		last_processed DATETIME,
		extraction_error TEXT,
		FOREIGN KEY (link) REFERENCES oglaf_rss_items(link)
	)`
	if err := db.ExecuteSchema(createComicStatusTable); err != nil {
		return fmt.Errorf("failed to create oglaf_comic_status table: %w", err)
	}

	// Create indexes for performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_oglaf_rss_items_link ON oglaf_rss_items(link)",
		"CREATE INDEX IF NOT EXISTS idx_oglaf_rss_items_pub_date ON oglaf_rss_items(pub_date)",
		"CREATE INDEX IF NOT EXISTS idx_oglaf_comic_status_processed ON oglaf_comic_status(image_extracted, last_processed)",
	}

	for _, index := range indexes {
		if err := db.ExecuteSchema(index); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	slog.Debug("Oglaf database schema initialized successfully")
	return nil
}

// saveRSSItem saves or updates an RSS item in the database
func saveRSSItem(db *database.Database, item *RSSItem) error {
	_, err := db.DB().Exec(`
		INSERT INTO oglaf_rss_items (guid, link, title, description, pub_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(link) DO UPDATE SET
			guid = excluded.guid,
			title = excluded.title,
			description = excluded.description,
			pub_date = excluded.pub_date,
			updated_at = excluded.updated_at`,
		item.GUID, item.Link, item.Title, item.Description, item.PubDate, time.Now(), time.Now())

	if err != nil {
		return fmt.Errorf("failed to save RSS item %s: %w", item.Link, err)
	}

	// Ensure comic status entry exists
	_, err = db.DB().Exec(`
		INSERT OR IGNORE INTO oglaf_comic_status (link, image_extracted, last_processed)
		VALUES (?, FALSE, NULL)`, item.Link)

	return err
}

// getRSSItemByLink retrieves an RSS item by its link
func getRSSItemByLink(db *database.Database, link string) (*RSSItem, error) {
	var item RSSItem
	var imageURL sql.NullString
	err := db.DB().QueryRow(`
		SELECT guid, link, title, description, pub_date, image_url 
		FROM oglaf_rss_items 
		WHERE link = ?`, link).Scan(
		&item.GUID, &item.Link, &item.Title, &item.Description, &item.PubDate, &imageURL)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get RSS item by link %s: %w", link, err)
	}

	if imageURL.Valid {
		item.ImageURL = imageURL.String
	} else {
		item.ImageURL = ""
	}

	return &item, nil
}

// getNewRSSItems identifies new RSS items that don't exist in the database
func getNewRSSItems(db *database.Database, allItems []*RSSItem) ([]*RSSItem, error) {
	var newItems []*RSSItem

	for _, item := range allItems {
		existing, err := getRSSItemByLink(db, item.Link)
		if err != nil {
			slog.Warn("Failed to check if RSS item exists", "link", item.Link, "error", err)
			continue
		}

		if existing == nil {
			newItems = append(newItems, item)
		}
	}

	return newItems, nil
}

// markImageExtracted marks a comic as having its image extracted
func markImageExtracted(db *database.Database, link, imageURL string) error {
	// Update RSS item with image URL
	_, err := db.DB().Exec(`
		UPDATE oglaf_rss_items 
		SET image_url = ?, updated_at = ?
		WHERE link = ?`, imageURL, time.Now(), link)
	if err != nil {
		return fmt.Errorf("failed to update RSS item image URL %s: %w", link, err)
	}

	// Update comic status
	_, err = db.DB().Exec(`
		UPDATE oglaf_comic_status 
		SET image_extracted = TRUE, last_processed = ?, extraction_error = NULL
		WHERE link = ?`, time.Now(), link)
	if err != nil {
		return fmt.Errorf("failed to update comic status %s: %w", link, err)
	}

	return nil
}

// markExtractionError marks an extraction error for a comic
func markExtractionError(db *database.Database, link, errorMsg string) error {
	_, err := db.DB().Exec(`
		UPDATE oglaf_comic_status 
		SET image_extracted = FALSE, last_processed = ?, extraction_error = ?
		WHERE link = ?`, time.Now(), errorMsg, link)
	if err != nil {
		return fmt.Errorf("failed to mark extraction error %s: %w", link, err)
	}

	return nil
}

// getUnprocessedComics retrieves comics that haven't had their images extracted yet
func getUnprocessedComics(db *database.Database, limit int) ([]*RSSItem, error) {
	rows, err := db.DB().Query(`
		SELECT r.guid, r.link, r.title, r.description, r.pub_date, r.image_url
		FROM oglaf_rss_items r
		JOIN oglaf_comic_status s ON r.link = s.link
		WHERE s.image_extracted = FALSE OR s.image_extracted IS NULL
		ORDER BY r.pub_date DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unprocessed comics: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("Failed to close unprocessed comics rows", "error", closeErr)
		}
	}()

	var items []*RSSItem
	for rows.Next() {
		var item RSSItem
		var imageURL sql.NullString
		err := rows.Scan(&item.GUID, &item.Link, &item.Title, &item.Description, &item.PubDate, &imageURL)
		if err != nil {
			slog.Error("Error scanning unprocessed comic row", "error", err)
			continue
		}

		if imageURL.Valid {
			item.ImageURL = imageURL.String
		} else {
			item.ImageURL = ""
		}

		items = append(items, &item)
	}

	return items, nil
}

// getProcessedComics retrieves comics that have been processed (have image URLs)
func getProcessedComics(db *database.Database, limit int) ([]*Item, error) {
	rows, err := db.DB().Query(`
		SELECT r.guid, r.link, r.title, r.description, r.pub_date, r.image_url
		FROM oglaf_rss_items r
		WHERE r.image_url IS NOT NULL
		ORDER BY r.pub_date DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query processed comics: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("Failed to close processed comics rows", "error", closeErr)
		}
	}()

	var items []*Item
	for rows.Next() {
		var item RSSItem
		var imageURL sql.NullString
		err := rows.Scan(&item.GUID, &item.Link, &item.Title, &item.Description, &item.PubDate, &imageURL)
		if err != nil {
			slog.Error("Error scanning processed comic row", "error", err)
			continue
		}

		if imageURL.Valid {
			item.ImageURL = imageURL.String
		} else {
			item.ImageURL = ""
		}

		oglafItem := &Item{
			RSSItem:  &item,
			imageURL: item.ImageURL,
		}
		items = append(items, oglafItem)
	}

	return items, nil
}

// cleanupOldData removes old RSS items and optimizes the database
func cleanupOldData(db *database.Database) error {
	// Remove RSS items older than 1 year
	cutoff := time.Now().AddDate(-1, 0, 0)

	result, err := db.DB().Exec(`
		DELETE FROM oglaf_rss_items 
		WHERE created_at < ?`, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup old RSS items: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		slog.Info("Cleaned up old RSS items", "count", rowsAffected)
	}

	// Clean up orphaned comic status entries
	result, err = db.DB().Exec(`
		DELETE FROM oglaf_comic_status 
		WHERE link NOT IN (SELECT link FROM oglaf_rss_items)`)
	if err != nil {
		return fmt.Errorf("failed to cleanup orphaned comic status: %w", err)
	}

	orphanedRows, _ := result.RowsAffected()
	if orphanedRows > 0 {
		slog.Info("Cleaned up orphaned comic status entries", "count", orphanedRows)
	}

	// Optimize database
	if _, err := db.DB().Exec("VACUUM"); err != nil {
		slog.Warn("Failed to vacuum database", "error", err)
	}

	return nil
}
