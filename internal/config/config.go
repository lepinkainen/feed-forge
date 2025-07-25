package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/spf13/viper"
)

// Config holds the central application configuration
// This manages both provider settings and authentication state in a unified YAML file
// Note: This combines configuration and state for simplicity, following the existing pattern
type Config struct {
	// Reddit OAuth provider configuration and OAuth2 state
	RedditOAuth struct {
		// OAuth2 Configuration
		ClientID     string `mapstructure:"client_id"`
		ClientSecret string `mapstructure:"client_secret"`
		RedirectURI  string `mapstructure:"redirect_uri"`

		// OAuth2 State (managed at runtime)
		RefreshToken string    `mapstructure:"refresh_token"`
		AccessToken  string    `mapstructure:"access_token"`
		ExpiresAt    time.Time `mapstructure:"expires_at"`

		// Feed Generation Settings
		OutputPath    string `mapstructure:"output_path"`    // Output file path
		ScoreFilter   int    `mapstructure:"score_filter"`   // Minimum score filter
		CommentFilter int    `mapstructure:"comment_filter"` // Minimum comment filter
	} `mapstructure:"reddit_oauth"`

	// HackerNews provider configuration
	HackerNews struct {
		MinPoints int `mapstructure:"min_points"` // Minimum points threshold
		Limit     int `mapstructure:"limit"`      // Maximum number of items
	} `mapstructure:"hackernews"`

	// Reddit JSON provider configuration
	RedditJSON struct {
		// JSON Feed Configuration
		FeedID   string `mapstructure:"feed_id"`  // Reddit feed ID (e.g., "12341234asdfdsf234")
		Username string `mapstructure:"username"` // Reddit username (e.g., "spez")

		// Feed Generation Settings
		OutputPath    string `mapstructure:"output_path"`    // Output file path
		ScoreFilter   int    `mapstructure:"score_filter"`   // Minimum score filter
		CommentFilter int    `mapstructure:"comment_filter"` // Minimum comment filter
	} `mapstructure:"reddit_json"`
}

// LoadConfig loads the configuration from a file
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = "config.yaml"
	}

	// If path is relative, try current directory first, then executable directory
	if !filepath.IsAbs(path) {
		// First try the current working directory
		if _, err := os.Stat(path); err != nil {
			// If not found in current directory, try executable directory
			if execPath, err := filesystem.GetDefaultPath(path); err == nil {
				if _, err := os.Stat(execPath); err == nil {
					path = execPath
				}
			}
			// If both fail, use original path (current directory) and let Viper handle the error
		}
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	// Set default values
	viper.SetDefault("reddit_oauth.client_id", "")
	viper.SetDefault("reddit_oauth.client_secret", "")
	viper.SetDefault("reddit_oauth.redirect_uri", "http://localhost:8080/callback")
	viper.SetDefault("reddit_oauth.output_path", "reddit.xml")
	viper.SetDefault("reddit_oauth.score_filter", 50)
	viper.SetDefault("reddit_oauth.comment_filter", 10)

	viper.SetDefault("reddit_json.feed_id", "")
	viper.SetDefault("reddit_json.username", "")
	viper.SetDefault("reddit_json.output_path", "reddit.xml")
	viper.SetDefault("reddit_json.score_filter", 50)
	viper.SetDefault("reddit_json.comment_filter", 10)

	viper.SetDefault("hackernews.min_points", 50)
	viper.SetDefault("hackernews.limit", 30)

	// Read configuration file
	if err := viper.ReadInConfig(); err != nil {
		// If config file doesn't exist, that's okay - we'll use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// SaveConfig saves the configuration to a file
func SaveConfig(config *Config, path string) error {
	if path == "" {
		path = "config.yaml"
	}

	// If path is relative, try current directory first, then executable directory
	if !filepath.IsAbs(path) {
		// First try the current working directory
		if _, err := os.Stat(path); err != nil {
			// If not found in current directory, try executable directory
			if execPath, err := filesystem.GetDefaultPath(path); err == nil {
				if _, err := os.Stat(execPath); err == nil {
					path = execPath
				}
			}
			// If both fail, use original path (current directory) and let Viper handle the error
		}
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	// Set values from config struct
	viper.Set("reddit_oauth.client_id", config.RedditOAuth.ClientID)
	viper.Set("reddit_oauth.client_secret", config.RedditOAuth.ClientSecret)
	viper.Set("reddit_oauth.redirect_uri", config.RedditOAuth.RedirectURI)
	viper.Set("reddit_oauth.refresh_token", config.RedditOAuth.RefreshToken)
	viper.Set("reddit_oauth.access_token", config.RedditOAuth.AccessToken)
	viper.Set("reddit_oauth.expires_at", config.RedditOAuth.ExpiresAt)
	viper.Set("reddit_oauth.output_path", config.RedditOAuth.OutputPath)
	viper.Set("reddit_oauth.score_filter", config.RedditOAuth.ScoreFilter)
	viper.Set("reddit_oauth.comment_filter", config.RedditOAuth.CommentFilter)

	viper.Set("reddit_json.feed_id", config.RedditJSON.FeedID)
	viper.Set("reddit_json.username", config.RedditJSON.Username)
	viper.Set("reddit_json.output_path", config.RedditJSON.OutputPath)
	viper.Set("reddit_json.score_filter", config.RedditJSON.ScoreFilter)
	viper.Set("reddit_json.comment_filter", config.RedditJSON.CommentFilter)

	viper.Set("hackernews.min_points", config.HackerNews.MinPoints)
	viper.Set("hackernews.limit", config.HackerNews.Limit)

	return viper.WriteConfig()
}
