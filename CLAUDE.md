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

**Run commands**:
- `task run-reddit` - Run Reddit feed generation
- `task run-hackernews` - Run Hacker News feed generation

**Direct execution**:
- `./build/feed-forge reddit --reauth` - Force Reddit re-authentication
- `./build/feed-forge reddit -o output.xml --min-score 100`
- `./build/feed-forge hacker-news -o output.xml --min-points 50`

## Architecture Overview

Feed-Forge is a unified RSS feed generator that consolidates functionality from `hntop-rss` and `red-rss` projects. It uses a **provider-based architecture** with a common interface for different feed sources.

### Core Components

**Provider Interface** (`internal/pkg/providers/provider.go`):
- Single method: `GenerateFeed(outfile string, reauth bool) error`
- Implemented by Reddit and Hacker News providers

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

**Authentication State Management**: Reddit provider manages OAuth2 tokens automatically with graceful fallbacks

## Important Implementation Details

**Reddit Authentication Gotcha**: The OAuth2 server must be properly shut down after token exchange. The `serverCancel()` call is critical to prevent hanging.

**Provider Instantiation**: Each provider is created with CLI flags that override config file values.

**OpenGraph Integration**: Feed items are enhanced with OpenGraph metadata for better client compatibility.

## Development Guidelines

This project follows `llm-shared` conventions:
- Always run `gofmt -w .` after Go code changes
- Use `task build` instead of `go build` to ensure tests and linting
- Tech stack guidelines: `llm-shared/project_tech_stack.md`
- Function analysis: `go run llm-shared/utils/gofuncs.go -dir .`