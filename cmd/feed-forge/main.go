// Package main provides the CLI entry point for feed-forge.
package main

import (
	xmlenc "encoding/xml"
	"fmt"
	"html/template"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
	"gopkg.in/yaml.v3"

	apipkg "github.com/lepinkainen/feed-forge/pkg/api"
	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/llm"
	"github.com/lepinkainen/feed-forge/pkg/notifications"
	"github.com/lepinkainen/feed-forge/pkg/preview"
	"github.com/lepinkainen/feed-forge/pkg/providers"

	"github.com/lepinkainen/feed-forge/internal/bulletin"

	// Import providers to trigger init() self-registration
	"github.com/lepinkainen/feed-forge/internal/feissarimokat"
	"github.com/lepinkainen/feed-forge/internal/fingerpori"
	"github.com/lepinkainen/feed-forge/internal/hackernews"
	"github.com/lepinkainen/feed-forge/internal/oglaf"
	redditjson "github.com/lepinkainen/feed-forge/internal/reddit-json"
	"github.com/lepinkainen/feed-forge/internal/tildes"
	"github.com/lepinkainen/feed-forge/internal/youtube"
)

// CLI structure
var CLI struct {
	Config            string `help:"Configuration file path" default:"config.yaml"`
	Debug             bool   `help:"Enable debug logging" default:"false"`
	OutputDir         string `help:"Base output directory for all generated feeds" default:"" yaml:"output-dir"`
	FeedBaseURL       string `help:"Public base URL for generated feeds and OPML" default:"https://endymion.xyz/rss/" yaml:"feed-base-url"`
	CacheDir          string `help:"Directory for cache databases" default:"" yaml:"cache-dir"`
	DiscordWebhookURL string `help:"Discord webhook URL for failure notifications" default:"" yaml:"discord-webhook-url"`

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
		Provider string `arg:"" name:"provider" help:"Provider name (e.g. reddit, hacker-news, fingerpori, oglaf, feissarimokat, tildes, youtube)."`
		Limit    int    `help:"Maximum number of items to fetch (0 = provider default)." default:"0"`
		Index    int    `help:"Output XML for specific item index (0-based) to stdout" default:"-1"`
	} `cmd:"preview" help:"Preview feed items interactively for any registered provider."`
	Oglaf struct {
		Outfile  string `help:"Output file path" short:"o" default:"oglaf.xml"`
		FeedURL  string `help:"Oglaf RSS feed URL" default:"https://www.oglaf.com/feeds/rss/"`
		Interval string `help:"Minimum time between regenerations" yaml:"interval"`
	} `cmd:"oglaf" help:"Generate RSS feed from Oglaf comics."`

	Tildes struct {
		Outfile  string   `help:"Output file path" short:"o" default:"tildes.xml"`
		Topic    string   `help:"Tildes topic name (without leading ~), e.g. tech" default:"tech" yaml:"topic"`
		Topics   []string `help:"Tildes topic names (without leading ~), repeat for multiple groups" yaml:"topics"`
		Interval string   `help:"Minimum time between regenerations" yaml:"interval"`
	} `cmd:"tildes" help:"Generate RSS feed from Tildes group Atom feeds."`

	YouTube struct {
		Outfile       string   `help:"Output file path" short:"o" default:"youtube.xml"`
		FeedURL       string   `help:"YouTube Atom feed URL" yaml:"feed-url"`
		FeedURLs      []string `name:"feed-urls" help:"YouTube Atom feed URLs, repeat for multiple channels" yaml:"feed-urls"`
		ChannelIDs    []string `name:"channel-ids" help:"YouTube channel IDs, repeat for multiple channels" yaml:"channel-ids"`
		Limit         int      `help:"Maximum number of items" default:"30" yaml:"limit"`
		IncludeShorts bool     `help:"Include YouTube Shorts" default:"false" yaml:"include-shorts"`
		Interval      string   `help:"Minimum time between regenerations" yaml:"interval"`
	} `cmd:"youtube" name:"youtube" help:"Generate RSS feed from YouTube channel Atom feeds."`

	YouTubeRSS struct {
		URL string `arg:"" name:"url" help:"YouTube channel URL, e.g. https://www.youtube.com/@Taskmaster"`
	} `cmd:"youtube-rss" name:"youtube-rss" help:"Print the RSS feed URL advertised by a YouTube channel page."`

	Generate struct{} `cmd:"generate" help:"Generate feeds for all configured providers."`

	BulletinFetch struct{} `cmd:"bulletin-fetch" name:"bulletin-fetch" help:"Poll bulletin source feeds, extract full text, and store new items."`

	BulletinPublish struct {
		Outfile string `help:"Output file path" short:"o" default:"bulletin.xml"`
		Slot    string `help:"Bulletin slot label (default: derived from time of day)"`
	} `cmd:"bulletin-publish" name:"bulletin-publish" help:"Deduplicate and summarise stored items into a digest Atom feed."`

	BulletinSummarize struct{} `cmd:"bulletin-summarize" name:"bulletin-summarize" help:"Debug: print the digest for current unpublished items to stdout without writing or marking anything."`
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
	case "tildes":
		return &tildes.Config{
			GenerateConfig: providers.GenerateConfig{
				Outfile:  CLI.Tildes.Outfile,
				Interval: CLI.Tildes.Interval,
			},
			Topic:  CLI.Tildes.Topic,
			Topics: CLI.Tildes.Topics,
		}
	case "youtube":
		return &youtube.Config{
			GenerateConfig: providers.GenerateConfig{
				Outfile:  CLI.YouTube.Outfile,
				Interval: CLI.YouTube.Interval,
			},
			FeedURL:       CLI.YouTube.FeedURL,
			FeedURLs:      CLI.YouTube.FeedURLs,
			ChannelIDs:    CLI.YouTube.ChannelIDs,
			Limit:         CLI.YouTube.Limit,
			IncludeShorts: CLI.YouTube.IncludeShorts,
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

	feedConfig := feed.Config(info.Preview.Config)

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

type feedResult struct {
	Provider string
	FeedName string
	Filename string        // e.g. "reddit.xml"
	Status   string        // "generated", "skipped", "failed"
	Err      error         // non-nil when Status == "failed"
	Duration time.Duration // time spent in GenerateFeed
}

type opmlDocument struct {
	XMLName xmlenc.Name `xml:"opml"`
	Version string      `xml:"version,attr"`
	Head    opmlHead    `xml:"head"`
	Body    opmlBody    `xml:"body"`
}

type opmlHead struct {
	Title string `xml:"title"`
}

type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	Text   string `xml:"text,attr"`
	Title  string `xml:"title,attr"`
	Type   string `xml:"type,attr"`
	XMLURL string `xml:"xmlUrl,attr"`
}

