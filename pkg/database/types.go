package database

import (
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/dbinterfaces"
)

var (
	// dbCache stores active database connections, keyed by path
	dbCache = make(map[string]*Database)
	// cacheMutex protects the dbCache
	cacheMutex = &sync.Mutex{}
)

// Database represents a thread-safe database connection
type Database struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// Ensure Database implements dbinterfaces.Database
var _ dbinterfaces.Database = (*Database)(nil)

// Config holds database configuration
type Config struct {
	Path    string
	Driver  string
	Timeout time.Duration
}

// DefaultConfig returns the default database configuration
func DefaultConfig() Config {
	return Config{
		Driver:  "sqlite",
		Timeout: 30 * time.Second,
	}
}

// NewDatabase creates a new database connection
func NewDatabase(config Config) (*Database, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if db, ok := dbCache[config.Path]; ok {
		return db, nil
	}

	if config.Driver == "" {
		config.Driver = "sqlite"
	}

	db, err := openForDriver(config)
	if err != nil {
		return nil, err
	}

	database := &Database{
		db:     db,
		dbPath: config.Path,
	}
	dbCache[config.Path] = database
	return database, nil
}

// openForDriver opens the connection for the configured driver. SQLite goes
// through the shared OpenSQLite helper; any other driver gets a plain connection
// with the same pool limits and health check, because the pragmas are
// SQLite-specific.
func openForDriver(config Config) (*sql.DB, error) {
	opts := SQLiteOptions{Path: config.Path, ConnMaxLifetime: time.Hour}
	if config.Driver == "sqlite" {
		return OpenSQLite(opts)
	}

	db, err := sql.Open(config.Driver, config.Path)
	if err != nil {
		return nil, err
	}

	applyPoolLimits(db, opts)

	if err := db.Ping(); err != nil {
		closeDBOnError(db)
		return nil, err
	}

	return db, nil
}

func closeDBOnError(db *sql.DB) {
	if err := db.Close(); err != nil {
		slog.Error("Failed to close database", "error", err)
	}
}

// Close closes the database connection
func (db *Database) Close() error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Remove the connection from the cache
	delete(dbCache, db.dbPath)

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.db != nil {
		return db.db.Close()
	}
	return nil
}

// DB returns the underlying sql.DB instance (thread-safe)
func (db *Database) DB() *sql.DB {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.db
}

// Path returns the database file path
func (db *Database) Path() string {
	return db.dbPath
}

// ExecuteSchema executes a schema statement
func (db *Database) ExecuteSchema(schema string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.db.Exec(schema)
	return err
}

// Transaction executes a function within a database transaction
func (db *Database) Transaction(fn func(*sql.Tx) error) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	tx, err := db.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				slog.Error("Failed to rollback transaction", "error", rollbackErr)
			}
			panic(r)
		}
	}()

	err = fn(tx)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			slog.Error("Failed to rollback transaction", "error", rollbackErr)
		}
		return err
	}

	return tx.Commit()
}
