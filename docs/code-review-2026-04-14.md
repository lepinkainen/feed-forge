# Full Code Review — feed-forge (2026-04-14)

Reviewer: Claude (Opus 4.6) · Scope: `cmd/`, `internal/`, `pkg/`, `templates/`,
build/lint config. Excluded: `llm-shared/`, generated artifacts. Read-only pass —
no code changes. See `/Users/shrike/.claude/plans/purrfect-weaving-dongarra.md`
for the methodology.

Findings are grouped by severity, then category. Each entry links a concrete
`file:line`. Severity ladder:

- **Critical** — data corruption, security boundary breach, panic on valid input
- **High** — incorrect behavior under realistic conditions, silent data loss
- **Medium** — code smell with concrete downside (perf cliff, testability)
- **Low** — idiomatic polish, consistency, ergonomics

---

## Critical

### [CRIT] OpenGraph fetcher vulnerable to SSRF via untrusted URLs
**File**: `pkg/opengraph/fetcher.go:68`, `pkg/urlutils/url.go:7-10`
**Category**: Security
**Finding**: `Fetcher.FetchData` takes a URL straight from feed items (Reddit
post links, HN submission URLs) and issues an HTTP GET. The only validation is
`urlutils.IsValidURL`, which accepts any scheme with a non-empty host —
including `http://169.254.169.254/…` (AWS metadata), `http://127.0.0.1:…`
(local services), `file://…` (Go's net/http rejects file, but other schemes
like `gopher://` older Gos accepted), and arbitrary internal IPs. A malicious
upstream post (or compromised proxy response) can coerce feed-forge into
probing the host network. The blocklist at `fetcher.go:425-447` is
substring-based and only covers social networks, not infrastructure.
**Suggested fix**: In `pkg/urlutils/url.go`, tighten `IsValidURL` (or add a new
`IsFetchableURL`) that requires scheme ∈ {http, https}, rejects hosts that
resolve to RFC1918 / loopback / link-local / metadata IPs, and rejects numeric
IP hosts outright unless explicitly allowed. Call it from `FetchData` before
the HTTP request. Consider pairing with a custom `http.Transport` dial hook
that also rejects private IPs (protects against DNS rebinding). (Body size is
already capped at 1MB via `io.LimitReader` in `pkg/opengraph/fetcher.go:250-251`
— no additional cap needed.)

---

## High

### [HIGH] HTML injection into Oglaf feed entries via unescaped title
**File**: `internal/oglaf/provider.go:153-161`
**Category**: Security
**Finding**: `comicDescription` interpolates `title` straight into an `alt=""`
attribute and the surrounding HTML via `fmt.Sprintf`. Titles come from the
Oglaf RSS feed (`rssTitleRegex` at `provider.go:22`) and are extracted with a
hand-rolled regex that preserves raw characters. A comic title containing `"`
or `<` breaks out of the attribute or element. Even absent malicious input,
real titles with embedded quotes will corrupt the generated Atom XML (the
content is then wrapped in CDATA at `templates/oglaf-atom.tmpl`, which hides
the problem until a CDATA-closing sequence appears — see the related CDATA
finding below).
**Suggested fix**: Use `html/template.HTMLEscapeString` on `title`, `link`, and
`imageURL` before formatting; or better, build the fragment with
`html/template.Template` so attribute escaping is context-aware.

### [HIGH] HTML injection into Feissarimokat feed content
**File**: `internal/feissarimokat/api.go:99-104`
**Category**: Security
**Finding**: `processItems` builds feed HTML by concatenating
`rssItem.Description` (raw upstream HTML) with `fmt.Fprintf(&html,
"\n<img src=%q alt=%q>", img, rssItem.ItemTitle)`. `%q` emits Go-syntax
double-quoted strings — not HTML-escaped attribute values. A title containing
a double-quote or backslash would be written with escape sequences like `\"`
that HTML parsers do not decode, producing broken markup; richer payloads
(e.g. `" onload="...`) go through untouched.
**Suggested fix**: Use `html/template` to emit the `<img>` tag, or use
`html.EscapeString` on both `img` and `rssItem.ItemTitle` and emit
`<img src="..." alt="...">` with ordinary `fmt.Fprintf`.

