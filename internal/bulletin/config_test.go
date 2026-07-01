package bulletin

import "testing"

func TestWithDefaultsFillsEmptyFields(t *testing.T) {
	got := Config{}.withDefaults()
	if got.Model != defaultModel {
		t.Errorf("Model = %q, want %q", got.Model, defaultModel)
	}
	if got.SimhashThreshold != defaultSimhashThreshold {
		t.Errorf("SimhashThreshold = %d, want %d", got.SimhashThreshold, defaultSimhashThreshold)
	}
	if got.MaxTokens != defaultMaxTokens {
		t.Errorf("MaxTokens = %d, want %d", got.MaxTokens, defaultMaxTokens)
	}
}

func TestWithDefaultsPreservesExplicitValues(t *testing.T) {
	in := Config{
		Model:            "claude-custom",
		SimhashThreshold: 7,
		MaxTokens:        1024,
		PromptFile:       "/tmp/prompt.txt",
	}
	got := in.withDefaults()
	if got.Model != "claude-custom" {
		t.Errorf("Model = %q, want claude-custom", got.Model)
	}
	if got.SimhashThreshold != 7 {
		t.Errorf("SimhashThreshold = %d, want 7", got.SimhashThreshold)
	}
	if got.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %d, want 1024", got.MaxTokens)
	}
	if got.PromptFile != "/tmp/prompt.txt" {
		t.Errorf("PromptFile = %q, want /tmp/prompt.txt", got.PromptFile)
	}
}

// A zero simhash-threshold is treated as "unset" and replaced by the default;
// this documents the accepted trade-off (exact-match-only clustering is not
// configurable) so it isn't mistaken for a bug later.
func TestWithDefaultsZeroThresholdBecomesDefault(t *testing.T) {
	got := Config{SimhashThreshold: 0}.withDefaults()
	if got.SimhashThreshold != defaultSimhashThreshold {
		t.Errorf("SimhashThreshold = %d, want %d", got.SimhashThreshold, defaultSimhashThreshold)
	}
}
