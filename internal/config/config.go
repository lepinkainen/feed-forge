package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds the central application configuration
// This manages both provider settings and authentication state in a unified YAML file
// Note: This combines configuration and state for simplicity, following the existing pattern
type Config struct {
	// Reddit provider configuration and OAuth2 state
	Reddit struct {
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
	} `mapstructure:"reddit"`

	// HackerNews provider configuration
	HackerNews struct {
		MinPoints int `mapstructure:"min_points"` // Minimum points threshold
		Limit     int `mapstructure:"limit"`      // Maximum number of items
	} `mapstructure:"hackernews"`
}

// LoadConfig loads the configuration from a file
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = "config.yaml"
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	// Set default values
	viper.SetDefault("reddit.client_id", "")
	viper.SetDefault("reddit.client_secret", "")
	viper.SetDefault("reddit.redirect_uri", "http://localhost:8080/callback")
	viper.SetDefault("reddit.output_path", "reddit.xml")
	viper.SetDefault("reddit.score_filter", 50)
	viper.SetDefault("reddit.comment_filter", 10)

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

	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	// Set values from config struct
	viper.Set("reddit.client_id", config.Reddit.ClientID)
	viper.Set("reddit.client_secret", config.Reddit.ClientSecret)
	viper.Set("reddit.redirect_uri", config.Reddit.RedirectURI)
	viper.Set("reddit.refresh_token", config.Reddit.RefreshToken)
	viper.Set("reddit.access_token", config.Reddit.AccessToken)
	viper.Set("reddit.expires_at", config.Reddit.ExpiresAt)
	viper.Set("reddit.output_path", config.Reddit.OutputPath)
	viper.Set("reddit.score_filter", config.Reddit.ScoreFilter)
	viper.Set("reddit.comment_filter", config.Reddit.CommentFilter)

	viper.Set("hackernews.min_points", config.HackerNews.MinPoints)
	viper.Set("hackernews.limit", config.HackerNews.Limit)

	return viper.WriteConfig()
}
