package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareGoldenAndBytes(t *testing.T) {
	oldUpdate := *update
	*update = false
	defer func() { *update = oldUpdate }()

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.golden")
	if err := os.WriteFile(path, []byte("expected"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	CompareGolden(t, path, "expected")
	CompareGoldenBytes(t, path, []byte("expected"))
}

func TestCompareGoldenSlice(t *testing.T) {
	oldUpdate := *update
	*update = false
	defer func() { *update = oldUpdate }()

	dir := t.TempDir()
	path := filepath.Join(dir, "slice.golden")
	if err := os.WriteFile(path, []byte(`["a","b"]`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	CompareGoldenSlice(t, path, []string{"a", "b"})
}

func TestUpdateGoldenHelpers(t *testing.T) {
	oldUpdate := *update
	*update = true
	defer func() { *update = oldUpdate }()

	dir := t.TempDir()
	textPath := filepath.Join(dir, "nested", "sample.golden")
	CompareGolden(t, textPath, "new text")

	content, err := os.ReadFile(textPath)
	if err != nil {
		t.Fatalf("ReadFile(text) error = %v", err)
	}
	if string(content) != "new text" {
		t.Fatalf("updated text golden = %q", content)
	}

	slicePath := filepath.Join(dir, "nested", "slice.golden")
	CompareGoldenSlice(t, slicePath, []string{"x", "y"})
	content, err = os.ReadFile(slicePath)
	if err != nil {
		t.Fatalf("ReadFile(slice) error = %v", err)
	}
	if string(content) != `["x","y"]` {
		t.Fatalf("updated slice golden = %q", content)
	}
}

func TestSlicesEqual(t *testing.T) {
	if !slicesEqual([]string{"a", "b"}, []string{"a", "b"}) {
		t.Fatal("slicesEqual(equal) = false, want true")
	}
	if slicesEqual([]string{"a"}, []string{"b"}) {
		t.Fatal("slicesEqual(different) = true, want false")
	}
	if slicesEqual([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("slicesEqual(different lengths) = true, want false")
	}
}
