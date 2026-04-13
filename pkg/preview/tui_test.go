package preview

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

func captureOutput(t *testing.T, fn func()) string {
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

func TestNewModelSortsNewestFirstAndInit(t *testing.T) {
	older := mockFeedItem{title: "older", createdAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	newer := mockFeedItem{title: "newer", createdAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)}

	model := NewModel([]providers.FeedItem{older, newer}, "Provider", "preview", feed.Config{Title: "Feed"})
	if model.items[0].Title() != "newer" || model.items[1].Title() != "older" {
		t.Fatalf("NewModel() items not sorted newest-first: %#v", model.items)
	}
	if model.cursor != 0 || model.viewMode != ListViewMode || model.selectedIndex != -1 {
		t.Fatalf("NewModel() initial state = %#v", model)
	}
	if cmd := model.Init(); cmd != nil {
		t.Fatalf("Init() = %#v, want nil", cmd)
	}
}

func TestModelUpdateWindowAndListNavigation(t *testing.T) {
	model := NewModel([]providers.FeedItem{
		mockFeedItem{title: "one", createdAt: time.Now()},
		mockFeedItem{title: "two", createdAt: time.Now().Add(time.Second)},
	}, "Provider", "preview", feed.Config{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m := updated.(Model)
	if m.width != 100 || m.height != 20 {
		t.Fatalf("WindowSize update = %#v", m)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", m.cursor)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.viewMode != DetailViewMode || m.selectedIndex != 1 {
		t.Fatalf("enter did not open detail view: %#v", m)
	}
}

func TestDetailAndXMLViewUpdates(t *testing.T) {
	model := NewModel([]providers.FeedItem{mockFeedItem{title: "one", createdAt: time.Now()}}, "Provider", "preview", feed.Config{})
	model.selectedIndex = 0
	model.viewMode = DetailViewMode

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m := updated.(Model)
	if m.viewMode != XMLViewMode {
		t.Fatalf("detail x toggle viewMode = %v, want XMLViewMode", m.viewMode)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(Model)
	if m.viewMode != DetailViewMode {
		t.Fatalf("xml x toggle viewMode = %v, want DetailViewMode", m.viewMode)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.viewMode != ListViewMode {
		t.Fatalf("esc viewMode = %v, want ListViewMode", m.viewMode)
	}
}

func TestViewAndRenderMethods(t *testing.T) {
	oldOverride := feed.GetTemplateOverrideFS()
	oldFallback := feed.GetTemplateFallbackFS()
	feed.SetTemplateOverrideFS(fstest.MapFS{
		"preview.tmpl": &fstest.MapFile{Data: []byte(`<feed>{{range .Items}}<entry><title>{{.Title}}</title></entry>{{end}}</feed>`)},
	})
	feed.SetTemplateFallbackFS(fstest.MapFS{})
	t.Cleanup(func() {
		feed.SetTemplateOverrideFS(oldOverride)
		feed.SetTemplateFallbackFS(oldFallback)
	})

	item := mockFeedItem{title: "Preview title", link: "https://example.com", commentsLink: "https://example.com", author: "alice", createdAt: time.Now()}
	model := NewModel([]providers.FeedItem{item}, "Provider", "preview", feed.Config{Title: "Feed"})
	model.height = 10

	list := model.renderListView()
	if !strings.Contains(list, "Feed Preview - Provider (1 items)") || !strings.Contains(list, "enter: view details") {
		t.Fatalf("renderListView() = %q", list)
	}
	if model.View() != list {
		t.Fatal("View() did not return list view content")
	}

	model.selectedIndex = 0
	model.viewMode = DetailViewMode
	detail := model.renderDetailView()
	if !strings.Contains(detail, "Title: Preview title") || !strings.Contains(detail, "esc: back to list") {
		t.Fatalf("renderDetailView() = %q", detail)
	}
	if model.View() != detail {
		t.Fatal("View() did not return detail view content")
	}

	model.viewMode = XMLViewMode
	xml := model.renderXMLView()
	if !strings.Contains(xml, "XML Entry Preview") || !strings.Contains(xml, "<entry><title>Preview title</title></entry>") {
		t.Fatalf("renderXMLView() = %q", xml)
	}
	if model.View() != xml {
		t.Fatal("View() did not return xml view content")
	}
}

func TestRenderViewsWithoutSelection(t *testing.T) {
	model := NewModel([]providers.FeedItem{mockFeedItem{title: "one", createdAt: time.Now()}}, "Provider", "preview", feed.Config{})
	if got := model.renderDetailView(); got != "No item selected" {
		t.Fatalf("renderDetailView() = %q, want no selection message", got)
	}
	if got := model.renderXMLView(); got != "No item selected" {
		t.Fatalf("renderXMLView() = %q, want no selection message", got)
	}
}

func TestRunWithEmptyItems(t *testing.T) {
	out := captureOutput(t, func() {
		if err := Run(nil, "Provider", "preview", feed.Config{}); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})
	if !strings.Contains(out, "No items to preview") {
		t.Fatalf("Run() output = %q", out)
	}
}
