package tildes

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T) []atomEntry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "topics.atom"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var feed atomFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return feed.Entries
}

func TestParseFixture(t *testing.T) {
	entries := loadFixture(t)
	if got, want := len(entries), 3; got != want {
		t.Fatalf("entry count = %d, want %d", got, want)
	}

	// Entry 0: text post — alternate link points back to the Tildes topic itself.
	textEntry := entries[0]
	if textEntry.ID != "https://tildes.net/~tech/1u7a/mp3_player_recommendations" {
		t.Errorf("text post id = %q", textEntry.ID)
	}
	if textEntry.alternateHref() != textEntry.ID {
		t.Errorf("text post alternate href = %q, want id %q", textEntry.alternateHref(), textEntry.ID)
	}
	if textEntry.Author.Name != "eggy" {
		t.Errorf("text post author = %q", textEntry.Author.Name)
	}
	if textEntry.Updated.IsZero() {
		t.Errorf("text post updated should parse to non-zero time.Time")
	}

	// Entry 1: link post — alternate link points to the external article.
	linkEntry := entries[1]
	const wantExternal = "https://arstechnica.com/ai/2026/05/energy-supplier-abandons-lake-tahoe-residents-to-serve-data-centers/"
	if linkEntry.alternateHref() != wantExternal {
		t.Errorf("link post alternate href = %q, want %q", linkEntry.alternateHref(), wantExternal)
	}
	if linkEntry.ID == wantExternal {
		t.Errorf("link post id should be the tildes topic, not the external URL; got %q", linkEntry.ID)
	}
}

func TestItemLinkSemantics(t *testing.T) {
	entries := loadFixture(t)

	textItem := &Item{entry: entries[0], group: "~tech"}
	if textItem.Link() != textItem.CommentsLink() {
		t.Errorf("text post: Link() != CommentsLink() (%q vs %q)", textItem.Link(), textItem.CommentsLink())
	}
	if !strings.HasPrefix(textItem.Title(), "[~tech] ") {
		t.Errorf("Title() missing [~tech] prefix: %q", textItem.Title())
	}
	if textItem.AuthorURI() != "https://tildes.net/user/eggy" {
		t.Errorf("AuthorURI() = %q", textItem.AuthorURI())
	}
	if got := textItem.Categories(); len(got) != 1 || got[0] != "~tech" {
		t.Errorf("Categories() = %v", got)
	}

	linkItem := &Item{entry: entries[1], group: "~tech"}
	if linkItem.Link() == linkItem.CommentsLink() {
		t.Errorf("link post: Link() should differ from CommentsLink(); both = %q", linkItem.Link())
	}
	if !strings.Contains(linkItem.Link(), "arstechnica.com") {
		t.Errorf("link post Link() = %q, expected arstechnica.com", linkItem.Link())
	}
	if !strings.Contains(linkItem.CommentsLink(), "tildes.net") {
		t.Errorf("link post CommentsLink() = %q, expected tildes.net", linkItem.CommentsLink())
	}
}

func TestParseVotesAndComments(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantVotes    int
		wantComments int
	}{
		{
			name:         "both present",
			body:         "<p>Votes: 22</p>\n<p>Comments: 29</p>",
			wantVotes:    22,
			wantComments: 29,
		},
		{
			name:         "missing both",
			body:         "<p>Just some content with no counters.</p>",
			wantVotes:    0,
			wantComments: 0,
		},
		{
			name:         "only votes",
			body:         "Votes: 5\nNo comments line here.",
			wantVotes:    5,
			wantComments: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotV, gotC := parseVotesAndComments(tc.body)
			if gotV != tc.wantVotes || gotC != tc.wantComments {
				t.Errorf("parseVotesAndComments() = (%d, %d), want (%d, %d)", gotV, gotC, tc.wantVotes, tc.wantComments)
			}
		})
	}
}

func TestNormalizeTopic(t *testing.T) {
	cases := map[string]string{
		"tech":      "tech",
		"~tech":     "tech",
		"  ~tech  ": "tech",
		"":          "",
		"~":         "",
	}
	for in, want := range cases {
		if got := normalizeTopic(in); got != want {
			t.Errorf("normalizeTopic(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeTopics(t *testing.T) {
	got := normalizeTopics("~tech", []string{"science", " tech ", "~games", ""})
	want := []string{"tech", "science", "games"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildFeedURL(t *testing.T) {
	if got, want := buildFeedURL("tech"), "https://tildes.net/~tech/topics.atom"; got != want {
		t.Errorf("buildFeedURL(\"tech\") = %q, want %q", got, want)
	}
	if got, want := buildFeedURL("science"), "https://tildes.net/~science/topics.atom"; got != want {
		t.Errorf("buildFeedURL(\"science\") = %q, want %q", got, want)
	}
}

func TestCleanContent(t *testing.T) {
	const linkPostBody = `
                    <p>Link URL: <a href="https://example.com/article">https://example.com/article</a></p>
                <p>Comments URL: <a href="https://tildes.net/~tech/x/y">https://tildes.net/~tech/x/y</a></p>
                <p>Votes: 20</p>
                <p>Comments: 7</p>
            `
	if got := cleanContent(linkPostBody); got != "" {
		t.Errorf("link-post body should clean to empty; got %q", got)
	}

	const textPostBody = `<p>The actual post body.</p>
                    <hr/>
                <p>Comments URL: <a href="https://tildes.net/~tech/x/y">link</a></p>
                <p>Votes: 22</p>
                <p>Comments: 29</p>
            `
	cleaned := cleanContent(textPostBody)
	if !strings.Contains(cleaned, "The actual post body.") {
		t.Errorf("cleaned content missing body: %q", cleaned)
	}
	if strings.Contains(cleaned, "Votes:") || strings.Contains(cleaned, "Comments URL:") {
		t.Errorf("cleaned content still has footer: %q", cleaned)
	}
}
