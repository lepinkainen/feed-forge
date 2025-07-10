package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirectoryExists(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filesystem_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		setup       func() string // Returns the actual file path to test
		cleanup     func(string)  // Cleanup function
	}{
		{
			name:        "current directory",
			filePath:    "test.txt",
			expectError: false,
			setup:       func() string { return "test.txt" },
			cleanup:     func(string) {},
		},
		{
			name:        "relative path single directory",
			filePath:    "",
			expectError: false,
			setup: func() string {
				return filepath.Join("subdir", "test.txt")
			},
			cleanup: func(path string) {
				os.RemoveAll("subdir")
			},
		},
		{
			name:        "relative path nested directories",
			filePath:    "",
			expectError: false,
			setup: func() string {
				return filepath.Join("subdir", "nested", "deep", "test.txt")
			},
			cleanup: func(path string) {
				os.RemoveAll("subdir")
			},
		},
		{
			name:        "absolute path in temp directory",
			filePath:    "",
			expectError: false,
			setup: func() string {
				return filepath.Join(tempDir, "newdir", "test.txt")
			},
			cleanup: func(path string) {
				os.RemoveAll(filepath.Join(tempDir, "newdir"))
			},
		},
		{
			name:        "nested absolute path",
			filePath:    "",
			expectError: false,
			setup: func() string {
				return filepath.Join(tempDir, "level1", "level2", "level3", "test.txt")
			},
			cleanup: func(path string) {
				os.RemoveAll(filepath.Join(tempDir, "level1"))
			},
		},
		{
			name:        "directory already exists",
			filePath:    "",
			expectError: false,
			setup: func() string {
				existingDir := filepath.Join(tempDir, "existing")
				os.MkdirAll(existingDir, 0755)
				return filepath.Join(existingDir, "test.txt")
			},
			cleanup: func(path string) {
				os.RemoveAll(filepath.Join(tempDir, "existing"))
			},
		},
		{
			name:        "permission denied (read-only parent)",
			filePath:    "",
			expectError: true,
			setup: func() string {
				if os.Getuid() == 0 {
					// Skip this test when running as root
					t.Skip("Skipping permission test when running as root")
				}
				readOnlyDir := filepath.Join(tempDir, "readonly")
				os.MkdirAll(readOnlyDir, 0755)
				os.Chmod(readOnlyDir, 0444) // Read-only
				return filepath.Join(readOnlyDir, "subdir", "test.txt")
			},
			cleanup: func(path string) {
				readOnlyDir := filepath.Join(tempDir, "readonly")
				os.Chmod(readOnlyDir, 0755) // Restore permissions for cleanup
				os.RemoveAll(readOnlyDir)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test case
			testPath := tt.setup()
			if testPath == "" {
				testPath = tt.filePath
			}

			// Ensure cleanup happens even if test fails
			defer tt.cleanup(testPath)

			// Test the function
			err := EnsureDirectoryExists(testPath)

			// Check error expectation
			if (err != nil) != tt.expectError {
				t.Errorf("EnsureDirectoryExists(%q) error = %v, expectError = %v",
					testPath, err, tt.expectError)
				return
			}

			// If no error expected, verify directory was created
			if !tt.expectError {
				dir := filepath.Dir(testPath)
				if dir != "." {
					if _, err := os.Stat(dir); os.IsNotExist(err) {
						t.Errorf("EnsureDirectoryExists(%q) did not create directory %q",
							testPath, dir)
					}
				}
			}
		})
	}
}

func TestEnsureDirectoryExists_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		expectError bool
	}{
		{
			name:        "empty string",
			filePath:    "",
			expectError: false, // filepath.Dir("") returns "."
		},
		{
			name:        "just filename no extension",
			filePath:    "filename",
			expectError: false,
		},
		{
			name:        "path with spaces",
			filePath:    filepath.Join("dir with spaces", "file with spaces.txt"),
			expectError: false,
		},
		{
			name:        "path with unicode characters",
			filePath:    filepath.Join("测试目录", "测试文件.txt"),
			expectError: false,
		},
		{
			name:        "very long path",
			filePath:    filepath.Join("very", "long", "path", "with", "many", "segments", "that", "goes", "quite", "deep", "into", "the", "filesystem", "hierarchy", "test.txt"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cleanup function
			defer func() {
				if tt.filePath != "" && tt.filePath != "filename" {
					dir := filepath.Dir(tt.filePath)
					if dir != "." && dir != "" {
						// Get the top-level directory to remove
						parts := filepath.SplitList(filepath.ToSlash(dir))
						if len(parts) > 0 {
							os.RemoveAll(parts[0])
						} else {
							// For single directory
							topDir := filepath.Clean(dir)
							for filepath.Dir(topDir) != "." && filepath.Dir(topDir) != topDir {
								topDir = filepath.Dir(topDir)
							}
							if topDir != "." {
								os.RemoveAll(topDir)
							}
						}
					}
				}
			}()

			err := EnsureDirectoryExists(tt.filePath)

			if (err != nil) != tt.expectError {
				t.Errorf("EnsureDirectoryExists(%q) error = %v, expectError = %v",
					tt.filePath, err, tt.expectError)
			}
		})
	}
}

func TestEnsureDirectoryExists_FilePermissions(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filesystem_perm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test that created directories have correct permissions
	testPath := filepath.Join(tempDir, "testdir", "file.txt")

	err = EnsureDirectoryExists(testPath)
	if err != nil {
		t.Fatalf("EnsureDirectoryExists() error = %v", err)
	}

	// Check that directory was created with correct permissions
	dirPath := filepath.Dir(testPath)
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Failed to stat created directory: %v", err)
	}

	if !info.IsDir() {
		t.Error("Created path is not a directory")
	}

	// Check permissions (should be 0755)
	perm := info.Mode().Perm()
	expectedPerm := os.FileMode(0755)
	if perm != expectedPerm {
		t.Errorf("Directory permissions = %o, expected %o", perm, expectedPerm)
	}
}

func TestEnsureDirectoryExists_PathManipulation(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string // expected directory to be created
	}{
		{
			name:     "unix style path",
			filePath: "dir/subdir/file.txt",
			expected: filepath.Join("dir", "subdir"),
		},
		{
			name:     "clean path with extra slashes",
			filePath: "dir//subdir///file.txt",
			expected: filepath.Join("dir", "subdir"),
		},
		{
			name:     "path with dot segments",
			filePath: "dir/./subdir/../subdir/file.txt",
			expected: filepath.Join("dir", "subdir"),
		},
		{
			name:     "path ending with slash",
			filePath: "dir/subdir/",
			expected: filepath.Join("dir", "subdir"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				// Clean up created directories
				topDir := "dir"
				os.RemoveAll(topDir)
			}()

			err := EnsureDirectoryExists(tt.filePath)
			if err != nil {
				t.Errorf("EnsureDirectoryExists(%q) error = %v", tt.filePath, err)
				return
			}

			// Verify the expected directory exists
			if _, err := os.Stat(tt.expected); os.IsNotExist(err) {
				t.Errorf("Expected directory %q was not created", tt.expected)
			}
		})
	}
}
