# LLM_ONLY_BUILD_TEST_CHANGE_RECIPES

## Commands

Primary commands from `Taskfile.yml`:

```bash
task build          # deps: test, lint; builds build/feed-forge
task test           # go test ./...
task lint           # goimports -w ., golangci-lint run, go vet ./..., go mod tidy
task clean          # remove build/, *.xml, *.db
task build-linux    # GOOS=linux GOARCH=amd64, deps test+lint
task test-ci        # go test -tags=ci -cover -v ./...
task update-golden  # go test -v ./... -update
task test-update    # refresh selected external fixtures via curl
```

Focused dev:

```bash
go test ./...
go test ./internal/hackernews
go test ./pkg/feed
go build ./cmd/feed-forge
```

Run examples from Taskfile may be stale for HN command naming; verify `cmd/feed-forge/main.go` Kong tags before using.

## Test layout

- Tests live adjacent to source with `_test.go`.
- Provider test data under `internal/<provider>/testdata`.
- Golden helpers in `pkg/testutil/golden.go`.
- `task update-golden` passes `-update` to all tests.
- External fixture refresh is `task test-update`, only for selected providers.

## Common change recipes

### Add new provider

Use `ai-docs/02-provider-contract.md` checklist.
Minimal touched files:

- `internal/newprovider/provider.go`
- `internal/newprovider/types.go`
- `internal/newprovider/api.go`
- `templates/newprovider-atom.tmpl`
- `cmd/feed-forge/main.go`
- `config_example.yaml`
- tests

After edits:

```bash
gofmt -w internal/newprovider cmd/feed-forge/main.go
goimports -w internal/newprovider cmd/feed-forge/main.go
go test ./internal/newprovider ./cmd/feed-forge ./pkg/feed ./pkg/providers
go test ./...
```

### Add provider config field

Touch:

- provider `Config` struct with YAML tag
- CLI subcommand struct in `cmd/feed-forge/main.go`
- `buildProviderConfig` mapping
- factory/constructor/provider field if runtime needs it
- `ConfigFactory` default if non-zero default needed for `generate`/`preview`
- `config_example.yaml`
- tests for direct command and `generate` YAML load if risk exists

Important:

- Direct commands use Kong-populated `CLI` and `buildProviderConfig`.
- `generate`/`preview` use `ConfigFactory` then `loadProviderConfigFromYAML`.
- If default only in CLI tag and not `ConfigFactory`, `generate` may miss it.

### Change feed template model

Touch:

- `pkg/feed/template.go` `TemplateData` or `TemplateItem`
- `pkg/feed/generator.go` `createGenericFeedData`
- templates using new field
- `pkg/preview/format.go` if XML preview should show behavior
- feed/template tests

Keep optional provider-specific fields via optional interfaces to avoid widening core `FeedItem` unless all providers should implement field.

### Change `generate` orchestration

Touch:

- `cmd/feed-forge/main.go`
- tests in `cmd/feed-forge/*_test.go`

Preserve:

- provider skip by interval uses resolved outfile path.
- result filename remains original configured relative filename for index links.
- concurrent providers should not race on shared global config.
- close providers that implement `Close()`.

### Change OpenGraph behavior

Touch:

- `pkg/opengraph/fetcher.go`, `pkg/opengraph/database.go`
- `pkg/urlutils` if URL safety changes
- provider feed config functions if proxy data changes
- templates if OG fields change

Preserve:

- safe URL validation.
- blocked internal/private addresses.
- recent failure caching.
- conditional revalidation.
- per-domain/per-URL concurrency controls.

### Change HTTP caching

Touch:

- `pkg/api/enhanced_client.go` conditional GET code
- `pkg/httpcache/cache.go`
- provider tests for 304 behavior
- `pkg/providerfeed/generator.go` if `ErrNotModified` semantics change

Preserve:

- `errors.Is(err, httpcache.ErrNotModified)` compatibility.
- output mtime bump when upstream unchanged and outfile exists.

## Known drift / verify spots

- README and `Taskfile.yml` mention `hacker-news`; current CLI struct tag in `main.go` appears `cmd:"hackernews"` unless changed. Verify before modifying docs/tasks.
- `docs/adding-a-provider.md` contains older `GenerateFeed(outfile string, _ bool)` example; current interface is `GenerateFeed(outfile string) error`.
- Project guidelines mention `internal/pkg/providers`, but actual shared provider package is `pkg/providers`.
- `go.mod` says Go `1.26.1`; ensure local toolchain supports it before running full commands.

## Lint/style rules

- Use `gofmt` and `goimports`.
- Idiomatic package names lower-case; existing `reddit-json` directory package name is `redditjson`.
- Keep endpoint URLs as vars when tests need httptest overrides.
- Use `%w` for wrapped sentinel errors.
- Do not call `os.Exit` outside CLI entrypoint.
- Do not commit real `config.yaml` credentials.

## Debugging commands

List registered CLI help after build:

```bash
go run ./cmd/feed-forge --help
```

Preview provider without TUI XML only:

```bash
go run ./cmd/feed-forge preview hackernews --index 0 --config config.yaml
```

Generate all configured feeds:

```bash
go run ./cmd/feed-forge --config config.yaml generate
```

Use custom cache/output dirs to avoid dirty workspace:

```bash
tmpdir=$(mktemp -d)
go run ./cmd/feed-forge --cache-dir "$tmpdir/cache" --config config.yaml --debug generate
```

## Safe edit heuristics for agents

- Read source file before editing, especially generated docs can be stale.
- Prefer AST/LSP for symbol discovery.
- For markdown/template-only changes, still run targeted template tests when available.
- For provider network parsing, add httptest-based unit tests; avoid live network in normal tests.
- Keep generated feed files, DB files, coverage outputs, and build artifacts out of source changes unless explicitly requested.