const opmlFilename = "feeds.opml"

func generateAll(configPath string) error {
	names, err := configuredProviders(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if len(names) == 0 {
		slog.Warn("No configured providers found in config file", "config", configPath)
		return nil
	}

	runStart := time.Now()

	var (
		wg      sync.WaitGroup
		results = make([]feedResult, len(names))
	)

	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			results[i] = generateProvider(configPath, name)
		}(i, name)
	}

	wg.Wait()

	notifyFailures(results, runStart)

	if err := generateFeedIndex(results); err != nil {
		slog.Error("Failed to generate feed index", "error", err)
	}

	var (
		transient []string
		hardCount int
	)
	for _, r := range results {
		if r.Status != "failed" {
			continue
		}
		if code, ok := apipkg.UpstreamStatusCode(r.Err); ok && code >= 400 && code < 600 {
			transient = append(transient, fmt.Sprintf("%s=%d", r.Provider, code))
		} else {
			hardCount++
		}
	}

	// When the Discord webhook is enabled it already reports failures, so skip
	// the stderr warning to avoid a duplicate notification via cron + mailrise.
	if len(transient) > 0 && CLI.DiscordWebhookURL == "" {
		slog.Warn("Transient upstream failures", "providers", strings.Join(transient, ","))
	}

	if hardCount > 0 {
		return fmt.Errorf("%d provider(s) failed", hardCount)
	}

	return nil
}

// notifyFailures sends a Discord webhook summary when a webhook URL is
// configured and at least one provider failed.
func notifyFailures(results []feedResult, runStart time.Time) {
	if CLI.DiscordWebhookURL == "" {
		return
	}
	if !slices.ContainsFunc(results, func(r feedResult) bool { return r.Status == "failed" }) {
		return
	}

	notifyResults := make([]notifications.ProviderResult, len(results))
	for i, r := range results {
		notifyResults[i] = notifications.ProviderResult{
			Name:     r.Provider,
			FeedName: r.FeedName,
			Status:   r.Status,
			Err:      r.Err,
			Duration: r.Duration,
		}
	}
	if err := notifications.SendDiscordWebhook(CLI.DiscordWebhookURL, notifications.RunResult{
		Providers: notifyResults,
		StartTime: runStart,
	}); err != nil {
		slog.Warn("Failed to send Discord notification", "error", err)
	}
}

