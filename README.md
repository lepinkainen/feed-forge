# Feed Forge

Feed Forge generates Atom feeds from sites that have no feed, a bad feed, or too much
traffic to read one item at a time.

## What it does

- **Eight sources.** Reddit, Hacker News, lobste.rs, Tildes, YouTube channels, and the
  Fingerpori, Feissarimokat, and Oglaf comics.
- **One CLI.** Every source is a subcommand. `generate` builds all configured feeds in
  one run.
- **Filters.** Set a minimum score, a minimum comment count, or an item limit per
  source.
- **Rich entries.** Feed Forge fetches OpenGraph metadata and adds the image and
  description to each entry.
- **Standard Atom.** There are no custom XML namespaces, so any reader can parse the
  output.
- **Interactive preview.** `preview <provider>` shows the items in a terminal before
  you publish the feed.
- **Bulletin digests.** The bulletin pipeline collects many news feeds, removes
  near-duplicate stories, and publishes one summarized digest twice a day.
- **New sources are small.** Add a package under `internal/`, implement the provider
  interface, and register it.

## Installation

### From source

```bash
git clone https://github.com/lepinkainen/feed-forge.git
cd feed-forge
task build
```

The binary appears at `build/feed-forge`. The build needs
[Task](https://taskfile.dev/) and Go 1.26 or later.

### With Go

```bash
go install github.com/lepinkainen/feed-forge/cmd/feed-forge@latest
```

## Usage

Copy the example configuration first:

```bash
cp config_example.yaml config.yaml
```

Then edit `config.yaml`. Reddit needs a feed ID and a username. The other sources work
with their defaults.

### Generate one feed

```bash
./build/feed-forge reddit -o reddit.xml --min-score 100 --min-comments 20
./build/feed-forge hackernews -o hackernews.xml --min-points 100 --limit 20
./build/feed-forge lobsters -o lobsters.xml --min-score 10 --tags go --tags rust
./build/feed-forge tildes -o tildes.xml --topic tech
./build/feed-forge youtube -o youtube.xml --channel-ids UCT5C7yaO3RVuOgwP8JVAujQ
./build/feed-forge fingerpori -o fingerpori.xml
./build/feed-forge feissarimokat -o feissarimokat.xml
./build/feed-forge oglaf -o oglaf.xml
```

### Generate every configured feed

```bash
./build/feed-forge generate
```

`generate` reads each provider section in `config.yaml` and runs those providers
concurrently. It skips a provider when its output file is newer than the `interval`
you set. This makes the command safe to run from cron every few minutes.

When you set `output-dir`, `generate` also writes an `index.html` page and a
`feeds.opml` file that lists every feed.

### Preview items

```bash
./build/feed-forge preview hackernews
./build/feed-forge preview lobsters
```

`preview` takes the same provider name as the generate command.

The preview fetches live items and shows them in the terminal. To print the Atom XML
for one item, add `--index 0`.

Preview writes no feed file. It does write the shared cache databases, because it
creates a real provider: `opengraph.db` and `http_cache.db` every time, plus a content
database such as `hackernews.db` for the providers that keep one. These files land in
`--cache-dir`. Preview is therefore not a read-only command.

### Get a YouTube feed URL

```bash
./build/feed-forge youtube-rss https://www.youtube.com/@Taskmaster
```

This prints the Atom feed URL that the channel page advertises. Put that URL in
`youtube.feed-urls`.

## Configuration

`config.yaml` holds every setting. Field names use kebab-case and match the CLI flag
names. A CLI flag always overrides the file.

```yaml
output-dir: "" # Base directory for all output files
feed-base-url: "https://example.com/rss/" # Public URL used in feeds.opml
cache-dir: "" # Cache databases. Defaults to ~/.cache/feed-forge

reddit:
  feed-id: "" # Required: from https://www.reddit.com/prefs/feeds/
  username: "" # Required: your Reddit username
  min-score: 50
  min-comments: 10
  outfile: reddit.xml
  interval: 15m # Minimum time between regenerations

hackernews:
  min-points: 50
  limit: 30
  outfile: hackernews.xml
  interval: 15m

lobsters:
  min-score: 10
  tags: [] # for example [go, rust]
  outfile: lobsters.xml
  interval: 30m

fingerpori:
  limit: 100
  outfile: fingerpori.xml
  interval: 24h # Daily comic, so a daily check is enough
```

Each provider uses one name everywhere: the YAML key, the CLI command, and the
`preview` argument are the same string.

`config_example.yaml` documents every provider and every key. Read it for the full
list.

CAUTION: If `config.yaml` holds a real API key, do not commit the file. Set
`ANTHROPIC_API_KEY` in the environment instead. If you must put the key in the file,
run `chmod 600 config.yaml` first.

### Global flags

| Flag | Effect |
|---|---|
| `--config <path>` | Configuration file path. The default is `config.yaml`. |
| `--debug` | Turn on debug logging. |
| `--output-dir <path>` | Base directory for generated files. |
| `--feed-base-url <url>` | Public base URL written into `feeds.opml`. |
| `--cache-dir <path>` | Directory for the cache databases. |
| `--discord-webhook-url <url>` | Send a Discord notification when a run fails. |

## Bulletin digests

The bulletin pipeline is separate from the providers. It polls many news feeds,
extracts the full article text, groups near-duplicate stories with SimHash, and asks
Claude for one topic-grouped digest. The result is a single Atom entry plus an HTML
page.

The pipeline has three stages. Run them from cron in this order:

```cron
*/30 *   * * *  feed-forge bulletin-fetch                     # collect items
45   7,17 * * *  feed-forge bulletin-generate                  # summarize the backlog
50   7,17 * * *  feed-forge bulletin-publish -o bulletin.xml   # render HTML and Atom
```

`bulletin-generate` is the only stage that calls the model, so it needs
`ANTHROPIC_API_KEY`. `bulletin-publish` writes nothing to the database. To rebuild
every HTML page after a template change, run `bulletin-publish` again.

To see the digest without writing anything, run `feed-forge bulletin-summarize`. Use
it to iterate on the prompt.

Configure the source feeds, the model, and the SimHash threshold in the `bulletin:`
section of `config.yaml`.

## Development

```bash
task build          # Build the binary. Runs tests and lint first
task test           # Run all tests
task lint           # Format and lint
task clean          # Remove build artifacts
task build-linux    # Cross-compile for Linux AMD64
task update-golden  # Refresh golden test files
task help           # List every task
```

Use `task` rather than raw `go` commands. `task build` runs the tests and the linter,
so a green build means the change is complete.

Some sources have shortcut tasks: `run-reddit`, `run-hackernews`, `run-fingerpori`,
`run-feissarimokat`, and `run-oglaf`.

### Architecture

Every source implements one interface in `pkg/providers/provider.go`:

```go
type FeedProvider interface {
    GenerateFeed(outfile string) error
    FetchItems(limit int) ([]FeedItem, error)
}
```

A provider embeds `BaseProvider`, which owns the shared SQLite caches for OpenGraph
metadata and HTTP validators. Each provider registers itself in an `init()` function,
so the CLI discovers it without a central list.

Feeds are rendered from Go templates in `templates/`. The templates are embedded in
the binary, and a local `templates/` directory overrides them.

```text
feed-forge/
├── cmd/feed-forge/     # CLI entry point
├── internal/
│   ├── bulletin/       # Bulletin pipeline
│   ├── feissarimokat/  # One package per provider
│   ├── fingerpori/
│   ├── hackernews/
│   ├── lobsters/
│   ├── oglaf/
│   ├── reddit-json/    # Registered as "reddit"
│   ├── tildes/
│   └── youtube/
├── pkg/                # Shared packages
├── templates/          # Atom and HTML templates
└── docs/               # Guides
```

To add a source, read `docs/adding-a-provider.md`.

## Requirements

- Go 1.26 or later, and Task, to build from source.
- A Reddit feed ID and username, for Reddit feeds.
- An Anthropic API key, for bulletin digests only.
- Network access.

## Contributing

1. Fork the repository.
2. Create a feature branch. Do not commit to `main`.
3. Make the change.
4. Run `task lint` and `task test`.
5. Open a pull request. Describe the observable change.

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/).

## License

MIT. Read the [LICENSE](LICENSE) file.
