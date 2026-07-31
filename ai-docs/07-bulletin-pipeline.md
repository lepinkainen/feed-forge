# Bulletin pipeline

`internal/bulletin/`. A separate code path. It does not implement `FeedProvider`. The
`generate` command does not discover it. It aggregates many high-frequency outlets
into periodic summarized digests. It replaces one feed per source.

## State

State lives in `bulletin.db`. An item with `bulletin_id IS NULL` is unpublished. No
generated bulletin includes it yet.

## Three decoupled stages

- `bulletin-fetch` accumulates items.
- `bulletin-generate` turns the unpublished backlog into one stored bulletin.
- `bulletin-publish` renders the stored bulletins into HTML and Atom.

`bulletin-generate` is idempotent and catches up. It consumes everything that is
unpublished. `bulletin-publish` is a pure render and writes nothing. To rebuild every
HTML page after a template change, run `bulletin-publish` again.

`bulletin-generate` and `bulletin-summarize` call the model. Both need
`ANTHROPIC_API_KEY`.

## Fetch (`fetch.go`)

Parses each source feed with `gofeed`. Fetches each new article page through
`httpcache.CachedGetWithStale`, which reuses conditional GET and ETag. Extracts the
full text with `go-shiori/go-readability`. Falls back to the content in the feed when
the extracted text is thin. `HasItem` skips URLs that are already stored, so article
pages are not fetched twice.

## Dedup (`simhash.go`, `dedup.go`)

Computes a 64-bit SimHash over the full text with stopwords stripped. A greedy
single-pass clustering groups stories within `simhash-threshold` Hamming distance. The
default is 3. Fingerprints are stored as SQLite INTEGER, as the int64 bit pattern.

## Generate (`publish.go`, `bulletin.Generate`)

The only stage that calls the model. The only stage that writes bulletins. Dedup runs
before summarization to save tokens. The cluster representatives and their source URLs
go to Anthropic in a single call (`claude-haiku-4-5`, `summarize.go`). The call
returns one topic-grouped HTML digest. `store.CreateBulletin` inserts the bulletin row
and marks its items published in one transaction. To override the prompt, set
`prompt-file`.

## Publish (`publish.go`, `bulletin.Publish`)

Reads all stored bulletins. Renders one Atom `<entry>` per digest through
`templates/bulletin-atom.tmpl`, limited to the newest `feedEntryLimit`. When
`output-dir` is set, exports HTML pages to `<output-dir>/html/` through
`templates/bulletin-page.html.tmpl`. Writes a dated archive page per bulletin plus a
stable `bulletin-latest.html`. The `index.html` from the `generate` command links to
`html/bulletin-latest.html` when that file exists.

## Configuration

Comes from the `bulletin:` section of `config.yaml` through
`loadProviderConfigFromYAML`. The Anthropic API key comes from the top-level
`anthropic:` section (`pkg/llm.Config`, key `api-key`). `llm.Config.ResolveAPIKey`
resolves it and falls back to the `ANTHROPIC_API_KEY` environment variable. The
`anthropic:` section is shared. Any future processor that calls a model can use it.
