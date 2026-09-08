// Package version holds the build version of feed-forge.
package version

// Version is the build version. It is overridden at build time with
// -ldflags "-X github.com/lepinkainen/feed-forge/pkg/version.Version=...".
var Version = "dev"

// UserAgent returns the product identity used for outbound HTTP requests.
func UserAgent() string {
	return "feed-forge/" + Version
}
