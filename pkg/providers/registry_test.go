package providers

import (
	"errors"
	"fmt"
	"maps"
	"sync"
	"testing"
)

func TestNewProviderRegistry(t *testing.T) {
	registry := NewProviderRegistry()

	if registry == nil {
		t.Errorf("NewProviderRegistry() returned nil")
	}

	if registry.providers == nil {
		t.Errorf("NewProviderRegistry() providers map is nil")
	}

	if len(registry.providers) != 0 {
		t.Errorf("NewProviderRegistry() should start with empty providers map")
	}
}

func TestProviderRegistry_Register(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ProviderRegistry)
		provider string
		info     *ProviderInfo
		wantErr  bool
	}{
		{
			name:     "successful registration",
			setup:    func(r *ProviderRegistry) {},
			provider: "test-provider",
			info: &ProviderInfo{
				Name:        "Test Provider",
				Description: "A test provider",
				Version:     "1.0.0",
				Factory: func(config any) (FeedProvider, error) {
					return &mockFeedProvider{}, nil
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate registration fails",
			setup: func(r *ProviderRegistry) {
				r.Register("existing-provider", &ProviderInfo{
					Name:        "Existing Provider",
					Description: "Already registered",
					Version:     "1.0.0",
					Factory: func(config any) (FeedProvider, error) {
						return &mockFeedProvider{}, nil
					},
				})
			},
			provider: "existing-provider",
			info: &ProviderInfo{
				Name:        "Duplicate Provider",
				Description: "Should fail",
				Version:     "2.0.0",
				Factory: func(config any) (FeedProvider, error) {
					return &mockFeedProvider{}, nil
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewProviderRegistry()
			tt.setup(registry)

			err := registry.Register(tt.provider, tt.info)

			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the provider was actually registered
				info, err := registry.Get(tt.provider)
				if err != nil {
					t.Errorf("Failed to retrieve registered provider: %v", err)
				}
				if info.Name != tt.info.Name {
					t.Errorf("Retrieved provider name = %v, want %v", info.Name, tt.info.Name)
				}
			}
		})
	}
}

func TestProviderRegistry_Get(t *testing.T) {
	registry := NewProviderRegistry()

	// Register a test provider
	testInfo := &ProviderInfo{
		Name:        "Test Provider",
		Description: "A test provider",
		Version:     "1.0.0",
		Factory: func(config any) (FeedProvider, error) {
			return &mockFeedProvider{}, nil
		},
	}
	registry.Register("test", testInfo)

	tests := []struct {
		name     string
		provider string
		want     *ProviderInfo
		wantErr  bool
	}{
		{
			name:     "existing provider",
			provider: "test",
			want:     testInfo,
			wantErr:  false,
		},
		{
			name:     "non-existent provider",
			provider: "non-existent",
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.Get(tt.provider)

			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProviderRegistry_List(t *testing.T) {
	registry := NewProviderRegistry()

	// Initially empty
	list := registry.List()
	if len(list) != 0 {
		t.Errorf("List() on empty registry should return empty slice, got %v", list)
	}

	// Add some providers
	providers := []string{"provider1", "provider2", "provider3"}
	for _, name := range providers {
		registry.Register(name, &ProviderInfo{
			Name:        name,
			Description: "Test provider",
			Version:     "1.0.0",
			Factory: func(config any) (FeedProvider, error) {
				return &mockFeedProvider{}, nil
			},
		})
	}

	list = registry.List()
	if len(list) != len(providers) {
		t.Errorf("List() returned %d providers, want %d", len(list), len(providers))
	}

	// Check all providers are present (order might differ)
	found := make(map[string]bool)
	for _, name := range list {
		found[name] = true
	}

	for _, expected := range providers {
		if !found[expected] {
			t.Errorf("List() missing provider %s", expected)
		}
	}
}

func TestProviderRegistry_CreateProvider(t *testing.T) {
	registry := NewProviderRegistry()

	// Register a provider that creates successfully
	registry.Register("success", &ProviderInfo{
		Name: "Success Provider",
		Factory: func(config any) (FeedProvider, error) {
			return &mockFeedProvider{}, nil
		},
	})

	// Register a provider that fails to create
	registry.Register("fail", &ProviderInfo{
		Name: "Fail Provider",
		Factory: func(config any) (FeedProvider, error) {
			return nil, errors.New("creation failed")
		},
	})

	tests := []struct {
		name     string
		provider string
		config   any
		wantErr  bool
	}{
		{
			name:     "successful creation",
			provider: "success",
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
			provider: "fail",
			config:   nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := registry.CreateProvider(tt.provider, tt.config)

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

func TestProviderRegistry_Concurrent(t *testing.T) {
	registry := NewProviderRegistry()

	const numGoroutines = 10
	const numProviders = 100

	var wg sync.WaitGroup

	// Concurrent registrations
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(offset int) {
			defer wg.Done()
			for j := range numProviders {
				name := fmt.Sprintf("provider-%d-%d", offset, j)
				info := &ProviderInfo{
					Name:        name,
					Description: "Concurrent test provider",
					Version:     "1.0.0",
					Factory: func(config any) (FeedProvider, error) {
						return &mockFeedProvider{}, nil
					},
				}
				registry.Register(name, info)
			}
		}(i)
	}

	// Concurrent reads
	wg.Add(numGoroutines)
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range 50 {
				registry.List()
			}
		}()
	}

	wg.Wait()

	// Verify final state
	list := registry.List()
	// We expect at most numGoroutines * numProviders (some registrations might fail due to duplicates)
	if len(list) > numGoroutines*numProviders {
		t.Errorf("Too many providers registered: %d", len(list))
	}
}

func TestDefaultRegistry(t *testing.T) {
	// Test that DefaultRegistry is initialized
	if DefaultRegistry == nil {
		t.Errorf("DefaultRegistry should not be nil")
	}

	// Test that it's a proper registry
	initialCount := len(DefaultRegistry.List())

	err := DefaultRegistry.Register("test-default", &ProviderInfo{
		Name:        "Test Default",
		Description: "Testing default registry",
		Version:     "1.0.0",
		Factory: func(config any) (FeedProvider, error) {
			return &mockFeedProvider{}, nil
		},
	})

	if err != nil {
		t.Errorf("Failed to register with DefaultRegistry: %v", err)
	}

	if len(DefaultRegistry.List()) != initialCount+1 {
		t.Errorf("DefaultRegistry should have one more provider after registration")
	}

	// Clean up for other tests
	delete(DefaultRegistry.providers, "test-default")
}

func TestRegisterProvider(t *testing.T) {
	// Save current state and restore after test
	originalProviders := make(map[string]*ProviderInfo)
	maps.Copy(originalProviders, DefaultRegistry.providers)
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
		Factory: func(config any) (FeedProvider, error) {
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
	maps.Copy(originalProviders, DefaultRegistry.providers)
	defer func() {
		DefaultRegistry.providers = originalProviders
	}()

	// Clear the registry for clean testing
	DefaultRegistry.providers = make(map[string]*ProviderInfo)

	info1 := &ProviderInfo{
		Name:        "First Provider",
		Description: "First registration",
		Version:     "1.0.0",
		Factory: func(config any) (FeedProvider, error) {
			return &mockFeedProvider{}, nil
		},
	}

	info2 := &ProviderInfo{
		Name:        "Second Provider",
		Description: "Duplicate registration",
		Version:     "2.0.0",
		Factory: func(config any) (FeedProvider, error) {
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
	maps.Copy(originalProviders, DefaultRegistry.providers)
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
		Factory: func(config any) (FeedProvider, error) {
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
	maps.Copy(originalProviders, DefaultRegistry.providers)
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
			Factory: func(config any) (FeedProvider, error) {
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
	maps.Copy(originalProviders, DefaultRegistry.providers)
	defer func() {
		DefaultRegistry.providers = originalProviders
	}()

	// Clear the registry for clean testing
	DefaultRegistry.providers = make(map[string]*ProviderInfo)

	// Register test providers
	DefaultRegistry.Register("create-success", &ProviderInfo{
		Name: "Create Success Provider",
		Factory: func(config any) (FeedProvider, error) {
			return &mockFeedProvider{}, nil
		},
	})

	DefaultRegistry.Register("create-fail", &ProviderInfo{
		Name: "Create Fail Provider",
		Factory: func(config any) (FeedProvider, error) {
			return nil, fmt.Errorf("creation failed")
		},
	})

	tests := []struct {
		name     string
		provider string
		config   any
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
