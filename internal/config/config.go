package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Reddit struct {
		ClientID      string    `mapstructure:"client_id"`
		ClientSecret  string    `mapstructure:"client_secret"`
		RedirectURI   string    `mapstructure:"redirect_uri"`
		RefreshToken  string    `mapstructure:"refresh_token"`
		AccessToken   string    `mapstructure:"access_token"`
		ExpiresAt     time.Time `mapstructure:"expires_at"`
		FeedType      string    `mapstructure:"feed_type"`
		EnhancedAtom  bool      `mapstructure:"enhanced_atom"`
		OutputPath    string    `mapstructure:"output_path"`
		ScoreFilter   int       `mapstructure:"score_filter"`
		CommentFilter int       `mapstructure:"comment_filter"`
	} `mapstructure:"reddit"`

	HackerNews struct {
		MinPoints int `mapstructure:"min_points"`
		Limit     int `mapstructure:"limit"`
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
	viper.SetDefault("reddit.feed_type", "atom")
	viper.SetDefault("reddit.enhanced_atom", true)
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
	viper.Set("reddit.feed_type", config.Reddit.FeedType)
	viper.Set("reddit.enhanced_atom", config.Reddit.EnhancedAtom)
	viper.Set("reddit.output_path", config.Reddit.OutputPath)
	viper.Set("reddit.score_filter", config.Reddit.ScoreFilter)
	viper.Set("reddit.comment_filter", config.Reddit.CommentFilter)

	viper.Set("hackernews.min_points", config.HackerNews.MinPoints)
	viper.Set("hackernews.limit", config.HackerNews.Limit)

	return viper.WriteConfig()
}
