# CRUSH.md - Feed Forge Development Guidelines

## Build & Test Commands

**Task Management**: Always use `task` commands instead of direct `go` commands
- `task build` - Build binary (runs test, lint automatically)
- `task test` - Run all tests  
- `task test ./path/to/package` - Run single package tests
- `task test -run TestName` - Run specific test
- `task test-ci` - CI tests with coverage (`go test -tags=ci -cover -v ./...`)
- `task lint` - Run formatters and linters
- `task update-golden` - Update golden test files
- Use built binary: `./build/feed-forge [provider] -o feed.xml`

**Search Tools**: Use `rg` (ripgrep) instead of `grep`, `fd` instead of `find`

## Code Style Guidelines

### Formatting & Imports
- Always run `goimports -w .` after changes (NOT gofmt)
- Use `any` instead of `interface{}`
- Error handling: Use `errors.Is()` and `errors.As()` for robust error handling
- Go version: 1.25.1+ (check versions.md for latest)

### Naming & Structure  
- Packages: lower_snake_case
- Exported symbols: PascalCase
- Tests: `TestXxx` format alongside source files
- Keep files focused and split when growing too large

### Architecture Patterns
- Implement `FeedProvider` interface for new providers
- Use provider registry/factory pattern (`pkg/providers/registry.go`)
- All providers inherit from `BaseProvider` with shared database connections
- Use enhanced HTTP clients from `pkg/api/` for all API calls

### Testing
- Golden file pattern for output validation
- Use relative paths for test data in `testdata/`
- Table-driven tests for provider logic
- Verify golden file diffs manually before committing
- Basic unit tests required for all features

### Configuration
- Central YAML config (`config.yaml`) loaded via Viper
- Provider-specific settings in unified config structure  
- CLI flags override config file values

### Git & Project Management
- NEVER commit to main/master directly
- Use feature branches for all changes
- Keep commits small and focused
- Task not complete until `task build` succeeds
- Build artifacts go in `build/` directory

### HTTP Requests
- Use custom user agent: `feed-forge/[version]` for external API calls
- All HTTP calls through `pkg/api` enhanced clients

Use modernc.org/sqlite for database operations.