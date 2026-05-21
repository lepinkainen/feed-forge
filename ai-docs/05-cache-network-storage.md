# LLM_ONLY_CACHE_NETWORK_STORAGE

## Cache directory

File: `pkg/filesystem/utils.go`

Cache dir resolution:

1. `filesystem.SetCacheDir(dir)` from CLI `--cache-dir` / YAML `cache-dir` if set.
2. `$XDG_CACHE_HOME/feed-forge`
3. `$HOME/.cache/feed-forge`

`filesystem.GetDefaultPath(filename)` returns `<cache-dir>/<filename>`.

Output directory helper:

- `EnsureDirectoryExists(filePath)` creates parent dir with `0750`.
- no-op when parent is `.`.

## BaseProvider storage

File: `pkg/providers/base.go`

Every `NewBaseProvider` creates:

- OpenGraph DB: `opengraph.db`
- HTTP validator cache DB: `http_cache.db`
- optional content DB if `DatabaseConfig.UseContentDB && ContentDBName != ""`

Content DB users:

- `hackernews`: `hackernews.db`
- `oglaf`: `oglaf.db`

Stateless providers still have OG DB and HTTP cache from base, even when not passed to generator.

## Generic provider content DB

File: `pkg/database/types.go`

`database.NewDatabase(Config)`:

- cached by path in process-global `dbCache`
- driver default `sqlite`
- SQLite pragmas: busy timeout, WAL, synchronous NORMAL, temp_store memory, mmap size
- pool: max open 10, max idle 5, lifetime 1h
- exposes `DB() *sql.DB`, `ExecuteSchema`, `Transaction`

Caution:

- `Close()` removes connection from cache and closes DB.
- Multiple providers sharing path could get same pointer; close timing matters.

## HTTP validator cache

File: `pkg/httpcache/cache.go`

Purpose: conditional GET validators only, not response body.

Table: `http_validators(url TEXT PRIMARY KEY, etag, last_modified, updated_at)`

Use:

```go
body, err := httpcache.CachedGet(ctx, client, store, url, headers)
```

Behavior:

1. read previous ETag/Last-Modified from store
2. call `api.EnhancedClient.GetConditional`
3. if HTTP 304 => return `httpcache.ErrNotModified`
4. if 200-ish => save validators and return body

Consumers:

- `feissarimokat` RSS feed
- `oglaf` RSS feed

`providerfeed.BuildGenerator` treats `ErrNotModified` specially only if `errors.Is(err, httpcache.ErrNotModified)`.

## OpenGraph DB

Files: `pkg/opengraph/database.go`, `pkg/opengraph/types.go`

Data fields:

- `URL`, `Title`, `Description`, `Image`, `SiteName`
- `ETag`, `LastModified`
- `FetchedAt`, `ExpiresAt`

Table: `opengraph_cache`

- unique URL
- fetch_success bool
- indexes on URL and expires_at
- migrations add ETag/LastModified if old DB

Cache semantics:

- `GetCachedData` returns only non-expired successful rows.
- failed fetches are cached as recent failures; fetcher can skip recent failures.
- expired successful data can be used for conditional revalidation.
- default cache TTL: `DefaultCacheHours = 24`.

## OpenGraph fetcher

File: `pkg/opengraph/fetcher.go`

`NewFetcher(db)` / `NewFetcherWithProxy(db, proxy)`.

Fetch path:

1. validate URL with `urlutils.IsFetchableURLWithContext`.
2. skip blocked URLs.
3. return fresh cached successful data if available.
4. skip recent failures.
5. if expired cached data has validators, conditional GET.
6. on 304, refresh expiry of expired data.
7. parse HTML OpenGraph tags.
8. save success or failure.

Concurrency/rate limits:

- global semaphore size 5.
- per-URL mutex via `sync.Map`.
- per-domain minimum 1 second between fetches.
- HTTP timeout 10s; per-fetch context timeout 15s.
- redirect limit 10.

Proxy:

- `ProxyConfig{URL, Secret}`.
- Only proxiable Reddit URLs use proxy.
- Proxy request uses proxy URL and header `X-Proxy-Secret`; target URL passed by fetcher internals (verify exact header/query before changing proxy contract).

## SSRF / safe outbound fetch

File: `pkg/urlutils/url.go`; fetcher also uses safe transport internals.

`IsFetchableURLWithContext` requires:

- scheme `http` or `https`
- host present
- hostname not `localhost`
- hostname not literal IP
- DNS resolves to at least one IP
- no resolved IP blocked by `IsBlockedFetchAddr`

Blocked IP classes:

- loopback
- private
- link-local unicast/multicast
- multicast
- unspecified
- IPv4 CGNAT `100.64.0.0/10`
- IPv4 link-local `169.254.0.0/16`
- benchmark `198.18.0.0/15`
- IPv4 `>=224.0.0.0`

When adding fetchers, reuse `api.EnhancedClient` or existing safe URL helpers; do not fetch arbitrary user-supplied URLs without checks.

## Enhanced HTTP client

File: `pkg/api/enhanced_client.go`

Features:

- default/user-agent headers
- rate limiter before request
- retry policy around operations
- JSON decode helper
- raw GET helper
- conditional GET helper
- logs duration/success/failure through slog

Constructors:

- `api.NewRedditClient(baseClient)`
  - timeout 30s
  - browser-like TLS transport
  - rate limiter 2s
  - default retry policy
  - Accept JSON
- `api.NewHackerNewsClient()`
  - timeout 30s
  - rate limiter 500ms
  - conservative retry policy
  - Accept JSON
- `api.NewGenericClient()`
  - timeout 30s
  - no rate limiter
  - conservative retry policy

Conditional response:

```go
type ConditionalResponse struct {
    NotModified bool
    Body []byte
    ETag string
    LastModified string
}
```

## Provider network endpoints

- Reddit: `https://www.reddit.com/.json?feed=<feed-id>&user=<username>` or `proxy-url`.
- Hacker News: Algolia front page + item endpoints.
- Fingerpori: `https://www.hs.fi/api/laneitems/39221/list/normal/290`.
- Feissarimokat: `https://static.feissarimokat.com/dynamic/latest/posts.rss` + post pages.
- Oglaf: default `https://www.oglaf.com/feeds/rss/` + comic pages.
- Tildes: group Atom feeds; see `internal/tildes/api.go`.
- YouTube: channel Atom feeds; channel IDs converted to YouTube feed URLs.

## Storage edit checklist

1. If adding content DB, set unique `ContentDBName` and `UseContentDB: true`.
2. Schema init must be idempotent.
3. Use WAL-friendly access patterns; avoid long write locks.
4. Close providers in new orchestration paths.
5. Preserve `ErrNotModified` wrapping with `%w` if caller needs `errors.Is`.
6. Do not store secrets in DB/cache/logs.
