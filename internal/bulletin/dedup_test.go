package bulletin

import "testing"

func TestClusterItemsGroupsNearDuplicates(t *testing.T) {
	items := []Item{
		{ID: 1, URL: "a", Title: "t", SimHash: SimHash("The Federal Communications Commission added foreign made drones to its covered list barring approval and blocking imports of new models it deems a national security risk citing vulnerabilities")},
		{ID: 2, URL: "b", Title: "t", SimHash: SimHash("The Federal Communications Commission added foreign made drones to its covered list barring approval and blocking imports of new models it deems an unacceptable national security risk citing vulnerabilities")},
		{ID: 3, URL: "c", Title: "t", SimHash: SimHash("Apple unveiled a redesigned MacBook Pro featuring a faster processor a brighter display and substantially improved battery life")},
	}

	clusters := clusterItems(items, defaultSimhashThreshold)
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}
	// First cluster is the two drone stories; representative is item 1.
	if len(clusters[0].Items) != 2 || clusters[0].Rep().ID != 1 {
		t.Errorf("first cluster wrong: %+v", clusters[0])
	}
	if len(clusters[1].Items) != 1 || clusters[1].Rep().ID != 3 {
		t.Errorf("second cluster wrong: %+v", clusters[1])
	}
}

func TestClusterItemsEmpty(t *testing.T) {
	if got := clusterItems(nil, 3); len(got) != 0 {
		t.Errorf("empty input: got %d clusters, want 0", len(got))
	}
	if got := clusterItems([]Item{}, 3); len(got) != 0 {
		t.Errorf("empty slice: got %d clusters, want 0", len(got))
	}
}

func TestClusterItemsSingleItem(t *testing.T) {
	clusters := clusterItems([]Item{{ID: 1, SimHash: 12345}}, 3)
	if len(clusters) != 1 || len(clusters[0].Items) != 1 {
		t.Errorf("single item: got %d clusters", len(clusters))
	}
}

func TestClusterItemsThresholdZero(t *testing.T) {
	// With threshold 0, only exact-same-fingerprint items cluster.
	items := []Item{
		{ID: 1, SimHash: 123},
		{ID: 2, SimHash: 123},
		{ID: 3, SimHash: 124},
	}
	clusters := clusterItems(items, 0)
	if len(clusters) != 2 {
		t.Fatalf("threshold 0: got %d clusters, want 2", len(clusters))
	}
	if len(clusters[0].Items) != 2 { // items 1+2 share fingerprint
		t.Errorf("first cluster: got %d items, want 2", len(clusters[0].Items))
	}
}

func TestClusterItemsAllIdentical(t *testing.T) {
	items := []Item{
		{ID: 1, SimHash: 999},
		{ID: 2, SimHash: 999},
		{ID: 3, SimHash: 999},
	}
	clusters := clusterItems(items, 3)
	if len(clusters) != 1 || len(clusters[0].Items) != 3 {
		t.Errorf("all identical: got %d clusters with %d items, want 1 cluster with 3 items",
			len(clusters), len(clusters[0].Items))
	}
}

func TestClusterItemsZeroHashAreSingletons(t *testing.T) {
	items := []Item{
		{ID: 1, URL: "a", SimHash: 0},
		{ID: 2, URL: "b", SimHash: 0},
	}
	clusters := clusterItems(items, 3)
	if len(clusters) != 2 {
		t.Errorf("zero-hash items should not cluster: got %d clusters", len(clusters))
	}
}
