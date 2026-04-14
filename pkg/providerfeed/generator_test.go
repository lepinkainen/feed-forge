package providerfeed

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/feedmeta"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

type stubItem struct{}

func (stubItem) Title() string        { return "Stub" }
func (stubItem) Link() string         { return "https://example.com/post/1" }
func (stubItem) CommentsLink() string { return "https://example.com/post/1" }
func (stubItem) Author() string       { return "Tester" }
func (stubItem) Score() int           { return 0 }
func (stubItem) CommentCount() int    { return 0 }
func (stubItem) CreatedAt() time.Time { return time.Unix(0, 0).UTC() }
func (stubItem) Categories() []string { return []string{"stub"} }
func (stubItem) ImageURL() string     { return "" }
func (stubItem) Content() string      { return "hello" }

func validPreview() *providers.PreviewInfo {
	return &providers.PreviewInfo{
		Config: feedmeta.Config{
			Title:       "Stub Feed",
			Link:        "https://example.com/",
			Description: "stub",
			Author:      "Tester",
			ID:          "https://example.com/",
		},
		ProviderName: "Stub",
		TemplateName: "feissarimokat-atom",
	}
}

func TestBuildGeneratorRequiresFetchFunc(t *testing.T) {
	gen := BuildGenerator(nil, validPreview(), nil, nil)
	err := gen(filepath.Join(t.TempDir(), "feed.xml"))
	if err == nil {
		t.Fatal("BuildGenerator with nil fetchItems: err = nil, want error")
	}
	if !strings.Contains(err.Error(), "feed generator is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildGeneratorRequiresPreview(t *testing.T) {
	gen := BuildGenerator(func(int) ([]providers.FeedItem, error) { return nil, nil }, nil, nil, nil)
	err := gen(filepath.Join(t.TempDir(), "feed.xml"))
	if err == nil {
		t.Fatal("BuildGenerator with nil preview: err = nil, want error")
	}
	if !strings.Contains(err.Error(), "preview metadata is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildGeneratorPropagatesFetchError(t *testing.T) {
	sentinel := errors.New("fetch failed")
	gen := BuildGenerator(
		func(int) ([]providers.FeedItem, error) { return nil, sentinel },
		validPreview(),
		nil,
		nil,
	)
	err := gen(filepath.Join(t.TempDir(), "feed.xml"))
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want wraps %v", err, sentinel)
	}
}

func TestBuildGeneratorWritesFeedAndCreatesDirs(t *testing.T) {
	preview := validPreview()
	outfile := filepath.Join(t.TempDir(), "nested", "out", "feed.xml")

	gen := BuildGenerator(
		func(int) ([]providers.FeedItem, error) {
			return []providers.FeedItem{stubItem{}}, nil
		},
		preview,
		nil,
		nil,
	)
	if err := gen(outfile); err != nil {
		t.Fatalf("gen(%s) error = %v", outfile, err)
	}

	contents, err := os.ReadFile(outfile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(contents), "Stub Feed") {
		t.Errorf("feed output missing configured title; got:\n%s", contents)
	}
}

func TestBuildGeneratorConfigFuncOverridesPreview(t *testing.T) {
	preview := validPreview()
	preview.Config.Title = "Original Title"

	called := false
	configFunc := func() feedmeta.Config {
		called = true
		cfg := preview.Config
		cfg.Title = "Overridden Title"
		return cfg
	}

	outfile := filepath.Join(t.TempDir(), "feed.xml")
	gen := BuildGenerator(
		func(int) ([]providers.FeedItem, error) { return nil, nil },
		preview,
		configFunc,
		nil,
	)
	if err := gen(outfile); err != nil {
		t.Fatalf("gen error = %v", err)
	}
	if !called {
		t.Fatal("configFunc was not called")
	}

	contents, err := os.ReadFile(outfile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(contents), "Overridden Title") {
		t.Errorf("feed output should reflect overridden config; got:\n%s", contents)
	}
	if strings.Contains(string(contents), "Original Title") {
		t.Errorf("feed output should not contain original title; got:\n%s", contents)
	}
}
