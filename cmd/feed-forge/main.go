// Package main provides the CLI entry point for feed-forge.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
	"gopkg.in/yaml.v3"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/preview"
	"github.com/lepinkainen/feed-forge/pkg/providers"

	// Import providers to trigger init() self-registration
	"github.com/lepinkainen/feed-forge/internal/feissarimokat"
	"github.com/lepinkainen/feed-forge/internal/fingerpori"
	"github.com/lepinkainen/feed-forge/internal/hackernews"
	"github.com/lepinkainen/feed-forge/internal/oglaf"
	redditjson "github.com/lepinkainen/feed-forge/internal/reddit-json"
)

// CLI structure
var CLI struct {
	Config    string `help:"Configuration file path" default:"config.yaml"`
	Debug     bool   `help:"Enable debug logging" default:"false"`
	OutputDir string `help:"Base output directory for all generated feeds" default:"" yaml:"output-dir"`
	CacheDir  string `help:"Directory for cache databases" default:"" yaml:"cache-dir"`

	Reddit struct {
		Outfile     string `help:"Output file path" short:"o" default:"reddit.xml"`
		MinScore    int    `help:"Minimum post score" default:"50"`
		MinComments int    `help:"Minimum comment count" default:"10"`
		FeedID      string `help:"Reddit feed ID"`
		Username    string `help:"Reddit username"`
		ProxyURL    string `help:"Proxy URL for Reddit API requests" yaml:"proxy-url"`
		ProxySecret string `help:"Shared secret for proxy authentication" yaml:"proxy-secret"`
		OGProxyURL  string `help:"Proxy URL for Reddit OpenGraph fetching" yaml:"og-proxy-url"`
		Interval    string `help:"Minimum time between regenerations" yaml:"interval"`
	} `cmd:"reddit" help:"Generate RSS feed from Reddit."`

	HackerNews struct {
		Outfile   string `help:"Output file path" short:"o" default:"hackernews.xml"`
		MinPoints int    `help:"Minimum points threshold" default:"50"`
		Limit     int    `help:"Maximum number of items" default:"30"`
		Interval  string `help:"Minimum time between regenerations" yaml:"interval"`
	} `cmd:"hackernews" help:"Generate RSS feed from Hacker News."`

	Fingerpori struct {
		Outfile  string `help:"Output file path" short:"o" default:"fingerpori.xml"`
		Limit    int    `help:"Maximum number of items" default:"100"`
		Interval string `help:"Minimum time between regenerations" yaml:"interval"`
	} `cmd:"fingerpori" help:"Generate RSS feed from Fingerpori comics."`

	Feissarimokat struct {
		Outfile  string `help:"Output file path" short:"o" default:"feissarimokat.xml"`
		Interval string `help:"Minimum time between regenerations" yaml:"interval"`
	} `cmd:"feissarimokat" help:"Generate RSS feed from Feissarimokat comics."`

	Preview struct {
		Provider string `arg:"" name:"provider" help:"Provider name (e.g. reddit, hacker-news, fingerpori, oglaf, feissarimokat)."`
		Limit    int    `help:"Maximum number of items to fetch (0 = provider default)." default:"0"`
		Index    int    `help:"Output XML for specific item index (0-based) to stdout" default:"-1"`
	} `cmd:"preview" help:"Preview feed items interactively for any registered provider."`
	Oglaf struct {
		Outfile  string `help:"Output file path" short:"o" default:"oglaf.xml"`
		FeedURL  string `help:"Oglaf RSS feed URL" default:"https://www.oglaf.com/feeds/rss/"`
		Interval string `help:"Minimum time between regenerations" yaml:"interval"`
	} `cmd:"oglaf" help:"Generate RSS feed from Oglaf comics."`

	Generate struct{} `cmd:"generate" help:"Generate feeds for all configured providers."`
}

func resolveConfigPath(args []string) string {
	for i := range args {
		arg := args[i]
		switch {
		case arg == "--config" && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(arg, "--config="):
			return strings.TrimPrefix(arg, "--config=")
		}
	}

	return findConfigFile()
}