### [HIGH] `xmlEscape` template helper round-trips through HTML entities
**File**: `pkg/feed/template_funcs.go:26-31`
**Category**: Correctness
**Finding**: The helper is `html.UnescapeString` followed by
`html.EscapeString`. Two issues: (1) `html.EscapeString` only escapes `& < > "
'` — it does NOT escape all characters that are invalid in XML 1.0 (control
characters U+0000–U+001F except tab/LF/CR will make the Atom feed
non-parseable). (2) Unescaping first is lossy for text that legitimately
contains the literal string `&amp;` — it becomes `&`, then gets re-encoded,
silently mutating user-visible text that was already well-formed. Feed
readers will see different text than the upstream source for any title
containing entity references.
**Suggested fix**: Swap to `encoding/xml.EscapeText` (writes to an
`io.Writer`) or inline the minimal escape set {`&`, `<`, `>`, `"`, `'`} with
control-char stripping. Remove the `UnescapeString` step — trust whichever
layer already decoded entities from the upstream source, or decode once at
ingestion and never re-decode in templates.

### [HIGH] Retry on 429 double-sleeps (backoff then backoff again)
**File**: `pkg/api/retry.go:165-173` + `:131-140`
**Category**: Correctness
**Finding**: When an operation returns a `http.StatusTooManyRequests` error,
the code sleeps `CalculateBackoff(attempt) * 2` at line 172, then the `for`
loop continues, and at the top of the next iteration sleeps
`CalculateBackoff(attempt-1)` again at line 139. Net effect: attempt 2 waits
~`2× + 1×` of `InitialBackoff`, attempt 3 waits ~`4× + 2×`, etc. For the
default policy (InitialBackoff=1s, Multiplier=2) a rate-limited request
three-attempt sequence sleeps ~9s instead of the intended ~5s. Worse, the
extra sleep holds a goroutine uninterruptibly — see the related context
finding.
**Suggested fix**: Remove the rate-limit-specific `time.Sleep` at line 172
(or remove the generic backoff at line 139 and keep only the rate-limited
one). Prefer: extract one `waitBeforeAttempt` helper called at the top of
each non-first attempt, and have it consult `IsRateLimitError(lastErr)` to
pick the multiplier. Add jitter while you're there.

### [HIGH] SQL table names interpolated via `fmt.Sprintf` throughout cache layer
**File**: `pkg/database/cache.go` (every query, e.g. `:57`, `:77`, `:92`,
`:125`, `:164`)
**Category**: Security
**Finding**: `Cache` stores `tableName` as a string and builds queries with
`fmt.Sprintf`. Identifier placeholders are not supported by `database/sql`
drivers, so this is the only option — but the constructor
`NewCache(db, tableName)` performs zero validation. Today every caller passes
a literal string, so the risk is latent, not active; but if any future config
or user input reaches `NewCache` the entire cache package becomes an
injection vector. `Cache` is exported from `pkg/database`, so it is a public
boundary.
**Suggested fix**: Validate `tableName` in `NewCache` against
`^[A-Za-z_][A-Za-z0-9_]{0,63}$`, return an error on mismatch. Add a test.
Document the constraint on the exported type.

---

## Medium

### [MED] OpenGraph fetcher uses `context.Background()` instead of propagating
**File**: `pkg/opengraph/fetcher.go:103`
**Category**: Correctness
**Finding**: `FetchData` creates its own 15s-timeout context from
`context.Background()`. Callers (e.g. feed-generation path) cannot cancel the
fetch — e.g. on SIGINT, or when the downstream consumer has already given up.
In `FetchConcurrent` at line 450, a slow site will block one of the 5
semaphore slots for the full 15s even after cancellation.
**Suggested fix**: Add a `ctx context.Context` parameter to `FetchData` and
`FetchConcurrent`; derive the timeout from it with `context.WithTimeout`.
Propagate from the feed-generation call sites in `pkg/providerfeed` down.

### [MED] Enhanced HTTP client has no context-accepting variant
**File**: `pkg/api/enhanced_client.go:91-109` (Get), `:154-172`
(GetAndDecode)
**Category**: Correctness
**Finding**: `Get` builds requests with `http.NewRequest` (no `ctx`), meaning
cancellation is per-client-timeout only. The retry loop at `retry.go:129`
uses `time.Sleep`, which is similarly uninterruptible. For CLI use this is
tolerable; for a long-running `generateAll` producing feeds in parallel
(`cmd/feed-forge/main.go:325`) it means Ctrl-C waits up to (MaxAttempts ×
MaxBackoff) per in-flight request.
**Suggested fix**: Add `GetWithContext(ctx, url, headers)` and
`GetAndDecodeWithContext(ctx, url, v, headers)`. In `ExecuteWithRetry`, take a
`ctx` and replace `time.Sleep(d)` with a `select` on `ctx.Done()` and a
timer. Phase in by making existing methods call the new ones with
`context.Background()` during migration.

