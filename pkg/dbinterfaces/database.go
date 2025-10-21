// Package dbinterfaces provides shared database interface definitions.
package dbinterfaces

import "io"

// Database defines the common interface for database operations
type Database interface {
	io.Closer // Close() error
}

// StatsProvider defines the interface for databases that provide statistics
type StatsProvider interface {
	GetStats() (map[string]any, error)
}

// CleanupProvider defines the interface for databases that support cleanup operations
type CleanupProvider interface {
	CleanupExpired() error
}
