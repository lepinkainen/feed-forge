package oglaf

import (
	"fmt"
	"testing"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/database"
)

func TestDatabaseSchema(t *testing.T) {
	db := newTestDB(t)

	// Test saving an RSS item
	item := &RSSItem{
		GUID:        "test-guid",
		Link:        "https://www.oglaf.com/test/",
		Title:       "Test Comic",
		Description: "Test description",
		PublishedAt: time.Now().UTC(),
		ImageURL:    "",
	}

	if err := saveRSSItem(db, item); err != nil {
		t.Fatalf("Failed to save RSS item: %v", err)
	}

	// Test retrieving the RSS item
	retrieved, err := getRSSItemByLink(db, item.Link)
	if err != nil {
		t.Fatalf("Failed to retrieve RSS item: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected to find RSS item but got nil")
	}

	if retrieved.GUID != item.GUID || retrieved.Title != item.Title {
		t.Errorf("Retrieved item doesn't match saved item")
	}

	// Test marking image as extracted
	imageURL := "https://media.oglaf.com/comic/test.jpg"
	if err := markImageExtracted(db, item.Link, imageURL); err != nil {
		t.Fatalf("Failed to mark image as extracted: %v", err)
	}

	// Test getting unprocessed comics (should be empty)
	unprocessed, err := getUnprocessedComics(db, 10)
	if err != nil {
		t.Fatalf("Failed to get unprocessed comics: %v", err)
	}
	if len(unprocessed) != 0 {
		t.Errorf("Expected 0 unprocessed comics, got %d", len(unprocessed))
	}

	// Test getting processed comics (should have 1)
	processed, err := getProcessedComics(db, 10)
	if err != nil {
		t.Fatalf("Failed to get processed comics: %v", err)
	}
	if len(processed) != 1 {
		t.Errorf("Expected 1 processed comic, got %d", len(processed))
	}
}

func TestNewRSSItemsDetection(t *testing.T) {
	db := newTestDB(t)

	// Create test items
	existingItem := &RSSItem{
		GUID:        "existing-guid",
		Link:        "https://www.oglaf.com/existing/",
		Title:       "Existing Comic",
		Description: "Existing description",
		PublishedAt: time.Now().Add(-1 * time.Hour).UTC(),
	}

	newItem := &RSSItem{
		GUID:        "new-guid",
		Link:        "https://www.oglaf.com/new/",
		Title:       "New Comic",
		Description: "New description",
		PublishedAt: time.Now().UTC(),
	}

	allItems := []*RSSItem{existingItem, newItem}

	// Save existing item
	if err := saveRSSItem(db, existingItem); err != nil {
		t.Fatalf("Failed to save existing item: %v", err)
	}

	// Test new item detection
	newItems, err := getNewRSSItems(db, allItems)
	if err != nil {
		t.Fatalf("Failed to detect new items: %v", err)
	}

	if len(newItems) != 1 {
		t.Errorf("Expected 1 new item, got %d", len(newItems))
	}

	if len(newItems) > 0 && newItems[0].Link != newItem.Link {
		t.Errorf("Expected new item with link %s, got %s", newItem.Link, newItems[0].Link)
	}
}

// newTestDB returns an in-memory database with the oglaf schema initialized.
func newTestDB(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.NewDatabase(database.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := initializeSchema(db); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}
	return db
}

// saveProcessed inserts an RSS item and marks its image as extracted so it
// appears in getProcessedComics results.
func saveProcessed(t *testing.T, db *database.Database, link string, published time.Time) {
	t.Helper()
	item := &RSSItem{
		GUID:        link,
		Link:        link,
		Title:       "Comic " + link,
		Description: "desc",
		PublishedAt: published.UTC(),
	}
	if err := saveRSSItem(db, item); err != nil {
		t.Fatalf("saveRSSItem(%s): %v", link, err)
	}
	if err := markImageExtracted(db, link, "https://media.oglaf.com/comic/"+link+".jpg"); err != nil {
		t.Fatalf("markImageExtracted(%s): %v", link, err)
	}
}