### [MED] `HTTPError.Err` never populated by producers
**File**: `pkg/api/enhanced_client.go:98-101`, `:161-164`
**Category**: Correctness
**Finding**: `HTTPError` declares an `Err error` field and implements
`Unwrap()` (`retry.go:117-120`), but every construction site passes only
`{StatusCode, Message}` — `Err` is always nil. `errors.Is` / `errors.As`
against wrapped sentinel errors will not behave as expected. Consumers using
`errors.As(err, &httpErr)` do get the struct, but chained wrapping is
silently broken.
**Suggested fix**: Either drop the `Err` field and `Unwrap` (if nothing wraps
it), or populate it at every construction site with the underlying cause
(`fmt.Errorf("..: %w", sourceErr)` equivalent). Pick one and enforce.

### [MED] `defaultHeaders` map mutated without mutex on a shared client
**File**: `pkg/api/enhanced_client.go` (map field + `SetDefaultHeader`-style
callers)
**Category**: Correctness
**Finding**: Clients are typically constructed once (e.g. shared HN client)
and used from multiple goroutines (`internal/hackernews/api.go:~110`).
Mutating `defaultHeaders` after construction races with the read in
`Get`/`GetAndDecode`. Today no caller mutates post-construction, but the API
allows it and there's no guardrail.
**Suggested fix**: Document immutability on the constructor and make the
field unexported-ReadOnly (or clone on read). Alternatively, wrap reads in
a `sync.RWMutex` if mutability is needed.

### [MED] `isBlockedURL` uses substring match (false positives, false
negatives)
**File**: `pkg/opengraph/fetcher.go:424-447`
**Category**: Correctness
**Finding**: `strings.Contains(targetURL, "reddit.com")` matches
`https://evil.com/?u=reddit.com`, and `"twitter.com"` matches
`https://twittercomplaints.example/`. Conversely, subdomain variants the
author did not list (`old.reddit.com` is caught incidentally, but
`reddit.co.uk` or query-string collisions are not matched predictably).
**Suggested fix**: Parse the URL with `net/url`, extract `u.Hostname()`, then
compare against an exact-match or suffix-match set (`strings.HasSuffix(host,
".reddit.com") || host == "reddit.com"`). This also pairs with the SSRF fix.

### [MED] `urlMutexes sync.Map` grows unbounded
**File**: `pkg/opengraph/fetcher.go:35`
**Category**: Performance
**Finding**: The per-URL mutex map is populated by `LoadOrStore` but never
cleaned up. Over a long-running server (or many `generateAll` invocations
without process restart) memory grows with unique URLs seen. Today
feed-forge is CLI-shaped so each run is bounded, but if anyone turns it into
a daemon this is a slow leak.
**Suggested fix**: Either (a) make the mutex acquisition stateless by using
`singleflight.Group` keyed on URL — it garbage-collects automatically; or
(b) document the CLI-only constraint and move the comment onto the field.

### [MED] Dead `cache` field on Fetcher
**File**: `pkg/opengraph/fetcher.go:31`
**Category**: Architecture
**Finding**: `cache map[string]*Data` is initialized in `newFetcher` but
never read or written thereafter — the actual cache is the `*Database`.
**Suggested fix**: Delete the field and its initialization.

### [MED] Token-bucket rate limiter loses sub-interval remainder
**File**: `pkg/api/ratelimit.go:100-112`
**Category**: Correctness
**Finding**: `refillTokens` computes tokens-to-add from `elapsed` and then
sets `lastRefill = now`. If the elapsed time corresponds to 2.6 tokens, two
tokens are added and the 0.6-token remainder is discarded. Over time this
under-delivers by up to ~50% of the configured rate.
**Suggested fix**: Advance `lastRefill` by `addedTokens × interval` instead
of `now`, so the fractional remainder is carried forward. Alternatively,
store tokens as a float and add `elapsed / interval` directly.

