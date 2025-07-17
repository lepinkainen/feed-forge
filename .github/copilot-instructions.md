# Copilot Instructions for feed-forge

## Project Overview

- **feed-forge** is a unified RSS feed generator with a provider-based architecture.
- Major providers: **Hacker News** and **Reddit** (see `internal/hackernews/`, `internal/reddit/`).
- Extensible: Add new feed sources by implementing the provider interface (`pkg/providers/provider.go`).
- CLI entry: `cmd/feed-forge/main.go` (uses Kong for CLI parsing).
- Configuration: Centralized YAML config (`config.yaml`), loaded via Viper (`internal/config/config.go`).

## Key Workflows

- **Build**: Use [Taskfile.dev](https://taskfile.dev) tasks, not raw `go` commands.
  - `task build` — Build all (includes lint/test)
  - `task test` — Run all tests
  - `task lint` — Lint/format
  - `task clean` — Clean artifacts
  - `task build-linux` — Cross-compile for Linux
  - `task update-golden` — Update golden test files
- **Run**: Use built binary in `build/` (e.g., `./build/feed-forge reddit ...`).
- **Test Data**: Use `testdata/fixtures/` for inputs, `testdata/golden/` for expected outputs. Golden files must be manually verified before commit.

## Architecture Patterns

- **Provider Interface**: All feed sources implement `GenerateFeed(outfile string, reauth bool) error`.
- **Provider Registry**: Dynamic provider management via registry/factory pattern (`pkg/providers/registry.go`).
- **Config Layers**:
  - Central config: YAML, persistent state, OAuth tokens
  - Provider config: JSON, remote fetch, feature flags
  - External config: Remote/cached data (e.g., domain categorization)
- **Testing**: Golden file pattern for output validation. Use relative paths for test data.

## Conventions & Integration

- **No direct Go commands**: Always use `task` for builds/tests/linting.
- **Config**: All provider and app settings in `config.yaml` (see example in root `README.md`).
- **OAuth2**: Reddit provider manages tokens and reauth automatically.
- **Shared LLM Tools**: See `llm-shared/` for function analyzers and doc validation utilities.
- **Documentation Validation**: Use `llm-shared/utils/validate-docs/` for doc/code consistency.

## Examples

- Build: `task build`
- Run Reddit: `./build/feed-forge reddit -o reddit.xml --min-score 100`
- Run Hacker News: `./build/feed-forge hacker-news -o hackernews.xml --min-points 50`
- Update golden files: `task update-golden`

## References

- Main entry: `cmd/feed-forge/main.go`
- Provider interface: `pkg/providers/provider.go`
- Config: `internal/config/config.go`, `config.yaml`
- Test data: `testdata/`
- LLM tools: `llm-shared/`

---

If any section is unclear or missing, please provide feedback for further refinement.
