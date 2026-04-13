package hackernews

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lepinkainen/feed-forge/pkg/filesystem"
)

func TestFactoryPropagatesConstructorError(t *testing.T) {
	cacheMarker, err := os.CreateTemp(t.TempDir(), "cache-file")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if err := cacheMarker.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	filesystem.SetCacheDir(cacheMarker.Name())
	t.Cleanup(func() {
		filesystem.SetCacheDir("")
	})

	provider, err := factory(&Config{MinPoints: 10, Limit: 5})
	if err == nil {
		t.Fatal("factory() error = nil, want propagated constructor error")
	}
	if provider != nil {
		t.Fatalf("factory() provider = %#v, want nil", provider)
	}
	if !strings.Contains(err.Error(), "create hackernews provider") {
		t.Fatalf("factory() error = %q, want provider context", err)
	}
	if !strings.Contains(err.Error(), filepath.Base(cacheMarker.Name())) {
		t.Fatalf("factory() error = %q, want underlying cache path context", err)
	}
}

func TestConvertToFeedItemsPreservesItemData(t *testing.T) {
	items := []Item{
		{ItemID: "1", ItemTitle: "Story 1", ItemCommentCount: 5, Points: 10},
		{ItemID: "2", ItemTitle: "Story 2", ItemCommentCount: 12, Points: 25},
	}

	feedItems := convertToFeedItems(items)

	if len(feedItems) != len(items) {
		t.Fatalf("convertToFeedItems() len = %d, want %d", len(feedItems), len(items))
	}

	for i, feedItem := range feedItems {
		if feedItem.Title() != items[i].ItemTitle {
			t.Fatalf("feedItem %d title = %q, want %q", i, feedItem.Title(), items[i].ItemTitle)
		}
		if feedItem.CommentCount() != items[i].ItemCommentCount {
			t.Fatalf("feedItem %d comment count = %d, want %d", i, feedItem.CommentCount(), items[i].ItemCommentCount)
		}
	}

	first := feedItems[0].(*Item)
	second := feedItems[1].(*Item)

	if first == second {
		t.Fatal("convertToFeedItems() returned the same pointer for multiple items")
	}

	items[0].ItemCommentCount = 42
	if feedItems[0].CommentCount() != 42 {
		t.Fatalf("feedItem 0 comment count after update = %d, want 42", feedItems[0].CommentCount())
	}
}
