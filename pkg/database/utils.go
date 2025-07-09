package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EnsureDirectoryExists creates the directory for the database file if it doesn't exist
func EnsureDirectoryExists(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "." {
		return nil // Current directory
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return nil
}

// GetDefaultPath returns a default database path in the executable directory
func GetDefaultPath(filename string) (string, error) {
	// Get the directory of the executable
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, filename), nil
}

// BackupDatabase creates a backup of the database file
func BackupDatabase(dbPath string) error {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database file does not exist: %s", dbPath)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := strings.TrimSuffix(dbPath, filepath.Ext(dbPath)) + "_backup_" + timestamp + filepath.Ext(dbPath)

	// Copy the database file
	input, err := os.ReadFile(dbPath)
	if err != nil {
		return fmt.Errorf("failed to read database file: %w", err)
	}

	err = os.WriteFile(backupPath, input, 0644)
	if err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// DatabaseExists checks if a database file exists
func DatabaseExists(dbPath string) bool {
	_, err := os.Stat(dbPath)
	return !os.IsNotExist(err)
}

// GetDatabaseSize returns the size of the database file in bytes
func GetDatabaseSize(dbPath string) (int64, error) {
	info, err := os.Stat(dbPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get database file info: %w", err)
	}

	return info.Size(), nil
}

// VacuumDatabase runs VACUUM on the database to reclaim space
func VacuumDatabase(db *Database) error {
	_, err := db.DB().Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	return nil
}

// AnalyzeDatabase runs ANALYZE on the database to update statistics
func AnalyzeDatabase(db *Database) error {
	_, err := db.DB().Exec("ANALYZE")
	if err != nil {
		return fmt.Errorf("failed to analyze database: %w", err)
	}

	return nil
}

// GetDatabaseInfo returns information about the database
func GetDatabaseInfo(db *Database) (map[string]interface{}, error) {
	info := make(map[string]interface{})

	// Get SQLite version
	var version string
	err := db.DB().QueryRow("SELECT sqlite_version()").Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("failed to get SQLite version: %w", err)
	}
	info["sqlite_version"] = version

	// Get database size
	if size, err := GetDatabaseSize(db.Path()); err == nil {
		info["file_size_bytes"] = size
	}

	// Get table count
	var tableCount int
	err = db.DB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tableCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get table count: %w", err)
	}
	info["table_count"] = tableCount

	return info, nil
}
