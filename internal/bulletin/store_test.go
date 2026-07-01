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
		FeedURL: "https://feed.example/rss", Category: "tech",
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
