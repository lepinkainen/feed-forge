# LLM_ONLY_RUNTIME_ARCHITECTURE

## Execution entry

File: `cmd/feed-forge/main.go`

Startup order:

1. `resolveConfigPath(os.Args[1:])`
   - explicit `--config VALUE` wins
   - explicit `--config=VALUE` wins
   - else `findConfigFile()`
2. `findConfigFile()` fallback order:
   - `$XDG_CONFIG_HOME/feed-forge/config.yaml`, or `$HOME/.config/feed-forge/config.yaml`
   - executable directory `config.yaml`
   - current dir `config.yaml`
3. `kong.Parse(&CLI, kong.Configuration(kongyaml.Loader, configPath))`
4. logging: `--debug` => `slog.LevelDebug`; default `slog.LevelWarn`
5. `--cache-dir` calls `filesystem.SetCacheDir` before provider construction
6. switch on `ctx.Command()`

## Global CLI fields

`CLI` root fields:

- `Config string`, default `config.yaml`; used only by path resolver/Kong loader.
- `Debug bool`; log level.
- `OutputDir string` / YAML `output-dir`; prepended to relative provider outfiles by `resolveOutfile`.
- `FeedBaseURL string` / YAML `feed-base-url`; public URL base used in generated OPML, default `https://endymion.xyz/rss/`.
- `CacheDir string` / YAML `cache-dir`; cache DB base path.

## Provider-specific commands

Direct commands build typed provider config from Kong-populated CLI substruct using `buildProviderConfig(name)`, then `providers.DefaultRegistry.CreateProvider(name, cfg)`, then `provider.GenerateFeed(resolveOutfile(outfile))`.

Direct commands:

- `reddit`
- `hackernews`
- `fingerpori`
- `feissarimokat`
- `oglaf`
- `tildes`
- `youtube`

Important asymmetry:

- `reddit` direct command explicitly rejects missing `feed-id` or `username`.
- `generate` path does not have same explicit guard; provider/API errors later if config bad.

## `preview <provider>` flow

Function: `previewFeed(providerName, limit, index, configPath)`

1. lookup `ProviderInfo` from registry
2. require `info.Preview != nil`
3. instantiate `info.ConfigFactory()` if present
4. load YAML section with `loadProviderConfigFromYAML`
5. create provider
6. call `FetchItems(limit)`
7. if `--index >= 0`: print Atom `<entry>` XML via `preview.FormatXMLItem`
8. else run Bubble Tea TUI via `preview.Run`

## `generate` flow

Functions: `generateAll`, `configuredProviders`, `generateProvider`, `generateFeedIndex`, `generateOPML`

`configuredProviders(configPath)`:

- reads YAML into `map[string]any`
- keeps keys that exist in `providers.DefaultRegistry`
- ignores unknown top-level keys like `output-dir`, `feed-base-url`, `cache-dir`

`generateAll(configPath)`:

- runs each configured provider concurrently using goroutines
- stores `feedResult{Provider, FeedName, Filename, Status}`
- statuses: `generated`, `skipped`, `failed`
- after all complete: `generateFeedIndex(results)`
- returns error if any provider failed

`generateProvider(configPath, name)`:

1. registry lookup
2. create config via `ConfigFactory`
3. decode provider YAML section into config
4. read common `GenerateConfig` with reflection via `providers.GetGenerateConfig`
5. default outfile: `<name>.xml` if empty
6. `result.Filename` is un-resolved relative configured outfile
7. `resolveOutfile` applies `output-dir`
8. parse interval with `parseInterval`; invalid/empty => `15m`
9. skip if outfile exists and mtime age < interval
10. create provider
11. defer `Close()` if provider implements it
12. call `GenerateFeed(outfile)`

`generateFeedIndex`:

- no-op unless `CLI.OutputDir != ""`
- includes non-failed feeds with filename set
- sorts by provider name
- template: `feed-index.html.tmpl` via `feed.ReadTemplateContent`
- output: `${output-dir}/index.html`
- links `feeds.opml`

`generateOPML`:

- writes `${output-dir}/feeds.opml`
- OPML version `2.0`
- one `<outline type="rss" xmlUrl="...">` per non-failed feed
- `text`/`title` use provider display name (`ProviderInfo.Preview.ProviderName`) so FreshRSS feed names match processor names
- `xmlUrl` uses `feed-base-url` joined with each feed filename

## Config decoding

- Kong YAML loads root + direct command fields for direct commands.
- `generate` and `preview` decode provider section manually via `yaml.Node.Decode(target)`.
- Provider config structs embed `providers.GenerateConfig` with `yaml:",inline"` to receive `outfile` and `interval`.
- Config keys use kebab-case YAML tags (`min-score`, `feed-url`, etc.).

## Package role map

- `cmd/feed-forge`: CLI wiring, config resolution, generate orchestration.
- `internal/<provider>`: source fetch/parse/filter, provider registration, item types.
- `pkg/providers`: registry, `FeedProvider`, `FeedItem`, `BaseProvider`.
- `pkg/providerfeed`: shared `GenerateFeed` implementation builder.
- `pkg/feed`: Atom generation, templates, template helpers, template FS override/fallback.
- `pkg/feedmeta`: feed metadata config.
- `pkg/api`: HTTP client wrapper, retries, rate limiters, conditional GET support.
- `pkg/httpcache`: SQLite ETag/Last-Modified validator cache.
- `pkg/opengraph`: OG fetcher/cache with SSRF guards and optional proxy.
- `pkg/database`: generic SQLite wrapper/cache for provider content DBs.
- `pkg/filesystem`: cache path and output directory helpers.
- `pkg/preview`: TUI and XML item preview.
- `pkg/config`: local/remote JSON/YAML config loader used by HN domain mapping.
- `pkg/urlutils`: URL validation, safe outbound fetch checks, relative URL resolution.
- `templates`: embedded Atom/index templates.
- `configs`: embedded JSON configs (HN domain mapping).
