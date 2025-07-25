# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

**Primary build system**: Task (taskfile.dev) - use `task` commands instead of direct `go` commands

**Essential commands**:

- `task build` - Build the application (includes test and lint)
- `task test` - Run all tests
- `task lint` - Run linter and formatter (gofmt, go vet, go mod tidy)
- `task clean` - Clean build artifacts
- `task build-linux` - Build for Linux AMD64
- `task build-ci` - Build for CI with coverage
- `task test-ci` - Run tests with CI tags and coverage
- `task update-golden` - Update golden files when tests are in stable state

**Run commands**:

- `task run-reddit` - Run Reddit OAuth feed generation (uses reddit-oauth)
- `task run-hackernews` - Run Hacker News feed generation

**Direct execution**:

- `./build/feed-forge reddit-oauth --reauth` - Force Reddit OAuth re-authentication
- `./build/feed-forge reddit-oauth -o output.xml --min-score 100` - Reddit with OAuth
- `./build/feed-forge reddit-json -o output.xml --min-score 100` - Reddit public JSON feed
- `./build/feed-forge hacker-news -o output.xml --min-points 50` - Hacker News feed

## Architecture Overview

Feed-Forge is a unified RSS feed generator. It uses a **provider-based architecture** with a common interface for different feed sources.

### Core Components

**Provider Interface** (`pkg/providers/provider.go`):

- Core interface: `FeedProvider` with method `GenerateFeed(outfile string, reauth bool) error`
- `FeedItem` interface for standardized feed entry handling with common fields (Title, Link, Score, etc.)
- `BaseProvider` struct (`pkg/providers/base.go`) provides common functionality for all providers
- Provider registry system with factory pattern for dynamic provider management and discovery

**CLI Entry Point** (`cmd/feed-forge/main.go`):

- Uses Kong for command-line parsing
- Supports `reddit-oauth`, `reddit-json`, and `hacker-news` subcommands
- Handles configuration loading and provider instantiation

**Configuration System** (`internal/config/config.go` and `pkg/config/loader.go`):

- Viper-based YAML configuration with fallback to defaults
- Unified config structure for all providers with CLI flag overrides
- Automatic config file creation and OAuth2 token persistence

**Provider Implementations**:

- `internal/reddit-oauth/` - Reddit OAuth2 authentication, API calls, feed generation (requires Reddit app credentials)
- `internal/reddit-json/` - Reddit JSON feed access, simplified authentication-free approach (public feeds only)
- `internal/hackernews/` - Hacker News API integration, categorization, story caching

**Shared Package Libraries**:

- `pkg/api/` - Enhanced HTTP client with rate limiting, retries, and standardized error handling
- `pkg/config/` - Configuration loading utilities with URL/file fallback support
- `pkg/database/` - SQLite caching, provider utilities, and database interfaces
- `pkg/feed/` - Atom feed generation, enhanced templates, custom XML formatting, and feed helpers
- `pkg/http/` - HTTP client utilities and response handling
- `pkg/opengraph/` - OpenGraph metadata fetching and caching
- `pkg/filesystem/` - File system utilities and path management
- `pkg/providers/` - Provider interfaces and base implementations
- `pkg/utils/` - URL utilities and common helper functions
- `pkg/testutil/` - Golden file testing utilities
- `pkg/interfaces/` - Shared interface definitions

### Key Architecture Patterns

**BaseProvider Pattern**:

- All providers inherit from `providers.BaseProvider` with shared database connections
- `DatabaseConfig` pattern for configuring provider-specific database needs
- Reddit OAuth provider: `UseContentDB: false` (stateless API calls)
- Reddit JSON provider: `UseContentDB: false` (stateless JSON parsing)
- HackerNews provider: `UseContentDB: true` with "hackernews.db" (story caching)
- All providers share OpenGraph database for metadata caching

**OAuth2 Authentication Flow** (Reddit):

- Local HTTP server on port 8080 for OAuth callback
- Token persistence in config.yaml
- Automatic token refresh with fallback to browser auth

**Database Integration**:

- SQLite for caching (`modernc.org/sqlite`)
- OpenGraph metadata caching (`pkg/opengraph/`) shared across all providers
- Provider-specific content databases (optional, configurable per provider)

**Enhanced Feed Generation**:

- Configurable enhanced Atom templates with provider-specific customization
- Multiple feed formats: standard Atom, enhanced Atom with custom namespaces
- OpenGraph integration for rich content with concurrent fetching
- Provider-specific metadata in custom XML namespaces (reddit:, hn:)
- Configurable filtering (score, comments, points)
- Support for multiple link types, enclosures, and extended author information

