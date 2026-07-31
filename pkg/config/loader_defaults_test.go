package config

import (
	"testing"
	"time"
)

func TestDefaultLoaderConfig(t *testing.T) {
	config := DefaultLoaderConfig()

	if config == nil {
		t.Errorf("DefaultLoaderConfig() returned nil")
		return
	}

	if config.Timeout != 10*time.Second {
		t.Errorf("DefaultLoaderConfig().Timeout = %v, want %v", config.Timeout, 10*time.Second)
	}

	if config.MaxRetries != 3 {
		t.Errorf("DefaultLoaderConfig().MaxRetries = %d, want %d", config.MaxRetries, 3)
	}

	if !config.FallbackToDefault {
		t.Errorf("DefaultLoaderConfig().FallbackToDefault should be true")
	}

	if config.RemoteURL != "" {
		t.Errorf("DefaultLoaderConfig().RemoteURL should be empty")
	}

	if config.LocalPath != "" {
		t.Errorf("DefaultLoaderConfig().LocalPath should be empty")
	}
}
