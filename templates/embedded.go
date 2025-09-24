package templates

import "embed"

// EmbeddedTemplates provides read-only access to template files compiled into the binary.
//
//go:embed *.tmpl
var EmbeddedTemplates embed.FS
