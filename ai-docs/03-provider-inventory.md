# LLM_ONLY_PROVIDER_INVENTORY

## Summary table

| Registry        | Package                  | Template             |      Content DB | OG DB passed |           HTTP cache use | Required config                                        |
| --------------- | ------------------------ | -------------------- | --------------: | -----------: | -----------------------: | ------------------------------------------------------ |
| `reddit`        | `internal/reddit-json`   | `reddit-atom`        |              no |          yes |                base only | `feed-id`, `username` unless `proxy-url` handles them  |
| `hackernews`    | `internal/hackernews`    | `hackernews-atom`    | `hackernews.db` |          yes |                base only | none                                                   |
| `fingerpori`    | `internal/fingerpori`    | `fingerpori-atom`    |              no |           no |                base only | none                                                   |
| `feissarimokat` | `internal/feissarimokat` | `feissarimokat-atom` |              no |           no | yes, RSS conditional GET | none                                                   |
| `oglaf`         | `internal/oglaf`         | `oglaf-atom`         |      `oglaf.db` |          yes | yes, RSS conditional GET | none                                                   |
| `tildes`        | `internal/tildes`        | `tildes-atom`        |              no |          yes |                base only | none; defaults `tech`                                  |
| `youtube`       | `internal/youtube`       | `youtube-atom`       |              no |          yes |                base only | at least one `feed-url`, `feed-urls`, or `channel-ids` |

## reddit

Package: `internal/reddit-json`

Config:

```yaml
reddit:
  feed-id: ""
  username: ""
  min-score: 50
  min-comments: 10
  outfile: reddit.xml
  interval: 15m
  proxy-url: ""
  proxy-secret: ""
  og-proxy-url: ""
```

Constructor:

- `NewRedditProvider(minScore, minComments int, feedID, username, proxyURL, proxySecret, ogProxyURL string)`
- `UseContentDB: false`
- `BuildGenerator(..., previewInfo, provider.feedConfig, provider.OgDB)`

Fetch flow:

1. `constructFeedURL(feedID, username, proxyURL)`
   - proxy URL wins
   - default `https://www.reddit.com/.json?feed=<feedID>&user=<username>`
2. `NewRedditAPI(feedURL, proxySecret, feedID, username)`
3. if `proxySecret`: set headers `X-Proxy-Secret`, `X-Feed-ID`, `X-Feed-User`
4. GET/decode Reddit JSON feed
5. `FilterPosts(posts, MinScore, MinComments)`
6. apply preview limit if >0

OpenGraph proxy:

- `feedConfig()` copies `previewInfo.Config` and sets `ProxyURL=OGProxyURL`, `ProxySecret=ProxySecret` only if both set.
- Used by `feed.createOGFetcher` for proxiable Reddit OG URLs.

Item type:

- `RedditPost` implements `FeedItem` in `types.go`.
- Optional likely includes subreddit/domain fields; verify before template edits.

## hackernews

Package: `internal/hackernews`

Config:

```yaml
hackernews:
  min-points: 50
  limit: 30
  outfile: hackernews.xml
  interval: 15m
```

Constructor:

- `NewProvider(minPoints, limit int, categoryMapper *CategoryMapper)`
- default category mapper: `LoadConfig("")`
- `UseContentDB: true`, `ContentDBName: hackernews.db`
- `BuildGenerator(..., previewInfo, nil, provider.OgDB)`

Fetch flow:

1. fetch Algolia front page: `https://hn.algolia.com/api/v1/search_by_date?tags=front_page&hitsPerPage=100`
2. initialize content DB schema
3. update stored items; get recently updated IDs
4. default item limit from config when `limit == 0`
5. query stored items by limit and min points
6. update stale stats concurrently via Algolia item endpoint `https://hn.algolia.com/api/v1/items/%s`
7. re-query
8. categorize by domain/content/points
9. convert to `[]providers.FeedItem`

Category mapper:

- File: `internal/hackernews/config.go`
- Loads local path if provided, else embedded `configs/domains.json`.
- Config shape: `{ "category_domains": { "category": ["domain"] } }`
- Matching: exact domain first, then substring contains.

Item notes:

- HN story URL may be empty; templates must handle link/comments semantics carefully.
- `ItemDomain()` optional method feeds `TemplateItem.Domain`.

## fingerpori

Package: `internal/fingerpori`

Config:

```yaml
fingerpori:
  limit: 100
  outfile: fingerpori.xml
  interval: 24h
```

