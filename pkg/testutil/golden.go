// Package testutil provides golden file testing utilities.
package testutil

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// CompareGolden compares the actual output with the golden file content.
// If the -update flag is provided, it updates the golden file with the actual output.
func CompareGolden(t *testing.T, goldenPath string, actual string) {
	t.Helper()

	if *update {
		updateGoldenFile(t, goldenPath, actual)
		return
	}

	expected := readGoldenFile(t, goldenPath)
	if actual != expected {
		t.Errorf("Golden file mismatch for %s\nExpected:\n%s\nActual:\n%s", goldenPath, expected, actual)
	}
}

// CompareGoldenBytes compares the actual output with the golden file content using byte slices.
// If the -update flag is provided, it updates the golden file with the actual output.
func CompareGoldenBytes(t *testing.T, goldenPath string, actual []byte) {
	t.Helper()
	CompareGolden(t, goldenPath, string(actual))
}

// readGoldenFile reads the content of a golden file.
func readGoldenFile(t *testing.T, goldenPath string) string {
	t.Helper()

	content, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden file %s: %v", goldenPath, err)
	}
	return string(content)
}

// CompareGoldenSlice compares the actual string slice with the golden file content.
// The golden file should contain a JSON array of strings.
// If the -update flag is provided, it updates the golden file with the actual output.
func CompareGoldenSlice(t *testing.T, goldenPath string, actual []string) {
	t.Helper()

	if *update {
		updateGoldenSlice(t, goldenPath, actual)
		return
	}

	expected := readGoldenSlice(t, goldenPath)
	if !slicesEqual(actual, expected) {
		t.Errorf("Golden file mismatch for %s\nExpected: %v\nActual: %v", goldenPath, expected, actual)
	}
}

// readGoldenSlice reads a JSON array from a golden file and returns it as a string slice.
func readGoldenSlice(t *testing.T, goldenPath string) []string {
	t.Helper()

	content, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden file %s: %v", goldenPath, err)
	}

	var result []string
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("Failed to parse JSON from golden file %s: %v", goldenPath, err)
	}

	return result
}

// updateGoldenSlice updates the golden file with the actual string slice as JSON.
func updateGoldenSlice(t *testing.T, goldenPath string, actual []string) {
	t.Helper()

	// Ensure the directory exists
	dir := filepath.Dir(goldenPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	// Marshal the slice to JSON
	data, err := json.Marshal(actual)
	if err != nil {
		t.Fatalf("Failed to marshal slice to JSON: %v", err)
	}

	// Write the JSON to the golden file
	if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
		t.Fatalf("Failed to update golden file %s: %v", goldenPath, err)
	}
	t.Logf("Updated golden file: %s", goldenPath)
}

// slicesEqual compares two string slices for equality.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// updateGoldenFile updates the golden file with the actual output.
func updateGoldenFile(t *testing.T, goldenPath string, actual string) {
	t.Helper()

	// Ensure the directory exists
	dir := filepath.Dir(goldenPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	// Write the actual output to the golden file
	if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
		t.Fatalf("Failed to update golden file %s: %v", goldenPath, err)
	}
	t.Logf("Updated golden file: %s", goldenPath)
}
