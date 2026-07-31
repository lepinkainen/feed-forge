package config

// Test configuration structure
type testConfig struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	Debug   bool   `json:"debug" yaml:"debug"`
	Timeout int    `json:"timeout" yaml:"timeout"`
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
