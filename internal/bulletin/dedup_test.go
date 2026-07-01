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
