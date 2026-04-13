package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

type stubConfig struct {
	providers.GenerateConfig `yaml:",inline"`
	Message                  string `yaml:"message"`
}

type stubItem struct {
	title        string
	link         string
	commentsLink string
	author       string
	createdAt    time.Time
}

func (s stubItem) Title() string        { return s.title }
func (s stubItem) Link() string         { return s.link }
func (s stubItem) CommentsLink() string { return s.commentsLink }
func (s stubItem) Author() string       { return s.author }
func (s stubItem) Score() int           { return 10 }
func (s stubItem) CommentCount() int    { return 2 }
func (s stubItem) CreatedAt() time.Time { return s.createdAt }
func (s stubItem) Categories() []string { return []string{"test"} }
func (s stubItem) ImageURL() string     { return "" }
func (s stubItem) Content() string      { return "" }

type stubProvider struct {
	cfg           *stubConfig
	items         []providers.FeedItem
	generateCalls int
	closeCalls    int
}

func (s *stubProvider) GenerateFeed(outfile string) error {
	s.generateCalls++
	if err := os.MkdirAll(filepath.Dir(outfile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outfile, []byte("generated:"+s.cfg.Message), 0o644)
}

func (s *stubProvider) FetchItems(limit int) ([]providers.FeedItem, error) {
	if limit > 0 && limit < len(s.items) {
		return s.items[:limit], nil
	}
	return s.items, nil
}

func (s *stubProvider) Close() error {
	s.closeCalls++
	return nil
}

func withTestRegistry(t *testing.T, setup func(r *providers.ProviderRegistry)) {
	t.Helper()
	old := providers.DefaultRegistry
	providers.DefaultRegistry = providers.NewProviderRegistry()
	t.Cleanup(func() { providers.DefaultRegistry = old })
	setup(providers.DefaultRegistry)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	return buf.String()
}

func TestPreviewFeed_PrintsXMLForIndexedItem(t *testing.T) {
	oldOverride := feed.GetTemplateOverrideFS()
	oldFallback := feed.GetTemplateFallbackFS()
	feed.SetTemplateOverrideFS(fstest.MapFS{
		"preview.tmpl": &fstest.MapFile{Data: []byte(`<feed>{{range .Items}}<entry><title>{{.Title}}</title><author>{{.Author}}</author></entry>{{end}}</feed>`)},
	})
	feed.SetTemplateFallbackFS(fstest.MapFS{})
	t.Cleanup(func() {
		feed.SetTemplateOverrideFS(oldOverride)
		feed.SetTemplateFallbackFS(oldFallback)
	})

	withTestRegistry(t, func(r *providers.ProviderRegistry) {
		provider := &stubProvider{items: []providers.FeedItem{stubItem{title: "Preview Title", link: "https://example.com", commentsLink: "https://example.com", author: "alice", createdAt: time.Now()}}}
		if err := r.Register("stubpreview", &providers.ProviderInfo{
			Name:    "stubpreview",
			Factory: func(config any) (providers.FeedProvider, error) { return provider, nil },
			Preview: &providers.PreviewInfo{ProviderName: "Stub Preview", TemplateName: "preview", FeedTitle: "Stub Feed", FeedLink: "https://example.com", Description: "desc", Author: "author", FeedID: "feed-id"},
		}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		out := captureStdout(t, func() {
			if err := previewFeed("stubpreview", 1, 0, ""); err != nil {
				t.Fatalf("previewFeed() error = %v", err)
			}
		})
		if !strings.Contains(out, "<entry>") || !strings.Contains(out, "Preview Title") || !strings.Contains(out, "alice") {
			t.Fatalf("previewFeed() output = %q", out)
		}
	})
}

func TestGenerateProvider_GeneratedAndSkipped(t *testing.T) {
	oldCLI := CLI
	t.Cleanup(func() { CLI = oldCLI })
	CLI.OutputDir = t.TempDir()

	provider := &stubProvider{items: []providers.FeedItem{stubItem{title: "Item", createdAt: time.Now()}}, cfg: &stubConfig{GenerateConfig: providers.GenerateConfig{Outfile: "stub.xml", Interval: "1h"}, Message: "hello"}}
	withTestRegistry(t, func(r *providers.ProviderRegistry) {
		if err := r.Register("stub", &providers.ProviderInfo{
			Name: "stub",
			Factory: func(config any) (providers.FeedProvider, error) {
				cfg := config.(*stubConfig)
				provider.cfg = cfg
				return provider, nil
			},
			ConfigFactory: func() any { return &stubConfig{} },
		}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(configPath, []byte("stub:\n  outfile: stub.xml\n  interval: 1h\n  message: hello\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		result := generateProvider(configPath, "stub")
		if result.Status != "generated" || result.Filename != "stub.xml" {
			t.Fatalf("generateProvider() = %#v", result)
		}
		if provider.generateCalls != 1 || provider.closeCalls != 1 {
			t.Fatalf("provider calls generate=%d close=%d", provider.generateCalls, provider.closeCalls)
		}

		result = generateProvider(configPath, "stub")
		if result.Status != "skipped" {
			t.Fatalf("second generateProvider() = %#v, want skipped", result)
		}
	})
}

func TestGenerateAll_GeneratesConfiguredProviders(t *testing.T) {
	oldCLI := CLI
	t.Cleanup(func() { CLI = oldCLI })
	CLI.OutputDir = t.TempDir()

	providersSeen := map[string]*stubProvider{}
	withTestRegistry(t, func(r *providers.ProviderRegistry) {
		for _, name := range []string{"alpha", "beta"} {
			name := name
			if err := r.Register(name, &providers.ProviderInfo{
				Name: name,
				Factory: func(config any) (providers.FeedProvider, error) {
					cfg := config.(*stubConfig)
					p := &stubProvider{cfg: cfg, items: []providers.FeedItem{stubItem{title: name, createdAt: time.Now()}}}
					providersSeen[name] = p
					return p, nil
				},
				ConfigFactory: func() any { return &stubConfig{} },
			}); err != nil {
				t.Fatalf("Register(%s) error = %v", name, err)
			}
		}

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		config := "alpha:\n  outfile: alpha.xml\n  interval: 0s\n  message: first\nbeta:\n  outfile: beta.xml\n  interval: 0s\n  message: second\nunknown:\n  outfile: ignored.xml\n"
		if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		if err := generateAll(configPath); err != nil {
			t.Fatalf("generateAll() error = %v", err)
		}

		for _, name := range []string{"alpha", "beta"} {
			path := filepath.Join(CLI.OutputDir, name+".xml")
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", path, err)
			}
			if !strings.Contains(string(content), "generated:") {
				t.Fatalf("output for %s = %q", name, string(content))
			}
		}

		indexContent, err := os.ReadFile(filepath.Join(CLI.OutputDir, "index.html"))
		if err != nil {
			t.Fatalf("ReadFile(index.html) error = %v", err)
		}
		for _, want := range []string{"alpha.xml", "beta.xml"} {
			if !strings.Contains(string(indexContent), want) {
				t.Fatalf("index.html missing %q", want)
			}
		}
	})
}
