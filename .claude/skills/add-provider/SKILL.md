---
name: add-provider
description: Add a new feed source to feed-forge, including its provider package, registry entry, CLI and YAML configuration, Atom output, and tests.
---

# Add a feed-forge provider

Use the [provider guide](../../../docs/adding-a-provider.md) for current code examples
and the detailed contracts. Read its relevant sections before implementation;
make sure API names and behavior match the repository source. This skill is the
integration checklist, not a second copy of the guide's implementation examples.

## Choose the Source

Inspect an upstream response and its polling policy. Decide the format, whether a
content database is needed, whether there are multiple feeds, and whether article
and discussion URLs differ. Keep a canonical URL in code unless source selection
is part of the requested feature.

For RSS/Atom, consider the existing `gofeed` dependency before adding XML mappings.
Fetch through `pkg/api` and parse the body; do not use a parser's independent URL
fetcher. For source-specific Atom mappings, reuse `pkg/atom.Decode`, `Feed[E]`,
`atom.Time`, `atom.AlternateHref`, and its common XML types. This preserves
text/HTML types that `gofeed` discards and handles declared charsets. For RSS
mappings, use `pkg/xmlutil.Decode`, never a bare `xml.Unmarshal`. Keep provider-specific normalization covered by fixtures.

Select examples by behavior, not as universal recipes:

- `internal/slashdot/`: canonical Atom source, cached bodies, per-item normalization.
- `internal/youtube/`: multiple feeds and bounded stale-body fallback.
- `internal/feissarimokat/`: RSS comic content.
- `internal/oglaf/`: persistent source items and incremental fetching.
- `internal/tildes/`: distinct article/discussion links and source-specific text.

Reddit's authentication and proxy machinery do not generalize to ordinary sources.

## Implement the Provider

- Embed `providers.BaseProvider`. A stateless provider has no content database, but
  the base still opens link-preview and HTTP cache databases.
- Embed `providers.GenerateConfig` with `yaml:",inline"` in `Config`. Give every
  additional field a kebab-case YAML tag. Keep configuration defaults consistent
  between `ConfigFactory` and CLI flags.
- Return `(providers.FeedProvider, error)` from construction; wrap errors with `%w`.
  Register with `providers.MustRegister`, including `Factory`, `ConfigFactory`, and
  `Preview` metadata.
- Install `providerfeed.BuildGenerator` through `SetGenerateFeedFunc`. The current
  interface is `GenerateFeed(outfile string) error`. Pass nil for link-preview
  storage when the template does not use external previews.
- Normalize and filter items, sort newest first, then apply a positive fetch limit.
  `CreatedAt()` returns `time.Time`; normalize dates at the source boundary.

Read [Fetch and Normalize Items](../../../docs/adding-a-provider.md#fetch-and-normalize-items)
for these source-boundary requirements:

- Malformed per-item scalar metadata must not discard valid siblings. Decode
  fallible fields separately, skip unusable required timestamps with a log message,
  and define fallbacks for optional counts.
- Trim titles. Do not unconditionally HTML-unescape XML-decoded text. Any extra
  decoding needs source evidence and a fixture; preserve literal entity notation.
- Preview needs items even on 304. Use body caching for a shared preview/generation
  cache, with recovery for missing output files and validator-only cache rows.
  Serving stale bodies on upstream failure is a separate, explicit policy.
- Reuse the enhanced client's central `feed-forge/<version>` default. Override
  only for source-specific identity requirements and cover actual request headers.
- Use the inherited `p.HTTPCacheStore()` accessor, which handles a nil embedded
  base or cache. Supply a temporary store for conditional-request tests.

## Render and Integrate

Read [Render Atom Safely](../../../docs/adding-a-provider.md#render-atom-safely).
Use `xmlEscape` for XML text and attributes. Ready-to-render HTML can be serialized
as `<content type="html">{{.Content | xmlEscape}}</content>`. Plain text needs HTML
escaping before it becomes an HTML body. Inside CDATA, use `{{.Content | cdata}}`
and `{{$og.Excerpt | cdata}}` to protect `]]>` and invalid XML characters. The
shared helper returns a fragment; the template supplies the outer CDATA wrapper.

Add `templates/<name>-atom.tmpl`; the embed pattern includes it automatically.
Make sure referenced fields exist in `TemplateItem` and the renderer populates them.
The link-preview map is `LinkPreviews`, keyed by `.Link`. `AuthorURI()` is recognized;
arbitrary provider methods are not automatically available to templates.

Make all five integration additions:

1. Import the package in `cmd/feed-forge/main.go` to trigger registration.
2. Add the CLI command with matching flags and an explicit `name` when Go field
   name derivation differs from the registry name. `cmd:"name"` alone does not
   override Kong's derived name.
3. Add its configuration mapping to `buildProviderConfig`.
4. Add its entry to `providerCmds()`. Ordinary providers do not need a custom
   dispatch branch. Keep preview help current if it enumerates provider names.
5. Add matching YAML blocks to `config_example.yaml` and `writeTestConfig` in
   `cmd/feed-forge/main_test.go`, including `outfile` and `interval`.

`generate` and `preview` load provider YAML separately from the active Kong command.
Missing YAML tags can therefore break those commands while direct invocation works.
Use the same name in the CLI, registry, YAML, and preview argument.

## Demonstrate the Behavior

Read [Test and Build](../../../docs/adding-a-provider.md#test-and-build). Use local
HTTP fixtures and temporary cache/output paths. Include source mapping and output
round trips, and regression cases for relevant encoding and malformed-entry behavior.
For conditional feeds, cover preview/generation in both orders, missing output, and
legacy validator-only rows. Run failing regression tests before implementation when
fixing a bug; do not assert `ErrNotModified` as successful item-fetch behavior for
preview.

Run `goimports -w .` after Go edits and `task build` before completion. Tests and lint
must pass. Run `task update-golden` only for intentional output changes and make sure
the diff matches them; `task test-update` downloads fixtures and runs no tests.
For web/template behavior, demonstrate functionality with Playwright. Respect source
polling limits during manual trials; use local fixtures for repeated requests.
