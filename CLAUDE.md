# CLAUDE.md

This file gives guidance to Claude Code (claude.ai/code) for work in this repository.

Terminology in this file is fixed. "Make sure that" is the only verb phrase for a
check. "Configuration" is the only noun for settings. "Run" is the only verb for
running a command.

## Build and Development Commands

Task (taskfile.dev) is the build system. Use `task` commands. Do not run `go build`,
`go test`, or `go vet` directly.

| Command | Action |
|---|---|
| `task build` | Build the binary. Runs `test` and `lint` first. |
| `task test` | Run all tests. |
| `task lint` | Run `gofmt`, `go vet`, `go mod tidy`, and `golangci-lint`. |
| `task clean` | Remove build artifacts and generated feed files. |
| `task build-linux` | Build for Linux AMD64. |
| `task build-ci` | Build for CI with coverage. |
| `task test-ci` | Run tests with the `ci` build tag and coverage. |
| `task update-golden` | Update golden files from the current code. Runs the tests. |
| `task test-update` | Download fresh live snapshots into tracked `testdata/`. Runs no tests. |
| `task vuln` | Run the vulnerability scanner. |
| `task deadcode` | Report unreachable code. |
| `task cognit` | Report cognitive complexity. |
| `task help` | List all tasks. |

A change is not complete until `task build` succeeds.

Run tasks exist for some providers only: `run-reddit`, `run-hackernews`,
`run-fingerpori`, `run-feissarimokat`, `run-oglaf`. Preview tasks exist for
`reddit`, `hackernews`, `fingerpori`, and `feissarimokat`. For other providers, run
the binary directly.

### Direct execution

```bash
./build/feed-forge reddit -o output.xml --min-score 100
./build/feed-forge hackernews -o output.xml --min-points 50
./build/feed-forge generate                 # all providers in config.yaml
./build/feed-forge preview hackernews       # interactive item preview
```

One name identifies a provider everywhere: the CLI command, the `config.yaml` section
key, the `preview` argument, and the registry name are the same string.

CAUTION: Kong derives a command name from the struct field name, so a field named
`HackerNews` becomes the command `hacker-news`. A `cmd:"hackernews"` tag alone does
**not** change the name. Add an explicit `name:"hackernews"` tag. When a command name
and its registry name drift apart, `ctx.Command()` returns a string that no
`dispatchCommand` branch matches, and the binary panics on a primary command.
`TestProviderCommandNamesMatchRegistry` and `TestProviderCommandsDispatch` in
`cmd/feed-forge/main_test.go` guard against this.

Global flags: `--config`, `--debug`, `--output-dir`, `--feed-base-url`,
`--cache-dir`, `--discord-webhook-url`.

### Bulletin commands

Read the Bulletin Pipeline section before you change these.

| Command | Action |
|---|---|
| `bulletin-fetch` | Poll source feeds, extract full text, and store new items. Cron every 30 minutes. |
| `bulletin-generate` | Deduplicate and summarize unpublished items into one stored bulletin. Cron at fixed slots, for example `45 7,17`. |
| `bulletin-publish -o bulletin.xml` | Render stored bulletins into HTML pages and the Atom feed. Cron immediately after generate. |
| `bulletin-summarize` | Print the digest for current unpublished items to stdout. Writes nothing. Use it to iterate on the prompt. |

`bulletin-generate` and `bulletin-summarize` call the model. Both need
`ANTHROPIC_API_KEY`.

## Architecture Overview

feed-forge is a CLI Atom feed generator for many sources. Each source is a provider.
All providers implement one interface, so the CLI treats them the same way.

### Provider model

`pkg/providers/provider.go` defines the `FeedProvider` contract. `BaseProvider`
(`pkg/providers/base.go`) holds the shared database connections, and every provider
embeds it. Most providers build `GenerateFeed` with `providerfeed.BuildGenerator` and
install it with `BaseProvider.SetGenerateFeedFunc`. Each provider self-registers in an
`init()` function with `providers.MustRegister`. The registry is the source of truth
for which providers exist; at present the set is `reddit`, `hackernews`, `fingerpori`,
`feissarimokat`, `oglaf`, `tildes`, `lobsters`, `lemmy`, and `youtube`, but check the
`MustRegister` calls rather than trusting this list. The code is in `internal/<name>/`.
The Reddit package directory is `internal/reddit-json/`.

For the full architecture — provider contract, registry, package roles, feed
templates, and preview — read `ai-docs/01-runtime-architecture.md`,
`ai-docs/02-provider-contract.md`, and `ai-docs/04-feeds-templates-preview.md`.
Confirm a fact in the source before you act on it.

To add a provider, use the `add-provider` skill in
`.claude/skills/add-provider/SKILL.md`. The long form is `docs/adding-a-provider.md`.

### CLI entry point

`cmd/feed-forge/main.go` parses the command line with Kong. It loads YAML through
`kong-yaml`, not Viper. The configuration helpers are in `pkg/config/loader.go`.

**IMPORTANT**: Kong populates the CLI sub-struct of the active command only. When you
run `generate` or `preview`, Kong leaves the provider sub-structs such as
`CLI.Reddit.*` empty. These two commands call `loadProviderConfigFromYAML()` to read
the provider section straight from YAML. Every provider `Config` struct must carry
`yaml` tags, or `generate` and `preview` will read zero values.

`generate` runs the configured providers concurrently. It skips a provider when the
output file is newer than its `interval`. When `output-dir` is set, it also writes
`index.html` and `feeds.opml`.

## Bulletin Pipeline (`internal/bulletin/`)

