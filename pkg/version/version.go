// Package version holds the build version of feed-forge.
package version

// Version is the build version. It is overridden at build time with
// -ldflags "-X github.com/lepinkainen/feed-forge/pkg/version.Version=...".
var Version = "dev"
