package slashdot

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/api"
	"github.com/lepinkainen/feed-forge/pkg/atom"
	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/httpcache"
	"github.com/lepinkainen/feed-forge/pkg/preview"
	"github.com/lepinkainen/feed-forge/pkg/providerfeed"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

func TestFetchItemsAgainstFixture(t *testing.T) {
	body, err := os.ReadFile("testdata/slashdot.atom")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.UserAgent(), "feed-forge/") {
			t.Errorf("unexpected user agent %q", r.UserAgent())
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write(body)
	}))
	defer server.Close()
	p := &Provider{feedURL: server.URL}
	for _, tc := range []struct{ limit, want int }{{0, 3}, {1, 1}, {10, 3}} {
		items, fetchErr := p.FetchItems(tc.limit)
		if fetchErr != nil {
			t.Fatal(fetchErr)
		}
		if len(items) != tc.want {
			t.Fatalf("limit %d: got %d items, want %d", tc.limit, len(items), tc.want)
		}
		item := items[0]
		if item.Title() != "Why Google Told Drivers to Drive a Longer Way On Purpose" {
			t.Errorf("unexpected newest title %q", item.Title())
		}
		if item.Author() != "EditorDavid" || item.CommentCount() != 6 || item.Score() != 0 {
			t.Errorf("unexpected metadata: %s, %d, %d", item.Author(), item.CommentCount(), item.Score())
		}
		if item.Link() != item.CommentsLink() || strings.Contains(item.Link(), "utm_") {
			t.Errorf("unexpected story URL %q", item.Link())
		}
		if !item.CreatedAt().Equal(time.Date(2026, 9, 8, 5, 34, 0, 0, time.UTC)) {
			t.Errorf("unexpected date %s", item.CreatedAt())
		}
		if categories := item.Categories(); len(categories) != 1 || categories[0] != "google" {
			t.Errorf("categories = %v", categories)
		}
		if !strings.Contains(item.Content(), "Google Maps") || strings.Contains(item.Content(), "share_submission") || strings.Contains(item.Content(), "Read more of this story") {
			t.Errorf("unexpected content %q", item.Content())
		}
	}
}

