package providers

import (
	"fmt"
	"testing"
)

func TestRegisterProvider(t *testing.T) {
	// Save current state and restore after test
	originalProviders := make(map[string]*ProviderInfo)
	for k, v := range DefaultRegistry.providers {
		originalProviders[k] = v
	}
	defer func() {
		DefaultRegistry.providers = originalProviders
	}()

	// Clear the registry for clean testing
	DefaultRegistry.providers = make(map[string]*ProviderInfo)

	// Test successful registration
	info := &ProviderInfo{
		Name:        "Test Provider",
		Description: "A test provider for convenience function",
		Version:     "1.0.0",
		Factory: func(config interface{}) (FeedProvider, error) {
			return &mockFeedProvider{}, nil
		},
	}

	RegisterProvider("test-convenience", info)

	// Verify it was registered
	registeredInfo, err := DefaultRegistry.Get("test-convenience")
	if err != nil {
		t.Errorf("RegisterProvider() failed to register provider: %v", err)
	}

	if registeredInfo.Name != info.Name {
		t.Errorf("RegisterProvider() registered wrong info, got %v, want %v", registeredInfo.Name, info.Name)
	}
}

func TestRegisterProvider_Duplicate(t *testing.T) {
	// Save current state and restore after test
	originalProviders := make(map[string]*ProviderInfo)
	for k, v := range DefaultRegistry.providers {
		originalProviders[k] = v
	}
	defer func() {
		DefaultRegistry.providers = originalProviders
	}()

	// Clear the registry for clean testing
	DefaultRegistry.providers = make(map[string]*ProviderInfo)

	info1 := &ProviderInfo{
		Name:        "First Provider",
		Description: "First registration",
		Version:     "1.0.0",
		Factory: func(config interface{}) (FeedProvider, error) {
			return &mockFeedProvider{}, nil
		},
	}

	info2 := &ProviderInfo{
		Name:        "Second Provider",
		Description: "Duplicate registration",
		Version:     "2.0.0",
		Factory: func(config interface{}) (FeedProvider, error) {
			return &mockFeedProvider{}, nil
		},
	}

	// First registration should succeed
	RegisterProvider("duplicate-test", info1)

	// Second registration should fail (but function doesn't panic)
	RegisterProvider("duplicate-test", info2)

	// Verify first registration is preserved
	registeredInfo, err := DefaultRegistry.Get("duplicate-test")
	if err != nil {
		t.Errorf("Failed to get registered provider: %v", err)
	}

	if registeredInfo.Name != info1.Name {
		t.Errorf("Duplicate registration overwrote original, got %v, want %v", registeredInfo.Name, info1.Name)
	}
}

func TestGetProvider(t *testing.T) {
	// Save current state and restore after test
	originalProviders := make(map[string]*ProviderInfo)
	for k, v := range DefaultRegistry.providers {
		originalProviders[k] = v
	}
	defer func() {
		DefaultRegistry.providers = originalProviders
	}()

	// Clear the registry for clean testing
	DefaultRegistry.providers = make(map[string]*ProviderInfo)

	// Register a test provider
	info := &ProviderInfo{
		Name:        "Get Test Provider",
		Description: "A test provider for GetProvider function",
		Version:     "1.0.0",
		Factory: func(config interface{}) (FeedProvider, error) {
			return &mockFeedProvider{}, nil
		},
	}
	DefaultRegistry.Register("get-test", info)

	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{
			name:     "existing provider",
			provider: "get-test",
			wantErr:  false,
		},
		{
			name:     "non-existent provider",
			provider: "non-existent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProvider(tt.provider)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got == nil {
					t.Errorf("GetProvider() returned nil without error")
				}
				if got.Name != info.Name {
					t.Errorf("GetProvider() returned wrong provider, got %v, want %v", got.Name, info.Name)
				}
			}
		})
	}
}

func TestListProviders(t *testing.T) {
	// Save current state and restore after test
	originalProviders := make(map[string]*ProviderInfo)
	for k, v := range DefaultRegistry.providers {
		originalProviders[k] = v
	}
	defer func() {
		DefaultRegistry.providers = originalProviders
	}()

	// Clear the registry for clean testing
	DefaultRegistry.providers = make(map[string]*ProviderInfo)

	// Initially empty
	list := ListProviders()
	if len(list) != 0 {
		t.Errorf("ListProviders() on empty registry should return empty slice, got %v", list)
	}

	// Add some providers
	providers := []string{"list-test-1", "list-test-2", "list-test-3"}
	for _, name := range providers {
		DefaultRegistry.Register(name, &ProviderInfo{
			Name:        name,
			Description: "List test provider",
			Version:     "1.0.0",
			Factory: func(config interface{}) (FeedProvider, error) {
				return &mockFeedProvider{}, nil
			},
		})
	}

	list = ListProviders()
	if len(list) != len(providers) {
		t.Errorf("ListProviders() returned %d providers, want %d", len(list), len(providers))
	}

	// Check all providers are present (order might differ)
	found := make(map[string]bool)
	for _, name := range list {
		found[name] = true
	}

	for _, expected := range providers {
		if !found[expected] {
			t.Errorf("ListProviders() missing provider %s", expected)
		}
	}
}

func TestCreateProvider(t *testing.T) {
	// Save current state and restore after test
	originalProviders := make(map[string]*ProviderInfo)
	for k, v := range DefaultRegistry.providers {
		originalProviders[k] = v
	}
	defer func() {
		DefaultRegistry.providers = originalProviders
	}()

	// Clear the registry for clean testing
	DefaultRegistry.providers = make(map[string]*ProviderInfo)

	// Register test providers
	DefaultRegistry.Register("create-success", &ProviderInfo{
		Name: "Create Success Provider",
		Factory: func(config interface{}) (FeedProvider, error) {
			return &mockFeedProvider{}, nil
		},
	})

	DefaultRegistry.Register("create-fail", &ProviderInfo{
		Name: "Create Fail Provider",
		Factory: func(config interface{}) (FeedProvider, error) {
			return nil, fmt.Errorf("creation failed")
		},
	})

	tests := []struct {
		name     string
		provider string
		config   interface{}
		wantErr  bool
	}{
		{
			name:     "successful creation",
			provider: "create-success",
			config:   map[string]string{"key": "value"},
			wantErr:  false,
		},
		{
			name:     "provider not found",
			provider: "non-existent",
			config:   nil,
			wantErr:  true,
		},
		{
			name:     "factory fails",
			provider: "create-fail",
			config:   nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := CreateProvider(tt.provider, tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && provider == nil {
				t.Errorf("CreateProvider() returned nil provider without error")
			}
		})
	}
}