// TestGetProcessedComicsChronologicalOrder is the regression test for the
// broken ORDER BY pub_date bug. Before the fix, pub_date was an RFC1123Z
// string and lexicographic sort scrambled chronology: items starting with
// "Wed, 05 Jul 2023" sorted above "Thu, 13 Feb 2025". This test uses dates
// deliberately chosen so that RFC1123Z string sort would disagree with
// chronological sort.
func TestGetProcessedComicsChronologicalOrder(t *testing.T) {
	db := newTestDB(t)

	// Pick dates whose RFC1123Z form would string-sort in the WRONG order.
	// RFC1123Z: "Mon, 02 Jan 2006 15:04:05 -0700" — string sort compares
	// day-of-week first, then day-of-month, then month name, then year.
	fixtures := []struct {
		link     string
		time     time.Time
		wantRank int // 0 = newest
	}{
		{"comic-newest", time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC), 0},
		{"comic-middle", time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), 1},
		{"comic-oldest", time.Date(2010, 8, 30, 0, 0, 0, 0, time.UTC), 2},
	}

	// Insert in arbitrary (not chronological) order so we know the sort
	// is coming from the query, not insertion order.
	for _, f := range []int{1, 2, 0} {
		saveProcessed(t, db, fixtures[f].link, fixtures[f].time)
	}

	got, err := getProcessedComics(db, 10)
	if err != nil {
		t.Fatalf("getProcessedComics: %v", err)
	}
	if len(got) != len(fixtures) {
		t.Fatalf("got %d items, want %d", len(got), len(fixtures))
	}

	want := []string{"comic-newest", "comic-middle", "comic-oldest"}
	for i, item := range got {
		if item.Link() != want[i] {
			t.Errorf("position %d: got link %q, want %q (full order: %v)",
				i, item.Link(), want[i], linkOrder(got))
		}
	}
}

// TestGetUnprocessedComicsChronologicalOrder mirrors the processed-comics
// sort test for unprocessed items, since the same sort bug existed in both
// queries.
func TestGetUnprocessedComicsChronologicalOrder(t *testing.T) {
	db := newTestDB(t)

	// Insert items but do NOT mark images extracted, so they stay unprocessed.
	items := []struct {
		link string
		time time.Time
	}{
		{"unproc-2010", time.Date(2010, 8, 30, 0, 0, 0, 0, time.UTC)},
		{"unproc-2026", time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)},
		{"unproc-2023", time.Date(2023, 7, 5, 0, 0, 0, 0, time.UTC)},
	}
	for _, it := range items {
		rss := &RSSItem{
			GUID:        it.link,
			Link:        it.link,
			Title:       it.link,
			Description: "desc",
			PublishedAt: it.time.UTC(),
		}
		if err := saveRSSItem(db, rss); err != nil {
			t.Fatalf("saveRSSItem: %v", err)
		}
	}

	got, err := getUnprocessedComics(db, 10)
	if err != nil {
		t.Fatalf("getUnprocessedComics: %v", err)
	}

	want := []string{"unproc-2026", "unproc-2023", "unproc-2010"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d", len(got), len(want))
	}
	for i, item := range got {
		if item.Link != want[i] {
			t.Errorf("position %d: got %q, want %q", i, item.Link, want[i])
		}
	}
}

