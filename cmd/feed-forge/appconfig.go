package main

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/lepinkainen/feed-forge/internal/bulletin"
	"github.com/lepinkainen/feed-forge/pkg/llm"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// defaultFeedBaseURL mirrors the Kong default of the --feed-base-url flag so a
// snapshot built straight from YAML resolves feed URLs the same way as one
// built from the parsed CLI.
const defaultFeedBaseURL = "https://endymion.xyz/rss/"

// Serve daemon schedule defaults, chosen to reproduce the cron setup the
// daemon replaces (generate */5, bulletin-fetch */30, digests at 45 7,17).
const (
	defaultGenerateTick  = 5 * time.Minute
	defaultFetchInterval = 30 * time.Minute
)

var defaultBulletinSlots = []slotTime{{7, 45}, {17, 45}}

// appConfig is an immutable snapshot of one validated configuration. The serve
// daemon swaps whole snapshots on reload, so a job never observes a half-updated
// configuration and an invalid file can never reach a run.
type appConfig struct {
	raw []byte // the YAML document every provider section decodes from

	OutputDir         string
	FeedBaseURL       string
	CacheDir          string
	DiscordWebhookURL string

	Serve    serveConfig
	Bulletin bulletin.Config
}

// globalYAML carries the top-level scalar keys of config.yaml. It mirrors the
// global flags on CLI; the serve daemon reads them from the file so a SIGHUP
// reload can pick up changes.
type globalYAML struct {
	OutputDir         string `yaml:"output-dir"`
	FeedBaseURL       string `yaml:"feed-base-url"`
	CacheDir          string `yaml:"cache-dir"`
	DiscordWebhookURL string `yaml:"discord-webhook-url"`
}

// serveConfig is the parsed `serve:` section: the daemon's schedules.
type serveConfig struct {
	GenerateTick          time.Duration
	BulletinFetchInterval time.Duration
	BulletinSlots         []slotTime
}

// serveYAML is the raw `serve:` section shape.
type serveYAML struct {
	GenerateTick          string   `yaml:"generate-tick"`
	BulletinFetchInterval string   `yaml:"bulletin-fetch-interval"`
	BulletinSlots         []string `yaml:"bulletin-slots"`
}

// slotTime is a time-of-day schedule slot in the local timezone.
type slotTime struct {
	hour, minute int
}

func (s slotTime) String() string {
	return fmt.Sprintf("%02d:%02d", s.hour, s.minute)
}

// parseSlot parses an "HH:MM" wall-clock slot.
func parseSlot(s string) (slotTime, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return slotTime{}, fmt.Errorf("invalid bulletin slot %q: expected HH:MM: %w", s, err)
	}
	return slotTime{t.Hour(), t.Minute()}, nil
}

// nextSlotFire returns the next wall-clock occurrence strictly after now of any
// slot, in now's location. An occurrence today that is not after now rolls to
// tomorrow. Returns the zero time when no slots are configured.
func nextSlotFire(now time.Time, slots []slotTime) time.Time {
	var next time.Time
	for _, s := range slots {
		c := time.Date(now.Year(), now.Month(), now.Day(), s.hour, s.minute, 0, 0, now.Location())
		if !c.After(now) {
			c = c.AddDate(0, 0, 1)
		}
		if next.IsZero() || c.Before(next) {
			next = c
		}
	}
	return next
}

// decodeSection unmarshals one top-level section of the YAML document into
// target. A missing section leaves target untouched.
func decodeSection(data []byte, section string, target any) error {
	var root map[string]yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}
	node, ok := root[section]
	if !ok {
		return nil
	}
	return node.Decode(target)
}

