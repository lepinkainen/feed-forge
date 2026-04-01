package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestResolveConfigPath(t *testing.T) {
	if got := resolveConfigPath([]string{"--config", "/tmp/custom.yaml"}); got != "/tmp/custom.yaml" {
		t.Fatalf("resolveConfigPath(--config) = %q", got)
	}
	if got := resolveConfigPath([]string{"--config=/tmp/inline.yaml"}); got != "/tmp/inline.yaml" {
		t.Fatalf("resolveConfigPath(--config=) = %q", got)
	}
}

func TestResolveOutfile(t *testing.T) {
	old := CLI.OutputDir
	t.Cleanup(func() { CLI.OutputDir = old })

	CLI.OutputDir = "/tmp/output"
	if got := resolveOutfile("reddit.xml"); got != filepath.Join("/tmp/output", "reddit.xml") {
		t.Fatalf("resolveOutfile(relative) = %q", got)
	}
	if got := resolveOutfile("/var/tmp/reddit.xml"); got != "/var/tmp/reddit.xml" {
		t.Fatalf("resolveOutfile(absolute) = %q", got)
	}
}

func TestConfiguredProviders(t *testing.T) {
	config := []byte(`
reddit:
  outfile: reddit.xml
unknown:
  enabled: true
hackernews:
  outfile: hackernews.xml
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, config, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := configuredProviders(path)
	if err != nil {
		t.Fatalf("configuredProviders() error = %v", err)
	}
	sort.Strings(got)
	want := []string{"hackernews", "reddit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configuredProviders() = %v, want %v", got, want)
	}
}

func TestParseInterval(t *testing.T) {
	if got := parseInterval("30m"); got != 30*time.Minute {
		t.Fatalf("parseInterval(valid) = %v", got)
	}
	if got := parseInterval(""); got != defaultInterval {
		t.Fatalf("parseInterval(empty) = %v", got)
	}
	if got := parseInterval("not-a-duration"); got != defaultInterval {
		t.Fatalf("parseInterval(invalid) = %v", got)
	}
}

func TestShouldSkipProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reddit.xml")
	if skip, age := shouldSkipProvider(path, time.Hour); skip || age != 0 {
		t.Fatalf("shouldSkipProvider(missing) = (%v, %v)", skip, age)
	}

	if err := os.WriteFile(path, []byte("feed"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if skip, _ := shouldSkipProvider(path, time.Hour); !skip {
		t.Fatal("shouldSkipProvider(recent) = false, want true")
	}

	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}
	if skip, _ := shouldSkipProvider(path, time.Hour); skip {
		t.Fatal("shouldSkipProvider(old) = true, want false")
	}
}
