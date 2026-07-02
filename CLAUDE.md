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

- `./build/feed-forge reddit -o output.xml --min-score 100` - Reddit feed
- `./build/feed-forge hacker-news -o output.xml --min-points 50` - Hacker News feed

**Bulletin aggregator** (see Bulletin Pipeline below):

- `./build/feed-forge bulletin-fetch` - Poll source feeds, extract full text, store items (cron every ~30m)
- `./build/feed-forge bulletin-generate` - Dedup + summarise unpublished items into a new stored bulletin; the only stage that calls the model (cron at fixed slots, e.g. `45 7,17`). Requires `ANTHROPIC_API_KEY`.
- `./build/feed-forge bulletin-publish -o bulletin.xml` - Render stored bulletins into HTML pages + the Atom feed; no model, no DB writes, so it can be re-run any time to rebuild every page (cron just after generate)
- `./build/feed-forge bulletin-summarize` - Debug: print the digest for current unpublished items to stdout without writing or marking anything (for prompt/model iteration). Requires `ANTHROPIC_API_KEY`.

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
- Supports `reddit` and `hacker-news` subcommands
- Handles configuration loading and provider instantiation
- **IMPORTANT**: Kong only populates CLI sub-struct fields for the active command. When running `generate` or `preview`, the provider-specific sub-structs (e.g., `CLI.Reddit.*`) are NOT populated. The `generate` and `preview` commands use `loadProviderConfigFromYAML()` to load provider config directly from YAML instead. Provider Config structs must have `yaml` tags for this to work.

**Configuration System** (`internal/config/config.go` and `pkg/config/loader.go`):

- Viper-based YAML configuration with fallback to defaults
- Unified config structure for all providers with CLI flag overrides
- Automatic config file creation and OAuth2 token persistence

**Provider Implementations**:

- `internal/reddit-json/` - Reddit feed access, simplified authentication-free approach (public feeds only)
- `internal/hackernews/` - Hacker News API integration, categorization, story caching

**Shared Package Libraries**:

- `pkg/api/` - Enhanced HTTP client with rate limiting, retries, and standardized error handling
- `pkg/config/` - Configuration loading utilities with URL/file fallback support
- `pkg/database/` - SQLite caching, provider utilities, and database interfaces
- `pkg/feed/` - Template-based Atom feed generation and feed helpers
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
- Reddit provider: `UseContentDB: false` (stateless JSON parsing)
- HackerNews provider: `UseContentDB: true` with "hackernews.db" (story caching)
- All providers share OpenGraph database for metadata caching

**Database Integration**:

- SQLite for caching (`modernc.org/sqlite`)
- OpenGraph metadata caching (`pkg/opengraph/`) shared across all providers
- Provider-specific content databases (optional, configurable per provider)

**Template-Based Feed Generation**:

- Go template-based Atom feeds with provider-specific customization
- OpenGraph integration for rich content with concurrent fetching
- Provider-specific metadata using standard Atom categories
- Configurable filtering (score, comments, points)
- Support for multiple link types and extended author information
- RSS reader compatible (no custom namespaces)

### Bulletin Pipeline (`internal/bulletin/`)

A **separate code path**, intentionally outside the provider registry — it does not implement `FeedProvider` and is not discovered by `generate`. It aggregates many high-frequency outlets into periodic summarised digests instead of one-feed-per-source.

- **Three decoupled stages**: `bulletin-fetch` (frequent cron) accumulates items; `bulletin-generate` (2×/day cron) turns the unpublished backlog into one stored bulletin; `bulletin-publish` (just after generate) renders the stored bulletins into HTML + Atom. State lives in `bulletin.db`; an item's `bulletin_id IS NULL` means unpublished (i.e. not yet folded into a generated bulletin). Generate is idempotent and catches up (it consumes everything unpublished); publish is a pure, side-effect-free render, so re-running it rebuilds every page from existing data — "recreate all the HTML" is just `bulletin-publish`.
- **Fetch** (`fetch.go`): parses each source feed with `gofeed`, fetches each new article page through `httpcache.CachedGetWithStale` (conditional GET/ETag reused), extracts full text with `go-shiori/go-readability`, falls back to the feed's own content when extraction is thin. `HasItem` skips already-stored URLs so article pages aren't re-fetched.
- **SimHash dedup** (`simhash.go`, `dedup.go`): 64-bit SimHash over stopword-stripped full text; greedy single-pass clustering groups stories within `simhash-threshold` Hamming distance (default 3). Fingerprints stored as SQLite INTEGER (int64 bit pattern).
- **Generate** (`publish.go`, `bulletin.Generate`): the only stage that calls the model and the only one that writes bulletins. Dedup happens *before* summarisation to save tokens; cluster representatives + source URLs go to Anthropic (`claude-haiku-4-5`, `summarize.go`) in a **single call** that returns a topic-grouped HTML digest. `store.CreateBulletin` inserts the bulletin row and marks its items published in one transaction. Prompt overridable via `prompt-file` for iteration.
- **Publish** (`publish.go`, `bulletin.Publish`): reads all stored bulletins (no model, no DB writes) and renders one Atom `<entry>` = whole digest via `templates/bulletin-atom.tmpl` (newest `feedEntryLimit`). When `output-dir` is set it also (re)exports HTML pages to `<output-dir>/html/` — a dated archive page per bulletin plus a stable `bulletin-latest.html` — via `templates/bulletin-page.html.tmpl`. Because it's a pure render over stored data it's safe to re-run to rebuild everything (e.g. after a template change). The `generate` **feed** command's `index.html` links to `html/bulletin-latest.html` when it exists.
- **Config**: `bulletin:` section in `config.yaml`, loaded via `loadProviderConfigFromYAML`. The Anthropic API key comes from the shared top-level `anthropic:` section (`pkg/llm.Config`, key `api-key`), resolved via `llm.Config.ResolveAPIKey` which falls back to the `ANTHROPIC_API_KEY` env var. This general section is reusable by any future model-using processor, not bulletin-specific.