## Common Development Patterns

**Error Handling**: Use `log/slog` for structured logging throughout the codebase. Enhanced HTTP client provides standardized error handling with retry logic and structured error types.

**HTTP Client Usage**: **CRITICAL** - Always use `pkg/api` enhanced clients for API calls. Provider-specific clients with built-in rate limiting and retry policies:

- `api.NewRedditClient(baseClient)` - Reddit-optimized with 1-second rate limiting
- `api.NewHackerNewsClient()` - HackerNews-optimized with conservative rate limiting
- `api.NewGenericClient()` - General purpose with minimal configuration

Never make direct HTTP calls - use these enhanced clients to avoid rate limiting and API failures.

**Feed Generation**: Use enhanced Atom templates for rich feeds:

- `feed.RedditEnhancedAtomConfig()` - Reddit-specific configuration
- `feed.HackerNewsEnhancedAtomConfig()` - HackerNews-specific configuration
- `feed.DefaultEnhancedAtomConfig()` - Base configuration for new providers

**Configuration**: All providers use shared configuration utilities (`pkg/config`) with URL/file fallback, format detection (JSON/YAML), and unified config structure

**Provider Factory Pattern**: Use provider registry (`providers.DefaultRegistry`) for dynamic provider discovery and instantiation. New providers register themselves with metadata and factory functions.

**Testing**: Use `//go:build !ci` to skip tests in CI environments when needed

**Golden File Testing**: Use `task update-golden` to update test fixtures when implementation changes are stable and verified. Golden files are stored in testdata directories for consistent test results. Always review golden file diffs before committing - they represent expected output changes.

**Authentication State Management**: Reddit provider manages OAuth2 tokens automatically with graceful fallbacks

## Important Implementation Details

**Reddit Authentication Gotcha**: The OAuth2 server must be properly shut down after token exchange. The `serverCancel()` call is critical to prevent hanging.

**Provider Instantiation**: Providers are created using factory functions:
- Reddit OAuth: `redditoauth.NewRedditProvider()`
- Reddit JSON: `redditjson.NewRedditProvider()`
- Hacker News: `hackernews.NewHackerNewsProvider()`

CLI flags override config file values. Each provider inherits from `BaseProvider` with database configuration.

**Enhanced HTTP Client**: All API calls use `pkg/api` enhanced client with configurable rate limiting, exponential backoff retries, and provider-specific policies

**Enhanced Feed Templates**: Providers use configurable Atom templates (`pkg/feed/enhanced_atom.go`) supporting custom namespaces, multiple links, rich content, and provider-specific metadata

**OpenGraph Integration**: Feed items are enhanced with OpenGraph metadata for better client compatibility, with concurrent fetching and caching.

## Project Structure

```text
feed-forge/
├── cmd/feed-forge/              # Main application entry point
├── internal/
│   ├── config/                  # Configuration management
│   ├── hackernews/              # Hacker News provider implementation
│   ├── reddit-oauth/            # Reddit OAuth provider implementation
│   └── reddit-json/             # Reddit JSON provider implementation
├── pkg/                         # Shared packages
│   ├── api/                     # Enhanced HTTP client with rate limiting and retries
│   ├── config/                  # Configuration loading utilities with fallback support
│   ├── database/                # SQLite caching and database interfaces
│   ├── feed/                    # Enhanced Atom generation, templates, and custom XML
│   ├── filesystem/              # File system utilities
│   ├── http/                    # HTTP client utilities and response handling
│   ├── interfaces/              # Shared interface definitions
│   ├── opengraph/               # OpenGraph metadata fetching and caching
│   ├── providers/               # Provider interfaces and base implementations
│   ├── testutil/                # Golden file testing utilities
│   └── utils/                   # URL and common utilities
├── testdata/                    # Test fixtures and golden files
└── llm-shared/                  # Development guidelines submodule
```

## Development Guidelines

This project follows `llm-shared` conventions:

- Always run `goimports -w .` after Go code changes (preferred over `gofmt` for automatic import management)
- Use `task build` instead of `go build` to ensure tests and linting
- Requires Go 1.24+ for compilation and development
- Tech stack guidelines: `llm-shared/project_tech_stack.md`
- Function analysis: `go run llm-shared/utils/gofuncs/gofuncs.go -dir .`

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.