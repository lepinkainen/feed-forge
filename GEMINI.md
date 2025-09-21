# Project Context for Gemini CLI

This document provides essential context for the Gemini CLI agent operating within the `feed-forge` project.

## Project Overview

`feed-forge` is a unified RSS feed generator with a provider-based architecture. The main providers are Hacker News and Reddit. The application is designed to be extensible, allowing new feed sources to be added by implementing the provider interface.

- **CLI Entrypoint**: `cmd/feed-forge/main.go`
- **Provider Interface**: `pkg/providers/provider.go`
- **Configuration**: `internal/config/config.go` (loads `config.yaml`)
- **Hacker News Provider**: `internal/hackernews/`
- **Reddit Providers**: `internal/reddit-json/` and `internal/reddit-oauth/`

## Developer Workflows

The project uses **Taskfile.dev** for managing development tasks. Do not use raw `go` commands for building, testing, or linting.

- **Build, Test, and Lint**: `task build`
- **Run Tests**: `task test`
- **Run Linter**: `task lint`
- **Update Golden Test Files**: `task update-golden`

To run the application, use the binary in the `build/` directory:

- `./build/feed-forge reddit-oauth -o reddit.xml --min-score 100`
- `./build/feed-forge hacker-news -o hackernews.xml --min-points 50`

## Architecture

- **Provider-Based Architecture**: Each feed source (e.g., Hacker News, Reddit) implements the `FeedProvider` interface defined in `pkg/providers/provider.go`. This allows for a consistent way to generate feeds from different sources.
- **Provider Registry**: A factory pattern is used to manage and discover providers dynamically. See `pkg/providers/registry.go`.
- **Configuration**: The application uses a centralized YAML configuration file (`config.yaml`) that is loaded using Viper. Provider-specific configurations can be defined within this file.
- **HTTP Clients**: Always use the enhanced HTTP clients from `pkg/api` for making API calls. These clients include built-in rate limiting and retry logic.
  - `api.NewRedditClient()`
  - `api.NewHackerNewsClient()`
- **Database**: SQLite is used for caching. The OpenGraph database (`pkg/opengraph/`) is shared across all providers for metadata caching. Some providers may also have their own content-specific databases.
- **Testing**: The project relies heavily on golden file testing. Use `task update-golden` to update the golden files after making changes.

## Gemini Added Memories

- Always use "task build" to test and lint the project instead of running "go test" or "go build"
- If llm-shared/ exists in the project, use it for up to date technology preferences and generic project guidance
