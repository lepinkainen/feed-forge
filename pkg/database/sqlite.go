package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/filesystem"

	_ "modernc.org/sqlite" // SQLite driver, registered as "sqlite"
)

// Default connection pool limits applied when SQLiteOptions leaves them unset.
const (
	DefaultMaxOpenConns = 10
	DefaultMaxIdleConns = 5
)

// DefaultBusyTimeout is how long a connection waits for a lock held by another
// connection or process before it gives up with SQLITE_BUSY.
const DefaultBusyTimeout = 5 * time.Second

// connectionPragmas are the pragmas that SQLite keeps per connection rather than
// in the database file. They must travel in the DSN: the driver then applies them
// to every connection it opens, including the ones database/sql adds to the pool
// later. Running them as statements after sql.Open would configure one pooled
// connection and leave the rest at their defaults, which means busy_timeout=0 and
// an immediate "database is locked (5) (SQLITE_BUSY)" on the first contended
// write.
func connectionPragmas(busyTimeout time.Duration) []string {
	return []string{
		fmt.Sprintf("busy_timeout=%d", busyTimeout.Milliseconds()),
		"synchronous=NORMAL",
		"temp_store=memory",
		"mmap_size=268435456",
	}
}

// SQLiteOptions configures a SQLite connection pool.
type SQLiteOptions struct {
	// Path is the database file path. Required.
	Path string
	// MaxOpenConns is the maximum number of open connections.
	// Zero means DefaultMaxOpenConns.
	MaxOpenConns int
	// MaxIdleConns is the maximum number of idle connections.
	// Zero means DefaultMaxIdleConns.
	MaxIdleConns int
	// ConnMaxLifetime is the maximum lifetime of a connection.
	// Zero means connections are reused forever.
	ConnMaxLifetime time.Duration
	// BusyTimeout is how long a connection waits for a lock before it returns
	// SQLITE_BUSY. Zero means DefaultBusyTimeout.
	BusyTimeout time.Duration
	// Isolated forces AcquireSQLite to open a private, unregistered handle
	// instead of the shared per-path handle. It is only read by AcquireSQLite;
	// OpenSQLite ignores it. Used by tests that need two real handles on one
	// path to reproduce cross-handle write contention.
	Isolated bool
}

// dsn builds the connection string. The pragmas ride along as query parameters so
// the driver applies them to every connection. The path is not prefixed with
// "file:", so the driver treats everything before the "?" as a literal path and no
// URI escaping is needed.
func (o SQLiteOptions) dsn() string {
	busyTimeout := o.BusyTimeout
	if busyTimeout <= 0 {
		busyTimeout = DefaultBusyTimeout
	}

	params := url.Values{"_pragma": connectionPragmas(busyTimeout)}
	return o.Path + "?" + params.Encode()
}

// OpenSQLite opens the SQLite database at opts.Path. It creates the parent
// directory, applies the shared pragmas, sets the connection pool limits, and
// verifies the connection with Ping. On any failure after the handle is open,
// the handle is closed before the error is returned.
func OpenSQLite(opts SQLiteOptions) (*sql.DB, error) {
	if err := filesystem.EnsureDirectoryExists(opts.Path); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", opts.dsn())
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", opts.Path, err)
	}

	if err := ensureWAL(context.Background(), db); err != nil {
		closeDBOnError(db)
		return nil, err
	}

	applyPoolLimits(db, opts)

	if err := db.Ping(); err != nil {
		closeDBOnError(db)
		return nil, fmt.Errorf("ping database %s: %w", opts.Path, err)
	}

	return db, nil
}

// applyPoolLimits sets the connection pool limits, substituting the defaults for
// unset values.
func applyPoolLimits(db *sql.DB, opts SQLiteOptions) {
	maxOpen := opts.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = DefaultMaxOpenConns
	}
	maxIdle := opts.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = DefaultMaxIdleConns
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(opts.ConnMaxLifetime)
}

// ensureWAL switches the database to write-ahead logging. Unlike the pragmas in
// connectionPragmas, journal_mode is stored in the database file, so one
// statement on one connection configures the file for every connection and every
// process that opens it afterwards. WAL lets readers run while a writer holds the
// write lock, which is what keeps overlapping commands such as bulletin-fetch and
// bulletin-publish, or the providers that generate runs concurrently, out of each
// other's way.
//
// The mode is only set when the file is not in WAL already: converting a database
// needs an exclusive lock, so a redundant conversion attempt can fail against a
// busy file. A database that cannot use WAL at all, such as an in-memory one,
// reports its own mode and is left alone.
func ensureWAL(ctx context.Context, db *sql.DB) error {
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		return fmt.Errorf("read journal mode: %w", err)
	}
	if strings.EqualFold(journalMode, "wal") || strings.EqualFold(journalMode, "memory") {
		return nil
	}

	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("set journal mode: %w", err)
	}

	return nil
}
