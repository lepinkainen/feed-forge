package database

import (
	"database/sql"
	"sync"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/interfaces"
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

// Ensure Database implements interfaces.Database
var _ interfaces.Database = (*Database)(nil)

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

	// If a connection for this path already exists, return it
	if db, ok := dbCache[config.Path]; ok {
		return db, nil
	}

	if config.Driver == "" {
		config.Driver = "sqlite"
	}

	db, err := sql.Open(config.Driver, config.Path)
	if err != nil {
		return nil, err
	}

	// Configure SQLite for better concurrency and performance (if using SQLite)
	if config.Driver == "sqlite" {
		pragmas := []string{
			"PRAGMA journal_mode=WAL",    // Enable WAL mode for concurrent readers/writers
			"PRAGMA busy_timeout=5000",   // 5 second timeout for lock contention
			"PRAGMA synchronous=NORMAL",  // Balance between performance and safety
			"PRAGMA temp_store=memory",   // Store temp tables in memory
			"PRAGMA mmap_size=268435456", // 256MB memory mapped I/O
		}

		for _, pragma := range pragmas {
			if _, err := db.Exec(pragma); err != nil {
				db.Close()
				return nil, err
			}
		}
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	database := &Database{
		db:     db,
		dbPath: config.Path,
	}

	// Store the new connection in the cache
	dbCache[config.Path] = database

	return database, nil
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
			tx.Rollback()
			panic(r)
		}
	}()

	err = fn(tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
