# LLM_ONLY_PROJECT_DOCS

Audience: language models/agents only. Human readability not goal. Treat docs as cached repo map; verify against source before edits.

## Repo

- Module: `github.com/lepinkainen/feed-forge`
- Language: Go. `go.mod` says `go 1.26.1`.
- App: CLI RSS/Atom feed generator for multiple sources.
- Entry: `cmd/feed-forge/main.go`
- Config example: `config_example.yaml`
- Templates: `templates/*.tmpl`, embedded by `templates/embedded.go`.

## Doc files

- `ai-docs/01-runtime-architecture.md`: CLI/config/generate flow, package roles.
- `ai-docs/02-provider-contract.md`: provider interface, registration, generator pattern, add-provider checklist.
- `ai-docs/03-provider-inventory.md`: current provider specifics.
- `ai-docs/04-feeds-templates-preview.md`: template data, feed generation, preview TUI.
- `ai-docs/05-cache-network-storage.md`: caches, DBs, HTTP clients, SSRF protections.
- `ai-docs/06-build-test-change-recipes.md`: commands, tests, common change paths, pitfalls.

## Fast facts

- Registered providers: `reddit`, `hackernews`, `fingerpori`, `feissarimokat`, `oglaf`, `tildes`, `youtube`.
- Main command modes: provider-specific commands, `preview <provider>`, `generate`.
- `generate` reads configured provider sections from YAML, runs providers concurrently, skips by outfile mtime + `interval`, writes `index.html` and `feeds.opml` when `output-dir` is set.
- `BaseProvider` always opens OpenGraph DB + HTTP validator cache. Content DB optional.
- Provider `GenerateFeed` usually delegated via `providerfeed.BuildGenerator` and `BaseProvider.SetGenerateFeedFunc`.
- Feed output uses Atom XML templates with override-first FS: local `templates/`, fallback embedded `templates` package.

## Verify before editing

Use source as truth:

- CLI/config: `cmd/feed-forge/main.go`
- Provider contracts: `pkg/providers/*.go`, `pkg/providerfeed/generator.go`
- Feed template model: `pkg/feed/*.go`
- Provider code: `internal/<provider>/`
- Commands: `Taskfile.yml`
