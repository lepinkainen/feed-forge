package providers

// FeedProvider defines the interface for a feed source.
type FeedProvider interface {
	GenerateFeed(outfile string, reauth bool) error
}
