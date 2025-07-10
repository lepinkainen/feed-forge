package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetDefaultPath(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "simple filename",
			filename: "test.db",
		},
		{
			name:     "filename with extension",
			filename: "database.sqlite",
		},
		{
			name:     "filename without extension",
			filename: "mydb",
		},
		{
			name:     "empty filename",
			filename: "",
		},
		{
			name:     "filename with special characters",
			filename: "test-db_v2.sqlite3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetDefaultPath(tt.filename)
			if err != nil {
				t.Errorf("GetDefaultPath(%q) error = %v", tt.filename, err)
				return
			}

			if result == "" {
				t.Error("GetDefaultPath() returned empty path")
				return
			}

			// Verify the path contains the filename
			if tt.filename != "" && !strings.Contains(result, tt.filename) {
				t.Errorf("GetDefaultPath(%q) = %q, does not contain filename", tt.filename, result)
			}

			// Verify the path is absolute
			if !filepath.IsAbs(result) {
				t.Errorf("GetDefaultPath(%q) = %q, should be absolute path", tt.filename, result)
			}

			// Verify the directory exists (it should be the executable directory)
			dir := filepath.Dir(result)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				t.Errorf("GetDefaultPath(%q) directory %q does not exist", tt.filename, dir)
			}
		})
	}
}

func TestDatabaseExists(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "db_exists_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	existingFile := filepath.Join(tempDir, "existing.db")
	if err := os.WriteFile(existingFile, []byte("test data"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name     string
		dbPath   string
		expected bool
	}{
		{
			name:     "existing file",
			dbPath:   existingFile,
			expected: true,
		},
		{
			name:     "non-existing file",
			dbPath:   filepath.Join(tempDir, "nonexistent.db"),
			expected: false,
		},
		{
			name:     "empty path",
			dbPath:   "",
			expected: false,
		},
		{
			name:     "directory instead of file",
			dbPath:   tempDir,
			expected: true, // Directory exists, so this returns true
		},
		{
			name:     "path with invalid characters",
			dbPath:   filepath.Join(tempDir, "file?.db"), // Invalid on some filesystems
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DatabaseExists(tt.dbPath)
			if result != tt.expected {
				t.Errorf("DatabaseExists(%q) = %v, expected %v", tt.dbPath, result, tt.expected)
			}
		})
	}
}

func TestGetDatabaseSize(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "db_size_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files with known sizes
	testData := "This is test data for size testing"
	smallFile := filepath.Join(tempDir, "small.db")
	if err := os.WriteFile(smallFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to create small test file: %v", err)
	}

	emptyFile := filepath.Join(tempDir, "empty.db")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty test file: %v", err)
	}

	tests := []struct {
		name         string
		dbPath       string
		expectError  bool
		expectedSize int64
	}{
		{
			name:         "small file",
			dbPath:       smallFile,
			expectError:  false,
			expectedSize: int64(len(testData)),
		},
		{
			name:         "empty file",
			dbPath:       emptyFile,
			expectError:  false,
			expectedSize: 0,
		},
		{
			name:         "non-existing file",
			dbPath:       filepath.Join(tempDir, "nonexistent.db"),
			expectError:  true,
			expectedSize: 0,
		},
		{
			name:         "directory instead of file",
			dbPath:       tempDir,
			expectError:  false,
			expectedSize: -1, // We don't know the exact size of a directory
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, err := GetDatabaseSize(tt.dbPath)

			if (err != nil) != tt.expectError {
				t.Errorf("GetDatabaseSize(%q) error = %v, expectError = %v", tt.dbPath, err, tt.expectError)
				return
			}

			if !tt.expectError {
				if tt.expectedSize >= 0 && size != tt.expectedSize {
					t.Errorf("GetDatabaseSize(%q) = %d, expected %d", tt.dbPath, size, tt.expectedSize)
				}
				if size < 0 {
					t.Errorf("GetDatabaseSize(%q) = %d, should not be negative", tt.dbPath, size)
				}
			}
		})
	}
}