## Common Development Patterns

**Error Handling**: Use `log/slog` for structured logging throughout the codebase. Enhanced HTTP client provides standardized error handling with retry logic and structured error types.

**Database Timestamps**: **CRITICAL** - Use `time.Time` fields in Go structs and `TIMESTAMP` column affinity in SQLite. Let the `modernc.org/sqlite` driver handle serialization — it round-trips `time.Time` as RFC3339Nano text, which is lexicographically sortable so `ORDER BY ... DESC` equals chronological DESC. Never store raw upstream date strings (RFC1123Z, custom formats, day-first locales) in a sortable column: string-sort on non-ISO formats silently disagrees with chronological order and breaks feed ordering. Parse source timestamps into `time.Time` at the API/RSS boundary, never later. The `TIMESTAMP`/`DATETIME` column declaration is what triggers the driver's auto-scan back into `time.Time`; `TEXT` columns will not auto-convert on `rows.Scan`.

**HTTP Client Usage**: **CRITICAL** - Always use `pkg/api` enhanced clients for API calls. Provider-specific clients with built-in rate limiting and retry policies:

- `api.NewRedditClient(baseClient)` - Reddit-optimized with 1-second rate limiting
- `api.NewHackerNewsClient()` - HackerNews-optimized with conservative rate limiting
- `api.NewGenericClient()` - General purpose with minimal configuration

Never make direct HTTP calls - use these enhanced clients to avoid rate limiting and API failures.

**Feed Generation**: Use unified template-based generation for rich feeds:

- `feed.GenerateAtomFeed()` - Unified feed generation with provider-agnostic logic
- `feed.SaveAtomFeedToFile()` - Generate and save Atom feeds to file
- `feed.NewTemplateGenerator()` - Create template generator for advanced use cases

**Configuration**: All providers use shared configuration utilities (`pkg/config`) with URL/file fallback, format detection (JSON/YAML), and unified config structure

**Provider Factory Pattern**: Use provider registry (`providers.DefaultRegistry`) for dynamic provider discovery and instantiation. New providers register themselves with metadata and factory functions.

**Testing**: Use `//go:build !ci` to skip tests in CI environments when needed

**Golden File Testing**: Use `task update-golden` to update test fixtures when implementation changes are stable and verified. Golden files are stored in testdata directories for consistent test results. Always review golden file diffs before committing - they represent expected output changes.

## Important Implementation Details

**Provider Instantiation**: Providers are created using factory functions:

- Reddit: `redditjson.NewRedditProvider()`
- Hacker News: `hackernews.NewHackerNewsProvider()`

CLI flags override config file values. Each provider inherits from `BaseProvider` with database configuration.

**Enhanced HTTP Client**: All API calls use `pkg/api` enhanced client with configurable rate limiting, exponential backoff retries, and provider-specific policies

**Template-Based Feed Generation**: Providers use Go templates for flexible Atom feed generation with OpenGraph integration, multiple links, rich content, and provider-specific metadata using standard categories

**OpenGraph Integration**: Feed items are enhanced with OpenGraph metadata for better client compatibility, with concurrent fetching and caching.

## Project Structure

```text
feed-forge/
├── cmd/feed-forge/              # Main application entry point
├── internal/
│   ├── config/                  # Configuration management
│   ├── hackernews/              # Hacker News provider implementation
│   └── reddit-json/             # Reddit JSON provider implementation
├── pkg/                         # Shared packages
│   ├── api/                     # Enhanced HTTP client with rate limiting and retries
│   ├── config/                  # Configuration loading utilities with fallback support
│   ├── database/                # SQLite caching and database interfaces
│   ├── feed/                    # Template-based Atom generation and feed helpers
│   ├── filesystem/              # File system utilities
│   ├── interfaces/              # Shared interface definitions
│   ├── opengraph/               # OpenGraph metadata fetching and caching
│   ├── providers/               # Provider interfaces and base implementations
│   ├── testutil/                # Golden file testing utilities
│   └── utils/                   # URL and common utilities
├── testdata/                    # Test fixtures and golden files
├── templates/                   # Go template files for feed generation
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
