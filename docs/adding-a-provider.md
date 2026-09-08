# Adding a New Provider

Providers implement `providers.FeedProvider`, register themselves, and use the shared
feed generator. One name identifies the provider in the registry, CLI, YAML,
`generate`, and `preview`.

The contracts live in [pkg/providers/provider.go](../pkg/providers/provider.go).
The [add-provider skill](../.claude/skills/add-provider/SKILL.md) is the agent checklist
for this guide. Make sure examples still match the source before adapting them.

## Choose the Source and Provider Shape

Inspect a real upstream response before choosing the parser and configuration.
Prefer a supported API or syndication feed when it provides the required data.
Keep a canonical source URL in code; expose URL configuration only when users need
to select an instance, channel, or another source variant.

For RSS and Atom, consider the existing `gofeed` dependency before adding XML
mappings. It provides charset handling, parsed dates, and namespace extensions such
as Slashdot's comment count. Fetch through `pkg/api`, then pass the response body to
`gofeed.Parser.Parse`; `ParseURL` would bypass the project's HTTP client. For Atom
sources needing explicit XML mappings, reuse [pkg/atom](../pkg/atom/atom.go):
`Decode[T]`, `Feed[E]`, the lenient `Time` date construct, `AlternateHref`, and the
common link, person, text, and category types. `Text.HTML()` maps text, html, and
xhtml constructs to HTML. It handles declared charsets while preserving
source-specific fields and text/HTML types. For RSS mappings, decode with
[pkg/xmlutil](../pkg/xmlutil/xmlutil.go) `Decode[T]`, never a bare `xml.Unmarshal`. Slashdot needs the summary type that `gofeed` discards. Keep normalization
in the provider and preserve source semantics with fixtures before changing parsers.

Decide whether the provider needs a content database, multiple feeds, a separate
article/discussion URL, or source-specific filtering. Set a default generation
interval consistent with the upstream polling policy. An interval used by
`generate` is not a rate limiter for direct provider or preview commands.

## Add the Provider Package

Keep the provider in `internal/yourprovider/`. A typical package has:

| File | Purpose |
| --- | --- |
| `provider.go` | Configuration, construction, registration, and `FetchItems` |
| `api.go` | HTTP requests, parsing, and source-specific normalization |
| `types.go` | Upstream mappings and the `FeedItem` implementation |
| `provider_test.go` | Fixture and integration tests |
| `testdata/` | Upstream fixtures and golden output where useful |

Small implementations can combine files. Add an Atom template under `templates/`
and update the CLI, its shared test configuration, and `config_example.yaml`.

### Configuration, Constructor, and Registration

This is the constructor and registry portion of a provider; implement `FetchItems`
separately using the next section. Keep only configuration fields that the source
actually supports.

```go
package yourprovider

import (
    "fmt"

    "github.com/lepinkainen/feed-forge/pkg/feedmeta"
    "github.com/lepinkainen/feed-forge/pkg/providerfeed"
    "github.com/lepinkainen/feed-forge/pkg/providers"
)

var previewInfo = &providers.PreviewInfo{
    Config: feedmeta.Config{
        Title:       "Your Provider",
        Link:        "https://example.com/",
        Description: "Stories from Your Provider",
        Author:      "Your Provider",
        ID:          "https://example.com/feed",
    },
    ProviderName: "Your Provider",
    TemplateName: "yourprovider-atom",
}

type Config struct {
    providers.GenerateConfig `yaml:",inline"`
    MinScore int `yaml:"min-score"`
}

type Provider struct {
    *providers.BaseProvider
    MinScore int
}

func NewProvider(minScore int) (providers.FeedProvider, error) {
    base, err := providers.NewBaseProvider(providers.DatabaseConfig{
        UseContentDB: false,
    })
    if err != nil {
        return nil, fmt.Errorf("initialize yourprovider: %w", err)
    }
    p := &Provider{BaseProvider: base, MinScore: minScore}
    p.SetGenerateFeedFunc(providerfeed.BuildGenerator(
        p.FetchItems, previewInfo, nil, nil,
    ))
    return p, nil
}

func factory(config any) (providers.FeedProvider, error) {
    cfg, ok := config.(*Config)
    if !ok || cfg == nil {
        return nil, fmt.Errorf("expected *yourprovider.Config")
    }
    return NewProvider(cfg.MinScore)
}

func init() {
    providers.MustRegister("yourprovider", &providers.ProviderInfo{
        Name:        "yourprovider",
        Description: "Generate an Atom feed from Your Provider",
        Version:     "1.0.0",
        Factory:     factory,
        ConfigFactory: func() any {
            return &Config{
                GenerateConfig: providers.GenerateConfig{Interval: "30m"},
            }
        },
        Preview: previewInfo,
    })
}
```

