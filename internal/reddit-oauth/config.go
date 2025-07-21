package reddit

import (
	"fmt"

	"github.com/lepinkainen/feed-forge/internal/config"
)

// validateRedditConfig validates the Reddit configuration section
// Note: This validates the global config.Config.Reddit section, not the old Reddit-specific Config struct
func validateRedditConfig(cfg *config.Config) error {
	if cfg.RedditOAuth.ClientID == "" {
		return fmt.Errorf("reddit.client_id is required")
	}

	if cfg.RedditOAuth.ScoreFilter < 0 {
		return fmt.Errorf("reddit.score_filter must be >= 0")
	}

	if cfg.RedditOAuth.CommentFilter < 0 {
		return fmt.Errorf("reddit.comment_filter must be >= 0")
	}

	return nil
}
