# LLM_ONLY_PROVIDER_CONTRACT

## Core interfaces

File: `pkg/providers/provider.go`

```go
type FeedProvider interface {
    GenerateFeed(outfile string) error
    FetchItems(limit int) ([]FeedItem, error)
}

type FeedItem interface {
    Title() string
    Link() string
    CommentsLink() string
    Author() string
    Score() int
    CommentCount() int
    CreatedAt() time.Time
    Categories() []string
    ImageURL() string
    Content() string
}
```

Optional item methods consumed by feed generation:

- `AuthorURI() string` => `TemplateItem.AuthorURI`
- `Subreddit() string` => `TemplateItem.Subreddit`
- `ItemDomain() string` => `TemplateItem.Domain`

## ProviderInfo / registry

Provider registration uses global `providers.DefaultRegistry`.

```go
type ProviderInfo struct {
    Name string
    Description string
    Version string
    Factory ProviderFactory
    ConfigFactory func() any
    Preview *PreviewInfo
}
```

Registration style in each `internal/<provider>/provider.go`:

```go
func init() {
    providers.MustRegister("provider-name", &providers.ProviderInfo{...})
}
```

Main imports provider packages so `init()` runs and typed configs are available in `buildProviderConfig`.

## Common config

File: `pkg/providers/provider.go`

```go
type GenerateConfig struct {
    Outfile  string `yaml:"outfile"`
    Interval string `yaml:"interval"`
}
```

Provider config pattern:

```go
type Config struct {
    providers.GenerateConfig `yaml:",inline"`
    // provider fields with yaml tags
}
```

`generate` command extracts common fields using `providers.GetGenerateConfig(config any)` via reflection. If provider config lacks embedded `GenerateConfig`, default outfile becomes `<provider>.xml` and interval default `15m`.

## BaseProvider

File: `pkg/providers/base.go`

Fields:

- `ContentDB *database.Database` optional provider content cache.
- `OgDB *opengraph.Database` always initialized by `NewBaseProvider`.
- `HTTPCache *httpcache.Store` always initialized by `NewBaseProvider`.
- internal `generateFeed func(outfile string) error` used by shared generator.

`NewBaseProvider(DatabaseConfig)`:

- OpenGraph DB path: `filesystem.GetDefaultPath("opengraph.db")`.
- HTTP validator cache path: `filesystem.GetDefaultPath("http_cache.db")`.
- optional content DB path: `filesystem.GetDefaultPath(ContentDBName)` if `UseContentDB`.
- cleans expired OpenGraph cache on startup.

Use `Close()` when provider can outlive one operation. `generate` path defers close if provider implements `Close()`. Direct command paths currently do not defer close.

## Shared GenerateFeed

File: `pkg/providerfeed/generator.go`

Most providers do:

```go
provider.SetGenerateFeedFunc(providerfeed.BuildGenerator(
    provider.FetchItems,
    previewInfo,
    optionalFeedConfigFunc,
    optionalOgDB,
))
```

`BuildGenerator` behavior:

1. call `FetchItems(0)`
2. if error is `httpcache.ErrNotModified` and outfile exists: bump outfile mtime, return nil
3. ensure output directory exists via `filesystem.EnsureDirectoryExists(outfile)`
4. choose feed metadata: `preview.Config`, overridden by `configFunc()` if non-nil
5. call `feed.SaveAtomFeedToFileWithEmbeddedTemplate(items, preview.TemplateName, outfile, cfg, ogDB)`
6. log feed generation

Pass `ogDB` only when template/source benefits from OpenGraph enrichment. Some providers pass nil.

## PreviewInfo

Required for `preview` command and shared generator metadata.

```go
type PreviewInfo struct {
    feedmeta.Config
    ProviderName string
    TemplateName string
}
```

Provider-level `previewInfo` usually contains:

- Title, Link, Description, Author, ID
- display provider name
- template name matching `templates/<template>.tmpl`

## Add-provider checklist

Source docs also exist at `docs/adding-a-provider.md`, but may be stale; follow current patterns below.

1. Add `internal/newprovider/` with:
   - `provider.go`
   - `types.go`
   - `api.go` or fetch helpers
   - tests adjacent to source
2. Define item type implementing `providers.FeedItem`.
3. Define `Config` embedding `providers.GenerateConfig`.
4. Define `previewInfo` with `TemplateName`.
5. Constructor uses `providers.NewBaseProvider`.
6. Constructor calls `SetGenerateFeedFunc(providerfeed.BuildGenerator(...))`.
7. Factory validates `config.(*Config)` and returns provider.
8. `init()` calls `providers.MustRegister("newprovider", info)`.
9. Add CLI subcommand struct in `cmd/feed-forge/main.go`.
10. Add import to main.
11. Add case to `buildProviderConfig`.
12. Add direct command case in `main()` switch unless intentionally generate-only.
13. Add config example in `config_example.yaml`.
14. Add template `templates/newprovider-atom.tmpl`; embedded automatically by `go:embed *.tmpl`.
15. Add tests for factory, FetchItems, feed output, config decode if non-trivial.
16. Run `gofmt`/`goimports`, `go test ./...`.

## Provider naming conventions

- Registry names are lower no spaces; existing mixed style: `hackernews` (not `hacker-news`), `reddit`, `youtube`.
- CLI command for Hacker News is `hackernews` in current `main.go` tags? Check tags before changing. README/Taskfile may contain stale `hacker-news` references.
- Template names use `<provider>-atom` with hyphens in some cases.

## Error handling expectations

- Fetch errors should return error, not `os.Exit`.
- Provider constructors return wrapped errors with provider context.
- Factory should return clear invalid config type error.
- When upstream 304 is meaningful, return `httpcache.ErrNotModified` so `BuildGenerator` can bump mtime.
- Content parsing should skip bad entries with warn if rest can proceed.

## LLM edit warnings

- Every provider has tests; update fixtures/golden files only when intended.
- Keep provider fetch functions testable: endpoint URLs often vars not consts for httptest overrides.
- Do not leak secrets in URLs/logs. Reddit proxy path uses headers `X-Proxy-Secret`, `X-Feed-ID`, `X-Feed-User`.
- If adding OpenGraph fetches, respect `urlutils.IsFetchableURLWithContext` and safe transport behavior.
