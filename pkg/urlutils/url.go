// Package urlutils provides URL and common helper functions.
package urlutils

import "net/url"

// IsValidURL checks if a URL is valid
func IsValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// ResolveURL resolves a relative URL against a base URL
// If the URL is already absolute, it returns it unchanged
func ResolveURL(baseURL, relativeURL string) (string, error) {
	// Parse the relative URL
	rel, err := url.Parse(relativeURL)
	if err != nil {
		return "", err
	}

	// If it's already absolute, return as-is
	if rel.IsAbs() {
		return relativeURL, nil
	}

	// Parse the base URL
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	// Resolve the relative URL against the base
	resolved := base.ResolveReference(rel)
	return resolved.String(), nil
}