### [MED] Single-process mutex serializes every transaction (latent)
**File**: `pkg/database/types.go:168-196`
**Category**: Performance
**Finding**: `Database.Transaction` holds `db.mu` for the entire lifetime of
the transaction. Combined with WAL mode, this defeats the point of WAL:
every goroutine calling `Transaction` would block every other one. In
current code `Transaction` has no production callers — only
`pkg/database/database_test.go:100, 124, 160`. So the design issue is real
but there is no present production impact. Flag for when
`Transaction` starts being used outside tests, or delete the method if it
stays unused.
**Suggested fix**: Drop `db.mu.Lock()` in `Transaction` (and
`ExecuteSchema`). `database/sql` already serializes access to a single
connection; the SQLite driver already serializes writes at the engine level.
If a coarse mutex is actually desired for migrations, use a dedicated
`schemaMu` for `ExecuteSchema` only. Verify WAL is enabled.

### [MED] `SimpleRateLimiter.Wait` holds its mutex during sleep
**File**: `pkg/api/ratelimit.go` (Wait method)
**Category**: Performance
**Finding**: Sleeping inside the critical section serializes all waiters
sequentially — the Nth concurrent call waits for the sum of the previous
N-1 delays before its own delay begins, instead of all contending in
parallel.
**Suggested fix**: Compute the next-slot timestamp under lock, release the
lock, then `time.Sleep` outside. Or swap to `time/rate.Limiter`.

### [MED] Cache `GetAll` does not check `rows.Err()`
**File**: `pkg/database/cache.go:181-191`
**Category**: Correctness
**Finding**: After `for rows.Next()` the caller never consults
`rows.Err()`, so an iteration terminated by driver-level error is
indistinguishable from end-of-rows. Same pattern appears in
`internal/hackernews/database.go` (`getAllItems`) — grep and fix.
**Suggested fix**: After the loop: `if err := rows.Err(); err != nil {
return nil, fmt.Errorf("iterate rows: %w", err) }`. Do the same in every
other `rows.Next()` loop in `pkg/database/`, `internal/hackernews/`,
`internal/oglaf/`.

### [MED] UTF-8-unsafe truncation in template helper
**File**: `pkg/feed/template_funcs.go:52-58`
**Category**: Correctness
**Finding**: `truncateText` byte-slices `s[:maxLen-3]`. A cut inside a
multibyte codepoint (very common for Finnish providers — umlauts are 2
bytes) produces invalid UTF-8 in the feed, which some XML parsers will
reject. Same bug in `pkg/preview/format.go` (`truncateText`,
`FormatCompactListItem`).
**Suggested fix**: Convert via `[]rune(s)` and slice on rune count, or scan
with `utf8.DecodeRuneInString` until the byte budget is exhausted without
splitting a codepoint. Add a test case with multibyte input.

### [MED] `cmd/feed-forge/main.go` provider dispatch duplicates six blocks
**File**: `cmd/feed-forge/main.go:474-578`
**Category**: Architecture
**Finding**: The central command switch has a near-identical 10-20-line
block per provider (reddit, hacker-news, oglaf, fingerpori, feissarimokat,
iltalehti). Adding a new provider requires editing this file in addition to
an `init()` registration. The registry in `pkg/providers/registry.go`
already centralizes factory lookup — the main file should not repeat it.
**Suggested fix**: Replace the switch with a single lookup:
`info, ok := providers.DefaultRegistry.Get(ctx.Command()); ...`. The
config-load path (`loadProviderConfigFromYAML`) can key off `info.Name` and
`info.ConfigFactory`. The `generateAll` goroutine at line 325 already does
this pattern — unify on it.

### [MED] `generate`/`preview` use a parallel config path from `run`
**File**: `cmd/feed-forge/main.go` (`buildProviderConfig` vs
`loadProviderConfigFromYAML`)
**Category**: Architecture
**Finding**: Kong-parsed CLI flags populate a provider struct for the
typed subcommand (e.g. `feed-forge reddit --min-score`), but the generic
`generate` and `preview` commands load YAML directly because Kong only fills
the active sub-struct. This gives two code paths that must be kept in sync
— a CLI-only flag will silently not work under `generate`.
**Suggested fix**: Make both commands go through the registry: Kong binds
provider-name to a string; both `run-specific` and `generate` construct the
config via `info.ConfigFactory()` and apply YAML → env → CLI overrides in
one place. Accept the short-term duplication if a cleaner fix is too
invasive; at minimum add a `go test` that asserts the two paths produce
identical configs from identical inputs.

