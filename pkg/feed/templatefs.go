package feed

import (
	"io/fs"
	"os"

	"github.com/lepinkainen/feed-forge/templates"
)

var (
	// templateOverrideFS points at the developer-provided filesystem (usually the local templates directory).
	templateOverrideFS fs.FS = os.DirFS("templates")
	// templateFallbackFS is the embedded filesystem baked into the binary.
	templateFallbackFS fs.FS = templates.EmbeddedTemplates
)

// SetTemplateOverrideFS switches the primary filesystem used when loading templates.
func SetTemplateOverrideFS(f fs.FS) {
	templateOverrideFS = f
}

// SetTemplateFallbackFS overrides the embedded filesystem used when no override file is available.
func SetTemplateFallbackFS(f fs.FS) {
	templateFallbackFS = f
}

func getTemplateOverrideFS() fs.FS {
	return templateOverrideFS
}

func getTemplateFallbackFS() fs.FS {
	return templateFallbackFS
}
