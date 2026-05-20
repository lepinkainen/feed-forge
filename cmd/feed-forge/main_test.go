package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lepinkainen/feed-forge/internal/feissarimokat"
	"github.com/lepinkainen/feed-forge/internal/fingerpori"
	"github.com/lepinkainen/feed-forge/internal/hackernews"
	"github.com/lepinkainen/feed-forge/internal/oglaf"
	redditjson "github.com/lepinkainen/feed-forge/internal/reddit-json"
	"github.com/lepinkainen/feed-forge/internal/tildes"
	"github.com/lepinkainen/feed-forge/internal/youtube"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// writeTestConfig writes a YAML config file with all providers configured
// and returns the file path.
func writeTestConfig(t *testing.T) string {
	t.Helper()

	yaml := `
reddit:
  feed-id: "test-feed-id"
  username: "testuser"
  min-score: 75
  min-comments: 20
  outfile: reddit.xml
  interval: 30m
  proxy-url: "https://example.com/proxy"
  proxy-secret: "secret123"
  og-proxy-url: "https://example.com/og-proxy"

hackernews:
  min-points: 100
  limit: 50
  outfile: hackernews.xml
  interval: 20m

fingerpori:
  limit: 200
  outfile: fingerpori.xml
  interval: 12h

feissarimokat:
  outfile: feissarimokat.xml
  interval: 48h

oglaf:
  feed-url: "https://custom.oglaf.com/rss/"
  outfile: oglaf.xml
  interval: 36h

tildes:
  topic: "tech"
  topics: ["science", "games"]
  outfile: tildes.xml
  interval: 30m

youtube:
  feed-urls:
    - "https://www.youtube.com/feeds/videos.xml?channel_id=UC1111111111111111111111"
    - "https://www.youtube.com/feeds/videos.xml?channel_id=UC2222222222222222222222"
  channel-ids:
    - "UC3333333333333333333333"
  limit: 25
  include-shorts: false
  outfile: youtube.xml
  interval: 45m
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadProviderConfigFromYAML_Reddit(t *testing.T) {
	configPath := writeTestConfig(t)

	cfg := &redditjson.Config{}
	if err := loadProviderConfigFromYAML(configPath, "reddit", cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.FeedID != "test-feed-id" {
		t.Errorf("FeedID = %q, want %q", cfg.FeedID, "test-feed-id")
	}
	if cfg.Username != "testuser" {
		t.Errorf("Username = %q, want %q", cfg.Username, "testuser")
	}
	if cfg.MinScore != 75 {
		t.Errorf("MinScore = %d, want 75", cfg.MinScore)
	}
	if cfg.MinComments != 20 {
		t.Errorf("MinComments = %d, want 20", cfg.MinComments)
	}
	if cfg.ProxyURL != "https://example.com/proxy" {
		t.Errorf("ProxyURL = %q, want %q", cfg.ProxyURL, "https://example.com/proxy")
	}
	if cfg.ProxySecret != "secret123" {
		t.Errorf("ProxySecret = %q, want %q", cfg.ProxySecret, "secret123")
	}
	if cfg.OGProxyURL != "https://example.com/og-proxy" {
		t.Errorf("OGProxyURL = %q, want %q", cfg.OGProxyURL, "https://example.com/og-proxy")
	}

	gc := providers.GetGenerateConfig(cfg)
	if gc.Outfile != "reddit.xml" {
		t.Errorf("Outfile = %q, want %q", gc.Outfile, "reddit.xml")
	}
	if gc.Interval != "30m" {
		t.Errorf("Interval = %q, want %q", gc.Interval, "30m")
	}
}

func TestLoadProviderConfigFromYAML_HackerNews(t *testing.T) {
	configPath := writeTestConfig(t)

	cfg := &hackernews.Config{}
	if err := loadProviderConfigFromYAML(configPath, "hackernews", cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.MinPoints != 100 {
		t.Errorf("MinPoints = %d, want 100", cfg.MinPoints)
	}
	if cfg.Limit != 50 {
		t.Errorf("Limit = %d, want 50", cfg.Limit)
	}

	gc := providers.GetGenerateConfig(cfg)
	if gc.Outfile != "hackernews.xml" {
		t.Errorf("Outfile = %q, want %q", gc.Outfile, "hackernews.xml")
	}
	if gc.Interval != "20m" {
		t.Errorf("Interval = %q, want %q", gc.Interval, "20m")
	}
}

func TestLoadProviderConfigFromYAML_Fingerpori(t *testing.T) {
	configPath := writeTestConfig(t)

	cfg := &fingerpori.Config{}
	if err := loadProviderConfigFromYAML(configPath, "fingerpori", cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Limit != 200 {
		t.Errorf("Limit = %d, want 200", cfg.Limit)
	}

	gc := providers.GetGenerateConfig(cfg)
	if gc.Outfile != "fingerpori.xml" {
		t.Errorf("Outfile = %q, want %q", gc.Outfile, "fingerpori.xml")
	}
	if gc.Interval != "12h" {
		t.Errorf("Interval = %q, want %q", gc.Interval, "12h")
	}
}

func TestLoadProviderConfigFromYAML_Feissarimokat(t *testing.T) {
	configPath := writeTestConfig(t)

	cfg := &feissarimokat.Config{}
	if err := loadProviderConfigFromYAML(configPath, "feissarimokat", cfg); err != nil {
		t.Fatal(err)
	}

	gc := providers.GetGenerateConfig(cfg)
	if gc.Outfile != "feissarimokat.xml" {
		t.Errorf("Outfile = %q, want %q", gc.Outfile, "feissarimokat.xml")
	}
	if gc.Interval != "48h" {
		t.Errorf("Interval = %q, want %q", gc.Interval, "48h")
	}
}

func TestLoadProviderConfigFromYAML_Tildes(t *testing.T) {
	configPath := writeTestConfig(t)

	cfg := &tildes.Config{}
	if err := loadProviderConfigFromYAML(configPath, "tildes", cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Topic != "tech" {
		t.Errorf("Topic = %q, want tech", cfg.Topic)
	}
	if len(cfg.Topics) != 2 || cfg.Topics[0] != "science" || cfg.Topics[1] != "games" {
		t.Errorf("Topics = %#v, want [science games]", cfg.Topics)
	}

	gc := providers.GetGenerateConfig(cfg)
	if gc.Outfile != "tildes.xml" {
		t.Errorf("Outfile = %q, want %q", gc.Outfile, "tildes.xml")
	}
	if gc.Interval != "30m" {
		t.Errorf("Interval = %q, want %q", gc.Interval, "30m")
	}
}

func TestLoadProviderConfigFromYAML_YouTube(t *testing.T) {
	configPath := writeTestConfig(t)

	cfg := &youtube.Config{}
	if err := loadProviderConfigFromYAML(configPath, "youtube", cfg); err != nil {
		t.Fatal(err)
	}

	if len(cfg.FeedURLs) != 2 {
		t.Fatalf("FeedURLs len = %d, want 2", len(cfg.FeedURLs))
	}
	if cfg.FeedURLs[0] != "https://www.youtube.com/feeds/videos.xml?channel_id=UC1111111111111111111111" {
		t.Errorf("FeedURLs[0] = %q", cfg.FeedURLs[0])
	}
	if len(cfg.ChannelIDs) != 1 || cfg.ChannelIDs[0] != "UC3333333333333333333333" {
		t.Errorf("ChannelIDs = %#v", cfg.ChannelIDs)
	}
	if cfg.Limit != 25 {
		t.Errorf("Limit = %d, want 25", cfg.Limit)
	}
	if cfg.IncludeShorts {
		t.Errorf("IncludeShorts = true, want false")
	}

	gc := providers.GetGenerateConfig(cfg)
	if gc.Outfile != "youtube.xml" {
		t.Errorf("Outfile = %q, want %q", gc.Outfile, "youtube.xml")
	}
	if gc.Interval != "45m" {
		t.Errorf("Interval = %q, want %q", gc.Interval, "45m")
	}
}

func TestLoadProviderConfigFromYAML_Oglaf(t *testing.T) {
	configPath := writeTestConfig(t)

	cfg := &oglaf.Config{}
	if err := loadProviderConfigFromYAML(configPath, "oglaf", cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.FeedURL != "https://custom.oglaf.com/rss/" {
		t.Errorf("FeedURL = %q, want %q", cfg.FeedURL, "https://custom.oglaf.com/rss/")
	}

	gc := providers.GetGenerateConfig(cfg)
	if gc.Outfile != "oglaf.xml" {
		t.Errorf("Outfile = %q, want %q", gc.Outfile, "oglaf.xml")
	}
	if gc.Interval != "36h" {
		t.Errorf("Interval = %q, want %q", gc.Interval, "36h")
	}
}

// TestAllRegisteredProviders_HaveYAMLTags verifies that every registered
// provider's Config struct can be populated from YAML via the generate command
// path. This catches the Kong issue where only the active subcommand's config
// gets populated — the generate command must use loadProviderConfigFromYAML
// and all config fields must have proper yaml tags.
func TestAllRegisteredProviders_HaveYAMLTags(t *testing.T) {
	configPath := writeTestConfig(t)

	for _, name := range providers.DefaultRegistry.List() {
		t.Run(name, func(t *testing.T) {
			info, err := providers.DefaultRegistry.Get(name)
			if err != nil {
				t.Fatal(err)
			}

			if info.ConfigFactory == nil {
				t.Skip("provider has no ConfigFactory")
			}

			cfg := info.ConfigFactory()
			if err := loadProviderConfigFromYAML(configPath, name, cfg); err != nil {
				t.Fatal(err)
			}

			gc := providers.GetGenerateConfig(cfg)
			if gc.Outfile == "" {
				t.Errorf("GenerateConfig.Outfile is empty after YAML loading — yaml tags may be missing or the inline embed is broken")
			}
			if gc.Interval == "" {
				t.Errorf("GenerateConfig.Interval is empty after YAML loading — yaml tags may be missing or the inline embed is broken")
			}
		})
	}
}
