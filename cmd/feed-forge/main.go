// Package main provides the CLI entry point for feed-forge.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
	"gopkg.in/yaml.v3"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/preview"
	"github.com/lepinkainen/feed-forge/pkg/providers"

	// Import providers to trigger init() self-registration
	"github.com/lepinkainen/feed-forge/internal/fingerpori"
	"github.com/lepinkainen/feed-forge/internal/hackernews"
	"github.com/lepinkainen/feed-forge/internal/oglaf"
	redditjson "github.com/lepinkainen/feed-forge/internal/reddit-json"
)

// CLI structure
var CLI struct {
	Config string `help:"Configuration file path" default:"config.yaml"`
	Debug  bool   `help:"Enable debug logging" default:"false"`

	Reddit struct {
		Outfile     string `help:"Output file path" short:"o" default:"reddit.xml"`
		MinScore    int    `help:"Minimum post score" default:"50"`
		MinComments int    `help:"Minimum comment count" default:"10"`
		FeedID      string `help:"Reddit feed ID"`
		Username    string `help:"Reddit username"`
	} `cmd:"reddit" help:"Generate RSS feed from Reddit."`

	HackerNews struct {
		Outfile   string `help:"Output file path" short:"o" default:"hackernews.xml"`
		MinPoints int    `help:"Minimum points threshold" default:"50"`
		Limit     int    `help:"Maximum number of items" default:"30"`
	} `cmd:"hacker-news" help:"Generate RSS feed from Hacker News."`

	Fingerpori struct {
		Outfile string `help:"Output file path" short:"o" default:"fingerpori.xml"`
		Limit   int    `help:"Maximum number of items" default:"100"`
	} `cmd:"fingerpori" help:"Generate RSS feed from Fingerpori comics."`

	Preview struct {
		Provider string `arg:"" name:"provider" help:"Provider name (e.g. reddit, hacker-news, fingerpori, oglaf)."`
		Limit    int    `help:"Maximum number of items to fetch (0 = provider default)." default:"0"`
		Index    int    `help:"Output XML for specific item index (0-based) to stdout" default:"-1"`
	} `cmd:"preview" help:"Preview feed items interactively for any registered provider."`
	Oglaf struct {
		Outfile string `help:"Output file path" short:"o" default:"oglaf.xml"`
		FeedURL string `help:"Oglaf RSS feed URL" default:"https://www.oglaf.com/feeds/rss/"`
	} `cmd:"oglaf" help:"Generate RSS feed from Oglaf comics."`
}

func resolveConfigPath(args []string, defaultPath string) string {
	for i := range args {
		arg := args[i]
		switch {
		case arg == "--config" && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(arg, "--config="):
			return strings.TrimPrefix(arg, "--config=")
		}
	}

	return defaultPath
}

func toKebabCase(name string) string {
	var b strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteRune('-')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeKey(key string) string {
	key = strings.ToLower(key)
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, " ", "")
	return key
}

