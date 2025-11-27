package youtube

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/database"
)

// CachedTitle represents a cached DeArrow title entry
type CachedTitle struct {
	VideoID       string
	OriginalTitle string
	DeArrowTitle  sql.NullString
	Votes         sql.NullInt64
	Locked        sql.NullBool
	Trusted       bool
	FetchedAt     time.Time
	UpdatedAt     time.Time
}

// initializeDeArrowCache initializes the database schema for DeArrow caching
func initializeDeArrowCache(db *database.Database) error {
	schema := `
		CREATE TABLE IF NOT EXISTS dearrow_titles (
			video_id TEXT PRIMARY KEY,
			original_title TEXT NOT NULL,
			dearrow_title TEXT,
			votes INTEGER,
			locked BOOLEAN,
			trusted BOOLEAN NOT NULL DEFAULT 0,
			fetched_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_dearrow_fetched_at ON dearrow_titles(fetched_at);
		CREATE INDEX IF NOT EXISTS idx_dearrow_video_id ON dearrow_titles(video_id);
	`

	if err := db.Execute(schema); err != nil {
		return fmt.Errorf("failed to initialize DeArrow cache schema: %w", err)
	}

	slog.Debug("DeArrow cache schema initialized")
	return nil
}

// getCachedTitle retrieves a cached DeArrow title from the database
func getCachedTitle(db *database.Database, videoID string) (*CachedTitle, error) {
	query := `
		SELECT video_id, original_title, dearrow_title, votes, locked, trusted, fetched_at, updated_at
		FROM dearrow_titles
		WHERE video_id = ?
	`

	var cached CachedTitle
	err := db.QueryRow(query, videoID).Scan(
		&cached.VideoID,
		&cached.OriginalTitle,
		&cached.DeArrowTitle,
		&cached.Votes,
		&cached.Locked,
		&cached.Trusted,
		&cached.FetchedAt,
		&cached.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query cached title: %w", err)
	}

	return &cached, nil
}

// saveCachedTitle saves a DeArrow title to the cache
func saveCachedTitle(db *database.Database, videoID, originalTitle, dearrowTitle string, votes int, locked, trusted bool) error {
	query := `
		INSERT INTO dearrow_titles (video_id, original_title, dearrow_title, votes, locked, trusted, fetched_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(video_id) DO UPDATE SET
			original_title = excluded.original_title,
			dearrow_title = excluded.dearrow_title,
			votes = excluded.votes,
			locked = excluded.locked,
			trusted = excluded.trusted,
			updated_at = excluded.updated_at
	`

	now := time.Now()
	var dearrowTitlePtr *string
	var votesPtr *int
	var lockedPtr *bool

	// Only set values if we have a DeArrow title
	if dearrowTitle != "" {
		dearrowTitlePtr = &dearrowTitle
		votesPtr = &votes
		lockedPtr = &locked
	}

	err := db.Execute(query, videoID, originalTitle, dearrowTitlePtr, votesPtr, lockedPtr, trusted, now, now)
	if err != nil {
		return fmt.Errorf("failed to save cached title: %w", err)
	}

	slog.Debug("Cached DeArrow title",
		"videoID", videoID,
		"hasDearrowTitle", dearrowTitle != "",
		"trusted", trusted)

	return nil
}

// shouldRefreshCache determines if a cached entry should be refreshed
// For now, we keep DeArrow titles indefinitely as they don't change frequently
// Only refresh if the entry is very old (> 30 days) to catch updates
func shouldRefreshCache(cached *CachedTitle) bool {
	if cached == nil {
		return true
	}

	// Refresh if older than 30 days
	return time.Since(cached.UpdatedAt) > 30*24*time.Hour
}