func TestBackupDatabase(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "db_backup_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test database file
	testData := "test database content"
	dbFile := filepath.Join(tempDir, "test.db")
	if err := os.WriteFile(dbFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	tests := []struct {
		name        string
		dbPath      string
		expectError bool
	}{
		{
			name:        "backup existing database",
			dbPath:      dbFile,
			expectError: false,
		},
		{
			name:        "backup non-existing database",
			dbPath:      filepath.Join(tempDir, "nonexistent.db"),
			expectError: true,
		},
		{
			name:        "empty path",
			dbPath:      "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := BackupDatabase(tt.dbPath)

			if (err != nil) != tt.expectError {
				t.Errorf("BackupDatabase(%q) error = %v, expectError = %v", tt.dbPath, err, tt.expectError)
				return
			}

			if !tt.expectError {
				// Verify backup file was created
				backupPattern := strings.TrimSuffix(tt.dbPath, filepath.Ext(tt.dbPath)) + "_backup_*" + filepath.Ext(tt.dbPath)
				matches, err := filepath.Glob(backupPattern)
				if err != nil {
					t.Errorf("Failed to search for backup files: %v", err)
					return
				}

				if len(matches) == 0 {
					t.Errorf("BackupDatabase(%q) did not create backup file", tt.dbPath)
					return
				}

				// Verify backup file content matches original
				backupFile := matches[0]
				backupData, err := os.ReadFile(backupFile)
				if err != nil {
					t.Errorf("Failed to read backup file: %v", err)
					return
				}

				if string(backupData) != testData {
					t.Errorf("Backup file content does not match original")
				}

				// Verify backup filename contains timestamp
				backupName := filepath.Base(backupFile)
				if !strings.Contains(backupName, "_backup_") {
					t.Errorf("Backup filename should contain '_backup_', got: %s", backupName)
				}

				// Clean up backup file
				os.Remove(backupFile)
			}
		})
	}
}

func TestBackupDatabase_TimestampFormat(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "db_backup_timestamp_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test database file
	dbFile := filepath.Join(tempDir, "timestamp_test.db")
	if err := os.WriteFile(dbFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	err = BackupDatabase(dbFile)
	if err != nil {
		t.Fatalf("BackupDatabase() error = %v", err)
	}

	// Find the backup file
	backupPattern := strings.TrimSuffix(dbFile, filepath.Ext(dbFile)) + "_backup_*" + filepath.Ext(dbFile)
	matches, err := filepath.Glob(backupPattern)
	if err != nil {
		t.Fatalf("Failed to search for backup files: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("Expected 1 backup file, found %d", len(matches))
	}

	backupFile := matches[0]
	backupName := filepath.Base(backupFile)

	// Extract timestamp from filename (format: timestamp_test_backup_20060102_150405.db)
	parts := strings.Split(backupName, "_backup_")
	if len(parts) != 2 {
		t.Fatalf("Unexpected backup filename format: %s", backupName)
	}

	timestampPart := strings.TrimSuffix(parts[1], filepath.Ext(backupFile))
	timestamp, err := time.Parse("20060102_150405", timestampPart)
	if err != nil {
		t.Errorf("Failed to parse timestamp from backup filename: %v", err)
		return
	}

	// Just verify the timestamp format is valid (we already parsed it successfully)
	// The parse succeeded, which means the format is correct
	if timestamp.IsZero() {
		t.Error("Backup timestamp should not be zero")
	}

	// Verify the timestamp looks reasonable (after year 2000, before year 2100)
	if timestamp.Year() < 2000 || timestamp.Year() > 2100 {
		t.Errorf("Backup timestamp year %d seems unreasonable", timestamp.Year())
	}

	// Clean up
	os.Remove(backupFile)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	expectedDriver := "sqlite"
	expectedTimeout := 30 * time.Second

	if config.Driver != expectedDriver {
		t.Errorf("DefaultConfig().Driver = %q, expected %q", config.Driver, expectedDriver)
	}

	if config.Timeout != expectedTimeout {
		t.Errorf("DefaultConfig().Timeout = %v, expected %v", config.Timeout, expectedTimeout)
	}

	// Path should be empty in default config
	if config.Path != "" {
		t.Errorf("DefaultConfig().Path = %q, expected empty string", config.Path)
	}
}