The bulletin is a separate code path. It does not implement `FeedProvider`, and
`generate` does not discover it. It aggregates many high-frequency outlets into
periodic summarized digests through three decoupled stages: `bulletin-fetch`
accumulates items, `bulletin-generate` turns the unpublished backlog into one stored
bulletin, and `bulletin-publish` renders the stored bulletins into HTML and Atom.
`bulletin-generate` and `bulletin-summarize` call the model and need
`ANTHROPIC_API_KEY`.

Read `ai-docs/07-bulletin-pipeline.md` for the stage internals, the SimHash dedup, the
prompt, and the configuration. Read it before you change these commands.

## Critical Rules

### Database timestamps

Use `time.Time` fields in Go structs and `TIMESTAMP` column affinity in SQLite. Let
the `modernc.org/sqlite` driver serialize the value. The driver round-trips
`time.Time` as RFC3339Nano text. That text sorts lexicographically, so
`ORDER BY ... DESC` equals chronological DESC.

CAUTION: Never store a raw upstream date string in a sortable column. RFC1123Z,
custom formats, and day-first locales sort in an order that disagrees with
chronological order, and feed ordering breaks silently.

Parse a source timestamp into `time.Time` at the API or RSS boundary. Do not parse it
later.

The `TIMESTAMP` or `DATETIME` column declaration is what makes the driver scan the
value back into `time.Time`. A `TEXT` column does not convert on `rows.Scan`.

### HTTP clients

Make every outbound call through a `pkg/api` enhanced client. These clients add rate
limiting, exponential backoff, and typed errors. A direct `http.Get` will hit rate
limits and fail.

- `api.NewRedditClient(baseClient)` — Reddit policy, 1-second rate limit.
- `api.NewHackerNewsClient()` — conservative Hacker News policy.
- `api.NewGenericClient()` — general purpose, minimal configuration.

Send the user agent `feed-forge/[version]` on external calls.

### Logging

Use `log/slog` for all logging.

### Configuration

`config.yaml` is the single configuration file. CLI flags override its values. Copy
`config_example.yaml` to `config.yaml` for local runs.

CAUTION: Never commit real credentials. Pass secrets through environment variables or
an untracked local configuration file. When you add a configuration key, document it
in `config_example.yaml`.

## Testing

Golden file tests are the main form of output validation. Golden files live in a
`testdata/` directory beside the code under test, such as `pkg/feed/testdata/` and
`internal/lobsters/testdata/`. The root `testdata/` directory holds shared fixtures
only.

- Write table-driven tests for provider logic.
- Name test files `<source>_test.go` and keep them beside the source.
- Use relative paths to reach `testdata/`.
- To refresh golden files, run `task update-golden`. This regenerates them from the
  current code and touches no network.
- Read the golden file diff before you commit it. The diff is the change in expected
  output.
- To skip a test in CI, add the `//go:build !ci` constraint.

CAUTION: `task test-update` is not a test command. It downloads live snapshots from
Oglaf, Feissarimokat, and the Hacker News Algolia API straight into tracked
`testdata/` directories, and it runs no tests. Use it only to refresh those upstream
fixtures on purpose, and read the diff before you commit it.

Every feature needs unit tests.

## Code Style

- Go 1.26.1. Read `llm-shared/versions.md` for current version guidance.
- After any Go change, run `goimports -w .`. Use `goimports`, not `gofmt`, because it
  also fixes imports.
- Write `any`, not `interface{}`.
- Compare errors with `errors.Is()` and `errors.As()`. Wrap with `%w`.
- Package names are lower case. Exported symbols are PascalCase. Tests are `TestXxx`.
- Keep files focused. Split a file when it grows large.
- Use `rg` instead of `grep` and `fd` instead of `find`.
- Use `modernc.org/sqlite` for database access. It needs no CGO.

## Git

- Never commit to `main` directly without user approval. Use a feature branch.
- Use Conventional Commits, for example `feat:`, `fix:`, `refactor:`, `chore:`. Keep
  the subject near 70 characters. Describe behavior changes in the body.
- Keep commits small and focused.
- Before you open a pull request, run `task lint` and `task test`. Report any manual
  feed check.
- Link the related issue. Describe the observable change. Attach the diff when a feed
  or a template changes.
- Build artifacts belong in `build/`. Keep scratch files out of tracked directories.

## Project Structure

- `cmd/feed-forge/` — CLI entry point, Kong command structs, and the `generate`
  command.
- `internal/<name>/` — one package per provider. `internal/reddit-json/` registers as
  `reddit`. `internal/bulletin/` is the bulletin pipeline, not a provider.
- `pkg/` — shared packages.
- `templates/` — Atom and HTML templates, embedded by `embedded.go`.
- `testdata/` — shared fixtures. Golden files sit beside their code.
- `configs/` — sample configurations. `proxy/` — Reddit proxy helper.
- `docs/` — human documentation. `ai-docs/` — agent-only repository map.
  `llm-shared/` — shared conventions submodule.

Read `ai-docs/00-index.md` and `ai-docs/01-runtime-architecture.md` for the full tree
and the package roles.

## Documentation Map

- `README.md` — installation and usage, for people.
- `docs/adding-a-provider.md` — long form provider guide.
- `ai-docs/` — dense agent-only repository map. `ai-docs/00-index.md` is the index.
  Faster to read than the source, but it can be stale. Confirm a fact in the source
  before you act on it.
- `llm-shared/project_tech_stack.md` — shared technology preferences.
- `llm-shared/utils/validate-docs/` — documentation and code consistency checker.
- Function inventory: `go run llm-shared/utils/gofuncs/gofuncs.go -dir .`

`AGENTS.md`, `GEMINI.md`, `CRUSH.md`, and `.github/copilot-instructions.md` are
symlinks to this file. Edit this file only.

# important-instruction-reminders

Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.
