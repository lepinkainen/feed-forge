package lobsters

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lepinkainen/feed-forge/pkg/providers"
)

func TestFetchItemsAgainstFixture(t *testing.T) {
	fixturePath := filepath.Join("testdata", "hottest.json")
	body, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(w, strings.NewReader(string(body)))
	}))
	defer srv.Close()

	items, err := fetchStories(srv.URL)
	if err != nil {
		t.Fatalf("fetchStories: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("fetched story count = %d, want 4", len(items))
	}

	var linkItem, textItem *Item
	for _, item := range items {
		if item.story.ShortID == "wxmi0v" {
			linkItem = item
		}
		if item.story.ShortID == "ab12cd" {
			textItem = item
		}
	}

	if linkItem == nil {
		t.Fatal("did not find expected link post in fixture")
	}
	if textItem == nil {
		t.Fatal("did not find expected text/Ask post in fixture")
	}

	// Link post: has an external url, so Link() and CommentsLink() differ.
	if linkItem.Link() == linkItem.CommentsLink() {
		t.Errorf("link post: Link() should differ from CommentsLink()")
	}
	if linkItem.Score() != 42 || linkItem.CommentCount() != 18 {
		t.Errorf("link post counts = (%d, %d), want (42, 18)", linkItem.Score(), linkItem.CommentCount())
	}

	// Text/Ask post: no url, so Link() falls back to CommentsLink().
	if textItem.Link() != textItem.CommentsLink() {
		t.Errorf("text post: Link() should equal CommentsLink()")
	}
	if textItem.Score() != 15 || textItem.CommentCount() != 34 {
		t.Errorf("text post counts = (%d, %d), want (15, 34)", textItem.Score(), textItem.CommentCount())
	}
	if textItem.Content() == "" {
		t.Errorf("text post content should not be empty")
	}
	if linkItem.Content() != "" {
		t.Errorf("link post content should be empty, got %q", linkItem.Content())
	}

	if got := linkItem.AuthorURI(); got != "https://lobste.rs/~carlana" {
		t.Errorf("linkItem.AuthorURI() = %q, want https://lobste.rs/~carlana", got)
	}
}

func TestFetchItemsMinScoreFilter(t *testing.T) {
	fixturePath := filepath.Join("testdata", "hottest.json")
	body, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(w, strings.NewReader(string(body)))
	}))
	defer srv.Close()

	items, err := fetchStories(srv.URL)
	if err != nil {
		t.Fatalf("fetchStories: %v", err)
	}

	const minScore = 20
	kept := 0
	for _, item := range items {
		if item.Score() >= minScore {
			kept++
		}
	}
	// wxmi0v (42) and zz9top (61) clear the bar; ab12cd (15) and q4rst9 (3) don't.
	if kept != 2 {
		t.Errorf("items with score >= %d = %d, want 2", minScore, kept)
	}
}

func TestRegistryRegistration(t *testing.T) {
	// The provider registers itself in init(). Smoke-test that the registry
	// produces the expected metadata and that the factory rejects bad config.
	info, err := providers.DefaultRegistry.Get("lobsters")
	if err != nil {
		t.Fatalf("registry lookup: %v", err)
	}
	if info == nil {
		t.Fatal("registry entry is nil")
	}
	if info.Name != "lobsters" || info.Preview == nil || info.Preview.TemplateName != "lobsters-atom" {
		t.Errorf("unexpected registry metadata: %+v", info)
	}

	if _, err := factory("not a config"); err == nil {
		t.Error("factory should reject wrong config type")
	}
}