func findConfigFile() string {
	const configFile = "config.yaml"

	// 1. XDG config dir
	xdgDir := os.Getenv("XDG_CONFIG_HOME")
	if xdgDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			xdgDir = filepath.Join(home, ".config")
		}
	}
	if xdgDir != "" {
		p := filepath.Join(xdgDir, "feed-forge", configFile)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// 2. Next to the executable
	if exePath, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exePath), configFile)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// 3. Current directory (fallback)
	return configFile
}

func resolveOutfile(outfile string) string {
	if CLI.OutputDir != "" && !filepath.IsAbs(outfile) {
		return filepath.Join(CLI.OutputDir, outfile)
	}
	return outfile
}

// buildProviderConfig maps CLI struct values (populated by Kong from YAML + flags)
// to the provider-specific Config struct expected by the registry factory.
func buildProviderConfig(name string) any {
	switch name {
	case "reddit":
		return &redditjson.Config{
			GenerateConfig: providers.GenerateConfig{
				Outfile:  CLI.Reddit.Outfile,
				Interval: CLI.Reddit.Interval,
			},
			MinScore:    CLI.Reddit.MinScore,
			MinComments: CLI.Reddit.MinComments,
			FeedID:      CLI.Reddit.FeedID,
			Username:    CLI.Reddit.Username,
			ProxyURL:    CLI.Reddit.ProxyURL,
			ProxySecret: CLI.Reddit.ProxySecret,
			OGProxyURL:  CLI.Reddit.OGProxyURL,
		}
	case "hackernews":
		return &hackernews.Config{
			GenerateConfig: providers.GenerateConfig{
				Outfile:  CLI.HackerNews.Outfile,
				Interval: CLI.HackerNews.Interval,
			},
			MinPoints: CLI.HackerNews.MinPoints,
			Limit:     CLI.HackerNews.Limit,
		}
	case "fingerpori":
		return &fingerpori.Config{
			GenerateConfig: providers.GenerateConfig{
				Outfile:  CLI.Fingerpori.Outfile,
				Interval: CLI.Fingerpori.Interval,
			},
			Limit: CLI.Fingerpori.Limit,
		}
	case "feissarimokat":
		return &feissarimokat.Config{
			GenerateConfig: providers.GenerateConfig{
				Outfile:  CLI.Feissarimokat.Outfile,
				Interval: CLI.Feissarimokat.Interval,
			},
		}
	case "oglaf":
		return &oglaf.Config{
			GenerateConfig: providers.GenerateConfig{
				Outfile:  CLI.Oglaf.Outfile,
				Interval: CLI.Oglaf.Interval,
			},
			FeedURL: CLI.Oglaf.FeedURL,
		}
	default:
		return nil
	}
}

func previewFeed(providerName string, limit, index int, configPath string) error {
	info, err := providers.DefaultRegistry.Get(providerName)
	if err != nil {
		return err
	}
	if info.Preview == nil {
		return fmt.Errorf("provider %q does not expose preview metadata", providerName)
	}

	var providerConfig any
	if info.ConfigFactory != nil {
		providerConfig = info.ConfigFactory()
		if loadErr := loadProviderConfigFromYAML(configPath, providerName, providerConfig); loadErr != nil {
			return fmt.Errorf("failed loading provider config: %w", loadErr)
		}
	}

	provider, err := providers.DefaultRegistry.CreateProvider(providerName, providerConfig)
	if err != nil {
		return err
	}

	items, err := provider.FetchItems(limit)
	if err != nil {
		return err
	}

	feedConfig := feed.Config{
		Title:       info.Preview.FeedTitle,
		Link:        info.Preview.FeedLink,
		Description: info.Preview.Description,
		Author:      info.Preview.Author,
		ID:          info.Preview.FeedID,
	}

	if index >= 0 {
		if index >= len(items) {
			return fmt.Errorf("index out of range: index=%d total=%d", index, len(items))
		}
		xml := preview.FormatXMLItem(items[index], info.Preview.TemplateName, feedConfig)
		fmt.Println(xml)
		return nil
	}

	providerDisplay := info.Preview.ProviderName
	if providerDisplay == "" {
		providerDisplay = info.Name
	}

	return preview.Run(items, providerDisplay, info.Preview.TemplateName, feedConfig)
}