func loadProviderConfig(configPath, providerName string, target any) error {
	if target == nil {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var root map[string]any
	err = yaml.Unmarshal(data, &root)
	if err != nil {
		return err
	}

	sectionRaw, ok := root[providerName]
	if !ok {
		return nil
	}

	section, ok := sectionRaw.(map[string]any)
	if !ok {
		return nil
	}

	sectionNormalized := make(map[string]any, len(section))
	for k, v := range section {
		sectionNormalized[normalizeKey(k)] = v
	}

	t := reflect.TypeOf(target)
	if t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be pointer to struct, got %T", target)
	}

	mapped := make(map[string]any)
	structType := t.Elem()
	for i := range structType.NumField() {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}

		for _, key := range []string{field.Name, strings.ToLower(field.Name), toKebabCase(field.Name)} {
			if value, ok := sectionNormalized[normalizeKey(key)]; ok {
				mapped[field.Name] = value
				break
			}
		}
	}

	if len(mapped) == 0 {
		return nil
	}

	encoded, err := yaml.Marshal(mapped)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(encoded, target)
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
		err = loadProviderConfig(configPath, providerName, providerConfig)
		if err != nil {
			return fmt.Errorf("failed loading provider config: %w", err)
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

	type sortableItem struct {
		item providers.FeedItem
		time int64
	}

	sortable := make([]sortableItem, len(items))
	for i, item := range items {
		sortable[i] = sortableItem{item: item, time: item.CreatedAt().UnixNano()}
	}

	// Preview should always show newest items first.
	sort.SliceStable(sortable, func(i, j int) bool {
		return sortable[i].time > sortable[j].time
	})

	for i, sorted := range sortable {
		items[i] = sorted.item
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

func main() {
	configPath := resolveConfigPath(os.Args[1:], "config.yaml")

	// Parse CLI with Kong YAML configuration file loading
	ctx := kong.Parse(&CLI,
		kong.Configuration(kongyaml.Loader, configPath, "config.yaml", "~/.feed-forge/config.yaml"),
	)

	// Configure logging level based on debug flag
	if CLI.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	} else {
		slog.SetLogLoggerLevel(slog.LevelWarn)
	}

	var provider providers.FeedProvider

	switch ctx.Command() {
	case "hacker-news":
		slog.Debug("Generating Hacker News feed...")

		// Create provider config from CLI arguments
		providerConfig := &hackernews.Config{
			MinPoints: CLI.HackerNews.MinPoints,
			Limit:     CLI.HackerNews.Limit,
		}

		// Create provider using registry
		var err error
		provider, err = providers.DefaultRegistry.CreateProvider("hacker-news", providerConfig)
		if err != nil {
			slog.Error("Failed to create Hacker News provider", "error", err)
			os.Exit(1)
		}

		if err := provider.GenerateFeed(CLI.HackerNews.Outfile, false); err != nil {
			slog.Error("Failed to generate Hacker News feed", "output_file", CLI.HackerNews.Outfile, "error", err)
			os.Exit(1)
		}

	case "reddit":
		slog.Debug("Generating Reddit feed...")

		// Validate required parameters
		if CLI.Reddit.FeedID == "" || CLI.Reddit.Username == "" {
			slog.Error("Reddit feed requires both feed_id and username to be set via CLI flags or config file")
			os.Exit(1)
		}

		// Create provider config from CLI arguments
		providerConfig := &redditjson.Config{
			MinScore:    CLI.Reddit.MinScore,
			MinComments: CLI.Reddit.MinComments,
			FeedID:      CLI.Reddit.FeedID,
			Username:    CLI.Reddit.Username,
		}

		// Create provider using registry
		var err error
		provider, err = providers.DefaultRegistry.CreateProvider("reddit", providerConfig)
		if err != nil {
			slog.Error("Failed to create Reddit provider", "error", err)
			os.Exit(1)
		}

		if err := provider.GenerateFeed(CLI.Reddit.Outfile, false); err != nil {
			slog.Error("Failed to generate Reddit feed", "output_file", CLI.Reddit.Outfile, "feed_id", CLI.Reddit.FeedID, "username", CLI.Reddit.Username, "error", err)
			os.Exit(1)
		}

	case "fingerpori":
		slog.Debug("Generating Fingerpori feed...")

		// Create provider config from CLI arguments
		providerConfig := &fingerpori.Config{
			Limit: CLI.Fingerpori.Limit,
		}

		// Create provider using registry
		var err error
		provider, err = providers.DefaultRegistry.CreateProvider("fingerpori", providerConfig)
		if err != nil {
			slog.Error("Failed to create Fingerpori provider", "error", err)
			os.Exit(1)
		}

		if err := provider.GenerateFeed(CLI.Fingerpori.Outfile, false); err != nil {
			slog.Error("Failed to generate Fingerpori feed", "output_file", CLI.Fingerpori.Outfile, "error", err)
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

		// Use CLI flag or default feed URL
		feedURL := CLI.Oglaf.FeedURL

		// Create Oglaf provider
		provider = oglaf.NewOglafProvider(feedURL)
		if provider == nil {
			slog.Error("Failed to create Oglaf provider")
			os.Exit(1)
		}

		if err := provider.GenerateFeed(CLI.Oglaf.Outfile, false); err != nil {
			slog.Error("Failed to generate Oglaf feed", "error", err)
			os.Exit(1)
		}

	default:
		panic(ctx.Command())
	}
}
