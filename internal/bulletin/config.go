package bulletin

import (
	"path/filepath"
	"strings"
)

// userAgent identifies the bulletin fetcher to source sites.
const userAgent = "feed-forge-bulletin/1.0 (+https://github.com/lepinkainen/feed-forge)"

// httpcacheDBPath returns the conditional-GET cache DB path sited next to the
// bulletin database (e.g. bulletin.db -> bulletin-httpcache.db).
func httpcacheDBPath(dbPath string) string {
	ext := filepath.Ext(dbPath)
	base := strings.TrimSuffix(dbPath, ext)
	return base + "-httpcache" + ext
}

// FeedSource is one source feed to aggregate into bulletins.
type FeedSource struct {
	URL string `yaml:"url"`
	// Name is the publisher's display name (e.g. "The Verge"), used as the link
	// text for this source in the digest. Falls back to the article host when empty.
	Name string `yaml:"name"`
}

// Config holds the bulletin pipeline configuration, loaded from the `bulletin:`
// section of config.yaml.
type Config struct {
	Model            string       `yaml:"model"`
	SimhashThreshold int          `yaml:"simhash-threshold"`
	MaxTokens        int          `yaml:"max-tokens"`
	PromptFile       string       `yaml:"prompt-file"`
	Feeds            []FeedSource `yaml:"feeds"`
}

// Defaults for unset config fields.
const (
	defaultModel            = "claude-haiku-4-5"
	defaultSimhashThreshold = 3
	defaultMaxTokens        = 4096
)

// withDefaults returns a copy of the config with zero-valued fields filled in.
func (c Config) withDefaults() Config {
	if c.Model == "" {
		c.Model = defaultModel
	}
	if c.SimhashThreshold == 0 {
		c.SimhashThreshold = defaultSimhashThreshold
	}
	if c.MaxTokens == 0 {
		c.MaxTokens = defaultMaxTokens
	}
	return c
}
