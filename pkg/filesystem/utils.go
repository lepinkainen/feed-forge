package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureDirectoryExists creates the directory for the given file path if it doesn't exist
func EnsureDirectoryExists(filePath string) error {
	dir := filepath.Dir(filePath)
	if dir == "." {
		return nil // Current directory
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return nil
}
