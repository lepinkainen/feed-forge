package reddit

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// LoadConfig loads configuration with fallback priority: URL -> local file -> defaults
func LoadConfig(configURL string) error {
	// Try remote configuration first if URL is provided
	if configURL != "" {
		slog.Info("Attempting to load config from URL", "url", configURL)
		err := loadConfigFromURL(configURL)
		if err != nil {
			slog.Warn("Failed to load config from URL, falling back to local file", "error", err)
		} else {
			slog.Info("Successfully loaded config from URL")
			return nil
		}
	}

	// Try local file
	err := loadConfigFromFile()
	if err != nil {
		slog.Warn("Failed to load config from file, using defaults", "error", err)
		return err
	}

	slog.Info("Successfully loaded config from file")
	return nil
}

// loadConfigFromURL loads configuration from a remote URL
func loadConfigFromURL(url string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch config from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error fetching config: %s", resp.Status)
	}

	var remoteConfig Config
	if err := json.NewDecoder(resp.Body).Decode(&remoteConfig); err != nil {
		return fmt.Errorf("failed to decode remote config: %w", err)
	}

	// Validate required fields
	if err := validateConfig(&remoteConfig); err != nil {
		return fmt.Errorf("invalid remote config: %w", err)
	}

	GlobalConfig = remoteConfig
	return nil
}

// loadConfigFromFile loads configuration from local JSON file
func loadConfigFromFile() error {
	file, err := os.ReadFile(ConfigFileName)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	if err := json.Unmarshal(file, &GlobalConfig); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&GlobalConfig); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	return nil
}

// SaveConfig saves the current configuration to a JSON file
func SaveConfig() error {
	data, err := json.MarshalIndent(GlobalConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := os.WriteFile(ConfigFileName, data, 0600); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	slog.Info("Configuration saved successfully")
	return nil
}

// validateConfig validates the configuration structure
func validateConfig(config *Config) error {
	if config.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}

	if config.FeedType != "rss" && config.FeedType != "atom" {
		return fmt.Errorf("feed_type must be 'rss' or 'atom'")
	}

	if config.OutputPath == "" {
		return fmt.Errorf("output_path is required")
	}

	if config.ScoreFilter < 0 {
		return fmt.Errorf("score_filter must be >= 0")
	}

	if config.CommentFilter < 0 {
		return fmt.Errorf("comment_filter must be >= 0")
	}

	return nil
}

// InitializeDefaultConfig sets up default configuration values
func InitializeDefaultConfig() {
	GlobalConfig.ScoreFilter = 0
	GlobalConfig.CommentFilter = 0
	GlobalConfig.FeedType = "atom"
	GlobalConfig.EnhancedAtom = true
	GlobalConfig.OutputPath = "reddit.xml"
}
