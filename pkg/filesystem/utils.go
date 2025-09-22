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

// GetDefaultPath returns a default file path in the executable directory
func GetDefaultPath(filename string) (string, error) {
	// Get the directory of the executable
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, filename), nil
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
