package bulletin

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
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

	if err := writeDatedHTML(dir, row, 1); err != nil {
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

// Serve fires digests at local wall-clock slots, so the slot label must come
// from the local hour, not the UTC hour: a 07:45 digest in New York is 12:45
// UTC in winter and would be titled "Afternoon Bulletin".
func TestGenerateLabelsSlotFromLocalTime(t *testing.T) {
	// Pick a zone whose label differs from the UTC label right now.
	utcLabel := slotFor(time.Now().UTC())
	var zone *time.Location
	for off := -12; off <= 14; off++ {
		z := time.FixedZone("test", off*3600)
		if slotFor(time.Now().In(z)) != utcLabel {
			zone = z
			break
		}
	}
	prevLocal := time.Local
	time.Local = zone
	t.Cleanup(func() { time.Local = prevLocal })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"m",` +
			`"content":[{"type":"text","text":"<p>digest</p>"}],"stop_reason":"end_turn",` +
			`"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)

	dbPath := filepath.Join(t.TempDir(), "bulletin.db")
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if _, err := s.InsertItem(testCtx, sampleItem("https://x/1")); err != nil {
		t.Fatalf("insert: %v", err)
	}
	_ = s.Close()

	if err := Generate(testCtx, GenerateOptions{DBPath: dbPath, APIKey: "test"}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	s, err = NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = s.Close() }()
	rows, err := s.AllBulletins(testCtx)
	if err != nil || len(rows) != 1 {
		t.Fatalf("AllBulletins = %d rows, err %v", len(rows), err)
	}
	want := slotFor(rows[0].PublishedAt.In(time.Local))
	if rows[0].Slot != want {
		t.Errorf("slot label = %q (UTC hour), want %q (local hour)", rows[0].Slot, want)
	}
}

// Two bulletins with the same date and slot label (for example slots 07:45 and
// 11:30, both "Morning") must each keep an archive page.
func TestRenderAllHTMLKeepsEveryArchivePage(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	older := Row{ID: 1, PublishedAt: day.Add(4*time.Hour + 45*time.Minute), Slot: "Morning", Content: "<p>older digest</p>"}
	newer := Row{ID: 2, PublishedAt: day.Add(8*time.Hour + 30*time.Minute), Slot: "Morning", Content: "<p>newer digest</p>"}
	older.Title = bulletinTitle(older.Slot, older.PublishedAt)
	newer.Title = bulletinTitle(newer.Slot, newer.PublishedAt)

	// Newest first, as Store.AllBulletins returns them.
	if err := renderAllHTML(dir, []Row{newer, older}); err != nil {
		t.Fatalf("renderAllHTML: %v", err)
	}

	pages, err := filepath.Glob(filepath.Join(dir, "bulletin-2026-*.html"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var archive strings.Builder
	for _, p := range pages {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		archive.Write(data)
	}
	for _, b := range []Row{newer, older} {
		if !strings.Contains(archive.String(), b.Content) {
			t.Errorf("bulletin %d has no archive page (%d dated pages for 2 bulletins)", b.ID, len(pages))
		}
	}
}
