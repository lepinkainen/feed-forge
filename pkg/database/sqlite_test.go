package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenSQLiteCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "test.db")

	db, err := OpenSQLite(SQLiteOptions{Path: path})
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file not created: %v", err)
	}
}

func TestOpenSQLiteAppliesPragmas(t *testing.T) {
	db, err := OpenSQLite(SQLiteOptions{Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout;").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busyTimeout)
	}

	var synchronous int
	if err := db.QueryRow("PRAGMA synchronous;").Scan(&synchronous); err != nil {
		t.Fatalf("read synchronous: %v", err)
	}
	if synchronous != 1 {
		t.Errorf("synchronous = %d, want 1 (NORMAL)", synchronous)
	}
}

// TestOpenSQLitePragmasApplyToEveryPooledConnection guards the SQLITE_BUSY
// regression: busy_timeout, synchronous, temp_store and mmap_size live on the
// connection, not in the database file, so a pool whose pragmas were run as
// statements after sql.Open configures one connection and leaves every later one
// at the SQLite defaults.
func TestOpenSQLitePragmasApplyToEveryPooledConnection(t *testing.T) {
	db, err := OpenSQLite(SQLiteOptions{Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	pragmas := map[string]int{
		"busy_timeout": 5000,
		"synchronous":  1,
		"temp_store":   2,
	}

	// Hold each connection open so that the next db.Conn call is forced to create
	// a fresh one instead of reusing a connection that is already configured.
	var held []*sql.Conn
	t.Cleanup(func() {
		for _, c := range held {
			_ = c.Close()
		}
	})

	for i := range 3 {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("db.Conn() #%d error = %v", i+1, err)
		}
		held = append(held, conn)

		for pragma, want := range pragmas {
			var got int
			if err := conn.QueryRowContext(ctx, "PRAGMA "+pragma+";").Scan(&got); err != nil {
				t.Fatalf("read %s on connection #%d: %v", pragma, i+1, err)
			}
			if got != want {
				t.Errorf("connection #%d: %s = %d, want %d", i+1, pragma, got, want)
			}
		}
	}
}

// TestOpenSQLiteContendedWriteWaits covers the SQLITE_BUSY failure mode:
// two handles on one database file, one of them writing while the other holds the
// write lock. With busy_timeout in force on every connection the second writer
// waits; without it the write fails immediately with SQLITE_BUSY.
func TestOpenSQLiteContendedWriteWaits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contended.db")
	ctx := context.Background()

	writer, err := OpenSQLite(SQLiteOptions{Path: path, BusyTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("OpenSQLite(writer) error = %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	if _, err := writer.ExecContext(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	blocker, err := OpenSQLite(SQLiteOptions{Path: path, BusyTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("OpenSQLite(blocker) error = %v", err)
	}
	t.Cleanup(func() { _ = blocker.Close() })

	// Pin the writer's first connection, so the contended write below has to run
	// on a connection the pool creates afterwards.
	pinned, err := writer.Conn(ctx)
	if err != nil {
		t.Fatalf("writer.Conn() error = %v", err)
	}
	t.Cleanup(func() { _ = pinned.Close() })

	// Take the write lock on the blocker and hold it briefly.
	tx, err := blocker.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO t (v) VALUES ('blocker')"); err != nil {
		t.Fatalf("blocker insert: %v", err)
	}

	released := make(chan struct{})
	go func() {
		time.Sleep(250 * time.Millisecond)
		close(released)
		if err := tx.Commit(); err != nil {
			t.Errorf("blocker commit: %v", err)
		}
	}()

	if _, err := writer.ExecContext(ctx, "INSERT INTO t (v) VALUES ('writer')"); err != nil {
		t.Fatalf("contended write failed instead of waiting: %v", err)
	}

	select {
	case <-released:
	default:
		t.Error("contended write completed before the write lock was released")
	}
}

func TestOpenSQLitePoolLimits(t *testing.T) {
	tests := []struct {
		name        string
		opts        SQLiteOptions
		wantMaxOpen int
	}{
		{
			name:        "unset uses defaults",
			opts:        SQLiteOptions{},
			wantMaxOpen: DefaultMaxOpenConns,
		},
		{
			name:        "explicit limits are kept",
			opts:        SQLiteOptions{MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: time.Hour},
			wantMaxOpen: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			opts.Path = filepath.Join(t.TempDir(), "test.db")

			db, err := OpenSQLite(opts)
			if err != nil {
				t.Fatalf("OpenSQLite() error = %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })

			if got := db.Stats().MaxOpenConnections; got != tt.wantMaxOpen {
				t.Errorf("MaxOpenConnections = %d, want %d", got, tt.wantMaxOpen)
			}
		})
	}
}

func TestOpenSQLiteUnwritableDirectory(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(parent, 0o500); err != nil {
		t.Fatalf("create read-only directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	if _, err := OpenSQLite(SQLiteOptions{Path: filepath.Join(parent, "sub", "test.db")}); err == nil {
		t.Fatal("OpenSQLite() error = nil, want an error for an unwritable directory")
	}
}