Constructor:

- `NewProvider(limit int)`
- `UseContentDB: false`
- `BuildGenerator(..., previewInfo, nil, nil)` so no OpenGraph fetch.

Fetch flow:

1. GET HS API `https://www.hs.fi/api/laneitems/39221/list/normal/290`
2. decode JSON into `[]Item`
3. parse `DisplayDate` with layout `2006-01-02T15:04:05.000-07:00`
4. derive image ID from `Picture.URL` path segment `[3]`
5. build image URL `https://images.sanoma-sndp.fi/<imageID>/normal/1440.jpg`
6. build content `<img src=... alt=...>`
7. apply limit

## feissarimokat

Package: `internal/feissarimokat`

Config:

```yaml
feissarimokat:
  outfile: feissarimokat.xml
  interval: 24h
```

Constructor:

- `NewProvider()`
- `UseContentDB: false`
- `BuildGenerator(..., previewInfo, nil, nil)`

Fetch flow:

1. `fetchRSSFeedWithCache(p.HTTPCache)` from `https://static.feissarimokat.com/dynamic/latest/posts.rss`
2. XML unmarshal RSS
3. for each item, fetch post page
4. scrape `<div class="postbody">` then image `src` attributes
5. make relative images absolute with `https://static.feissarimokat.com`
6. content = RSS description + embedded image tags
7. apply limit

HTTP cache:

- 304 surfaces via `httpcache.ErrNotModified`, wrapped in fetch error? Verify wrapping behavior when changing; `BuildGenerator` only catches direct `errors.Is`.

## oglaf

Package: `internal/oglaf`

Config:

```yaml
oglaf:
  feed-url: https://www.oglaf.com/feeds/rss/
  outfile: oglaf.xml
  interval: 24h
```

Constructor:

- `NewOglafProvider(feedURL string)`
- `UseContentDB: true`, `ContentDBName: oglaf.db`
- `BuildGenerator(..., previewInfo, nil, provider.OgDB)`

Fetch/process flow:

1. initialize content DB schema
2. cleanup old data
3. conditional GET RSS via `httpcache.CachedGet`
4. parse RSS with regexes, skip bad/missing `pubDate`
5. identify new RSS items in content DB
6. cache new RSS items
7. backfill image URLs for up to 50 unprocessed comics by fetching page and extracting strip/media URL
8. mark extracted/error in DB
9. return latest `feedItemLimit = 25` processed comics
10. if RSS 304 and no backfilled items: return `httpcache.ErrNotModified`
11. apply preview limit

Stability note:

- Feed window fixed to recent processed comics to avoid reader duplicate churn.
- Do not return arbitrary just-backfilled batch directly.

## tildes

Package: `internal/tildes`

Config:

```yaml
tildes:
  topic: tech
  topics: []
  outfile: tildes.xml
  interval: 30m
```

Constructor:

- `NewTildesProvider(topics ...string)`
- strips leading `~`, default `tech`
- `UseContentDB: false`
- `BuildGenerator(..., previewInfo, p.feedConfig, p.OgDB)`

Fetch flow:

1. normalize `topic` + `topics`
2. for each topic, fetch Atom feed from Tildes group URL
3. parse votes/comment counts from entry content
4. clean content
5. create `Item` with group `~topic`
6. merge all topics
7. sort newest-first
8. apply limit

Feed config:

- single topic customizes title/link/description/id to group.
- multi-topic sets generic Tildes groups metadata.

## youtube

Package: `internal/youtube`

Config:

```yaml
youtube:
  feed-url: ""
  feed-urls: []
  channel-ids: []
  limit: 30
  include-shorts: false
  outfile: youtube.xml
  interval: 30m
```

Constructor:

- `NewYouTubeProvider(feedURLs []string, limit int, includeShorts bool)`
- `UseContentDB: false`
- `BuildGenerator(..., previewInfo, p.feedConfig, p.OgDB)`

Factory:

- `normalizeFeedURLs(feedURL, feedURLs, channelIDs)`
- errors if no feed URL/channel ID.

Fetch flow:

1. fetch all configured YouTube Atom feeds
2. skip Shorts unless `include-shorts` true (`entry.isShort()`)
3. dedupe by `VideoID`, fallback alternate href
4. map to `Item{entry, channelTitle}`
5. sort newest-first
6. apply configured limit unless preview limit overrides

Feed config:

- if exactly one valid YouTube feed URL, feed ID set to that URL.
