package linkpreview

import "time"

// Data represents OpenGraph metadata extracted from a webpage
type Data struct {
	URL          string    `json:"url"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Image        string    `json:"image"`
	SiteName     string    `json:"site_name"`
	Excerpt      string    `json:"excerpt"`
	ETag         string    `json:"etag"`
	LastModified string    `json:"last_modified"`
	FetchedAt    time.Time `json:"fetched_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Constants for OpenGraph caching
const (
	DefaultCacheHours = 24
	DefaultDBFile     = "linkpreview.db"
)
