package tildes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		votes, comments := parseVotesAndComments(entries[i].Content.HTML())
		item := &Item{
			entry:        entries[i],
			group:        group,
			cleanContent: cleanContent(entries[i].Content.HTML()),
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

func TestAtomCharsetAndTextPreserved(t *testing.T) {
	body := "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><feed xmlns=\"http://www.w3.org/2005/Atom\"><entry><title>Caf\xe9</title><id>https://tildes.net/topic</id><updated>2026-09-08T00:00:00Z</updated><content type=\"html\">&lt;p&gt;Text &amp;amp; entities&lt;/p&gt;</content></entry></feed>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer srv.Close()
	entries, err := fetchAtomFeed(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Title != "Café" || entries[0].Content.HTML() != "<p>Text &amp; entities</p>" {
		t.Fatalf("entries = %+v", entries)
	}
}

// TestMalformedTimestampDoesNotDiscardFeed guards against one bad <updated>
// aborting the whole feed: the entry decodes with a zero time and every
// sibling survives.
func TestMalformedTimestampDoesNotDiscardFeed(t *testing.T) {
	body := `<feed xmlns="http://www.w3.org/2005/Atom">
<entry><title>Minute precision</title><id>https://tildes.net/a</id><updated>2026-09-08T02:30+00:00</updated><content type="html">a</content></entry>
<entry><title>Garbage</title><id>https://tildes.net/b</id><updated>yesterday</updated><content type="html">b</content></entry>
<entry><title>Empty</title><id>https://tildes.net/c</id><updated></updated><content type="html">c</content></entry>
<entry><title>Valid</title><id>https://tildes.net/d</id><updated>2026-09-08T01:00:00Z</updated><content type="html">d</content></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer srv.Close()
	entries, err := fetchAtomFeed(srv.URL)
	if err != nil {
		t.Fatalf("one malformed timestamp discarded the feed: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("entries = %d, want 4", len(entries))
	}
	if got := entries[0].Updated.Time; !got.Equal(time.Date(2026, 9, 8, 2, 30, 0, 0, time.UTC)) {
		t.Errorf("minute precision parsed as %s", got)
	}
	if !entries[1].Updated.IsZero() || !entries[2].Updated.IsZero() {
		t.Errorf("malformed timestamps should be zero: %s, %s", entries[1].Updated, entries[2].Updated)
	}
	if entries[3].Updated.IsZero() {
		t.Error("valid sibling lost its timestamp")
	}
}