### [MED] Panic on unknown command instead of graceful error
**File**: `cmd/feed-forge/main.go:577`
**Category**: Correctness
**Finding**: The default branch of the command dispatch `panic(ctx.Command())`
for an unrecognized command. Kong should prevent this in practice, but the
panic prints a stack trace to the user on what is a routine input error.
**Suggested fix**: Replace with `fmt.Errorf("unknown command %q",
ctx.Command())` (or `ctx.Error(...)`) and let main.go's normal error path
handle exit.

### [MED] `LoadFromURLWithFallback` returns nil on both failures
**File**: `pkg/config/loader.go` (LoadFromURLWithFallback)
**Category**: Correctness
**Finding**: When URL fetch fails *and* the local file is missing, with
`FallbackToDefault: true` the function returns `(nil, nil)` — indicating
success with no config. Callers then dereference nil or proceed with zero
values. The intent is clearly "fall back to defaults," but returning nil
forces every caller to re-materialize the default, and most will forget.
**Suggested fix**: Return `*Config{/* zero-value default */}` instead of nil
when falling back, and rename to make the nil-safety contract obvious in
godoc. Or split into `LoadFromURL` + explicit `LoadOrDefault`.

### [MED] YAML parsing is not strict (unknown fields silently ignored)
**File**: `pkg/config/loader.go` (everywhere `yaml.Unmarshal` is called)
**Category**: Correctness
**Finding**: A typo in a YAML key (`min-scores` instead of `min-score`) is
silently dropped; users wonder why their filter is not applied.
**Suggested fix**: Use `yaml.NewDecoder(r); dec.KnownFields(true); dec.Decode
(&cfg)`. Surface unknown-field errors with a short hint.

### [MED] Global `filesystem.cacheDir` package variable, no synchronization
**File**: `pkg/filesystem/utils.go:19`
**Category**: Architecture
**Finding**: `cacheDir` is a package-level `string` set via `SetCacheDir` and
read via `getCacheDir`. It is set once at startup from `main`, which is
functionally fine today, but there is no mutex and the variable is mutable
— a future test that calls `SetCacheDir` concurrently with production
reads hits a data race.
**Suggested fix**: Either thread a `*CacheDir` value explicitly through the
call sites (no globals), or guard with a `sync.Once`-style single-set
pattern that panics on double-set. For tests, pass an explicit path.

### [MED] OpenGraph DB re-implements SQLite boilerplate instead of using
`pkg/database`
**File**: `pkg/opengraph/database.go`
**Category**: Architecture
**Finding**: The OpenGraph database opens its own `*sql.DB`, writes its
own schema execution, and manages its own close — duplicating much of what
`pkg/database/types.go` already provides. Two diverging code paths for
"open a SQLite file with pragmas" invite drift (e.g., one enables WAL, the
other forgets).
**Suggested fix**: Refactor `opengraph.Database` to embed or wrap
`*database.Database`. Keep domain-specific methods (`SaveCachedData`,
`GetCachedData`, `HasRecentFailure`), delegate plumbing.

### [MED] Atom feed-level metadata is not XML-escaped in templates
**File**: `templates/hackernews-atom.tmpl:3-9`,
`templates/reddit-atom.tmpl:13`
**Category**: Correctness
**Finding**: Feed `<title>`, `<subtitle>`, `<id>`, `<author>` fields come
from `feedmeta.Config` which for reddit includes dynamic `Subreddit` at
line 13. These are emitted with `{{ .Subreddit }}` instead of `{{ .Subreddit
| xmlEscape }}`. Today `feedmeta.Config` is populated from code constants
(per-provider `previewInfo`) and `Subreddit` comes from config/CLI, not from
upstream API payloads — so user-visible impact is currently nil. The
finding stands as defense-in-depth / template discipline: an item-level
`xmlEscape` on every field is inconsistent with feed-level bare
interpolation, and becomes correctness-real if user-provided metadata ever
lands in these fields.
**Suggested fix**: Pipe every dynamic value through `xmlEscape` in every
template. Grep for `{{ \.` without `| xmlEscape` for a full audit. Also:
once `xmlEscape` is fixed per the HIGH finding above, this becomes
correctness-critical.

