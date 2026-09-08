package bulletin

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/testutil"
)

func TestWriteAtomPreservesContent(t *testing.T) {
	const content = "<p>see <code>a[i]]>b</code> &amp; text\x00\x1f</p>"
	const want = "<p>see <code>a[i]]>b</code> &amp; text</p>"
	out := filepath.Join(t.TempDir(), "bulletin.xml")
	if err := writeAtom(out, "https://feeds.example/bulletin.xml", []Row{{ID: 1, Content: content}}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Content string `xml:"entry>content"`
	}
	if err := xml.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(parsed.Content); got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

// TestWriteAtomGolden locks the Atom feed output shape, including RFC3339 stamps
// and the CDATA "]]>" sanitisation. Regenerate with: task update-golden.
func TestWriteAtomGolden(t *testing.T) {
	bulletins := []Row{
		{
			ID:          2,
			PublishedAt: time.Date(2026, 7, 1, 18, 0, 0, 0, time.UTC),
			Slot:        "Evening",
			Title:       bulletinTitle("Evening", time.Date(2026, 7, 1, 18, 0, 0, 0, time.UTC)),
			Content:     "<h2>Technology</h2>\n<p>A chip launched. <a href=\"https://x/1\">[1]</a></p>",
		},
		{
			ID:          1,
			PublishedAt: time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC),
			Slot:        "Morning",
			Title:       bulletinTitle("Morning", time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)),
			// Contains a raw ]]> that must be split so the feed stays valid XML.
			Content: "<p>Edge case with a ]]> terminator inside.</p>",
		},
	}

	out := filepath.Join(t.TempDir(), "bulletin.xml")
	if err := writeAtom(out, "https://feeds.example/bulletin.xml", bulletins); err != nil {
		t.Fatalf("writeAtom: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	testutil.CompareGolden(t, filepath.Join("testdata", "bulletin-atom.xml.golden"), string(data))
}

func TestWriteHTMLPages(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	digest := "<h2>Science</h2>\n<p>A telescope switched on. <a href=\"https://x/1\">[1]</a></p>"
	row := Row{
		PublishedAt: now,
		Slot:        "Morning",
		Title:       bulletinTitle("Morning", now),
		Content:     digest,
	}

	if err := writeDatedHTML(dir, row); err != nil {
		t.Fatalf("writeDatedHTML: %v", err)
	}
	if err := writeLatestHTML(dir, row); err != nil {
		t.Fatalf("writeLatestHTML: %v", err)
	}

	dated := filepath.Join(dir, "bulletin-2026-07-01-morning.html")
	latest := filepath.Join(dir, LatestPageName)

	for _, path := range []string{dated, latest} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(data)
		if !strings.Contains(body, digest) {
			t.Errorf("%s missing digest fragment", filepath.Base(path))
		}
		if !strings.Contains(body, "Morning Bulletin") {
			t.Errorf("%s missing title", filepath.Base(path))
		}
		if !strings.HasPrefix(body, "<!DOCTYPE html>") {
			t.Errorf("%s is not a full HTML document", filepath.Base(path))
		}
	}
}
