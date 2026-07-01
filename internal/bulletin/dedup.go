package bulletin

// Cluster is a group of items judged to cover the same story.
type Cluster struct {
	Items []Item
}

// Rep returns the representative (first-seen) item of the cluster.
func (c Cluster) Rep() Item { return c.Items[0] }

// clusterItems groups items whose SimHash fingerprints are within threshold
// Hamming distance of a cluster representative. Greedy single-pass: each item
// joins the first cluster whose representative it is near, else starts its own.
//
// Items with a zero fingerprint (empty/all-stopword text) can't be compared
// reliably and always form singleton clusters.
func clusterItems(items []Item, threshold int) []Cluster {
	var clusters []Cluster
	for _, it := range items {
		placed := false
		if it.SimHash != 0 {
			for i := range clusters {
				rep := clusters[i].Rep()
				if rep.SimHash != 0 && Hamming(rep.SimHash, it.SimHash) <= threshold {
					clusters[i].Items = append(clusters[i].Items, it)
					placed = true
					break
				}
			}
		}
		if !placed {
			clusters = append(clusters, Cluster{Items: []Item{it}})
		}
	}
	return clusters
}
