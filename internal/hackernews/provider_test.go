package hackernews

import "testing"

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
