package bulletin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/testutil"
)

func TestCDATASafe(t *testing.T) {
	// A digest containing a literal "]]>" must not be able to close the CDATA
	// section early; the sequence is split but the visible text is unchanged.
	in := "<p>see <code>a[i]]>b</code></p>"
	got := cdataSafe(in)
	// Split closes and reopens the CDATA around the ">"; wrapped and re-parsed,
	// the visible text is unchanged, so no raw "]]>" can terminate the section.
	if want := "<p>see <code>a[i]]]]><![CDATA[>b</code></p>"; got != want {
		t.Errorf("cdataSafe = %q, want %q", got, want)
	}
	if recovered := strings.ReplaceAll(got, "]]><![CDATA[", ""); recovered != in {
		t.Errorf("round-trip through CDATA changed text: %q != %q", recovered, in)
	}
	if cdataSafe("no terminator here") != "no terminator here" {
		t.Error("cdataSafe altered text with no ]]> sequence")
	}
}

// TestWriteAtomGolden locks the Atom feed output shape, including RFC3339 stamps
// and the CDATA "]]>" sanitisation. Regenerate with: task update-golden (or
// go test ./internal/bulletin/ -run TestWriteAtomGolden -update).
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
