package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFile_JSON(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test JSON file
	jsonContent := `{
		"name": "test-config",
		"version": "1.0.0",
		"debug": true,
		"timeout": 30
	}`

	jsonFile := filepath.Join(tempDir, "config.json")
	err := os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}

	var config testConfig
	err = loadFromFile(jsonFile, &config)

	if err != nil {
		t.Errorf("loadFromFile() error = %v", err)
		return
	}

	// Verify loaded values
	if config.Name != "test-config" {
		t.Errorf("config.Name = %q, want %q", config.Name, "test-config")
	}

	if config.Version != "1.0.0" {
		t.Errorf("config.Version = %q, want %q", config.Version, "1.0.0")
	}

	if !config.Debug {
		t.Errorf("config.Debug = %v, want %v", config.Debug, true)
	}

	if config.Timeout != 30 {
		t.Errorf("config.Timeout = %d, want %d", config.Timeout, 30)
	}
}

func TestLoadFromFile_YAML_UnknownFieldFails(t *testing.T) {
	tempDir := t.TempDir()
	yamlContent := `name: strict-config
version: 1.0.0
unknown: true`
	yamlFile := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to create test YAML file: %v", err)
	}

	var config testConfig
	err := loadFromFile(yamlFile, &config)
	if err == nil {
		t.Fatal("loadFromFile() error = nil, want strict unknown-field failure")
	}
	if !containsString(err.Error(), "field unknown not found") {
		t.Fatalf("loadFromFile() error = %v, want unknown field detail", err)
	}
}

func TestLoadFromFile_YAML(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test YAML file
	yamlContent := `name: test-config-yaml
version: 2.0.0
debug: false
timeout: 60`

	yamlFile := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test YAML file: %v", err)
	}

	var config testConfig
	err = loadFromFile(yamlFile, &config)

	if err != nil {
		t.Errorf("loadFromFile() error = %v", err)
		return
	}

	// Verify loaded values
	if config.Name != "test-config-yaml" {
		t.Errorf("config.Name = %q, want %q", config.Name, "test-config-yaml")
	}

	if config.Version != "2.0.0" {
		t.Errorf("config.Version = %q, want %q", config.Version, "2.0.0")
	}

	if config.Debug {
		t.Errorf("config.Debug = %v, want %v", config.Debug, false)
	}

	if config.Timeout != 60 {
		t.Errorf("config.Timeout = %d, want %d", config.Timeout, 60)
	}
}

func TestLoadFromFile_Errors(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		filename    string
		content     string
		wantErr     bool
		errorSubstr string
	}{
		{
			name:        "file not found",
			filename:    "nonexistent.json",
			content:     "",
			wantErr:     true,
			errorSubstr: "configuration not found",
		},
		{
			name:        "invalid JSON",
			filename:    "invalid.json",
			content:     `{"name": "test", invalid}`,
			wantErr:     true,
			errorSubstr: "configuration is invalid",
		},
		{
			name:        "invalid YAML",
			filename:    "invalid.yaml",
			content:     "name: test\n  invalid: : yaml",
			wantErr:     true,
			errorSubstr: "configuration is invalid",
		},
		{
			name:        "XML parsed as YAML fails",
			filename:    "config.xml",
			content:     `<config><name>test</name></config>`,
			wantErr:     true,
			errorSubstr: "configuration is invalid", // XML content fails YAML parsing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config testConfig
			var err error

			if tt.name == "file not found" {
				// Test with non-existent file
				err = loadFromFile(filepath.Join(tempDir, tt.filename), &config)
			} else {
				// Create file with test content
				filePath := filepath.Join(tempDir, tt.filename)
				writeErr := os.WriteFile(filePath, []byte(tt.content), 0644)
				if writeErr != nil {
					t.Fatalf("Failed to create test file: %v", writeErr)
				}
				err = loadFromFile(filePath, &config)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("loadFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && !containsString(err.Error(), tt.errorSubstr) {
				t.Errorf("loadFromFile() error = %v, should contain %q", err, tt.errorSubstr)
			}
		})
	}
}
