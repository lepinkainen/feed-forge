# LLM_ONLY_FEEDS_TEMPLATES_PREVIEW

## Feed generation API

File: `pkg/feed/generator.go`

Main functions:

- `GenerateAtomFeed(items, templateName, templatePath, config, ogDB)`
- `GenerateAtomFeedWithContext(ctx, ...)`
- `SaveAtomFeedToFile(items, templateName, templatePath, outputPath, config, ogDB)`
- `GenerateAtomFeedWithEmbeddedTemplate(items, templateName, config, ogDB)`
- `SaveAtomFeedToFileWithEmbeddedTemplate(items, templateName, outputPath, config, ogDB)`

Internal `generateAtomFeed`:

1. new `TemplateGenerator`
2. load template via caller-supplied load func
3. collect external item URLs with `externalItemURLs`
4. if `ogDB != nil`, create OG fetcher and `FetchConcurrentWithContext(ctx, urls)`
5. convert `[]providers.FeedItem` to `*TemplateData`
6. execute template into string

`externalItemURLs` rules:

- skip empty `Link()`
- skip when `Link() == CommentsLink()`
- dedupe links

## Feed metadata

Type: `pkg/feedmeta.Config` (aliased as `feed.Config`)

Fields:

- `Title`
- `Link`
- `Description`
- `Author`
- `ID`
- `ProxyURL` optional OG fetch proxy URL
- `ProxySecret` optional proxy auth secret

Provider `previewInfo.Config` is default metadata. Provider may supply dynamic `feedConfig()` to `providerfeed.BuildGenerator`.

## Template model

File: `pkg/feed/template.go`

`TemplateData` passed to templates:

```go
type TemplateData struct {
    FeedTitle string
    FeedLink string
    FeedDescription string
    FeedAuthor string
    FeedID string
    Updated string
    Generator string
    Items []TemplateItem
    OpenGraphData map[string]*opengraph.Data
}
```

`TemplateItem` fields:

```go
type TemplateItem struct {
    Title string
    Link string
    CommentsLink string
    ID string
    Updated string
    Published string
    Author string
    AuthorURI string
    Categories []string
    Score int
    Comments int
    Content string
    Summary string
    ImageURL string
    Subreddit string
    Domain string
}
```

Mapping in `createGenericFeedData`:

- `Title` <= `item.Title()`
- `Link` <= `item.Link()`
- `CommentsLink` <= `item.CommentsLink()`
- `ID` <= `item.CommentsLink()`
- `Updated`, `Published` <= `item.CreatedAt().Format(time.RFC3339)`
- `Author` <= `item.Author()`
- `Categories` <= `item.Categories()`
- `Score` <= `item.Score()`
- `Comments` <= `item.CommentCount()`
- `Content` <= `item.Content()`
- `Summary` <= `fmt.Sprintf("Score: %d | Comments: %d", score, comments)`
- `ImageURL` <= `item.ImageURL()`
- optional `AuthorURI`, `Subreddit`, `Domain` from optional item interfaces

Potential issue to consider when editing:

- `ID` is always comments link. For providers where comments link empty/non-unique, templates may need a fallback or item methods must guarantee stable non-empty comments link.

## Template funcs

File: `pkg/feed/template_funcs.go`

Func map:

- `xmlEscape`: escapes XML chars and drops invalid XML 1.0 runes.
- `formatTime`: `time.Time` -> RFC3339.
- `formatDate`: parse RFC3339 string -> `2 January 2006`; returns input if parse fails.
- `formatScore`: `Score: %d | Comments: %d`.
- `joinStrings`: `strings.Join`.
- `contains`: `strings.Contains`.
- `hasPrefix`: `strings.HasPrefix`.
- `truncate`: length truncation with `...`.

Older `feed.EscapeXML` in `pkg/feed/types.go` unescapes then escapes; current templates likely use `xmlEscape`.

## Template loading

Files: `pkg/feed/template.go`, `pkg/feed/templatefs.go`, `templates/embedded.go`

Default filesystems:

- override FS: `os.DirFS("templates")`
- fallback FS: `templates.EmbeddedTemplates`

`LoadTemplateWithFallback(name)`:

1. filename = `<name>.tmpl`
2. try override FS
3. if not found, try embedded FS
4. error `ErrTemplateNotFound` if neither

`ReadTemplateContent(filename)` uses same override/fallback pattern. Used for `feed-index.html.tmpl`. `feeds.opml` is generated directly in Go, not from a template.

Tests can call:

- `feed.SetTemplateOverrideFS(f)`
- `feed.SetTemplateFallbackFS(f)`

## Existing templates

- `templates/reddit-atom.tmpl`
- `templates/hackernews-atom.tmpl`
- `templates/fingerpori-atom.tmpl`
- `templates/feissarimokat-atom.tmpl`
- `templates/oglaf-atom.tmpl`
- `templates/tildes-atom.tmpl`
- `templates/youtube-atom.tmpl`
- `templates/feed-index.html.tmpl`

Embedded by:

```go
//go:embed *.tmpl
var EmbeddedTemplates embed.FS
```

## OpenGraph data in templates

`OpenGraphData` map key is item external `Link()`.
Only links from `externalItemURLs` are fetched.
If item link equals comments link, no OG data fetched.

`opengraph.Data` fields:

- URL, Title, Description, Image, SiteName
- ETag, LastModified
- FetchedAt, ExpiresAt

When changing templates, handle missing map entries and empty fields.

## Preview package

Files: `pkg/preview/format.go`, `pkg/preview/tui.go`

Non-TUI formatting:

- `FormatCompactListItem(index, item)` => one-line list with score/comments/time/title.
- `FormatDetailedItem(item)` => full metadata and content preview.
- `FormatXMLItem(item, templateName, config)` => generate one-item feed using real template, extract first `<entry>...</entry>` regex.

TUI model:

- sorts incoming items newest-first in `NewModel`
- list view keys: `up/down`, `j/k`, `enter` detail, `x` XML, `q` quit
- detail/XML view keys: `esc` back, `x` toggle, `q` quit

CLI preview:

- `feed-forge preview <provider> --limit N`
- `feed-forge preview <provider> --index I` prints XML entry to stdout and skips TUI

## Template edit checklist

1. Verify `TemplateItem` fields exist; do not invent fields without updating Go model.
2. Use `xmlEscape` for XML text/attributes unless content intentionally raw and safe.
3. Preserve stable Atom IDs.
4. For provider-specific fields (`Subreddit`, `Domain`, `AuthorURI`), ensure item implements optional method.
5. Run relevant feed/template tests and `go test ./...`.