`BaseProvider` supplies `GenerateFeed(outfile string) error` after
`SetGenerateFeedFunc`. Do not duplicate directory creation, serialization, or feed
logging. `BuildGenerator` takes an optional configuration function as its third
argument; use it when instance configuration changes feed metadata. Pass `p.OgDB`
as the fourth argument only when the template needs external link previews.

`UseContentDB: false` means no source-content database. The base still opens the
shared link-preview and HTTP cache databases. For persistent source items, set
`UseContentDB: true` and a `ContentDBName`, such as `yourprovider.db`. Call `Close`
on concrete providers when managing their lifetime directly; it is available on
`BaseProvider`, but is not part of the `FeedProvider` interface.

## Fetch and Normalize Items

`FetchItems(limit int) ([]providers.FeedItem, error)` should fetch, normalize,
filter, sort newest first, and then apply a positive limit. Keep `0` usable for the
shared generator's request for all available items.

### HTTP Identity and Caching

Use `api.NewGenericClient()` for ordinary sources and the source-specific enhanced
clients for Reddit and Hacker News. All outbound source requests must go through
`pkg/api`. The required identity is `feed-forge/<version>` using `pkg/version`.

The enhanced client supplies this identity centrally; reuse its default instead
of adding a per-provider override. Keep overrides only for source-specific
identity requirements, such as Reddit's username suffix, and make sure the actual
request header has the intended value in an HTTP fixture test.

For providers whose preview and generation share a feed cache, cache the response
body as well as validators:

```go
body, _, err := httpcache.CachedGetWithStale(ctx, client, store, feedURL, nil, 0)
if err != nil {
    return nil, fmt.Errorf("fetch yourprovider feed: %w", err)
}
// Parse body with the selected parser, then normalize each item.
```

