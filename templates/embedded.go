// Package templates provides embedded Go templates for feed generation.
package templates

import "embed"

// EmbeddedTemplates provides read-only access to template files compiled into the binary.
//
//go:embed *.tmpl
var EmbeddedTemplates embed.FS

// EmbeddedFonts holds the self-hosted webfonts referenced by the bulletin HTML
// page. bulletin-publish copies these into the output directory so rendered
// pages have no external font dependency.
//
//go:embed fonts/*.woff2
var EmbeddedFonts embed.FS
