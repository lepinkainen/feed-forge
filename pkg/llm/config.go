// Package llm holds shared configuration for LLM providers. It is intentionally
// general (not tied to any one feed pipeline) so any processor that calls a
// model can reuse it via the top-level `anthropic:` config section.
package llm

import "os"

// APIKeyEnv is the environment variable consulted when no key is set in config.
const APIKeyEnv = "ANTHROPIC_API_KEY" // #nosec G101 -- environment variable name, not a credential.

// Config is the general `anthropic:` configuration section. Additional shared
// settings (default models, base URL, ...) can be added here later and reused by
// any processor that talks to Anthropic.
type Config struct {
	APIKey string `yaml:"api-key"`
}

// ResolveAPIKey returns the configured API key, falling back to the APIKeyEnv
// environment variable when the config value is empty. This keeps existing
// env-only deployments working while allowing the key to live in config.
func (c Config) ResolveAPIKey() string {
	if c.APIKey != "" {
		return c.APIKey
	}
	return os.Getenv(APIKeyEnv)
}
