package providers

import "log/slog"

// RegisterProvider is a convenience function to register a provider with the default registry.
func RegisterProvider(name string, info *ProviderInfo) {
	if err := DefaultRegistry.Register(name, info); err != nil {
		slog.Warn("Failed to register provider", "provider", name, "error", err)
	} else {
		slog.Debug("Registered provider", "provider", name, "description", info.Description)
	}
}

// GetProvider is a convenience function to get a provider from the default registry.
func GetProvider(name string) (*ProviderInfo, error) {
	return DefaultRegistry.Get(name)
}

// ListProviders is a convenience function to list all providers in the default registry.
func ListProviders() []string {
	return DefaultRegistry.List()
}

// CreateProvider is a convenience function to create a provider from the default registry.
func CreateProvider(name string, config any) (FeedProvider, error) {
	return DefaultRegistry.CreateProvider(name, config)
}
