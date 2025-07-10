package utils

import "net/url"

// IsValidURL checks if a URL is valid
func IsValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	return err == nil && u.Scheme != "" && u.Host != ""
}
