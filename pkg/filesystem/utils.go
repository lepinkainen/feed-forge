// Package filesystem provides file system utilities and path management.
package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Common file system errors
var (
	ErrFileNotFound = errors.New("file not found")
	ErrDirNotFound  = errors.New("directory not found")
)

// cacheDir holds the configured cache directory.
// Set via SetCacheDir before any provider is created.
var cacheDir string

// SetCacheDir configures the directory used for cache databases.
func SetCacheDir(dir string) {
	cacheDir = dir
}

// getCacheDir returns the cache directory, defaulting to XDG_CACHE_HOME/feed-forge.
func getCacheDir() (string, error) {
	if cacheDir != "" {
		return cacheDir, nil
	}

	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "feed-forge"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ".cache", "feed-forge"), nil
}

// GetDefaultPath returns a file path in the cache directory.
func GetDefaultPath(filename string) (string, error) {
	dir, err := getCacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, filename), nil
}

// EnsureDirectoryExists creates the directory for the given file path if it doesn't exist
func EnsureDirectoryExists(filePath string) error {
	dir := filepath.Dir(filePath)
	if dir == "." {
		return nil // Current directory
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrDirNotFound, dir)
		}
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return nil
}