// loadProviderConfigFromYAML unmarshals a provider's YAML section directly into
// its Config struct. Used by the generate command where Kong doesn't populate
// command-level sub-structs.
func loadProviderConfigFromYAML(configPath, providerName string, target any) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var root map[string]yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}

	section, ok := root[providerName]
	if !ok {
		return nil
	}

	return section.Decode(target)
}

func configuredProviders(configPath string) ([]string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	var names []string
	for key := range root {
		if _, err := providers.DefaultRegistry.Get(key); err == nil {
			names = append(names, key)
		}
	}

	return names, nil
}

const defaultInterval = 15 * time.Minute

func parseInterval(s string) time.Duration {
	if s == "" {
		return defaultInterval
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultInterval
	}
	return d
}

// shouldSkipProvider checks if the output file is younger than the interval.
// Returns (skip, age) where age is the time since the file was last modified.
func shouldSkipProvider(outfile string, interval time.Duration) (bool, time.Duration) {
	info, err := os.Stat(outfile)
	if err != nil {
		return false, 0
	}
	age := time.Since(info.ModTime())
	return age < interval, age
}

func generateAll(configPath string) error {
	names, err := configuredProviders(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if len(names) == 0 {
		slog.Warn("No configured providers found in config file", "config", configPath)
		return nil
	}

	var failed int
	for _, name := range names {
		info, err := providers.DefaultRegistry.Get(name)
		if err != nil {
			slog.Error("Provider not found", "provider", name, "error", err)
			failed++
			continue
		}

		var providerConfig any
		if info.ConfigFactory != nil {
			providerConfig = info.ConfigFactory()
			if loadErr := loadProviderConfigFromYAML(configPath, name, providerConfig); loadErr != nil {
				slog.Error("Failed to load config", "provider", name, "error", loadErr)
				failed++
				continue
			}
		}

		gc := providers.GetGenerateConfig(providerConfig)

		outfile := gc.Outfile
		if outfile == "" {
			outfile = name + ".xml"
		}
		outfile = resolveOutfile(outfile)

		interval := parseInterval(gc.Interval)
		if skip, age := shouldSkipProvider(outfile, interval); skip {
			slog.Info("Skipping provider", "provider", name, "age", age.Truncate(time.Second), "interval", interval)
			continue
		}

		provider, err := providers.DefaultRegistry.CreateProvider(name, providerConfig)
		if err != nil {
			slog.Error("Failed to create provider", "provider", name, "error", err)
			failed++
			continue
		}

		slog.Info("Generating feed", "provider", name, "outfile", outfile)
		if err := provider.GenerateFeed(outfile, false); err != nil {
			slog.Error("Failed to generate feed", "provider", name, "error", err)
			failed++
			continue
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d provider(s) failed", failed)
	}

	return nil
}

func main() {
	configPath := resolveConfigPath(os.Args[1:])

	// Parse CLI with Kong YAML configuration file loading
	ctx := kong.Parse(&CLI,
		kong.Name("feed-forge"),
		kong.Description("A unified RSS feed generator with multiple provider support."),
		kong.UsageOnError(),
		kong.Configuration(kongyaml.Loader, configPath),
	)

	// Configure logging level based on debug flag
	if CLI.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	} else {
		slog.SetLogLoggerLevel(slog.LevelWarn)
	}

	slog.Debug("Using configuration file", "path", configPath)

	if CLI.CacheDir != "" {
		filesystem.SetCacheDir(CLI.CacheDir)
	}

	switch ctx.Command() {
	case "hackernews":
		slog.Debug("Generating Hacker News feed...")

		providerConfig := buildProviderConfig("hackernews")
		provider, err := providers.DefaultRegistry.CreateProvider("hackernews", providerConfig)
		if err != nil {
			slog.Error("Failed to create Hacker News provider", "error", err)
			os.Exit(1)
		}

		outfile := resolveOutfile(CLI.HackerNews.Outfile)
		if err := provider.GenerateFeed(outfile, false); err != nil {
			slog.Error("Failed to generate Hacker News feed", "output_file", outfile, "error", err)
			os.Exit(1)
		}

	case "reddit":
		slog.Debug("Generating Reddit feed...")

		if CLI.Reddit.FeedID == "" || CLI.Reddit.Username == "" {
			slog.Error("Reddit feed requires both feed_id and username to be set via CLI flags or config file")
			os.Exit(1)
		}

		providerConfig := buildProviderConfig("reddit")
		provider, err := providers.DefaultRegistry.CreateProvider("reddit", providerConfig)
		if err != nil {
			slog.Error("Failed to create Reddit provider", "error", err)
			os.Exit(1)
		}

		outfile := resolveOutfile(CLI.Reddit.Outfile)
		if err := provider.GenerateFeed(outfile, false); err != nil {
			slog.Error("Failed to generate Reddit feed", "output_file", outfile, "feed_id", CLI.Reddit.FeedID, "username", CLI.Reddit.Username, "error", err)
			os.Exit(1)
		}

	case "fingerpori":
		slog.Debug("Generating Fingerpori feed...")

		providerConfig := buildProviderConfig("fingerpori")
		provider, err := providers.DefaultRegistry.CreateProvider("fingerpori", providerConfig)
		if err != nil {
			slog.Error("Failed to create Fingerpori provider", "error", err)
			os.Exit(1)
		}

		outfile := resolveOutfile(CLI.Fingerpori.Outfile)
		if err := provider.GenerateFeed(outfile, false); err != nil {
			slog.Error("Failed to generate Fingerpori feed", "output_file", outfile, "error", err)
			os.Exit(1)
		}

	case "feissarimokat":
		slog.Debug("Generating Feissarimokat feed...")

		providerConfig := buildProviderConfig("feissarimokat")
		provider, err := providers.DefaultRegistry.CreateProvider("feissarimokat", providerConfig)
		if err != nil {
			slog.Error("Failed to create Feissarimokat provider", "error", err)
			os.Exit(1)
		}

		outfile := resolveOutfile(CLI.Feissarimokat.Outfile)
		if err := provider.GenerateFeed(outfile, false); err != nil {
			slog.Error("Failed to generate Feissarimokat feed", "error", err)
			os.Exit(1)
		}

	case "preview <provider>":
		slog.Debug("Previewing provider feed...", "provider", CLI.Preview.Provider)

		if err := previewFeed(CLI.Preview.Provider, CLI.Preview.Limit, CLI.Preview.Index, configPath); err != nil {
			slog.Error("Preview failed", "provider", CLI.Preview.Provider, "error", err)
			os.Exit(1)
		}

	case "oglaf":
		slog.Debug("Generating Oglaf feed...")

		providerConfig := buildProviderConfig("oglaf")
		provider, err := providers.DefaultRegistry.CreateProvider("oglaf", providerConfig)
		if err != nil {
			slog.Error("Failed to create Oglaf provider", "error", err)
			os.Exit(1)
		}

		outfile := resolveOutfile(CLI.Oglaf.Outfile)
		if err := provider.GenerateFeed(outfile, false); err != nil {
			slog.Error("Failed to generate Oglaf feed", "error", err)
			os.Exit(1)
		}

	case "generate":
		slog.Debug("Generating feeds for all configured providers...")

		if err := generateAll(configPath); err != nil {
			slog.Error("Failed to generate feeds", "error", err)
			os.Exit(1)
		}

	default:
		panic(ctx.Command())
	}
}
