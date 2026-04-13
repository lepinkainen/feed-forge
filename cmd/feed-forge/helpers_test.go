package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/internal/feissarimokat"
	"github.com/lepinkainen/feed-forge/internal/fingerpori"
	"github.com/lepinkainen/feed-forge/internal/hackernews"
	"github.com/lepinkainen/feed-forge/internal/oglaf"
	redditjson "github.com/lepinkainen/feed-forge/internal/reddit-json"
)

func TestResolveConfigPath(t *testing.T) {
	if got := resolveConfigPath([]string{"--config", "/tmp/custom.yaml"}); got != "/tmp/custom.yaml" {
		t.Fatalf("resolveConfigPath(--config) = %q", got)
	}
	if got := resolveConfigPath([]string{"--config=/tmp/inline.yaml"}); got != "/tmp/inline.yaml" {
		t.Fatalf("resolveConfigPath(--config=) = %q", got)
	}
}

func TestFindConfigFile_XDGPreferred(t *testing.T) {
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() { _ = os.Setenv("XDG_CONFIG_HOME", oldXDG) })

	xdgDir := t.TempDir()
	configPath := filepath.Join(xdgDir, "feed-forge", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(configPath, []byte("reddit: {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Setenv("XDG_CONFIG_HOME", xdgDir); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}

	if got := findConfigFile(); got != configPath {
		t.Fatalf("findConfigFile() = %q, want %q", got, configPath)
	}
}

func TestFindConfigFile_FallsBackToCurrentDirectory(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
		_ = os.Setenv("XDG_CONFIG_HOME", oldXDG)
	})

	cwd := t.TempDir()
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	if err := os.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "missing-xdg")); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}

	if got := findConfigFile(); got != "config.yaml" {
		t.Fatalf("findConfigFile() = %q, want %q", got, "config.yaml")
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

func TestBuildProviderConfig(t *testing.T) {
	oldCLI := CLI
	t.Cleanup(func() { CLI = oldCLI })

	CLI.Reddit.Outfile = "reddit.xml"
	CLI.Reddit.Interval = "30m"
	CLI.Reddit.MinScore = 10
	CLI.Reddit.MinComments = 5
	CLI.Reddit.FeedID = "feed-id"
	CLI.Reddit.Username = "alice"
	CLI.Reddit.ProxyURL = "https://proxy.example"
	CLI.Reddit.ProxySecret = "secret"
	CLI.Reddit.OGProxyURL = "https://og.example"
	CLI.HackerNews.Outfile = "hackernews.xml"
	CLI.HackerNews.Interval = "15m"
	CLI.HackerNews.MinPoints = 42
	CLI.HackerNews.Limit = 9
	CLI.Fingerpori.Outfile = "fingerpori.xml"
	CLI.Fingerpori.Interval = "45m"
	CLI.Fingerpori.Limit = 3
	CLI.Feissarimokat.Outfile = "feissarimokat.xml"
	CLI.Feissarimokat.Interval = "1h"
	CLI.Oglaf.Outfile = "oglaf.xml"
	CLI.Oglaf.Interval = "2h"
	CLI.Oglaf.FeedURL = "https://example.com/oglaf.xml"

	redditCfg, ok := buildProviderConfig("reddit").(*redditjson.Config)
	if !ok {
		t.Fatalf("buildProviderConfig(reddit) type mismatch")
	}
	if redditCfg.MinScore != 10 || redditCfg.MinComments != 5 || redditCfg.Username != "alice" || redditCfg.OGProxyURL != "https://og.example" {
		t.Fatalf("unexpected reddit config: %#v", redditCfg)
	}

	hnCfg, ok := buildProviderConfig("hackernews").(*hackernews.Config)
	if !ok {
		t.Fatalf("buildProviderConfig(hackernews) type mismatch")
	}
	if hnCfg.MinPoints != 42 || hnCfg.Limit != 9 || hnCfg.Outfile != "hackernews.xml" {
		t.Fatalf("unexpected hackernews config: %#v", hnCfg)
	}

	fingerCfg, ok := buildProviderConfig("fingerpori").(*fingerpori.Config)
	if !ok {
		t.Fatalf("buildProviderConfig(fingerpori) type mismatch")
	}
	if fingerCfg.Limit != 3 || fingerCfg.Outfile != "fingerpori.xml" {
		t.Fatalf("unexpected fingerpori config: %#v", fingerCfg)
	}

	feissariCfg, ok := buildProviderConfig("feissarimokat").(*feissarimokat.Config)
	if !ok {
		t.Fatalf("buildProviderConfig(feissarimokat) type mismatch")
	}
	if feissariCfg.Outfile != "feissarimokat.xml" || feissariCfg.Interval != "1h" {
		t.Fatalf("unexpected feissarimokat config: %#v", feissariCfg)
	}

	oglafCfg, ok := buildProviderConfig("oglaf").(*oglaf.Config)
	if !ok {
		t.Fatalf("buildProviderConfig(oglaf) type mismatch")
	}
	if oglafCfg.Outfile != "oglaf.xml" || oglafCfg.Interval != "2h" || oglafCfg.FeedURL != "https://example.com/oglaf.xml" {
		t.Fatalf("unexpected oglaf config: %#v", oglafCfg)
	}

	if got := buildProviderConfig("unknown"); got != nil {
		t.Fatalf("buildProviderConfig(unknown) = %#v, want nil", got)
	}
}

func TestGenerateFeedIndex_SkipsWithoutOutputDir(t *testing.T) {
	old := CLI.OutputDir
	t.Cleanup(func() { CLI.OutputDir = old })
	CLI.OutputDir = ""

	if err := generateFeedIndex([]feedResult{{Provider: "reddit", Filename: "reddit.xml", Status: "generated"}}); err != nil {
		t.Fatalf("generateFeedIndex() error = %v", err)
	}
}

func TestGenerateFeedIndex_WritesSortedNonFailedFeeds(t *testing.T) {
	old := CLI.OutputDir
	t.Cleanup(func() { CLI.OutputDir = old })
	CLI.OutputDir = t.TempDir()

	results := []feedResult{
		{Provider: "reddit", Filename: "reddit.xml", Status: "generated"},
		{Provider: "oglaf", Filename: "oglaf.xml", Status: "skipped"},
		{Provider: "broken", Filename: "broken.xml", Status: "failed"},
		{Provider: "hackernews", Filename: "hackernews.xml", Status: "generated"},
	}

	if err := generateFeedIndex(results); err != nil {
		t.Fatalf("generateFeedIndex() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(CLI.OutputDir, "index.html"))
	if err != nil {
		t.Fatalf("ReadFile(index.html) error = %v", err)
	}
	body := string(content)

	for _, want := range []string{"hackernews.xml", "oglaf.xml", "reddit.xml"} {
		if !strings.Contains(body, want) {
			t.Fatalf("index.html missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "broken.xml") {
		t.Fatalf("index.html should not include failed feed:\n%s", body)
	}

	oglafIndex := strings.Index(body, "oglaf")
	redditIndex := strings.Index(body, "reddit")
	hnIndex := strings.Index(body, "hackernews")
	if !(hnIndex >= 0 && oglafIndex >= 0 && redditIndex >= 0 && hnIndex < oglafIndex && oglafIndex < redditIndex) {
		t.Fatalf("providers not sorted in index.html:\n%s", body)
	}
}
