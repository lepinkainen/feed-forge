# pkg/config

This package loads a configuration document from a local file or a remote URL, with
fallback between them. It does not handle the main `config.yaml`.

## What loads what

feed-forge has two separate configuration paths. Do not confuse them.

**1. The main configuration: `config.yaml`.** Kong and `kong-yaml` load it in
`cmd/feed-forge/main.go`. Viper is not used, and there is no `internal/config`
package. A CLI flag always overrides a file value.

Kong fills the sub-struct of the active command only. The `generate` and `preview`
commands therefore call `loadProviderConfigFromYAML()` in `main.go` to read a provider
section directly. Every provider `Config` struct needs `yaml` tags for that to work.

**2. Reference data: this package.** Use it for data that lives outside `config.yaml`
and can come from a URL. The only current caller is
`internal/hackernews/config.go`, which loads the domain-to-category map.

## API

```go
// Try localPath, then remoteURL. Unmarshal into target.
func LoadOrFetch(localPath, remoteURL string, target any) error

// The same, with explicit timeout, retry, and fallback options.
func LoadFromURLWithFallback(config *LoaderConfig, target any) error

// Defaults: 10s timeout, 3 retries, fallback on.
func DefaultLoaderConfig() *LoaderConfig
```

`LoadOrFetch` is the entry point for providers. It detects JSON or YAML from the
content, so the caller does not name the format.

Remote fetches go through `pkg/api`, so they get rate limiting and retries.

Errors: `ErrConfigNotFound`, `ErrConfigInvalid`, `ErrUnsupportedFormat`. Compare them
with `errors.Is()`.

## Pattern for reference data

Give every remote source an embedded default. `internal/hackernews/config.go` shows
the shape:

1. If a local path is set, call `LoadOrFetch`.
2. If that fails, read the copy embedded in `configs/`.
3. If that also fails, log a warning and turn the feature off.

A network failure must never stop feed generation.

CAUTION: Do not put credentials in a file that this package fetches from a URL. API
keys belong in `config.yaml` or in an environment variable. Read the `anthropic:`
section of `config_example.yaml` for the pattern.