func generateProvider(configPath, name string) feedResult {
	result := feedResult{Provider: name, Status: "failed"}

	info, err := providers.DefaultRegistry.Get(name)
	if err != nil {
		slog.Error("Provider not found", "provider", name, "error", err)
		return result
	}
	result.FeedName = feedName(info, name)

	var providerConfig any
	if info.ConfigFactory != nil {
		providerConfig = info.ConfigFactory()
		if loadErr := loadProviderConfigFromYAML(configPath, name, providerConfig); loadErr != nil {
			slog.Error("Failed to load provider config", "provider", name, "error", loadErr)
			return result
		}
	}

	gc := providers.GetGenerateConfig(providerConfig)

	outfile := gc.Outfile
	if outfile == "" {
		outfile = name + ".xml"
	}
	result.Filename = outfile
	outfile = resolveOutfile(outfile)

	interval := parseInterval(gc.Interval)
	if skip, age := shouldSkipProvider(outfile, interval); skip {
		slog.Info("Skipping provider", "provider", name, "age", age.Truncate(time.Second), "interval", interval)
		result.Status = "skipped"
		return result
	}

	provider, err := providers.DefaultRegistry.CreateProvider(name, providerConfig)
	if err != nil {
		slog.Error("Failed to create provider", "provider", name, "error", err)
		result.Err = err
		return result
	}

	if closer, ok := provider.(interface{ Close() error }); ok {
		defer func() {
			if err := closer.Close(); err != nil {
				slog.Error("Failed to close provider", "provider", name, "error", err)
			}
		}()
	}

	slog.Info("Generating feed", "provider", name, "outfile", outfile)
	start := time.Now()
	if err := provider.GenerateFeed(outfile); err != nil {
		result.Err = err
		result.Duration = time.Since(start)
		if !apipkg.IsTransientUpstreamError(err) {
			slog.Error("Failed to generate feed", "provider", name, "error", err)
		}
		return result
	}
	result.Duration = time.Since(start)

	result.Status = "generated"
	return result
}

func generateFeedIndex(results []feedResult) error {
	if CLI.OutputDir == "" {
		slog.Info("Skipping feed index generation: output-dir not configured")
		return nil
	}

	var feeds []feedResult
	for _, r := range results {
		if r.Status != "failed" && r.Filename != "" {
			feeds = append(feeds, r)
		}
	}

	sort.Slice(feeds, func(i, j int) bool {
		return feeds[i].Provider < feeds[j].Provider
	})

	tmplContent, err := feed.ReadTemplateContent("feed-index.html.tmpl")
	if err != nil {
		return err
	}

	htmlTmpl, err := template.New("feed-index").Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("failed to parse index template: %w", err)
	}

	indexPath := filepath.Join(CLI.OutputDir, "index.html")
	file, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}
	defer func() { _ = file.Close() }()

	data := struct {
		Feeds        []feedResult
		OPMLFilename string
		BulletinLink string
	}{Feeds: feeds, OPMLFilename: opmlFilename, BulletinLink: bulletinIndexLink()}
	if err := htmlTmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute index template: %w", err)
	}

	if err := generateOPML(feeds); err != nil {
		return err
	}

	slog.Info("Generated feed index", "path", indexPath)
	return nil
}

// bulletinHTMLSubdir is the subdirectory under OutputDir where bulletin HTML
// pages are written.
const bulletinHTMLSubdir = "html"

// bulletinHTMLDir returns the directory for bulletin HTML pages
// (<output-dir>/html), or "" when no output directory is configured (HTML
// export disabled).
func bulletinHTMLDir() string {
	if CLI.OutputDir == "" {
		return ""
	}
	return filepath.Join(CLI.OutputDir, bulletinHTMLSubdir)
}