### [MED] `BaseProvider.generateFeed` as a function field (service locator
anti-pattern)
**File**: `pkg/providers/base.go:19`, `:71-81`
**Category**: Architecture
**Finding**: The base provider holds a `func(outfile string) error` that
concrete providers must set via `SetGenerateFeedFunc`. This turns a
compile-time interface contract into a run-time one: forgetting to call
`SetGenerateFeedFunc` is a runtime "generate feed is not configured"
error. It exists only because `providerfeed.BuildGenerator` needs access
to the provider's `FetchItems` and `OgDB` before the struct is fully
assembled.
**Suggested fix**: Make `GenerateFeed` an interface method that concrete
providers implement explicitly as a one-liner calling
`providerfeed.BuildGenerator(p.FetchItems, ...)(outfile)`. Or invert:
`providerfeed.Generate(p, outfile)` takes the provider as a parameter.

### [MED] `GetGenerateConfig` uses reflection to reach embedded field
**File**: `pkg/providers/provider.go` (GetGenerateConfig)
**Category**: Architecture
**Finding**: Every provider Config embeds `GenerateConfig`, but the registry
pulls it out via `reflect.FieldByName("GenerateConfig")`. This silently
fails at runtime if a future provider forgets the embed or renames it.
**Suggested fix**: Define an interface `GenerateConfigProvider { Generate()
*GenerateConfig }` and have each Config declare a three-line method. Keeps
the check at compile time; fails to build, not to run.

### [MED] Shared rate-limited client across 10 concurrent HN workers
**File**: `internal/hackernews/api.go:~110` (fanout), enhanced client
construction
**Category**: Performance
**Finding**: `updateItemStats` spawns 10 `wg.Go` workers that all share a
single `RateLimiter` inside the enhanced client. With the
`SimpleRateLimiter.Wait` issue above, N-1 workers block behind the first.
With the token-bucket remainder-loss bug, the effective rate is lower than
configured. The combination can stretch what should be a ~1-second enrich
pass into >5s on a cold cache.
**Suggested fix**: Validate with a benchmark after the rate-limiter fixes.
Consider using `golang.org/x/time/rate` directly (already in the stdlib's
de-facto dependency cone via `golang.org/x/net`).

### [MED] Missing golangci-lint rules for the invariants that matter here
**File**: `.golangci.yml`
**Category**: Testing
**Finding**: Enabled: govet(shadow), errcheck, staticcheck, unused,
ineffassign, misspell, gocritic, revive. Missing checks that would have
caught findings in this review: `errorlint` (catches
`fmt.Errorf(%s, err)` vs `%w`), `bodyclose` (response body leak detection
— `pkg/api` *is* fine, but future regressions), `contextcheck` (missing
context propagation — catches the `context.Background()` finding),
`noctx` (detects `http.Get` without context — catches the enhanced-client
finding), `sqlclosecheck` (rows/stmt leaks — catches `GetAll` + similar),
`gosec` (catches the SSRF and SQL concat patterns, with tuning).
**Suggested fix**: Enable those linters. Expect a batch of new findings;
most should be addressed as part of the fixes above.

---

## Low

### [LOW] Range-loop copies each element into a fresh variable
**File**: `internal/reddit-json/provider.go:136-138`
**Category**: Correctness
**Finding**: `for i, post := range filteredPosts { feedItems[i] = &post }`
copies each element into `post` and points to the copy. Correct under
Go 1.22+ per-iteration semantics — this is an efficiency nit, not a
correctness bug — but the extra copy is avoidable.
**Suggested fix**: `feedItems[i] = &filteredPosts[i]` — references the slice
element directly. Match `convertToFeedItems` in `internal/hackernews`.

### [LOW] `GenerateAtomFeed` vs `GenerateAtomFeedWithEmbeddedTemplate`
duplicate each other
**File**: `pkg/feed/template.go:21-63` and `:79-121`
**Category**: Architecture
**Finding**: The two functions differ only in which `fs.FS` they pass into
`LoadTemplateWithFallback`. The public API shape encourages callers to
memorize which to call.
**Suggested fix**: Collapse to one function taking an `override fs.FS`
(nil = embedded only). Existing call sites update to pass nil.

### [LOW] Taskfile run-/preview- tasks duplicated per provider
**File**: `Taskfile.yml`
**Category**: Architecture
**Finding**: Six near-identical `run-<provider>` and six `preview-<provider>`
tasks. Task supports matrix / for loops.
**Suggested fix**: One `run:` task with a provider variable, one `preview:`
similarly. Document via `task run -- reddit` style invocation.

