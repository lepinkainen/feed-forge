package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/spf13/viper"
)

// Config holds the central application configuration
type Config struct {
	// HackerNews provider configuration
	HackerNews struct {
		MinPoints int `mapstructure:"min_points"` // Minimum points threshold
		Limit     int `mapstructure:"limit"`      // Maximum number of items
	} `mapstructure:"hackernews"`

	// Reddit provider configuration
	Reddit struct {
		// Feed Configuration
		FeedID   string `mapstructure:"feed_id"`  // Reddit feed ID (e.g., "12341234asdfdsf234")
		Username string `mapstructure:"username"` // Reddit username (e.g., "spez")

		// Feed Generation Settings
		OutputPath    string `mapstructure:"output_path"`    // Output file path
		ScoreFilter   int    `mapstructure:"score_filter"`   // Minimum score filter
		CommentFilter int    `mapstructure:"comment_filter"` // Minimum comment filter
	} `mapstructure:"reddit"`
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
	viper.SetDefault("reddit.feed_id", "")
	viper.SetDefault("reddit.username", "")
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
	viper.Set("reddit.feed_id", config.Reddit.FeedID)
	viper.Set("reddit.username", config.Reddit.Username)
	viper.Set("reddit.output_path", config.Reddit.OutputPath)
	viper.Set("reddit.score_filter", config.Reddit.ScoreFilter)
	viper.Set("reddit.comment_filter", config.Reddit.CommentFilter)

	viper.Set("hackernews.min_points", config.HackerNews.MinPoints)
	viper.Set("hackernews.limit", config.HackerNews.Limit)

	return viper.WriteConfig()
}
