package config

import (
	"testing"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		data     []byte
		expected string
	}{
		{
			name:     "JSON file extension",
			path:     "config.json",
			data:     []byte(`{"test": true}`),
			expected: "json",
		},
		{
			name:     "YAML file extension",
			path:     "config.yaml",
			data:     []byte(`test: true`),
			expected: "yaml",
		},
		{
			name:     "YML file extension",
			path:     "config.yml",
			data:     []byte(`test: true`),
			expected: "yaml",
		},
		{
			name:     "JSON content detection",
			path:     "config",
			data:     []byte(`{"test": true}`),
			expected: "json",
		},
		{
			name:     "JSON array content detection",
			path:     "config",
			data:     []byte(`[{"test": true}]`),
			expected: "json",
		},
		{
			name:     "YAML content fallback",
			path:     "config",
			data:     []byte(`test: true`),
			expected: "yaml",
		},
		{
			name:     "Mixed extension vs content - extension wins",
			path:     "config.json",
			data:     []byte(`test: true`),
			expected: "json",
		},
		{
			name:     "Whitespace handling",
			path:     "config",
			data:     []byte(`  {"test": true}  `),
			expected: "json",
		},
		{
			name:     "Empty content defaults to YAML",
			path:     "config",
			data:     []byte(``),
			expected: "yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectFormat(tt.path, tt.data)
			if result != tt.expected {
				t.Errorf("detectFormat(%q, %q) = %q, want %q", tt.path, string(tt.data), result, tt.expected)
			}
		})
	}
}
