package database

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

func TestAcquireSQLiteSharesHandlePerPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.db")

	a, err := AcquireSQLite(SQLiteOptions{Path: path})
	if err != nil {
		t.Fatalf("AcquireSQLite(a) error = %v", err)
	}
	t.Cleanup(func() { _ = a.Release() })

	b, err := AcquireSQLite(SQLiteOptions{Path: path})
	if err != nil {
		t.Fatalf("AcquireSQLite(b) error = %v", err)
	}
	t.Cleanup(func() { _ = b.Release() })

	if a.DB != b.DB {
		t.Fatal("AcquireSQLite returned distinct *sql.DB for the same path, want a shared handle")
	}
}

func TestReleaseClosesOnlyAtZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "refcount.db")

	a, err := AcquireSQLite(SQLiteOptions{Path: path})
	if err != nil {
		t.Fatalf("AcquireSQLite(a) error = %v", err)
	}
	b, err := AcquireSQLite(SQLiteOptions{Path: path})
	if err != nil {
		t.Fatalf("AcquireSQLite(b) error = %v", err)
	}

	// One holder released: the shared handle must stay open for the other.
	if err := a.Release(); err != nil {
		t.Fatalf("a.Release() error = %v", err)
	}
	if err := b.PingContext(context.Background()); err != nil {
		t.Fatalf("handle closed while a reference remained: %v", err)
	}

	// Last holder released: the handle closes and the path leaves the pool.
	if err := b.Release(); err != nil {
		t.Fatalf("b.Release() error = %v", err)
	}
	if err := b.PingContext(context.Background()); err == nil {
		t.Fatal("Ping succeeded after the last release, want a closed handle")
	}

	poolMu.Lock()
	_, stillPooled := poolByKey[mustAbs(t, path)]
	poolMu.Unlock()
	if stillPooled {
		t.Fatal("path still in pool after the last release")
	}
}

func TestReleaseIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idempotent.db")

	h, err := AcquireSQLite(SQLiteOptions{Path: path})
	if err != nil {
		t.Fatalf("AcquireSQLite error = %v", err)
	}
	if err := h.Release(); err != nil {
		t.Fatalf("first Release error = %v", err)
	}
	if err := h.Release(); err != nil {
		t.Fatalf("second Release should be a no-op, got %v", err)
	}
}

func TestAcquireSQLiteMemoryNotShared(t *testing.T) {
	a, err := AcquireSQLite(SQLiteOptions{Path: ":memory:"})
	if err != nil {
		t.Fatalf("AcquireSQLite(a) error = %v", err)
	}
	t.Cleanup(func() { _ = a.Release() })
	b, err := AcquireSQLite(SQLiteOptions{Path: ":memory:"})
	if err != nil {
		t.Fatalf("AcquireSQLite(b) error = %v", err)
	}
	t.Cleanup(func() { _ = b.Release() })

	if a.DB == b.DB {
		t.Fatal("two :memory: acquires shared one handle, want distinct databases")
	}

	ctx := context.Background()
	if _, err := a.ExecContext(ctx, "CREATE TABLE t (v INTEGER)"); err != nil {
		t.Fatalf("create table in a: %v", err)
	}
	// The same statement must succeed in b: if b saw a's schema they would be
	// the same database.
	if _, err := b.ExecContext(ctx, "CREATE TABLE t (v INTEGER)"); err != nil {
		t.Fatalf("in-memory databases are not isolated: %v", err)
	}

	poolMu.Lock()
	pooled := len(poolByKey)
	poolMu.Unlock()
	if pooled != 0 {
		t.Fatalf("in-memory acquires registered %d pool entries, want 0", pooled)
	}
}

func TestAcquireReleaseConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.db")

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			h, err := AcquireSQLite(SQLiteOptions{Path: path})
			if err != nil {
				t.Errorf("AcquireSQLite error = %v", err)
				return
			}
			if err := h.PingContext(context.Background()); err != nil {
				t.Errorf("Ping error = %v", err)
			}
			if err := h.Release(); err != nil {
				t.Errorf("Release error = %v", err)
			}
		}()
	}
	wg.Wait()

	poolMu.Lock()
	_, stillPooled := poolByKey[mustAbs(t, path)]
	poolMu.Unlock()
	if stillPooled {
		t.Fatal("path still pooled after every holder released")
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("filepath.Abs(%q) error = %v", path, err)
	}
	return abs
}