// bulletinIndexLink returns the relative href from the feed index (in OutputDir)
// to the latest bulletin HTML page (in OutputDir/html), or "" when HTML export
// is disabled or the page has not been published yet.
func bulletinIndexLink() string {
	dir := bulletinHTMLDir()
	if dir == "" {
		return ""
	}
	if _, err := os.Stat(filepath.Join(dir, bulletin.LatestPageName)); err != nil {
		return ""
	}
	return bulletinHTMLSubdir + "/" + bulletin.LatestPageName
}

func feedName(info *providers.ProviderInfo, fallback string) string {
	if info != nil && info.Preview != nil && info.Preview.ProviderName != "" {
		return info.Preview.ProviderName
	}
	if info != nil && info.Name != "" {
		return info.Name
	}
	return fallback
}

func generateOPML(feeds []feedResult) error {
	baseURL := CLI.FeedBaseURL
	if baseURL == "" {
		baseURL = "https://endymion.xyz/rss/"
	}

	outlines := make([]opmlOutline, 0, len(feeds))
	for _, f := range feeds {
		xmlURL, err := url.JoinPath(baseURL, f.Filename)
		if err != nil {
			return fmt.Errorf("build OPML URL for %s: %w", f.Filename, err)
		}
		name := f.FeedName
		if name == "" {
			name = f.Provider
		}
		outlines = append(outlines, opmlOutline{
			Text:   name,
			Title:  name,
			Type:   "rss",
			XMLURL: xmlURL,
		})
	}

	doc := opmlDocument{
		Version: "2.0",
		Head:    opmlHead{Title: "Feed Forge feeds"},
		Body:    opmlBody{Outlines: outlines},
	}
	payload, err := xmlenc.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal OPML: %w", err)
	}
	payload = append([]byte(xmlenc.Header), payload...)
	payload = append(payload, '\n')

	opmlPath := filepath.Join(CLI.OutputDir, opmlFilename)
	if err := os.WriteFile(opmlPath, payload, 0o600); err != nil {
		return fmt.Errorf("write OPML file: %w", err)
	}

	slog.Info("Generated OPML feed list", "path", opmlPath)
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

	dispatchCommand(ctx.Command(), configPath)
}

func dispatchCommand(command, configPath string) {
	type providerSpec struct {
		key, name, outfile string
		extra              []any
	}
	providerCmds := map[string]providerSpec{
		"hackernews":    {"hackernews", "Hacker News", CLI.HackerNews.Outfile, nil},
		"fingerpori":    {"fingerpori", "Fingerpori", CLI.Fingerpori.Outfile, nil},
		"feissarimokat": {"feissarimokat", "Feissarimokat", CLI.Feissarimokat.Outfile, nil},
		"oglaf":         {"oglaf", "Oglaf", CLI.Oglaf.Outfile, nil},
		"tildes":        {"tildes", "Tildes", CLI.Tildes.Outfile, nil},
		"youtube":       {"youtube", "YouTube", CLI.YouTube.Outfile, nil},
	}
	if spec, ok := providerCmds[command]; ok {
		runProvider(spec.key, spec.name, spec.outfile, spec.extra...)
		return
	}

	if handleBulletinCommand(command, configPath) {
		return
	}

	switch command {
	case "reddit":
		if CLI.Reddit.FeedID == "" || CLI.Reddit.Username == "" {
			slog.Error("Reddit feed requires both feed_id and username to be set via CLI flags or config file")
			os.Exit(1)
		}
		runProvider("reddit", "Reddit", CLI.Reddit.Outfile, "feed_id", CLI.Reddit.FeedID, "username", CLI.Reddit.Username)
	case "preview <provider>":
		slog.Debug("Previewing provider feed...", "provider", CLI.Preview.Provider)
		if err := previewFeed(CLI.Preview.Provider, CLI.Preview.Limit, CLI.Preview.Index, configPath); err != nil {
			slog.Error("Preview failed", "provider", CLI.Preview.Provider, "error", err)
			os.Exit(1)
		}
	case "youtube-rss <url>":
		feedURL, err := youtube.DiscoverFeedURL(CLI.YouTubeRSS.URL)
		if err != nil {
			slog.Error("Failed to discover YouTube RSS feed", "url", CLI.YouTubeRSS.URL, "error", err)
			os.Exit(1)
		}
		fmt.Println(feedURL)
	case "generate":
		slog.Debug("Generating feeds for all configured providers...")
		if err := generateAll(configPath); err != nil {
			slog.Error("Failed to generate feeds", "error", err)
			os.Exit(1)
		}
	default:
		panic(command)
	}
}

