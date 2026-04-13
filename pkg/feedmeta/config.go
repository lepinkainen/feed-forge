package feedmeta

// Config contains metadata for feed generation.
type Config struct {
	Title       string
	Link        string
	Description string
	Author      string
	ID          string
	ProxyURL    string // Optional proxy URL for fetching OG data from blocked domains
	ProxySecret string // Shared secret for proxy authentication
}
