package bulletin

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

var testCtx = context.Background()

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "bulletin.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func sampleItem(url string) Item {
	return Item{
		FeedURL: "https://feed.example/rss", FeedName: "Example",
		URL: url, Title: "Title", RawText: "body text here",
		SimHash: SimHash("body text here"), FetchedAt: time.Now().UTC(),
	}
}

func TestInsertItemDedupesByURL(t *testing.T) {
	s := newTestStore(t)

	ok, err := s.InsertItem(testCtx, sampleItem("https://x/1"))
	if err != nil || !ok {
		t.Fatalf("first insert: ok=%v err=%v", ok, err)
	}
	ok, err = s.InsertItem(testCtx, sampleItem("https://x/1"))
	if err != nil {
		t.Fatalf("second insert err: %v", err)
	}
	if ok {
		t.Error("duplicate URL should not insert a new row")
	}

	has, err := s.HasItem(testCtx, "https://x/1")
	if err != nil || !has {
		t.Errorf("HasItem: has=%v err=%v", has, err)
	}
	if has, _ := s.HasItem(testCtx, "https://x/nope"); has {
		t.Error("HasItem should be false for unknown URL")
	}
}

func TestUnpublishedAndCreateBulletin(t *testing.T) {
	s := newTestStore(t)
	for _, u := range []string{"https://x/1", "https://x/2", "https://x/3"} {
		if _, err := s.InsertItem(testCtx, sampleItem(u)); err != nil {
			t.Fatalf("insert %s: %v", u, err)
		}
	}

	items, err := s.UnpublishedItems(testCtx)
	if err != nil {
		t.Fatalf("UnpublishedItems: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 unpublished, got %d", len(items))
	}

	ids := []int64{items[0].ID, items[1].ID}
	bid, err := s.CreateBulletin(testCtx, Row{
		PublishedAt: time.Now().UTC(),
		Slot:        "Morning",
		Title:       "Morning Bulletin",
		Content:     "<h2>X</h2><p>hi</p>",
	}, ids)
	if err != nil {
		t.Fatalf("CreateBulletin: %v", err)
	}
	if bid == 0 {
		t.Error("expected non-zero bulletin id")
	}

	latest, err := s.LatestBulletins(testCtx, 10)
	if err != nil {
		t.Fatalf("LatestBulletins: %v", err)
	}
	if len(latest) != 1 || latest[0].ID != bid || latest[0].Content == "" {
		t.Errorf("LatestBulletins wrong: %+v", latest)
	}

	remaining, err := s.UnpublishedItems(testCtx)
	if err != nil {
		t.Fatalf("UnpublishedItems after publish: %v", err)
	}
	if len(remaining) != 1 {
		t.Errorf("expected 1 unpublished after publishing 2, got %d", len(remaining))
	}
}

func TestSimHashRoundTrip(t *testing.T) {
	s := newTestStore(t)
	want := SimHash("the federal communications commission covered list drones")
	it := sampleItem("https://x/1")
	it.SimHash = want
	if _, err := s.InsertItem(testCtx, it); err != nil {
		t.Fatalf("insert: %v", err)
	}
	items, _ := s.UnpublishedItems(testCtx)
	if items[0].SimHash != want {
		t.Errorf("simhash round-trip: got %d, want %d", items[0].SimHash, want)
	}
}

// Two Generate runs that both read the backlog before either commits (cron
// overlap, or a manual bulletin-generate while serve runs) must not publish
// the same item twice. Counterexample: formal/tla/BulletinGenerate.tla with
// CrossProcess.cfg.
func TestCreateBulletinRejectsAlreadyPublishedItems(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.InsertItem(testCtx, sampleItem("https://x/1")); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Both runs read the same backlog, then each calls the model.
	first, err := s.UnpublishedItems(testCtx)
	if err != nil {
		t.Fatalf("UnpublishedItems: %v", err)
	}
	second, err := s.UnpublishedItems(testCtx)
	if err != nil {
		t.Fatalf("UnpublishedItems: %v", err)
	}
	itemID := first[0].ID

	firstID, err := s.CreateBulletin(testCtx, Row{PublishedAt: time.Now().UTC(), Slot: "Morning", Content: "<p>A</p>"}, []int64{itemID})
	if err != nil {
		t.Fatalf("first CreateBulletin: %v", err)
	}
	secondID, err := s.CreateBulletin(testCtx, Row{PublishedAt: time.Now().UTC(), Slot: "Morning", Content: "<p>B</p>"}, []int64{second[0].ID})
	if err == nil {
		t.Errorf("second CreateBulletin over an already-published backlog succeeded (bulletin %d)", secondID)
	}

	var owner int64
	if err := s.db.QueryRowContext(testCtx, `SELECT bulletin_id FROM items WHERE id = ?`, itemID).Scan(&owner); err != nil {
		t.Fatalf("query owner: %v", err)
	}
	if owner != firstID {
		t.Errorf("item %d moved from bulletin %d to bulletin %d; bulletin %d still summarises it", itemID, firstID, owner, firstID)
	}

	all, err := s.AllBulletins(testCtx)
	if err != nil {
		t.Fatalf("AllBulletins: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("stored %d bulletins for one item, want 1", len(all))
	}
}
