package utils

import "testing"

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "valid http URL",
			url:      "http://example.com",
			expected: true,
		},
		{
			name:     "valid https URL",
			url:      "https://example.com",
			expected: true,
		},
		{
			name:     "valid URL with path",
			url:      "https://example.com/path/to/resource",
			expected: true,
		},
		{
			name:     "valid URL with query params",
			url:      "https://example.com/search?q=test&page=1",
			expected: true,
		},
		{
			name:     "valid URL with fragment",
			url:      "https://example.com/page#section",
			expected: true,
		},
		{
			name:     "valid URL with port",
			url:      "https://example.com:8080/api",
			expected: true,
		},
		{
			name:     "valid FTP URL",
			url:      "ftp://files.example.com/file.txt",
			expected: true,
		},
		{
			name:     "empty string",
			url:      "",
			expected: false,
		},
		{
			name:     "just domain without scheme",
			url:      "example.com",
			expected: false,
		},
		{
			name:     "scheme without host",
			url:      "https://",
			expected: false,
		},
		{
			name:     "invalid scheme",
			url:      "invalid://example.com",
			expected: true, // url.Parse accepts any scheme as valid
		},
		{
			name:     "malformed URL",
			url:      "ht tp://example.com",
			expected: false,
		},
		{
			name:     "URL with spaces",
			url:      "https://example .com",
			expected: false,
		},
		{
			name:     "localhost URL",
			url:      "http://localhost:3000",
			expected: true,
		},
		{
			name:     "IP address URL",
			url:      "http://192.168.1.1:8080",
			expected: true,
		},
		{
			name:     "URL with unicode domain",
			url:      "https://测试.com",
			expected: true,
		},
		{
			name:     "just scheme",
			url:      "https",
			expected: false,
		},
		{
			name:     "path without scheme or host",
			url:      "/path/to/resource",
			expected: false,
		},
		{
			name:     "query string only",
			url:      "?q=test",
			expected: false,
		},
		{
			name:     "fragment only",
			url:      "#section",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidURL(tt.url)
			if result != tt.expected {
				t.Errorf("IsValidURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}