func TestFeedErrorsAndEncoding(t *testing.T) {
	for _, tc := range []struct {
		name, body, title string
		status            int
		wantError         bool
	}{
		{name: "malformed", body: "<feed", wantError: true},
		{name: "wrong format", body: "<html/>", wantError: true},
		{name: "not found", status: http.StatusNotFound, wantError: true},
		{name: "latin1", body: "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><feed xmlns=\"http://www.w3.org/2005/Atom\"><entry><title>Caf\xe9 &amp; &#34;tea&#34;</title><updated>2026-09-08T01:00:00Z</updated></entry></feed>", title: `Café & "tea"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			entries, err := fetchAtomFeed(server.URL, nil)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error %v", err, tc.wantError)
			}
			if !tc.wantError && (len(entries) != 1 || entries[0].Title != tc.title) {
				t.Fatalf("unexpected entries %+v", entries)
			}
		})
	}
}

func TestConditionalFetch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		steps  []string
		legacy bool
	}{
		{"preview then generate", []string{"preview", "generate", "preview"}, false},
		{"generate then preview", []string{"generate", "preview", "generate"}, false},
		{"validator-only cache", []string{"generate", "preview", "generate"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := os.ReadFile("testdata/slashdot.atom")
			if err != nil {
				t.Fatal(err)
			}
			var fullResponses, notModified int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("If-None-Match") == `"slashdot"` {
					notModified++
					w.WriteHeader(http.StatusNotModified)
					return
				}
				fullResponses++
				w.Header().Set("ETag", `"slashdot"`)
				_, _ = w.Write(body)
			}))
			defer server.Close()
			cachePath := filepath.Join(t.TempDir(), "http_cache.db")
			if tc.legacy {
				store, openErr := httpcache.NewStore(cachePath)
				if openErr != nil {
					t.Fatal(openErr)
				}
				if err := store.Save(server.URL, api.CacheValidators{ETag: `"slashdot"`}); err != nil {
					t.Fatal(err)
				}
				if err := store.Close(); err != nil {
					t.Fatal(err)
				}
			}
			for step, action := range tc.steps {
				// Reopen the persistent cache to model separate CLI invocations.
				store, openErr := httpcache.NewStore(cachePath)
				if openErr != nil {
					t.Fatal(openErr)
				}
				p := &Provider{BaseProvider: &providers.BaseProvider{HTTPCache: store}, feedURL: server.URL}
				var output string
				if action == "preview" {
					items, fetchErr := p.FetchItems(1)
					if fetchErr != nil {
						_ = store.Close()
						t.Fatalf("step %d preview: %v", step, fetchErr)
					}
					if len(items) != 1 {
						t.Fatalf("preview items = %d, want 1", len(items))
					}
					output = preview.FormatXMLItem(items[0], previewInfo.TemplateName, previewInfo.Config)
				} else {
					// Each generation targets a missing file, including after a 304.
					outfile := filepath.Join(t.TempDir(), "slashdot.xml")
					generate := providerfeed.BuildGenerator(p.FetchItems, previewInfo, nil, nil)
					if err := generate(outfile); err != nil {
						_ = store.Close()
						t.Fatalf("step %d generate: %v", step, err)
					}
					data, readErr := os.ReadFile(outfile)
					if readErr != nil {
						t.Fatal(readErr)
					}
					output = string(data)
				}
				if err := store.Close(); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(output, "Why Google Told Drivers to Drive a Longer Way On Purpose") {
					t.Fatalf("%s has no story: %s", action, output)
				}
			}
			if fullResponses != 1 || notModified != 2 {
				t.Fatalf("responses: 200=%d, 304=%d; want 1, 2", fullResponses, notModified)
			}
		})
	}
}

func TestMalformedEntryDoesNotDiscardSiblings(t *testing.T) {
	for _, tc := range []struct {
		name, updated, comments string
		wantItems, wantComments int
	}{
		{"invalid timestamp", "yesterday", "8", 1, 0},
		{"empty timestamp", "", "8", 1, 0},
		{"missing timestamp", "MISSING", "8", 1, 0},
		{"zero timestamp", "0001-01-01T00:00:00Z", "8", 1, 0},
		{"minute precision", "2026-09-08T02:30+00:00", "8", 2, 8},
		{"comma count", "2026-09-08T02:30:00Z", "1,234", 2, 1234},
		{"invalid count", "2026-09-08T02:30:00Z", "many", 2, 0},
		{"empty count", "2026-09-08T02:30:00Z", "", 2, 0},
		{"negative count", "2026-09-08T02:30:00Z", "-5", 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updated := "<updated>" + tc.updated + "</updated>"
			if tc.updated == "MISSING" {
				updated = ""
			}
			body := fmt.Sprintf(`<feed xmlns="http://www.w3.org/2005/Atom" xmlns:slash="http://purl.org/rss/1.0/modules/slash/">
<entry><id>https://slashdot.org/story/edge</id><title>Edge</title>%s<slash:comments>%s</slash:comments></entry>
<entry><id>https://slashdot.org/story/valid</id><title>Valid sibling</title><updated>2026-09-08T01:00:00Z</updated></entry></feed>`, updated, tc.comments)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			p := &Provider{feedURL: server.URL}
			items, err := p.FetchItems(0)
			if err != nil {
				t.Fatalf("one bad field discarded feed: %v", err)
			}
			if len(items) != tc.wantItems {
				t.Fatalf("items = %d, want %d", len(items), tc.wantItems)
			}
			if items[len(items)-1].Title() != "Valid sibling" {
				t.Fatal("valid sibling missing")
			}
			if tc.wantItems == 2 {
				if items[0].CommentCount() != tc.wantComments {
					t.Errorf("comments = %d, want %d", items[0].CommentCount(), tc.wantComments)
				}
				if !items[0].CreatedAt().Equal(time.Date(2026, 9, 8, 2, 30, 0, 0, time.UTC)) {
					t.Errorf("date = %s", items[0].CreatedAt())
				}
			}
		})
	}
}

