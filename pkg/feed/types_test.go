package feed

import (
	"path/filepath"
	"testing"

	"github.com/lepinkainen/feed-forge/pkg/testutil"
)

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		goldenFile string
	}{
		{
			name:       "no special characters",
			input:      "Hello World",
			goldenFile: "no_special_chars.xml",
		},
		{
			name:       "ampersand",
			input:      "Tom & Jerry",
			goldenFile: "ampersand.xml",
		},
		{
			name:       "less than",
			input:      "5 < 10",
			goldenFile: "less_than.xml",
		},
		{
			name:       "greater than",
			input:      "10 > 5",
			goldenFile: "greater_than.xml",
		},
		{
			name:       "double quotes",
			input:      `Say "Hello"`,
			goldenFile: "double_quotes.xml",
		},
		{
			name:       "single quotes",
			input:      "Don't do it",
			goldenFile: "single_quotes.xml",
		},
		{
			name:       "all special characters",
			input:      `<tag attr="value" class='style'>Tom & Jerry</tag>`,
			goldenFile: "all_special_chars.xml",
		},
		{
			name:       "empty string",
			input:      "",
			goldenFile: "empty_string.xml",
		},
		{
			name:       "multiple ampersands",
			input:      "A & B & C",
			goldenFile: "multiple_ampersands.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeXML(tt.input)
			goldenPath := filepath.Join("testdata", "escape_xml", tt.goldenFile)
			testutil.CompareGolden(t, goldenPath, result)
		})
	}
}
