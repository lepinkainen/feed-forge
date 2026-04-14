package feed

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/providers"
	"github.com/lepinkainen/feed-forge/pkg/testutil"
)

func TestTemplateFuncsHelpers(t *testing.T) {
	funcs := TemplateFuncs()

	if got := funcs["xmlEscape"].(func(string) string)("Fish &amp; Chips <3"); got != "Fish &amp;amp; Chips &lt;3" {
		t.Fatalf("xmlEscape() preserved entity text = %q", got)
	}
	if got := funcs["xmlEscape"].(func(string) string)("bad\x00chars\x1f and \"quotes\" 'apostrophes'"); got != "badchars and &quot;quotes&quot; &#39;apostrophes&#39;" {
		t.Fatalf("xmlEscape() strips controls and escapes quotes = %q", got)
	}

	ts := time.Date(2024, 3, 14, 15, 9, 26, 0, time.UTC)
	if got := funcs["formatTime"].(func(time.Time) string)(ts); got != "2024-03-14T15:09:26Z" {
		t.Fatalf("formatTime() = %q", got)
	}

	if got := funcs["formatDate"].(func(string) string)("2024-03-14T15:09:26Z"); got != "14 March 2024" {
		t.Fatalf("formatDate() = %q", got)
	}
	if got := funcs["formatDate"].(func(string) string)("not-a-date"); got != "not-a-date" {
		t.Fatalf("formatDate() invalid input = %q", got)
	}

	if got := funcs["formatScore"].(func(int, int) string)(42, 7); got != "Score: 42 | Comments: 7" {
		t.Fatalf("formatScore() = %q", got)
	}

	if got := funcs["truncate"].(func(string, int) string)("abcdefghijkl", 8); got != "abcde..." {
		t.Fatalf("truncate() = %q", got)
	}
	if got := funcs["truncate"].(func(string, int) string)("short", 8); got != "short" {
		t.Fatalf("truncate() unchanged = %q", got)
	}
}

func TestTemplateGeneratorLoadTemplateWithFallbackAndReadTemplateContent(t *testing.T) {
	oldOverride := GetTemplateOverrideFS()
	oldFallback := GetTemplateFallbackFS()
	t.Cleanup(func() {
		SetTemplateOverrideFS(oldOverride)
		SetTemplateFallbackFS(oldFallback)
	})

	SetTemplateOverrideFS(fstest.MapFS{
		"sample.tmpl": &fstest.MapFile{Data: []byte("override:{{.FeedTitle}}")},
	})
	SetTemplateFallbackFS(fstest.MapFS{
		"sample.tmpl": &fstest.MapFile{Data: []byte("fallback:{{.FeedTitle}}")},
		"other.tmpl":  &fstest.MapFile{Data: []byte("fallback-only")},
	})

	tg := NewTemplateGenerator()
	if err := tg.LoadTemplateWithFallback("sample"); err != nil {
		t.Fatalf("LoadTemplateWithFallback() error = %v", err)
	}

	var out strings.Builder
	if err := tg.GenerateFromTemplate("sample", &TemplateData{FeedTitle: "Feed"}, &out); err != nil {
		t.Fatalf("GenerateFromTemplate() error = %v", err)
	}
	if got := out.String(); got != "override:Feed" {
		t.Fatalf("override template output = %q", got)
	}

	content, err := ReadTemplateContent("other.tmpl")
	if err != nil {
		t.Fatalf("ReadTemplateContent() error = %v", err)
	}
	if content != "fallback-only" {
		t.Fatalf("ReadTemplateContent() = %q", content)
	}
}

func TestTemplateGeneratorLoadTemplateWithFallbackErrors(t *testing.T) {
	oldOverride := GetTemplateOverrideFS()
	oldFallback := GetTemplateFallbackFS()
	t.Cleanup(func() {
		SetTemplateOverrideFS(oldOverride)
		SetTemplateFallbackFS(oldFallback)
	})

	SetTemplateOverrideFS(nil)
	SetTemplateFallbackFS(nil)

	tg := NewTemplateGenerator()
	err := tg.LoadTemplateWithFallback("missing")
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("LoadTemplateWithFallback() error = %v, want ErrTemplateNotFound", err)
	}

	_, err = ReadTemplateContent("missing.tmpl")
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("ReadTemplateContent() error = %v, want ErrTemplateNotFound", err)
	}
}

