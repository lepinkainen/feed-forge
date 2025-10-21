// Package configs provides embedded configuration files for feed-forge.
package configs

import "embed"

// EmbeddedConfigs exposes embedded configuration files for read-only access.
//
//go:embed *.json
var EmbeddedConfigs embed.FS
