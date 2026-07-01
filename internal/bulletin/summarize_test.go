package bulletin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{name: "shorter than limit", s: "hello", n: 10, want: "hello"},
		{name: "exactly at limit", s: "hello", n: 5, want: "hello"},
		{name: "over limit", s: "hello world", n: 5, want: "hello…"},
		{name: "empty string", s: "", n: 5, want: ""},
		{name: "zero limit", s: "hello", n: 0, want: "…"},
		{name: "unicode runes", s: "héllo wörld", n: 5, want: "héllo…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.s, tt.n); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

func TestRenderClusters(t *testing.T) {
	clusters := []Cluster{
		{Items: []Item{
			{Title: "Story A", URL: "https://x/1", RawText: "text a"},
			{Title: "Story A (mirror)", URL: "https://x/2", RawText: "text a dup"},
		}},
		{Items: []Item{
			{Title: "Story B", URL: "https://x/3", RawText: "text b"},
		}},
	}
	got := renderClusters(clusters)
	if !strings.Contains(got, "[Story 1] Story A") {
		t.Error("missing first story header")
	}
	if !strings.Contains(got, "https://x/1") || !strings.Contains(got, "https://x/2") {
		t.Error("missing source URLs")
	}
	if !strings.Contains(got, "[Story 2] Story B") {
		t.Error("missing second story header")
	}
	if !strings.Contains(got, "text a") {
		t.Error("missing representative excerpt")
	}
}

func TestRenderClustersEmpty(t *testing.T) {
	if got := renderClusters(nil); got != "" {
		t.Errorf("empty clusters: got %q, want empty", got)
	}
}

func TestNewSummarizerMissingAPIKey(t *testing.T) {
	if _, err := NewSummarizer(Config{}, ""); err == nil {
		t.Fatal("expected error when API key is empty, got nil")
	} else if !strings.Contains(err.Error(), "API key") {
		t.Errorf("error = %q, want it to mention the missing API key", err)
	}
}

func TestNewSummarizerMissingPromptFile(t *testing.T) {
	cfg := Config{PromptFile: filepath.Join(t.TempDir(), "does-not-exist.txt")}
	if _, err := NewSummarizer(cfg, "test-key"); err == nil {
		t.Fatal("expected error for missing prompt file, got nil")
	} else if !strings.Contains(err.Error(), "read prompt file") {
		t.Errorf("error = %q, want a read prompt file error", err)
	}
}

func TestNewSummarizerInvalidPromptTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.tmpl")
	if err := os.WriteFile(path, []byte("stories: {{.Stories"), 0o600); err != nil {
		t.Fatalf("write temp prompt: %v", err)
	}
	cfg := Config{PromptFile: path}
	if _, err := NewSummarizer(cfg, "test-key"); err == nil {
		t.Fatal("expected error for invalid template, got nil")
	} else if !strings.Contains(err.Error(), "parse prompt template") {
		t.Errorf("error = %q, want a parse prompt template error", err)
	}
}

func TestNewSummarizerDefaultsSucceed(t *testing.T) {
	s, err := NewSummarizer(Config{}, "test-key")
	if err != nil {
		t.Fatalf("NewSummarizer with defaults: %v", err)
	}
	if s.model != defaultModel {
		t.Errorf("model = %q, want %q", s.model, defaultModel)
	}
	if s.template == nil {
		t.Error("expected a parsed prompt template, got nil")
	}
}
