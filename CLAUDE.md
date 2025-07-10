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

- `task run-reddit` - Run Reddit feed generation
- `task run-hackernews` - Run Hacker News feed generation

**Direct execution**:

- `./build/feed-forge reddit --reauth` - Force Reddit re-authentication
- `./build/feed-forge reddit -o output.xml --min-score 100`
- `./build/feed-forge hacker-news -o output.xml --min-points 50`

## Architecture Overview

Feed-Forge is a unified RSS feed generator. It uses a **provider-based architecture** with a common interface for different feed sources.

### Core Components

**Provider Interface** (`internal/pkg/providers/provider.go`):

- Core method: `GenerateFeed(outfile string, reauth bool) error`
- Implemented by Reddit and Hacker News providers
- Includes `FeedItem` interface for standardized feed entry handling
- Provider registry system for dynamic provider management and factory pattern

**CLI Entry Point** (`cmd/feed-forge/main.go`):

- Uses Kong for command-line parsing
- Supports `reddit` and `hacker-news` subcommands
- Handles configuration loading and provider instantiation

**Configuration System** (`internal/config/config.go`):

- Viper-based YAML configuration
- Unified config structure for all providers
- Automatic config file creation and token persistence

**Provider Implementations**:

- `internal/reddit/` - Reddit OAuth2 authentication, API calls, feed generation
- `internal/hackernews/` - Hacker News API integration, categorization

**Shared Package Libraries**:

- `pkg/database/` - SQLite caching, provider utilities, and database interfaces
- `pkg/feed/` - Atom feed generation, custom XML formatting, and feed helpers
- `pkg/http/` - HTTP client utilities and response handling
- `pkg/opengraph/` - OpenGraph metadata fetching and caching
- `pkg/filesystem/` - File system utilities and path management
- `pkg/utils/` - URL utilities and common helper functions
- `pkg/testutil/` - Golden file testing utilities
- `pkg/interfaces/` - Shared interface definitions

### Key Architecture Patterns

**OAuth2 Authentication Flow** (Reddit):

- Local HTTP server on port 8080 for OAuth callback
- Token persistence in config.yaml
- Automatic token refresh with fallback to browser auth

**Database Integration**:

- SQLite for caching (`modernc.org/sqlite`)
- OpenGraph metadata caching (`pkg/opengraph/`)
- Persistent storage for feed optimization

**Feed Generation**:

- Custom Atom feed format with enhanced metadata
- OpenGraph integration for rich content
- Configurable filtering (score, comments, points)

## Common Development Patterns

**Error Handling**: Use `log/slog` for structured logging throughout the codebase

**Configuration**: All providers read from unified `config.yaml` structure with Viper

**Testing**: Use `//go:build !ci` to skip tests in CI environments when needed

**Golden File Testing**: Use `task update-golden` to update test fixtures when implementation changes. Golden files are stored in testdata directories for consistent test results.

**Authentication State Management**: Reddit provider manages OAuth2 tokens automatically with graceful fallbacks

## Important Implementation Details

**Reddit Authentication Gotcha**: The OAuth2 server must be properly shut down after token exchange. The `serverCancel()` call is critical to prevent hanging.

**Provider Instantiation**: Each provider is created with CLI flags that override config file values.

**OpenGraph Integration**: Feed items are enhanced with OpenGraph metadata for better client compatibility.

## Project Structure

```text
feed-forge/
├── cmd/feed-forge/              # Main application entry point
├── internal/
│   ├── config/                  # Configuration management
│   ├── hackernews/              # Hacker News provider implementation
│   ├── reddit/                  # Reddit provider implementation
│   └── pkg/providers/           # Provider interface and registry
├── pkg/                         # Shared packages
│   ├── config/                  # Configuration utilities
│   ├── database/                # SQLite caching and database interfaces
│   ├── feed/                    # Atom feed generation and custom XML
│   ├── filesystem/              # File system utilities
│   ├── http/                    # HTTP client and response handling
│   ├── interfaces/              # Shared interface definitions
│   ├── opengraph/               # OpenGraph metadata fetching
│   ├── testutil/                # Golden file testing utilities
│   └── utils/                   # URL and common utilities
├── testdata/                    # Test fixtures and golden files
└── llm-shared/                  # Development guidelines submodule
```

## Development Guidelines

This project follows `llm-shared` conventions:

- Always run `gofmt -w .` after Go code changes
- Use `task build` instead of `go build` to ensure tests and linting
- Requires Go 1.24+ for compilation and development
- Tech stack guidelines: `llm-shared/project_tech_stack.md`
- Function analysis: `go run llm-shared/utils/gofuncs.go -dir .`
