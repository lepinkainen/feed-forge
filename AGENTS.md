# Repository Guidelines

## Project Structure & Module Organization

`cmd/feed-forge/` contains the CLI entry point that wires providers and configuration. Provider implementations live under `internal/reddit/` and `internal/hackernews/`, with shared interfaces in `internal/pkg/providers/`. Configuration helpers are in `internal/config/`, while reusable feed builders and OpenGraph helpers sit in `pkg/feed/` and `pkg/opengraph/`. Sample configs and templates are kept in `config_example.yaml`, `configs/`, and `templates/`. Build artifacts land in `build/`; keep transient files out of source directories.

## Build, Test, and Development Commands

Use Task for repeatable workflows: `task build` compiles the binary (after running tests and lint), `task run-reddit` and `task run-hackernews` execute the built CLI against the default config, and `task clean` removes generated binaries and feeds. For focused work you can call `go test ./...` or `go build ./cmd/feed-forge` directly; keep `go mod tidy` in sync if dependencies change.

## Coding Style & Naming Conventions

Go files must remain `gofmt`-clean; `task lint` enforces `goimports`, `golangci-lint`, `go vet`, and `go mod tidy`. Stick to idiomatic Go naming: packages are lower_snake, exported symbols use PascalCase, and tests follow `TestXxx`. Keep files short and focused; prefer splitting helper types into the package that owns them.

## Testing Guidelines

Primary coverage comes from `go test ./...`; CI runs `task test-ci` (`go test -tags=ci -cover -v ./...`). Place tests alongside sources with `_test.go` suffix. Golden files under `testdata/` can be refreshed via `task update-golden` once outputs are vetted. Include table-driven cases for provider logic and ensure new providers hook into existing test suites.

## Commit & Pull Request Guidelines

Follow the conventional commit verbs already in history (e.g., `feat:`, `refactor:`, `chore:`). Keep subjects under ~70 characters, with detail in the body when behavior changes or config defaults move. Before opening a PR, run `task lint test` and note any manual verification (sample feed files, config migrations). Link related issues, describe observable changes, and attach feed diffs or screenshots when outputs matter.

## Configuration & Secrets

Never commit real API credentials. Copy `config_example.yaml` to `config.yaml` for local runs and inject secrets via environment variables or your private config. Document new configuration keys in the README and templates so other agents can discover them quickly.

## LLM Shared Resources

The `llm-shared/` directory bundles organization-wide playbooks and helper tools.

- **Overview**: `llm-shared/README.md` maps the shared resources; start here when in doubt.
- **Project-wide practices**: `llm-shared/project_tech_stack.md` covers branch hygiene, Taskfile expectations (`build`, `lint`, `test`, CI variants), validation helpers, and recommendations like using `validate-docs` for structure checks.
- **GitHub workflow**: `llm-shared/GITHUB.md` and `llm-shared/create-gh-labels.sh` describe the label taxonomy (`needs-plan`, `in-progress`, etc.) and provide a bootstrap script to install the full label set via `gh`.
- **Shell tooling**: `llm-shared/shell_commands.md` standardizes on `rg`/`fd` and includes common search patterns to speed up code exploration.
- **Language guides**: `llm-shared/languages/go.md` is the canonical Go playbook (Go 1.24, `goimports`, golangci-lint config, CI best practices). Additional Python/JavaScript notes live alongside it if cross-language work crops up.
- **Utilities**: `llm-shared/utils/` ships analyzers such as `gofuncs`, `pyfuncs.py`, `jsfuncs.js`, and the `validate-docs` CLI for quick structural audits.
- **Templates**: `llm-shared/templates/` holds starter `Taskfile.yml`, `.gitignore` variants, changelog, and GitHub workflow templates that match the shared guidelines.
