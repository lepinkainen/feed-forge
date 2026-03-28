# Repository Guidelines

## Project Structure & Module Organization

- `cmd/feed-forge/` hosts the CLI entry point that wires providers, config, and logging.
- Provider code lives in `internal/reddit/` and `internal/hackernews/`; shared interfaces reside in `internal/pkg/providers/`.
- Configuration helpers are under `internal/config/`, while reusable feed and OpenGraph utilities live in `pkg/feed/` and `pkg/opengraph/`.
- Sample configs and templates stay in `config_example.yaml`, `configs/`, and `templates/`; avoid storing generated outputs there.
- Build artifacts land in `build/`; keep transient files and scratch scripts out of source trees.

## Build, Test, and Development Commands

- `task build` compiles the binary after running tests and lint.
- `task run-reddit` / `task run-hackernews` execute the CLI with the default config to produce feeds.
- `task clean` removes compiled binaries and generated feed files.
- For focused loops, `go test ./...` runs all tests and `go build ./cmd/feed-forge` verifies the CLI builds locally.

## Coding Style & Naming Conventions

- Go 1.24, `gofmt`, and `goimports` are required; `task lint` enforces `golangci-lint`, `go vet`, and `go mod tidy`.
- Follow idiomatic Go naming: packages lower_snake, exported symbols PascalCase, tests `TestXxx`.
- Keep helpers close to their packages and prefer short, focused files.

## Testing Guidelines

- Primary coverage comes from `go test ./...`; CI runs `task test-ci` (`go test -tags=ci -cover -v ./...`).
- Place tests alongside sources with `_test.go` suffix and use table-driven cases for provider logic.
- Golden files in `testdata/` can be refreshed via `task update-golden` once outputs are validated.

## Commit & Pull Request Guidelines

- Use conventional commits (e.g., `feat:`, `refactor:`, `chore:`) with ~70 character subjects and descriptive bodies for behavior changes.
- Before opening a PR, run `task lint test` and note any manual feed verification.
- Link related issues, describe observable changes, and attach diffs or screenshots when feeds or templates change.

## Security & Configuration Tips

- Never commit real API credentials; copy `config_example.yaml` to `config.yaml` for local runs.
- Inject secrets via environment variables or personal configs, and document new keys in READMEs or templates.
- Keep each change within the workspace sandbox; transient experimentation belongs outside tracked directories or in `build/`.