### [LOW] Regex-based RSS parsing in Oglaf provider
**File**: `internal/oglaf/provider.go:20-29`
**Category**: Architecture
**Finding**: The file has 7 compiled regexes for what `encoding/xml` already
does correctly — and does handle XML namespaces, CDATA, and nested
entities without surprise. Feissarimokat already uses `encoding/xml`.
**Suggested fix**: Port to `encoding/xml.Decoder` with a `struct { Item []…
\`xml:"item"\` }`. Removes ~40 lines of parsing and four silent failure
modes (see the guards at `:362-369`).

### [LOW] `defer rows.Close()` error swallowed silently
**File**: multiple (pkg/database/cache.go:175-179, hackernews, oglaf)
**Category**: Correctness
**Finding**: A `defer` that logs Close errors to `slog.Error` is fine in
most places, but in iteration loops the Close error can mask a real query
error. Minor.
**Suggested fix**: After a successful iteration, call `rows.Close()`
explicitly and check its return value; leave the defer as the fallback.
Only worth doing if errorlint is enabled.

### [LOW] `config_example.yaml` hard-codes "Feed Forge" as author
**File**: `config_example.yaml`
**Category**: Correctness
**Finding**: Minor — example, not code.
**Suggested fix**: No action needed.

### [LOW] `_ = resp.Body.Close()` suppresses close errors silently
**File**: `internal/oglaf/provider.go:191-193`, `:308-310`
**Category**: Correctness
**Finding**: Consistent pattern, but a Close error on HTTP response is
worth at least a `slog.Debug`.
**Suggested fix**: Swap for `if err := resp.Body.Close(); err != nil {
slog.Debug("close body", "err", err) }`. Optional.

---

## Summary

**By severity**:

| Severity | Count |
| --- | --- |
| Critical | 1 |
| High | 5 |
| Medium | 24 |
| Low | 7 |
| **Total** | **37** |

**By category**:

| Category | Count |
| --- | --- |
| Security | 4 |
| Correctness | 18 |
| Architecture | 10 |
| Performance | 4 |
| Testing | 1 |
| **Total** | **37** |

---

## Suggested fix order (top 5)

Priorities — address Critical + highest-impact Highs first; each item below
is load-bearing for the ones that follow it.

1. **SSRF in OpenGraph fetcher** (Critical) — `pkg/opengraph/fetcher.go:68`,
   `pkg/urlutils/url.go`. Tighten URL validation to reject private
   IPs/non-http(s) schemes; switch `isBlockedURL` to parsed-host matching
   at the same time (Medium finding). One patch closes both the critical
   boundary breach and a related Medium. (Body size already capped.)
2. **xmlEscape is wrong** (High) — `pkg/feed/template_funcs.go:26-31`. Fix
   before shipping any template changes; this is silently mutating feed
   content today. Also audit templates for feed-level metadata that does
   not pipe through the helper (Medium finding bundles here).
3. **HTML injection in Oglaf & Feissarimokat providers** (High) —
   `internal/oglaf/provider.go:153-161`,
   `internal/feissarimokat/api.go:99-104`. Swap `fmt.Sprintf` for
   `html/template` or `html.EscapeString`. Low-risk, immediate.
4. **Retry double-sleep + context plumbing in `pkg/api`** (High + related
   Mediums) — `pkg/api/retry.go:165-173` is the correctness bug;
   consolidate on one sleep per attempt and add jitter. Thread `ctx` into
   `ExecuteWithRetry`, `Get`/`GetAndDecode`, and down to
   `opengraph.Fetcher.FetchData` (currently uses `context.Background()`
   at `fetcher.go:103`). One refactor closes the High and the two
   context-missing Mediums.
5. **Add missing golangci-lint rules** (Medium) — `.golangci.yml`. Enable
   `errorlint`, `bodyclose`, `contextcheck`, `noctx`, `sqlclosecheck`,
   `gosec`. Cheap to land and catches future regressions of the four
   classes above. Run *after* the fixes so new findings don't mix with
   existing code changes.

Beyond these five, the Medium-category `cmd/feed-forge/main.go` dispatch
deduplication is the highest-ROI structural cleanup but not urgent. The
downgraded "transaction mutex" finding can be deferred until
`Database.Transaction` gets a production caller (today only tests use it).
