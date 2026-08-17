package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
)

// opengraph.db and http_cache.db are shared files, but every provider opens its
// own handle on them and the generate command runs its providers concurrently.
// That produced one connection pool per provider on each shared file. The pool
// below hands every caller on a given path the same *sql.DB and refcounts it, so
// the file is opened once and closed only when the last holder releases it.
//
// Refcounting is mandatory rather than a nicety: each provider closes its own
// databases when it finishes, so a plain shared map would let the first provider
// to finish close the handle the other providers still hold, giving
// "sql: database is closed" on the rest.
var (
	poolMu    sync.Mutex
	poolByKey = make(map[string]*pooledDB)
)

type pooledDB struct {
	db   *sql.DB
	refs int
}

// Handle is a reference to a shared *sql.DB obtained from AcquireSQLite.
//
// Callers must release it with Release, never Close. Close is promoted from the
// embedded *sql.DB and shuts the underlying handle for every other holder while
// bypassing the refcount; Release closes the handle only when the last reference
// is gone.
type Handle struct {
	*sql.DB
	key      string // "" for a standalone (unregistered) handle
	released sync.Once
}

// AcquireSQLite returns a shared handle for opts.Path, opening the database on
// the first acquire and reusing the same *sql.DB on later acquires of the same
// path. The path is canonicalized with filepath.Abs so two spellings of one file
// share a handle.
//
// An in-memory (":memory:"), empty, or Isolated path is never shared: every
// in-memory open is a distinct database, so keying them together would silently
// merge unrelated databases. Those cases return a standalone handle whose Release
// simply closes it.
func AcquireSQLite(opts SQLiteOptions) (*Handle, error) {
	if opts.Isolated || opts.Path == "" || opts.Path == ":memory:" {
		db, err := OpenSQLite(opts)
		if err != nil {
			return nil, err
		}
		return &Handle{DB: db}, nil
	}

	key, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path %s: %w", opts.Path, err)
	}

	poolMu.Lock()
	defer poolMu.Unlock()

	if e, ok := poolByKey[key]; ok {
		e.refs++
		return &Handle{DB: e.db, key: key}, nil
	}

	db, err := OpenSQLite(opts)
	if err != nil {
		return nil, err
	}
	poolByKey[key] = &pooledDB{db: db, refs: 1}
	return &Handle{DB: db, key: key}, nil
}

// Release drops this reference to the shared handle. When the last reference is
// released the underlying *sql.DB is closed and the path leaves the pool. Release
// is idempotent: a second call on the same Handle is a no-op that returns nil, so
// wrapping types can release from Close without guarding against a double close.
func (h *Handle) Release() error {
	if h == nil {
		return nil
	}

	var err error
	h.released.Do(func() {
		if h.key == "" {
			err = h.Close()
			return
		}

		poolMu.Lock()
		defer poolMu.Unlock()

		e, ok := poolByKey[h.key]
		if !ok {
			return
		}
		e.refs--
		if e.refs <= 0 {
			delete(poolByKey, h.key)
			err = e.db.Close()
		}
	})
	return err
}