func TestTitleTextRoundTrip(t *testing.T) {
	for _, tc := range []struct{ name, raw, want string }{
		{"whitespace", "  Headline \n\t", "Headline"},
		{"literal entities", "Use &amp;lt;template&amp;gt; &amp;amp; &amp;copy; literally", "Use &lt;template&gt; &amp; &copy; literally"},
		{"XML entities", "Use &lt;template&gt; &amp; quotes &#34;like this&#34;", `Use <template> & quotes "like this"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>https://slashdot.org/story/title</id><title>` + tc.raw + `</title><updated>2026-09-08T01:00:00Z</updated></entry></feed>`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			p := &Provider{feedURL: server.URL}
			items, err := p.FetchItems(0)
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 1 {
				t.Fatalf("items = %d", len(items))
			}
			if items[0].Title() != tc.want {
				t.Errorf("title = %q, want %q", items[0].Title(), tc.want)
			}
			output, err := feed.GenerateAtomFeedWithEmbeddedTemplate(items, previewInfo.TemplateName, previewInfo.Config, nil)
			if err != nil {
				t.Fatal(err)
			}
			var rendered struct {
				Title string `xml:"entry>title"`
			}
			if err := xml.Unmarshal([]byte(output), &rendered); err != nil {
				t.Fatal(err)
			}
			if rendered.Title != tc.want {
				t.Errorf("rendered title = %q, want %q", rendered.Title, tc.want)
			}
		})
	}
}

func TestItemAndAtomRendering(t *testing.T) {
	entry, ok := parseEntry(atomEntry{
		ID:    "https://slashdot.org/story/example?utm_source=feed&keep=yes",
		Title: `A & B < C`, Author: "Editor", Updated: atom.Time{Time: time.Date(2026, 9, 8, 1, 0, 0, 0, time.UTC)}, Comments: "12",
		Summary: atomText{Type: "html", Text: `<p>Body &amp; a literal ]]&gt; marker</p>`},
	})
	if !ok {
		t.Fatal("parseEntry rejected a valid entry")
	}
	item := &Item{entry: entry}
	if item.Link() != "https://slashdot.org/story/example?keep=yes" {
		t.Fatalf("ID fallback = %q", item.Link())
	}
	output, err := feed.GenerateAtomFeedWithEmbeddedTemplate([]providers.FeedItem{item}, previewInfo.TemplateName, previewInfo.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Entries []struct {
			Title   string `xml:"title"`
			Content string `xml:"content"`
			ID      string `xml:"id"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid generated Atom: %v\n%s", err, output)
	}
	if len(parsed.Entries) != 1 {
		t.Fatalf("entries = %d", len(parsed.Entries))
	}
	got := parsed.Entries[0]
	if got.Title != item.Title() || got.ID != item.Link() || !strings.Contains(got.Content, item.Content()) || !strings.Contains(got.Content, "12") {
		t.Fatalf("unexpected entry %+v", got)
	}
}

// TestSummaryTextConstructs covers every Atom text construct type: html is
// used as-is, xhtml is unwrapped from its <div>, plain text is escaped.
func TestSummaryTextConstructs(t *testing.T) {
	for _, tc := range []struct{ name, summary, want string }{
		{"html", `<summary type="html">&lt;p&gt;Body &amp;amp; more&lt;/p&gt;</summary>`, "<p>Body &amp; more</p>"},
		{"xhtml", `<summary type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml"><p>Body <a href="x">link</a></p></div></summary>`, `<p>Body <a href="x">link</a></p>`},
		{"text", `<summary>&lt;script&gt;plain text&lt;/script&gt;</summary>`, "<p>&lt;script&gt;plain text&lt;/script&gt;</p>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>https://slashdot.org/story/x</id><title>T</title><updated>2026-09-08T01:00:00Z</updated>` + tc.summary + `</entry></feed>`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			p := &Provider{feedURL: server.URL}
			items, err := p.FetchItems(0)
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 1 || items[0].Content() != tc.want {
				t.Fatalf("content = %q, want %q", items[0].Content(), tc.want)
			}
		})
	}
}