// handleBulletinCommand dispatches the bulletin-* subcommands. Returns true when
// it handled the command (kept separate to keep dispatchCommand's complexity low).
func handleBulletinCommand(command, configPath string) bool {
	var (
		err     error
		handled = true
	)
	switch command {
	case "bulletin-fetch":
		err = runBulletinFetch(configPath)
	case "bulletin-publish":
		err = runBulletinPublish(configPath)
	case "bulletin-summarize":
		err = runBulletinSummarize(configPath)
	default:
		handled = false
	}
	if handled && err != nil {
		slog.Error("Bulletin command failed", "command", command, "error", err)
		os.Exit(1)
	}
	return handled
}

// loadBulletinConfig reads the `bulletin:` section of the config file.
func loadBulletinConfig(configPath string) (bulletin.Config, error) {
	var cfg bulletin.Config
	if err := loadProviderConfigFromYAML(configPath, "bulletin", &cfg); err != nil {
		return cfg, fmt.Errorf("load bulletin config: %w", err)
	}
	return cfg, nil
}

// resolveAnthropicAPIKey reads the general `anthropic:` config section and
// resolves the API key, falling back to the ANTHROPIC_API_KEY env var. Shared by
// any processor that summarises via Anthropic.
func resolveAnthropicAPIKey(configPath string) (string, error) {
	var cfg llm.Config
	if err := loadProviderConfigFromYAML(configPath, "anthropic", &cfg); err != nil {
		return "", fmt.Errorf("load anthropic config: %w", err)
	}
	return cfg.ResolveAPIKey(), nil
}

// bulletinDBPath resolves the bulletin database path, honouring --cache-dir.
func bulletinDBPath() (string, error) {
	return filesystem.GetDefaultPath("bulletin.db")
}

func runBulletinFetch(configPath string) error {
	cfg, err := loadBulletinConfig(configPath)
	if err != nil {
		return err
	}
	dbPath, err := bulletinDBPath()
	if err != nil {
		return err
	}
	return bulletin.Fetch(cfg, dbPath)
}

func runBulletinPublish(configPath string) error {
	cfg, err := loadBulletinConfig(configPath)
	if err != nil {
		return err
	}
	dbPath, err := bulletinDBPath()
	if err != nil {
		return err
	}

	apiKey, err := resolveAnthropicAPIKey(configPath)
	if err != nil {
		return err
	}

	outfile := resolveOutfile(CLI.BulletinPublish.Outfile)
	feedURL := CLI.FeedBaseURL
	if CLI.FeedBaseURL != "" {
		if joined, err := url.JoinPath(CLI.FeedBaseURL, filepath.Base(CLI.BulletinPublish.Outfile)); err == nil {
			feedURL = joined
		}
	}
	return bulletin.Publish(bulletin.PublishOptions{
		Config:      cfg,
		DBPath:      dbPath,
		Outfile:     outfile,
		HTMLDir:     bulletinHTMLDir(),
		FeedBaseURL: feedURL,
		Slot:        CLI.BulletinPublish.Slot,
		APIKey:      apiKey,
	})
}

func runBulletinSummarize(configPath string) error {
	cfg, err := loadBulletinConfig(configPath)
	if err != nil {
		return err
	}
	dbPath, err := bulletinDBPath()
	if err != nil {
		return err
	}
	apiKey, err := resolveAnthropicAPIKey(configPath)
	if err != nil {
		return err
	}
	digest, err := bulletin.SummarizeDryRun(cfg, dbPath, apiKey)
	if err != nil {
		return err
	}
	fmt.Println(digest)
	return nil
}

func runProvider(key, displayName, outfileFlag string, extraKV ...any) {
	slog.Debug("Generating " + displayName + " feed...")

	providerConfig := buildProviderConfig(key)
	provider, err := providers.DefaultRegistry.CreateProvider(key, providerConfig)
	if err != nil {
		slog.Error("Failed to create "+displayName+" provider", "error", err)
		os.Exit(1)
	}

	outfile := resolveOutfile(outfileFlag)
	if err := provider.GenerateFeed(outfile); err != nil {
		args := append([]any{"output_file", outfile}, extraKV...)
		args = append(args, "error", err)
		slog.Error("Failed to generate "+displayName+" feed", args...)
		os.Exit(1)
	}
}