// TestGetProcessedComicsStableAcrossRuns is the regression test for the
// "FreshRSS shows 100 items as new" symptom. With the correct sort, running
// the query twice against an unchanged DB must return exactly the same
// sequence of IDs. A new item at the top must bump the oldest out without
// perturbing the items in between.
func TestGetProcessedComicsStableAcrossRuns(t *testing.T) {
	db := newTestDB(t)

	// Seed a handful of processed comics across different dates.
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		saveProcessed(t, db, fmt.Sprintf("comic-%02d", i), base.AddDate(0, i, 0))
	}

	first, err := getProcessedComics(db, 10)
	if err != nil {
		t.Fatalf("first query: %v", err)
	}
	second, err := getProcessedComics(db, 10)
	if err != nil {
		t.Fatalf("second query: %v", err)
	}
	assertSameLinks(t, "repeat query", first, second)

	// Add a strictly newer comic; expect it at position 0, everything else unchanged.
	saveProcessed(t, db, "comic-new", base.AddDate(1, 0, 0))

	after, err := getProcessedComics(db, 10)
	if err != nil {
		t.Fatalf("after insert query: %v", err)
	}
	if len(after) != len(first)+1 {
		t.Fatalf("after insert: got %d items, want %d", len(after), len(first)+1)
	}
	if after[0].Link() != "comic-new" {
		t.Errorf("new item should be first, got %q", after[0].Link())
	}
	// Everything after position 0 must exactly match the original order.
	for i := 1; i < len(after); i++ {
		if after[i].Link() != first[i-1].Link() {
			t.Errorf("position %d changed after inserting newer item: got %q, want %q",
				i, after[i].Link(), first[i-1].Link())
		}
	}
}

// TestGetProcessedComicsLimit ensures the limit bounds results to the most
// recent N, which is what keeps the generated feed size stable.
func TestGetProcessedComicsLimit(t *testing.T) {
	db := newTestDB(t)

	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 30 {
		// Each comic is one day newer than the last.
		saveProcessed(t, db, fmt.Sprintf("comic-%03d", i), base.AddDate(0, 0, i))
	}

	got, err := getProcessedComics(db, 10)
	if err != nil {
		t.Fatalf("getProcessedComics: %v", err)
	}
	if len(got) != 10 {
		t.Fatalf("got %d items, want exactly 10", len(got))
	}
	// The newest 10 are comic-029..comic-020 in descending order.
	for i, item := range got {
		want := fmt.Sprintf("comic-%03d", 29-i)
		if item.Link() != want {
			t.Errorf("position %d: got %q, want %q", i, item.Link(), want)
		}
	}
}

// TestSaveRSSItemRejectsZeroPublishedAt guards the invariant that
// PublishedAt is always set before it reaches the database. A silently
// zero time would land at the unix epoch in sort order and confuse the
// feed output.
func TestSaveRSSItemRejectsZeroPublishedAt(t *testing.T) {
	db := newTestDB(t)
	err := saveRSSItem(db, &RSSItem{
		GUID:  "zero",
		Link:  "https://www.oglaf.com/zero/",
		Title: "Zero",
		// PublishedAt deliberately zero
	})
	if err == nil {
		t.Fatal("expected error when saving item with zero PublishedAt, got nil")
	}
}

// TestParsePubDateAcceptsOglafFormat exercises the format parser against the
// exact shape Oglaf's RSS serves, which is the only input source in prod.
func TestParsePubDateAcceptsOglafFormat(t *testing.T) {
	cases := []string{
		"Sun, 12 Apr 2026 00:00:00 +0000",
		"Wed, 05 Jul 2023 00:00:00 +0000",
		"Thu, 13 Feb 2025 00:00:00 +0000",
	}
	for _, in := range cases {
		if _, err := parsePubDate(in); err != nil {
			t.Errorf("parsePubDate(%q) unexpectedly failed: %v", in, err)
		}
	}
}

func linkOrder(items []*Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Link()
	}
	return out
}

func assertSameLinks(t *testing.T, label string, a, b []*Item) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("%s: different lengths: %d vs %d", label, len(a), len(b))
	}
	for i := range a {
		if a[i].Link() != b[i].Link() {
			t.Errorf("%s: position %d differs: %q vs %q", label, i, a[i].Link(), b[i].Link())
		}
	}
}