func TestTemplateGeneratorLoadTemplatesFromDir(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.tmpl":    "A",
		"b.tmpl":    "B",
		"notes.txt": "ignored",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}

	tg := NewTemplateGenerator()
	if err := tg.LoadTemplatesFromDir(dir); err != nil {
		t.Fatalf("LoadTemplatesFromDir() error = %v", err)
	}

	got := tg.GetAvailableTemplates()
	sort.Strings(got)
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("GetAvailableTemplates() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("GetAvailableTemplates()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGenerateAtomFeedWithEmbeddedTemplate_Golden(t *testing.T) {
	oldOverride := GetTemplateOverrideFS()
	oldFallback := GetTemplateFallbackFS()
	t.Cleanup(func() {
		SetTemplateOverrideFS(oldOverride)
		SetTemplateFallbackFS(oldFallback)
	})

	SetTemplateOverrideFS(fstest.MapFS{
		"simple.tmpl": &fstest.MapFile{Data: []byte(`<feed><title>{{.FeedTitle}}</title>{{range .Items}}<entry><title>{{xmlEscape .Title}}</title><summary>{{.Summary}}</summary><author>{{.Author}}</author><authoruri>{{.AuthorURI}}</authoruri><domain>{{.Domain}}</domain></entry>{{end}}</feed>`)},
	})
	SetTemplateFallbackFS(fstest.MapFS{})

	items := []providers.FeedItem{
		&mockFeedItem{
			title:        "Hello &amp; Goodbye",
			link:         "https://example.com/posts/1",
			commentsLink: "https://news.ycombinator.com/item?id=1",
			author:       "alice",
			score:        42,
			commentCount: 7,
			createdAt:    time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
			categories:   []string{"example.com", "High Score 20+"},
		},
	}

	got, err := GenerateAtomFeedWithEmbeddedTemplate(items, "simple", Config{Title: "Example Feed"}, nil)
	if err != nil {
		t.Fatalf("GenerateAtomFeedWithEmbeddedTemplate() error = %v", err)
	}

	testutil.CompareGolden(t, filepath.Join("testdata", "generated", "simple-atom.xml.golden"), got)
}

func TestGenerateFromTemplate_EscapesFeedMetadata(t *testing.T) {
	tg := NewTemplateGenerator()
	const templateContent = `<feed><title>{{.FeedTitle | xmlEscape}}</title><link href="{{.FeedLink | xmlEscape}}"/><id>{{.FeedID | xmlEscape}}</id><author>{{.FeedAuthor | xmlEscape}}</author><subtitle>{{.FeedDescription | xmlEscape}}</subtitle><generator>{{.Generator | xmlEscape}}</generator></feed>`

	templatePath := filepath.Join(t.TempDir(), "feed-metadata.tmpl")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := tg.LoadTemplate("feed-metadata", templatePath); err != nil {
		t.Fatalf("LoadTemplate() error = %v", err)
	}

	var out strings.Builder
	if err := tg.GenerateFromTemplate("feed-metadata", &TemplateData{
		FeedTitle:       `Fish &amp; Chips <Feed>`,
		FeedLink:        `https://example.com/?q=fish&chips=1`,
		FeedID:          "feed\x00id",
		FeedAuthor:      `A "B"`,
		FeedDescription: "Line\x1fbreak",
		Generator:       `Forge's <gen>`,
	}, &out); err != nil {
		t.Fatalf("GenerateFromTemplate() error = %v", err)
	}

	got := out.String()
	expected := []string{
		`<title>Fish &amp;amp; Chips &lt;Feed&gt;</title>`,
		`<link href="https://example.com/?q=fish&amp;chips=1"/>`,
		`<id>feedid</id>`,
		`<author>A &quot;B&quot;</author>`,
		`<subtitle>Linebreak</subtitle>`,
		`<generator>Forge&#39;s &lt;gen&gt;</generator>`,
	}
	for _, want := range expected {
		if !strings.Contains(got, want) {
			t.Fatalf("generated output missing %q in %s", want, got)
		}
	}
}

func TestGenerateFromTemplate_MissingTemplate(t *testing.T) {
	tg := NewTemplateGenerator()
	err := tg.GenerateFromTemplate("missing", &TemplateData{}, io.Discard)
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("GenerateFromTemplate() error = %v, want ErrTemplateNotFound", err)
	}
}