// configuredProvidersFromBytes returns the registry names that have a top-level
// section in the YAML document.
func configuredProvidersFromBytes(data []byte) ([]string, error) {
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

// validateConfig is the single definition of a valid configuration, shared by
// serve startup, the SIGHUP reload, and the validate-config command. It parses
// the whole document eagerly — every configured provider section, the bulletin
// section, and the serve schedules — so a snapshot it returns can never fail to
// decode later.
func validateConfig(data []byte) (*appConfig, error) {
	var root map[string]yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	var globals globalYAML
	if err := yaml.Unmarshal(data, &globals); err != nil {
		return nil, fmt.Errorf("parse global keys: %w", err)
	}
	if globals.FeedBaseURL == "" {
		globals.FeedBaseURL = defaultFeedBaseURL
	}

	if err := validateProviderSections(data); err != nil {
		return nil, err
	}

	bulletinCfg, err := validateBulletinSection(data)
	if err != nil {
		return nil, err
	}

	serveCfg, err := validateServeSection(data)
	if err != nil {
		return nil, err
	}

	warnMissingAnthropicKey(data, bulletinCfg)

	return &appConfig{
		raw:               data,
		OutputDir:         globals.OutputDir,
		FeedBaseURL:       globals.FeedBaseURL,
		CacheDir:          globals.CacheDir,
		DiscordWebhookURL: globals.DiscordWebhookURL,
		Serve:             serveCfg,
		Bulletin:          bulletinCfg,
	}, nil
}

// validateProviderSections eagerly decodes every configured provider's section
// into its Config struct and checks the interval strictly. Runtime interval
// parsing keeps its lenient 15m fallback; validation is where a typo must
// surface.
func validateProviderSections(data []byte) error {
	names, err := configuredProvidersFromBytes(data)
	if err != nil {
		return err
	}
	sort.Strings(names)

	for _, name := range names {
		info, err := providers.DefaultRegistry.Get(name)
		if err != nil || info.ConfigFactory == nil {
			continue
		}
		cfg := info.ConfigFactory()
		if err := decodeSection(data, name, cfg); err != nil {
			return fmt.Errorf("provider %s: %w", name, err)
		}
		if interval := providers.GetGenerateConfig(cfg).Interval; interval != "" {
			if _, err := time.ParseDuration(interval); err != nil {
				return fmt.Errorf("provider %s: invalid interval %q: %w", name, interval, err)
			}
		}
	}
	return nil
}

// validateBulletinSection decodes the bulletin section and checks its feeds and
// prompt file.
func validateBulletinSection(data []byte) (bulletin.Config, error) {
	var cfg bulletin.Config
	if err := decodeSection(data, "bulletin", &cfg); err != nil {
		return cfg, fmt.Errorf("bulletin: %w", err)
	}
	for i, f := range cfg.Feeds {
		if f.URL == "" {
			return cfg, fmt.Errorf("bulletin: feeds[%d] has no url", i)
		}
	}
	if cfg.PromptFile != "" {
		if _, err := os.Stat(cfg.PromptFile); err != nil {
			return cfg, fmt.Errorf("bulletin: prompt-file: %w", err)
		}
	}
	return cfg, nil
}

// validateServeSection decodes and parses the serve schedules, applying the
// cron-parity defaults for unset keys.
func validateServeSection(data []byte) (serveConfig, error) {
	var raw serveYAML
	if err := decodeSection(data, "serve", &raw); err != nil {
		return serveConfig{}, fmt.Errorf("serve: %w", err)
	}

	cfg := serveConfig{
		GenerateTick:          defaultGenerateTick,
		BulletinFetchInterval: defaultFetchInterval,
		BulletinSlots:         defaultBulletinSlots,
	}

	if raw.GenerateTick != "" {
		d, err := time.ParseDuration(raw.GenerateTick)
		if err != nil || d <= 0 {
			return cfg, fmt.Errorf("serve: invalid generate-tick %q", raw.GenerateTick)
		}
		cfg.GenerateTick = d
	}
	if raw.BulletinFetchInterval != "" {
		d, err := time.ParseDuration(raw.BulletinFetchInterval)
		if err != nil || d <= 0 {
			return cfg, fmt.Errorf("serve: invalid bulletin-fetch-interval %q", raw.BulletinFetchInterval)
		}
		cfg.BulletinFetchInterval = d
	}
	if raw.BulletinSlots != nil {
		slots := make([]slotTime, 0, len(raw.BulletinSlots))
		seen := make(map[slotTime]bool)
		for _, s := range raw.BulletinSlots {
			slot, err := parseSlot(s)
			if err != nil {
				return cfg, fmt.Errorf("serve: %w", err)
			}
			if seen[slot] {
				return cfg, fmt.Errorf("serve: duplicate bulletin slot %q", s)
			}
			seen[slot] = true
			slots = append(slots, slot)
		}
		cfg.BulletinSlots = slots
	}
	return cfg, nil
}

// warnMissingAnthropicKey warns — without failing validation — when bulletin
// feeds are configured but no Anthropic key resolves. The key may legitimately
// exist only in the daemon's environment, not in the shell running
// validate-config, so this cannot be a hard error.
func warnMissingAnthropicKey(data []byte, bulletinCfg bulletin.Config) {
	if len(bulletinCfg.Feeds) == 0 {
		return
	}
	var cfg llm.Config
	if err := decodeSection(data, "anthropic", &cfg); err != nil {
		slog.Warn("anthropic section did not decode", "error", err)
		return
	}
	if cfg.ResolveAPIKey() == "" {
		slog.Warn("bulletin feeds are configured but no Anthropic API key resolves; bulletin-generate will fail",
			"hint", "set anthropic.api-key or ANTHROPIC_API_KEY")
	}
}

// snapshotForOneShot builds the configuration snapshot for a one-shot command
// from the Kong-parsed globals (flags override YAML there) plus one read of the
// raw document. It does not validate: one-shot commands keep their historical
// lenient behavior.
func snapshotForOneShot(configPath string) (*appConfig, error) {
	// #nosec G304 G703 -- the config path is an explicit CLI input.
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var bulletinCfg bulletin.Config
	if err := decodeSection(data, "bulletin", &bulletinCfg); err != nil {
		return nil, fmt.Errorf("load bulletin config: %w", err)
	}
	return &appConfig{
		raw:               data,
		OutputDir:         CLI.OutputDir,
		FeedBaseURL:       CLI.FeedBaseURL,
		CacheDir:          CLI.CacheDir,
		DiscordWebhookURL: CLI.DiscordWebhookURL,
		Bulletin:          bulletinCfg,
	}, nil
}
