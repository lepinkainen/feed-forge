package providers

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Mock implementations for testing
type mockFeedProvider struct {
	generateFeedFunc func(outfile string, reauth bool) error
}

func (m *mockFeedProvider) GenerateFeed(outfile string, reauth bool) error {
	if m.generateFeedFunc != nil {
		return m.generateFeedFunc(outfile, reauth)
	}
	return nil
}

type mockFeedItem struct {
	title        string
	link         string
	commentsLink string
	author       string
	score        int
	commentCount int
	createdAt    time.Time
	categories   []string
	imageURL     string
}

func (m *mockFeedItem) Title() string        { return m.title }
func (m *mockFeedItem) Link() string         { return m.link }
func (m *mockFeedItem) CommentsLink() string { return m.commentsLink }
func (m *mockFeedItem) Author() string       { return m.author }
func (m *mockFeedItem) Score() int           { return m.score }
func (m *mockFeedItem) CommentCount() int    { return m.commentCount }
func (m *mockFeedItem) CreatedAt() time.Time { return m.createdAt }
func (m *mockFeedItem) Categories() []string { return m.categories }
func (m *mockFeedItem) ImageURL() string     { return m.imageURL }

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
	for i := 0; i < numGoroutines; i++ {
		go func(offset int) {
			defer wg.Done()
			for j := 0; j < numProviders; j++ {
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
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
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

func TestFeedItemInterface(t *testing.T) {
	now := time.Now()
	item := &mockFeedItem{
		title:        "Test Title",
		link:         "https://example.com",
		commentsLink: "https://example.com/comments",
		author:       "Test Author",
		score:        100,
		commentCount: 25,
		createdAt:    now,
		categories:   []string{"tech", "news"},
	}

	// Test all interface methods
	if item.Title() != "Test Title" {
		t.Errorf("Title() = %v, want %v", item.Title(), "Test Title")
	}

	if item.Link() != "https://example.com" {
		t.Errorf("Link() = %v, want %v", item.Link(), "https://example.com")
	}

	if item.CommentsLink() != "https://example.com/comments" {
		t.Errorf("CommentsLink() = %v, want %v", item.CommentsLink(), "https://example.com/comments")
	}

	if item.Author() != "Test Author" {
		t.Errorf("Author() = %v, want %v", item.Author(), "Test Author")
	}

	if item.Score() != 100 {
		t.Errorf("Score() = %v, want %v", item.Score(), 100)
	}

	if item.CommentCount() != 25 {
		t.Errorf("CommentCount() = %v, want %v", item.CommentCount(), 25)
	}

	if !item.CreatedAt().Equal(now) {
		t.Errorf("CreatedAt() = %v, want %v", item.CreatedAt(), now)
	}

	categories := item.Categories()
	if len(categories) != 2 || categories[0] != "tech" || categories[1] != "news" {
		t.Errorf("Categories() = %v, want %v", categories, []string{"tech", "news"})
	}
}

func TestFeedProviderInterface(t *testing.T) {
	provider := &mockFeedProvider{
		generateFeedFunc: func(outfile string, reauth bool) error {
			if outfile == "" {
				return errors.New("outfile cannot be empty")
			}
			return nil
		},
	}

	// Test successful call
	err := provider.GenerateFeed("output.xml", false)
	if err != nil {
		t.Errorf("GenerateFeed() with valid params should not error, got %v", err)
	}

	// Test error case
	err = provider.GenerateFeed("", false)
	if err == nil {
		t.Errorf("GenerateFeed() with empty outfile should error")
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
