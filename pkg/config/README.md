# Configuration System

## Overview
The feed-forge configuration system has three distinct layers:

1. **Central Configuration** (`internal/config/config.go`)
   - Application-wide settings (log level, data directory)
   - Provider configuration (API keys, endpoints)
   - Loaded from YAML file using Viper
   - Manages OAuth2 tokens and persistent state

2. **Provider Configuration** (e.g., `internal/reddit/config.go`)
   - Provider-specific configuration loading utilities
   - Handles remote configuration fetching with fallbacks
   - JSON-based configuration for specific features

3. **External Configuration** (e.g., HackerNews domain categorization)
   - External data loaded from URLs
   - Cached locally with fallback
   - Used for dynamic configuration that may be updated remotely

## Usage Guidelines

### Central Configuration
- Use for settings that need to persist across application restarts
- OAuth2 tokens and authentication state
- API keys and credentials
- Application-level settings

### Provider Configuration
- Use for provider-specific settings that may be loaded from remote sources
- Domain categorization rules
- Feature flags and dynamic configuration
- Settings that may be updated without restarting the application

### External Configuration
- Use for configuration that may be updated remotely
- Reference data (like domain categorization rules)
- Settings that should be shared across multiple instances

## Configuration Files

- `config.yaml` - Central application configuration (Viper-managed)
- `reddit.json` - Reddit provider-specific state (optional, for local overrides)
- Remote configuration URLs - For dynamic external configuration

## Best Practices

1. **Separation of Concerns**: Keep authentication state separate from application settings
2. **Fallback Strategy**: Always provide reasonable defaults and fallback mechanisms
3. **Error Handling**: Configuration loading should be robust and provide clear error messages
4. **Security**: Store sensitive information (API keys, tokens) securely
5. **Documentation**: Always document configuration options and their purposes