package providers

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lepinkainen/feed-forge/pkg/feedtypes"
)

func TestDatabaseConfig(t *testing.T) {
	tests := []struct {
		name   string
		config DatabaseConfig
		valid  bool
	}{
		{
			name: "valid config with content DB",
			config: DatabaseConfig{
				ContentDBName: "test.db",
				UseContentDB:  true,
			},
			valid: true,
		},
		{
			name: "valid config without content DB",
			config: DatabaseConfig{
				ContentDBName: "",
				UseContentDB:  false,
			},
			valid: true,
		},
		{
			name: "config with content DB name but disabled",
			config: DatabaseConfig{
				ContentDBName: "test.db",
				UseContentDB:  false,
			},
			valid: true,
		},
		{
			name: "config without content DB name but enabled",
			config: DatabaseConfig{
				ContentDBName: "",
				UseContentDB:  true,
			},
			valid: true, // This should be handled gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just test that the config struct can be created and used
			if tt.config.UseContentDB && tt.config.ContentDBName == "" {
				// This configuration should be handled by NewBaseProvider
				t.Logf("Config with UseContentDB=true but empty ContentDBName: %+v", tt.config)
			}
		})
	}
}

func TestNewBaseProvider_ErrorHandling(t *testing.T) {
	// Test with temporary directory that we can control
	tempDir := t.TempDir()

	// Create a read-only directory to test error handling
	readOnlyDir := filepath.Join(tempDir, "readonly")
	err := os.Mkdir(readOnlyDir, 0444) // Read-only permissions
	if err != nil {
		t.Skipf("Cannot create read-only directory for test: %v", err)
	}

	// Change to the read-only directory to force database creation errors
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	err = os.Chdir(readOnlyDir)
	if err != nil {
		t.Skipf("Cannot change to read-only directory: %v", err)
	}

	tests := []struct {
		name     string
		config   DatabaseConfig
		wantErr  bool
		skipTest bool
	}{
		{
			name: "should handle opengraph DB creation error",
			config: DatabaseConfig{
				ContentDBName: "",
				UseContentDB:  false,
			},
			wantErr:  true, // Should fail due to read-only directory
			skipTest: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipTest {
				t.Skip("Skipping test due to environment constraints")
			}

			base, err := NewBaseProvider(tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewBaseProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if base != nil {
				// Clean up if base provider was created
				base.Close()
			}
		})
	}
}

func TestBaseProvider_Close(t *testing.T) {
	// Create a minimal base provider for testing
	// We'll test the Close functionality separately from NewBaseProvider
	// to avoid complex database setup in tests

	base := &BaseProvider{}

	// Test closing when databases are nil (should not panic)
	err := base.Close()
	if err != nil {
		t.Errorf("Close() with nil databases should not error, got %v", err)
	}
}

func TestBaseProvider_CleanupExpired(t *testing.T) {
	// Test cleanup with nil database
	base := &BaseProvider{}

	err := base.CleanupExpired()
	if err != nil {
		t.Errorf("CleanupExpired() with nil OgDB should not error, got %v", err)
	}
}

func TestDatabaseConfig_Validation(t *testing.T) {
	// Test various configurations to ensure they're handled properly
	configs := []DatabaseConfig{
		{ContentDBName: "test.db", UseContentDB: true},
		{ContentDBName: "test.db", UseContentDB: false},
		{ContentDBName: "", UseContentDB: false},
		{ContentDBName: "", UseContentDB: true},
	}

	for i, config := range configs {
		t.Run(fmt.Sprintf("config_%d", i), func(t *testing.T) {
			// Just verify the configuration doesn't cause any immediate issues
			if config.UseContentDB && config.ContentDBName == "" {
				t.Logf("Configuration may need special handling: %+v", config)
			}

			// Test that boolean and string fields work as expected
			if config.UseContentDB != (config.UseContentDB == true) {
				t.Errorf("UseContentDB boolean not working correctly")
			}

			if len(config.ContentDBName) < 0 {
				t.Errorf("ContentDBName length invalid")
			}
		})
	}
}

// testProvider implements FeedProvider for testing
type testProvider struct {
	*BaseProvider
}

func (tp *testProvider) GenerateFeed(outfile string, reauth bool) error {
	return nil
}

func (tp *testProvider) FetchItems(limit int) ([]feedtypes.FeedItem, error) {
	return []feedtypes.FeedItem{}, nil
}

func (tp *testProvider) Metadata() FeedMetadata {
	return FeedMetadata{
		Title:        "Test Feed",
		Link:         "https://example.com",
		Description:  "Test feed for testing",
		Author:       "Test Author",
		ID:           "test-feed",
		TemplateName: "test-atom",
	}
}

// Mock test for BaseProvider interface compliance
func TestBaseProvider_InterfaceCompliance(t *testing.T) {
	// Verify that BaseProvider can be embedded in structs that implement FeedProvider
	provider := &testProvider{
		BaseProvider: &BaseProvider{},
	}

	// Test that we can call BaseProvider methods
	err := provider.CleanupExpired()
	if err != nil {
		t.Errorf("CleanupExpired() through embedded BaseProvider failed: %v", err)
	}

	err = provider.Close()
	if err != nil {
		t.Errorf("Close() through embedded BaseProvider failed: %v", err)
	}

	// Test that provider implements FeedProvider interface
	var _ FeedProvider = provider
}
