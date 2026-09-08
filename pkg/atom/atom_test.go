package atom

import (
	"testing"
	"time"
)

func TestDecodePreservesTextTypesAndExtensions(t *testing.T) {
	type entry struct {
		Title    string `xml:"title"`
		Summary  Text   `xml:"summary"`
		Comments string `xml:"http://purl.org/rss/1.0/modules/slash/ comments"`
	}
	body := []byte(`<?xml version="1.0" encoding="ISO-8859-1"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:slash="http://purl.org/rss/1.0/modules/slash/">
<entry><title>Caf` + "\xe9" + ` &amp;lt;template&amp;gt;</title><summary type="html">&lt;p&gt;A &amp;amp; B&lt;/p&gt;</summary><slash:comments>1,234</slash:comments></entry>
<entry><summary>&lt;literal&gt;</summary><slash:comments>unknown</slash:comments></entry>
</feed>`)
	feed, err := Decode[Feed[entry]](body)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(feed.Entries))
	}
	first, second := feed.Entries[0], feed.Entries[1]
	if first.Title != "Café &lt;template&gt;" || first.Summary.Type != "html" || first.Summary.Text != "<p>A &amp; B</p>" || first.Comments != "1,234" {
		t.Errorf("first entry = %#v", first)
	}
	if second.Summary.Type != "" || second.Summary.Text != "<literal>" || second.Comments != "unknown" {
		t.Errorf("second entry = %#v", second)
	}
}

func TestDecodeRejectsInvalidFeed(t *testing.T) {
	for _, body := range []string{
		`<feed xmlns="http://www.w3.org/2005/Atom"><entry></feed>`,
		`<rss/>`,
		`<feed xmlns="urn:other"/>`,
		`<?xml version="1.0" encoding="unknown-charset"?><feed xmlns="http://www.w3.org/2005/Atom"/>`,
	} {
		if _, err := Decode[Feed[struct{}]]([]byte(body)); err == nil {
			t.Errorf("Decode(%q) succeeded, want error", body)
		}
	}
}

func TestTimeIsLenient(t *testing.T) {
	type entry struct {
		Updated Time `xml:"updated"`
	}
	for _, tc := range []struct {
		raw  string
		want time.Time
	}{
		{"2026-09-08T02:30:15Z", time.Date(2026, 9, 8, 2, 30, 15, 0, time.UTC)},
		{"2026-09-08T02:30:15.5+02:00", time.Date(2026, 9, 8, 0, 30, 15, 500000000, time.UTC)},
		{"2026-09-08T02:30+00:00", time.Date(2026, 9, 8, 2, 30, 0, 0, time.UTC)},
		{" 2026-09-08T02:30:00Z\n", time.Date(2026, 9, 8, 2, 30, 0, 0, time.UTC)},
		{"", time.Time{}},
		{"yesterday", time.Time{}},
		{"2026-09-08", time.Time{}},
	} {
		body := `<feed xmlns="http://www.w3.org/2005/Atom"><entry><updated>` + tc.raw + `</updated></entry></feed>`
		feed, err := Decode[Feed[entry]]([]byte(body))
		if err != nil {
			t.Fatalf("Decode(%q): %v", tc.raw, err)
		}
		if got := feed.Entries[0].Updated.Time; !got.Equal(tc.want) {
			t.Errorf("updated %q = %s, want %s", tc.raw, got, tc.want)
		}
	}
}

func TestTextHTML(t *testing.T) {
	type entry struct {
		Summary Text `xml:"summary"`
	}
	for _, tc := range []struct{ name, raw, want string }{
		{"html", `<summary type="html">&lt;p&gt;A &amp;amp; B&lt;/p&gt;</summary>`, "<p>A &amp; B</p>"},
		{"text", `<summary>a &lt; b</summary>`, "a &lt; b"},
		{"text explicit", `<summary type="text">&lt;script&gt;</summary>`, "&lt;script&gt;"},
		{"xhtml", `<summary type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml"><p>Body <a href="x">link</a></p></div></summary>`, `<p>Body <a href="x">link</a></p>`},
		{"xhtml without div", `<summary type="xhtml"><p>Body</p></summary>`, `<p>Body</p>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `<feed xmlns="http://www.w3.org/2005/Atom"><entry>` + tc.raw + `</entry></feed>`
			feed, err := Decode[Feed[entry]]([]byte(body))
			if err != nil {
				t.Fatal(err)
			}
			if got := feed.Entries[0].Summary.HTML(); got != tc.want {
				t.Errorf("HTML() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAlternateHref(t *testing.T) {
	for _, tc := range []struct {
		name  string
		links []Link
		want  string
	}{
		{"none", nil, ""},
		{"explicit alternate", []Link{{Rel: "self", Href: "s"}, {Rel: "alternate", Href: "a"}}, "a"},
		{"empty rel", []Link{{Rel: "replies", Href: "r"}, {Href: "e"}}, "e"},
		{"skips empty href", []Link{{Rel: "alternate"}, {Rel: "alternate", Href: "second"}}, "second"},
		{"only other rels", []Link{{Rel: "self", Href: "s"}}, ""},
	} {
		if got := AlternateHref(tc.links); got != tc.want {
			t.Errorf("%s: AlternateHref = %q, want %q", tc.name, got, tc.want)
		}
	}
}
