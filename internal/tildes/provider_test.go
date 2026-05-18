package tildes

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
	fixturePath := filepath.Join("testdata", "topics.atom")
	body, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = io.Copy(w, strings.NewReader(string(body)))
	}))
	defer srv.Close()

	entries, err := fetchAtomFeed(srv.URL)
	if err != nil {
		t.Fatalf("fetchAtomFeed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("fetched entry count = %d, want 3", len(entries))
	}

	// Exercise the same mapping used inside Provider.FetchItems without
	// spinning up the BaseProvider (which would create real databases).
	const group = "~tech"

	var linkItem, textItem *Item
	for i := range entries {
		votes, comments := parseVotesAndComments(entries[i].Content)
		item := &Item{
			entry:        entries[i],
			group:        group,
			cleanContent: cleanContent(entries[i].Content),
			votes:        votes,
			commentCount: comments,
		}
		if strings.Contains(item.Link(), "arstechnica.com") {
			linkItem = item
		}
		if strings.Contains(item.entry.Title, "MP3 player") {
			textItem = item
		}
	}

	if linkItem == nil {
		t.Fatal("did not find expected link post in fixture")
	}
	if textItem == nil {
		t.Fatal("did not find expected text post in fixture")
	}

	if linkItem.Score() != 20 || linkItem.CommentCount() != 7 {
		t.Errorf("link post counts = (%d, %d), want (20, 7)", linkItem.Score(), linkItem.CommentCount())
	}
	if textItem.Score() != 22 || textItem.CommentCount() != 29 {
		t.Errorf("text post counts = (%d, %d), want (22, 29)", textItem.Score(), textItem.CommentCount())
	}
	if textItem.Link() != textItem.CommentsLink() {
		t.Errorf("text post: Link() should equal CommentsLink()")
	}
	if textItem.Content() == "" {
		t.Errorf("text post content should not be empty after cleaning")
	}
	if linkItem.Content() != "" {
		t.Errorf("link post content should be empty after cleaning, got %q", linkItem.Content())
	}
}

func TestRegistryRegistration(t *testing.T) {
	// The provider registers itself in init(). Smoke-test that the registry
	// produces the expected metadata and that the factory rejects bad config.
	info, err := providers.DefaultRegistry.Get("tildes")
	if err != nil {
		t.Fatalf("registry lookup: %v", err)
	}
	if info == nil {
		t.Fatal("registry entry is nil")
	}
	if info.Name != "tildes" || info.Preview == nil || info.Preview.TemplateName != "tildes-atom" {
		t.Errorf("unexpected registry metadata: %+v", info)
	}

	if _, err := factory("not a config"); err == nil {
		t.Error("factory should reject wrong config type")
	}
}
