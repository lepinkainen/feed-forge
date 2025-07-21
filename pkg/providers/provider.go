package providers

import (
	"fmt"
	"sync"
	"time"
)

// FeedProvider defines the interface for a feed source.
type FeedProvider interface {
	GenerateFeed(outfile string, reauth bool) error
}

// FeedItem defines the essential fields for any feed entry.
type FeedItem interface {
	Title() string
	Link() string
	CommentsLink() string
	Author() string
	Score() int
	CommentCount() int
	CreatedAt() time.Time
	Categories() []string
	ImageURL() string
}

// ProviderFactory creates a new instance of a provider.
type ProviderFactory func(config interface{}) (FeedProvider, error)

// ProviderInfo contains metadata about a provider.
type ProviderInfo struct {
	Name        string
	Description string
	Version     string
	Factory     ProviderFactory
}

// ProviderRegistry manages registered feed providers.
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]*ProviderInfo
}

// NewProviderRegistry creates a new provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]*ProviderInfo),
	}
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(name string, info *ProviderInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s is already registered", name)
	}

	r.providers[name] = info
	return nil
}

// Get retrieves a provider by name.
func (r *ProviderRegistry) Get(name string) (*ProviderInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return info, nil
}

// List returns all registered provider names.
func (r *ProviderRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}

// CreateProvider creates a new instance of the specified provider.
func (r *ProviderRegistry) CreateProvider(name string, config interface{}) (FeedProvider, error) {
	info, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	return info.Factory(config)
}

// Global registry instance
var DefaultRegistry = NewProviderRegistry()
