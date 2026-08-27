package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateConfig_Valid(t *testing.T) {
	promptFile := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("summarise"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	config := `
output-dir: /srv/feeds
feed-base-url: https://example.com/rss/
discord-webhook-url: https://discord.example/hook
hackernews:
  outfile: hackernews.xml
  interval: 30m
xkcd:
bulletin:
  prompt-file: ` + promptFile + `
  feeds:
    - url: https://example.com/feed.xml
      name: Example
serve:
  generate-tick: 2m
  bulletin-fetch-interval: 45m
  bulletin-slots: ["06:15", "18:30"]
anthropic:
  api-key: test-key
`
	cfg, err := validateConfig([]byte(config))
	if err != nil {
		t.Fatalf("validateConfig() error = %v", err)
	}
	if cfg.OutputDir != "/srv/feeds" || cfg.FeedBaseURL != "https://example.com/rss/" || cfg.DiscordWebhookURL != "https://discord.example/hook" {
		t.Fatalf("globals = %#v", cfg)
	}
	if cfg.Serve.GenerateTick != 2*time.Minute || cfg.Serve.BulletinFetchInterval != 45*time.Minute {
		t.Fatalf("serve durations = %#v", cfg.Serve)
	}
	wantSlots := []slotTime{{6, 15}, {18, 30}}
	if len(cfg.Serve.BulletinSlots) != 2 || cfg.Serve.BulletinSlots[0] != wantSlots[0] || cfg.Serve.BulletinSlots[1] != wantSlots[1] {
		t.Fatalf("slots = %v, want %v", cfg.Serve.BulletinSlots, wantSlots)
	}
	if len(cfg.Bulletin.Feeds) != 1 || cfg.Bulletin.Feeds[0].Name != "Example" {
		t.Fatalf("bulletin = %#v", cfg.Bulletin)
	}
}

func TestValidateConfig_Defaults(t *testing.T) {
	cfg, err := validateConfig([]byte("hackernews:\n"))
	if err != nil {
		t.Fatalf("validateConfig() error = %v", err)
	}
	if cfg.FeedBaseURL != defaultFeedBaseURL {
		t.Errorf("FeedBaseURL = %q, want default", cfg.FeedBaseURL)
	}
	if cfg.Serve.GenerateTick != defaultGenerateTick || cfg.Serve.BulletinFetchInterval != defaultFetchInterval {
		t.Errorf("serve durations = %#v, want defaults", cfg.Serve)
	}
	if len(cfg.Serve.BulletinSlots) != 2 || cfg.Serve.BulletinSlots[0] != (slotTime{7, 45}) || cfg.Serve.BulletinSlots[1] != (slotTime{17, 45}) {
		t.Errorf("slots = %v, want cron-parity defaults", cfg.Serve.BulletinSlots)
	}
}

func TestValidateConfig_EmptySlotListDisablesSlots(t *testing.T) {
	cfg, err := validateConfig([]byte("serve:\n  bulletin-slots: []\n"))
	if err != nil {
		t.Fatalf("validateConfig() error = %v", err)
	}
	if len(cfg.Serve.BulletinSlots) != 0 {
		t.Fatalf("slots = %v, want empty", cfg.Serve.BulletinSlots)
	}
}

func TestValidateConfig_Errors(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{"broken yaml", "hackernews: [\n", "parse config"},
		{"bad provider field type", "hackernews:\n  min-points: notanumber\n", "provider hackernews"},
		{"bad interval", "hackernews:\n  interval: every-5-minutes\n", "invalid interval"},
		{"bad slot", `serve: {bulletin-slots: ["25:99"]}`, "invalid bulletin slot"},
		{"duplicate slot", `serve: {bulletin-slots: ["07:45", "07:45"]}`, "duplicate bulletin slot"},
		{"bad generate-tick", `serve: {generate-tick: fast}`, "invalid generate-tick"},
		{"zero generate-tick", `serve: {generate-tick: 0s}`, "invalid generate-tick"},
		{"bad fetch interval", `serve: {bulletin-fetch-interval: -5m}`, "invalid bulletin-fetch-interval"},
		{"bulletin feed missing url", "bulletin:\n  feeds:\n    - name: NoURL\n", "has no url"},
		{"bulletin prompt file missing", "bulletin:\n  prompt-file: /nonexistent/prompt.txt\n", "prompt-file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateConfig([]byte(tt.config))
			if err == nil {
				t.Fatal("validateConfig() = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateConfig() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfig_MissingAnthropicKeyIsNotAnError(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	config := "bulletin:\n  feeds:\n    - url: https://example.com/feed.xml\n"
	if _, err := validateConfig([]byte(config)); err != nil {
		t.Fatalf("validateConfig() error = %v, want nil (missing key must only warn)", err)
	}
}

func TestParseSlot(t *testing.T) {
	if got, err := parseSlot("07:45"); err != nil || got != (slotTime{7, 45}) {
		t.Fatalf("parseSlot(07:45) = %v, %v", got, err)
	}
	for _, bad := range []string{"7:45pm", "24:00", "07:60", "0745", ""} {
		if _, err := parseSlot(bad); err == nil {
			t.Errorf("parseSlot(%q) = nil error, want error", bad)
		}
	}
}

func TestNextSlotFire(t *testing.T) {
	slots := []slotTime{{17, 45}, {7, 45}} // deliberately unsorted
	day := func(d, h, m int) time.Time {
		return time.Date(2026, 8, d, h, m, 0, 0, time.Local)
	}

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"before first slot", day(27, 6, 0), day(27, 7, 45)},
		{"between slots", day(27, 12, 0), day(27, 17, 45)},
		{"after last slot wraps to tomorrow", day(27, 20, 0), day(28, 7, 45)},
		{"exactly at a slot picks the next", day(27, 7, 45), day(27, 17, 45)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextSlotFire(tt.now, slots); !got.Equal(tt.want) {
				t.Fatalf("nextSlotFire(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}

	if got := nextSlotFire(day(27, 12, 0), nil); !got.IsZero() {
		t.Fatalf("nextSlotFire(no slots) = %v, want zero", got)
	}

	single := []slotTime{{9, 0}}
	if got := nextSlotFire(day(27, 9, 30), single); !got.Equal(day(28, 9, 0)) {
		t.Fatalf("nextSlotFire(single, after) = %v", got)
	}
}

func TestSnapshotForOneShot(t *testing.T) {
	oldCLI := CLI
	t.Cleanup(func() { CLI = oldCLI })
	CLI.OutputDir = "/srv/from-flags"
	CLI.FeedBaseURL = "https://flags.example/rss/"

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	config := "output-dir: /srv/from-yaml\nbulletin:\n  feeds:\n    - url: https://example.com/feed.xml\n"
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := snapshotForOneShot(configPath)
	if err != nil {
		t.Fatalf("snapshotForOneShot() error = %v", err)
	}
	// One-shot commands honor the Kong-parsed globals (flags override YAML),
	// not the raw YAML values.
	if cfg.OutputDir != "/srv/from-flags" || cfg.FeedBaseURL != "https://flags.example/rss/" {
		t.Fatalf("globals = %#v, want Kong-parsed values", cfg)
	}
	if len(cfg.Bulletin.Feeds) != 1 {
		t.Fatalf("bulletin = %#v", cfg.Bulletin)
	}

	if _, err := snapshotForOneShot(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("snapshotForOneShot(missing) = nil, want error")
	}
}

func TestDecodeSection(t *testing.T) {
	data := []byte("hackernews:\n  outfile: hn.xml\n")

	var target struct {
		Outfile string `yaml:"outfile"`
	}
	if err := decodeSection(data, "hackernews", &target); err != nil {
		t.Fatalf("decodeSection() error = %v", err)
	}
	if target.Outfile != "hn.xml" {
		t.Fatalf("Outfile = %q", target.Outfile)
	}

	// A missing section must leave the target untouched.
	target.Outfile = "unchanged"
	if err := decodeSection(data, "missing", &target); err != nil {
		t.Fatalf("decodeSection(missing) error = %v", err)
	}
	if target.Outfile != "unchanged" {
		t.Fatalf("missing section modified target: %q", target.Outfile)
	}
}
