package feed

import (
	"fmt"
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

// GetTemplateOverrideFS returns the primary template filesystem.
func GetTemplateOverrideFS() fs.FS {
	return templateOverrideFS
}

// GetTemplateFallbackFS returns the embedded fallback template filesystem.
func GetTemplateFallbackFS() fs.FS {
	return templateFallbackFS
}

// ReadTemplateContent reads raw template file content using the override/fallback pattern.
func ReadTemplateContent(filename string) (string, error) {
	if overrideFS := GetTemplateOverrideFS(); overrideFS != nil {
		content, err := fs.ReadFile(overrideFS, filename)
		if err == nil {
			return string(content), nil
		}
	}
	if fallbackFS := GetTemplateFallbackFS(); fallbackFS != nil {
		content, err := fs.ReadFile(fallbackFS, filename)
		if err == nil {
			return string(content), nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrTemplateNotFound, filename)
}
