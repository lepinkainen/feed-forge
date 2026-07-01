package llm

import "testing"

func TestResolveAPIKeyPrefersConfig(t *testing.T) {
	t.Setenv(APIKeyEnv, "from-env")
	if got := (Config{APIKey: "from-config"}).ResolveAPIKey(); got != "from-config" {
		t.Errorf("ResolveAPIKey() = %q, want from-config", got)
	}
}

func TestResolveAPIKeyFallsBackToEnv(t *testing.T) {
	t.Setenv(APIKeyEnv, "from-env")
	if got := (Config{}).ResolveAPIKey(); got != "from-env" {
		t.Errorf("ResolveAPIKey() = %q, want from-env", got)
	}
}

func TestResolveAPIKeyEmptyWhenNeitherSet(t *testing.T) {
	t.Setenv(APIKeyEnv, "")
	if got := (Config{}).ResolveAPIKey(); got != "" {
		t.Errorf("ResolveAPIKey() = %q, want empty", got)
	}
}
