package bulletin

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// schema is applied idempotently on Store open. Timestamps use TIMESTAMP
// affinity so the modernc.org/sqlite driver round-trips time.Time as
// sortable RFC3339Nano text (see CLAUDE.md database guidance).
const schema = `
CREATE TABLE IF NOT EXISTS bulletins (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	published_at TIMESTAMP NOT NULL,
	slot         TEXT NOT NULL,
	title        TEXT NOT NULL DEFAULT '',
	content      TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS items (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	feed_url    TEXT NOT NULL,
	category    TEXT NOT NULL,
	url         TEXT NOT NULL UNIQUE,
	title       TEXT NOT NULL,
	raw_text    TEXT NOT NULL,
	simhash     INTEGER NOT NULL,
	fetched_at  TIMESTAMP NOT NULL,
	bulletin_id INTEGER REFERENCES bulletins(id)
);
CREATE INDEX IF NOT EXISTS idx_items_unpublished ON items(bulletin_id) WHERE bulletin_id IS NULL;
`

// Item is a stored source-feed entry.
type Item struct {
	ID         int64
	FeedURL    string
	Category   string
	URL        string
	Title      string
	RawText    string
	SimHash    uint64
	FetchedAt  time.Time
	BulletinID sql.NullInt64
}

// Store wraps the bulletin SQLite database.
type Store struct {
	db *sql.DB
}

// NewStore opens (creating if needed) the bulletin database at dbPath and
// applies the schema.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open bulletin db: %w", err)
	}
	if err := configureSQLite(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply bulletin schema: %w", err)
	}
	return &Store{db: db}, nil
}

// configureSQLite applies pragmas so that overlapping bulletin-fetch and
// bulletin-publish processes sharing the database file wait for each other
// instead of failing immediately with "database is locked".
func configureSQLite(db *sql.DB) error {
	ctx := context.Background()
	for _, pragma := range []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure bulletin db %q: %w", pragma, err)
		}
	}
	return nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

// InsertItem stores a newly fetched item. Items are keyed by URL; a repeat URL
// is ignored (ON CONFLICT DO NOTHING) so re-fetching the same feed is cheap and
// idempotent. Returns true if a new row was inserted.
func (s *Store) InsertItem(ctx context.Context, it Item) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO items (feed_url, category, url, title, raw_text, simhash, fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(url) DO NOTHING`,
		it.FeedURL, it.Category, it.URL, it.Title, it.RawText,
		//nolint:gosec // G115: SimHash is a 64-bit fingerprint stored as SQLite INTEGER; bit pattern preserved.
		int64(it.SimHash), it.FetchedAt,
	)
	if err != nil {
		return false, fmt.Errorf("insert item: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// HasItem reports whether an item with the given URL is already stored. Used to
// skip the expensive article fetch+extract for entries seen in a prior run.
func (s *Store) HasItem(ctx context.Context, url string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM items WHERE url = ? LIMIT 1`, url).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("has item: %w", err)
	}
	return true, nil
}

// UnpublishedItems returns all items not yet assigned to a bulletin, oldest
// first.
func (s *Store) UnpublishedItems(ctx context.Context) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, feed_url, category, url, title, raw_text, simhash, fetched_at
		 FROM items WHERE bulletin_id IS NULL ORDER BY fetched_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query unpublished: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []Item
	for rows.Next() {
		var it Item
		var sh int64
		if err := rows.Scan(&it.ID, &it.FeedURL, &it.Category, &it.URL,
			&it.Title, &it.RawText, &sh, &it.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		//nolint:gosec // G115: reinterpreting the stored INTEGER bit pattern back into the uint64 fingerprint.
		it.SimHash = uint64(sh)
		items = append(items, it)
	}
	return items, rows.Err()
}

// Row is a stored, rendered bulletin.
type Row struct {
	ID          int64
	PublishedAt time.Time
	Slot        string
	Title       string
	Content     string // digest HTML fragment
}

// CreateBulletin records a published bulletin (including its rendered digest) and
// marks the given item IDs as belonging to it, in a single transaction. Returns
// the new bulletin ID.
func (s *Store) CreateBulletin(ctx context.Context, b Row, itemIDs []int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO bulletins (published_at, slot, title, content) VALUES (?, ?, ?, ?)`,
		b.PublishedAt, b.Slot, b.Title, b.Content)
	if err != nil {
		return 0, fmt.Errorf("insert bulletin: %w", err)
	}
	bulletinID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("bulletin id: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `UPDATE items SET bulletin_id = ? WHERE id = ?`)
	if err != nil {
		return 0, fmt.Errorf("prepare mark: %w", err)
	}
	defer func() { _ = stmt.Close() }()
	for _, id := range itemIDs {
		if _, err := stmt.ExecContext(ctx, bulletinID, id); err != nil {
			return 0, fmt.Errorf("mark item %d: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return bulletinID, nil
}

// LatestBulletins returns up to limit most recent bulletins, newest first, for
// re-rendering the feed.
func (s *Store) LatestBulletins(ctx context.Context, limit int) ([]Row, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, published_at, slot, title, content
		 FROM bulletins WHERE content <> '' ORDER BY published_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query bulletins: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Row
	for rows.Next() {
		var b Row
		if err := rows.Scan(&b.ID, &b.PublishedAt, &b.Slot, &b.Title, &b.Content); err != nil {
			return nil, fmt.Errorf("scan bulletin: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