// TestFooterStrippingToleratesMarkupVariants guards the footer regex against
// upstream attribute reordering: any <div> carrying class="share_submission"
// starts the footer, whatever else the tag or the preceding <p> carries.
func TestFooterStrippingToleratesMarkupVariants(t *testing.T) {
	const story = `<p>Story body.</p>`
	for _, tc := range []struct{ name, footer string }{
		{"current", `<p><div class="share_submission" style="position:relative;"><a class="slashpop" href="https://twitter.com/x">Share</a></div></p><p>Read more of this story at Slashdot.</p>`},
		{"reordered attributes", `<div style="position:relative;" class="share_submission"><a href="x">Share</a></div>`},
		{"paragraph with attributes", `<p class="share"><div id="s" class="share_submission"><a href="x">Share</a></div></p>`},
		{"no paragraph", `<div class="share_submission"></div><p>Read more of this story at Slashdot.</p>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry, ok := parseEntry(atomEntry{
				ID: "https://slashdot.org/story/x", Updated: atom.Time{Time: time.Date(2026, 9, 8, 1, 0, 0, 0, time.UTC)},
				Summary: atomText{Type: "html", Text: story + tc.footer},
			})
			if !ok {
				t.Fatal("parseEntry rejected a valid entry")
			}
			if entry.content != story {
				t.Errorf("content = %q, want %q", entry.content, story)
			}
		})
	}
	// A summary without a footer is left alone.
	entry, _ := parseEntry(atomEntry{Updated: atom.Time{Time: time.Now()}, Summary: atomText{Type: "html", Text: story}})
	if entry.content != story {
		t.Errorf("footerless content = %q", entry.content)
	}
}

// TestParagraphs guards the paragraph reconstruction for Slashdot's
// tag-stripped bodies: blank lines become <p> boundaries, existing block
// markup is left alone.
func TestParagraphs(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"empty", "", ""},
		{"single", "One paragraph.", "<p>One paragraph.</p>"},
		{"blank line with space", "First.\n \nSecond.", "<p>First.</p><p>Second.</p>"},
		{"multiple blank lines", "First.\n\n\n\nSecond.\r\n\r\nThird.", "<p>First.</p><p>Second.</p><p>Third.</p>"},
		{"single newline kept", "Line one\nline two", "<p>Line one\nline two</p>"},
		{"inline tags still wrapped", "See <a href=\"x\">here</a>.\n\nMore.", "<p>See <a href=\"x\">here</a>.</p><p>More.</p>"},
		{"existing paragraphs untouched", "<p>A</p>\n\n<p>B</p>", "<p>A</p>\n\n<p>B</p>"},
		{"existing br untouched", "A<br/>\n\nB", "A<br/>\n\nB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := paragraphs(tc.in); got != tc.want {
				t.Errorf("paragraphs(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRegistryRegistration(t *testing.T) {
	info, err := providers.DefaultRegistry.Get("slashdot")
	if err != nil {
		t.Fatal(err)
	}
	cfg, ok := info.ConfigFactory().(*Config)
	if !ok || cfg.Interval != "30m" || info.Preview.TemplateName != "slashdot-atom" {
		t.Fatalf("unexpected registry defaults: %+v", info)
	}
	if _, err := factory("invalid"); err == nil {
		t.Error("expected invalid configuration error")
	}
}