This example uses cached bodies on 304 and returns on any upstream error. A
`maxStale` of `0` means no age limit, so on failure the call still returns a cached
body with `stale=true` next to the error; returning on the error is what rejects
it. Stale data on upstream failure is a separate policy: to opt in, set a bounded
maximum stale age, handle the returned `stale` flag and error explicitly, and log
the fallback. See
[YouTube's fetcher](../internal/youtube/api.go) for that policy.

`CachedGet` stores validators only. Its `ErrNotModified` result supplies no items:
preview would fail, and generation cannot create a missing output file. The shared
generator handles this sentinel only by touching an existing file's modification
time. Use validator-only caching only where the calling flow can satisfy those
conditions. `CachedGetWithStale` fetches unconditionally when an existing cache row
has validators but no body, so it can upgrade that row.

Use the inherited `p.HTTPCacheStore()` accessor. It returns nil when the embedded
`BaseProvider` or its cache is nil, allowing fixture tests without databases.
Supply a base with a temporary store when testing conditional requests. Do not
duplicate cache-accessor wrappers in individual providers.

### Parsing and Item Semantics

Make sure the selected parser handles the upstream encoding. `pkg/xmlutil.Decode`
(and `pkg/atom.Decode`) configures `charset.NewReaderLabel` for non-UTF-8 feeds. It
decodes XML entities once and leaves normalization to the provider.

Parse timestamps into `time.Time` at the source boundary. Use `atom.Time` for Atom
date fields: it never fails decoding and yields the zero time for a malformed
value. Other fallible per-item fields can first be decoded as strings, then
normalized independently: one malformed optional count must not abort XML decoding
for every story. Log and skip an item
whose required timestamp is unusable; use a documented fallback for optional
metadata. Add alternate date formats or grouped-number handling when the source
requires them. A structurally malformed feed or failed HTTP request still returns
an error. Store normalized times in SQLite `TIMESTAMP` columns, never sortable raw
date strings.

Trim surrounding title whitespace. XML decoding already resolves XML entities, so
do not unconditionally apply `html.UnescapeString` afterward. Atom titles without
a `type` attribute are plain text. Additional HTML decoding needs evidence from
the source's declared text type or a recorded encoding quirk, plus a regression
fixture. Preserve deliberately literal text such as `&lt;template&gt;`.

Implement every method in `providers.FeedItem`:

| Method | Meaning |
| --- | --- |
| `Title()` | Normalized title text |
| `Link()` | External article URL, or story URL when there is no distinct article |
| `CommentsLink()` | Discussion URL; equal to `Link()` when there is no separate discussion |
| `Author()` | Source author or editor |
| `Score()` / `CommentCount()` | Available counts, otherwise zero |
| `CreatedAt()` | Normalized `time.Time` |
| `Categories()` | Source topics or categories |
| `ImageURL()` | Thumbnail URL, otherwise empty |
| `Content()` | Body content with its HTML/plain-text contract matched to the template |

The shared renderer uses `CommentsLink()` as the entry ID. Optional `AuthorURI()`
is recognized by the renderer; other provider-specific methods are not
automatically exposed. Make sure any new template field exists in
`pkg/feed/template.go` and is populated in `pkg/feed/generator.go`.

## Render Atom Safely

Add `templates/yourprovider-atom.tmpl`. The embed pattern in
`templates/embedded.go` includes new `.tmpl` files automatically. Start with a
similar template for layout, then make sure its escaping suits the new source.
The link-preview map is `LinkPreviews`, keyed by the item's `Link()`.

Use `xmlEscape` for XML text and attribute values. For a ready-to-render HTML body,
this existing pattern safely serializes the HTML as Atom character data:

```gotemplate
<content type="html">{{.Content | xmlEscape}}</content>
```

An Atom reader recovers the original HTML after XML decoding. If the upstream body
is plain text, escape it for HTML first; HTML escaping and XML escaping protect
different layers. Avoid applying HTML escaping to a body that is already HTML.

The Atom templates use the shared `cdata` function from `feed.TemplateFuncs()`:

```gotemplate
<content type="html"><![CDATA[{{.Content | cdata}}]]></content>
```

It strips invalid XML code points and splits literal `]]>` sequences while
preserving the HTML. It returns a fragment for use inside an existing CDATA
section, not the outer wrapper. Apply it to every raw HTML insertion, including
`{{$og.Excerpt | cdata}}`. Plain text inside an HTML body still needs HTML escaping
first; YouTube uses `{{.Content | xmlEscape | cdata}}`. The template regression
tests round-trip content through XML decoding for every Atom template.

## Wire the CLI and Configuration

Make these additions in `cmd/feed-forge/main.go`:

1. Import `internal/yourprovider` to trigger self-registration.
2. Add a command field with flags matching the provider configuration:

   ```go
   YourProvider struct {
       Outfile  string `help:"Output file path" short:"o" default:"yourprovider.xml" yaml:"outfile"`
       MinScore int    `help:"Minimum score" default:"0" yaml:"min-score"`
       Interval string `help:"Minimum time between regenerations" default:"30m" yaml:"interval"`
   } `cmd:"" name:"yourprovider" help:"Generate an Atom feed from Your Provider."`
   ```

3. Add a case in `buildProviderConfig`:

   ```go
   case "yourprovider":
       return &yourprovider.Config{
           GenerateConfig: providers.GenerateConfig{
               Outfile:  CLI.YourProvider.Outfile,
               Interval: CLI.YourProvider.Interval,
           },
           MinScore: CLI.YourProvider.MinScore,
       }
   ```

4. Add an entry inside the map returned by `providerCmds()`:

   ```go
   "yourprovider": {"yourprovider", "Your Provider", CLI.YourProvider.Outfile, nil},
   ```

Simple providers use that map and the shared dispatcher. A custom dispatch branch
is needed only for additional preflight behavior. Add the name to preview help if
that help lists provider names.

Kong derives command names from Go field names: `YourProvider` becomes
`your-provider` unless `name:"yourprovider"` overrides it. A `cmd:"yourprovider"`
tag alone does not override that derivation. Make sure the registry name, command
name, configuration section, and preview argument match exactly.

Kong fills only the active command's configuration. `generate` and `preview` load
the provider section separately through `loadProviderConfigFromYAML`, starting from
`ConfigFactory` defaults. Every provider configuration needs YAML tags and the
inline `GenerateConfig` embed. Use kebab-case keys and matching CLI/default values.

Add the following to both `config_example.yaml` and the YAML fixture in
`writeTestConfig` in `cmd/feed-forge/main_test.go`:

```yaml
yourprovider:
  min-score: 0
  outfile: yourprovider.xml
  interval: 30m
```

`generate` discovers configured providers through the registry. With `output-dir`
set, it includes their output in the HTML index and OPML. No separate provider list
is needed for these outputs.

## Test and Build

Use fixture-driven HTTP tests with `httptest.NewServer`, and use `t.TempDir()` for
cache databases and output files. Avoid unit tests that fetch the live site or open
the user's default databases. A recorded fixture should cover the source's actual
shape; clearly synthetic cases can cover malformed input.

Make sure tests cover the behavior relevant to the provider:

- Item mapping, sorting, filtering, and limits applied after sorting.
- Charset handling, surrounding title whitespace, XML entities, and literal entity
  text preserved through parsing and generated output.
- Valid siblings retained when another item's timestamp or optional metadata is bad.
- Preview before generation and generation before preview with 200 then 304 replies;
  reopen the same cache between calls and generate into a missing output file.
- Recovery from a validator-only cache row when adopting body caching.
- Generated XML parsed successfully, with HTML content preserved through an XML
  round trip, including `]]>` and invalid XML code points where applicable.

The existing CLI tests cover command names, dispatch registration, and YAML loading:
`TestProviderCommandNamesMatchRegistry`, `TestProviderCommandsDispatch`, and
`TestAllRegisteredProviders_HaveYAMLTags`. They depend on the shared test YAML block.

For bug fixes, add regression tests and run `task test` to demonstrate the failure
before changing implementation. Run `goimports -w .` after Go edits. Run
`task build` before completion; it includes tests and lint, and lint must pass with
zero errors. Do not run `go build`, `go test`, or `go vet` directly.

Run `task update-golden` only when expected output changes intentionally, and make
sure the diff matches that change. `task test-update` downloads live fixtures and
runs no tests; use it only for an intentional upstream fixture refresh.

For manual output inspection, run `./build/feed-forge preview yourprovider --index 0`
and `./build/feed-forge yourprovider -o build/yourprovider.xml`. Respect upstream
polling limits; exercise repeated-request scenarios against local fixtures. For web
or template behavior, make sure functionality is demonstrated with Playwright
before claiming completion.

## Existing Examples

- [Slashdot](../internal/slashdot/) — canonical Atom source, body caching, per-item
  normalization, and escaped HTML serialization.
- [YouTube](../internal/youtube/) — multiple feeds, source extensions, and bounded
  stale-body fallback.
- [Feissarimokat](../internal/feissarimokat/) — RSS comics and HTML body mapping.
- [Oglaf](../internal/oglaf/) — persistent source content and incremental fetching.
- [Tildes](../internal/tildes/) — distinct article and discussion URLs with
  source-specific title/content normalization.

Read examples for the relevant behavior, not as universal defaults. Reddit's
authentication, proxy, and link-preview machinery make it a poor starting point for
an unrelated source.
