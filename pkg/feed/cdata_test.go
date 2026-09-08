package feed

import (
	"encoding/xml"
	"html"
	"io/fs"
	"strings"
	"testing"
	"text/template"

	"github.com/lepinkainen/feed-forge/pkg/linkpreview"
	"github.com/lepinkainen/feed-forge/templates"
)

func TestCDATAFragmentRoundTrip(t *testing.T) {
	fn, ok := TemplateFuncs()["cdata"].(func(string) string)
	if !ok {
		t.Fatal("missing shared cdata function")
	}
	for _, tc := range []struct{ raw, want string }{
		{"", ""},
		{`<p>A &amp; B</p>`, `<p>A &amp; B</p>`},
		{"]]>middle]]>", "]]>middle]]>"},
		{"bad\x00\x1f\ufffechars", "badchars"},
		{"]]\x00>", "]]>"},
	} {
		var parsed struct {
			Text string `xml:",chardata"`
		}
		output := "<content><![CDATA[" + fn(tc.raw) + "]]></content>"
		if err := xml.Unmarshal([]byte(output), &parsed); err != nil {
			t.Fatalf("invalid XML for %q: %v", tc.raw, err)
		}
		if parsed.Text != tc.want {
			t.Errorf("round trip = %q, want %q", parsed.Text, tc.want)
		}
	}
}

func TestAllAtomTemplatesPreserveContent(t *testing.T) {
	names, err := fs.Glob(templates.EmbeddedTemplates, "*-atom.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		for _, field := range []string{"content", "excerpt"} {
			if field == "excerpt" && name != "reddit-atom.tmpl" && name != "hackernews-atom.tmpl" && name != "lobsters-atom.tmpl" && name != "lemmy-atom.tmpl" && name != "tildes-atom.tmpl" {
				continue
			}
			t.Run(name+"/"+field, func(t *testing.T) {
				const raw = "<p>Body &amp; literal ]]> marker\x00\x1f</p>"
				const clean = "<p>Body &amp; literal ]]> marker</p>"
				item := TemplateItem{Title: "Story", ID: "https://example.com/story", Link: "https://example.com/article", CommentsLink: "https://example.com/story", Updated: "2026-09-08T00:00:00Z", Published: "2026-09-08T00:00:00Z"}
				previews := map[string]*linkpreview.Data{}
				if field == "content" {
					item.Content = raw
				} else {
					previews[item.Link] = &linkpreview.Data{Excerpt: raw}
				}
				data := struct {
					TemplateData
					Entries            []TemplateItem
					SelfLink, Subtitle string
				}{TemplateData: TemplateData{FeedTitle: "Feed", FeedID: "urn:feed", Updated: item.Updated, Items: []TemplateItem{item}, LinkPreviews: previews}, Entries: []TemplateItem{item}}
				body, err := templates.EmbeddedTemplates.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				tmpl, err := template.New(name).Funcs(TemplateFuncs()).Parse(string(body))
				if err != nil {
					t.Fatal(err)
				}
				var out strings.Builder
				if err := tmpl.Execute(&out, data); err != nil {
					t.Fatal(err)
				}
				var parsed struct {
					Contents []string `xml:"entry>content"`
				}
				if err := xml.Unmarshal([]byte(out.String()), &parsed); err != nil {
					t.Fatalf("invalid generated XML: %v", err)
				}
				want := clean
				if name == "youtube-atom.tmpl" {
					want = html.EscapeString(clean)
				}
				if len(parsed.Contents) != 1 || !strings.Contains(parsed.Contents[0], want) {
					t.Fatalf("content lost or double-escaped: %q, want %q", parsed.Contents, want)
				}
			})
		}
	}
}
