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

## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Auto-syncs to JSONL for version control
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**

```bash
bd ready --json
```

**Create new issues:**

```bash
bd create "Issue title" -t bug|feature|task -p 0-4 --json
bd create "Issue title" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**

```bash
bd update bd-42 --status in_progress --json
bd update bd-42 --priority 1 --json
```

**Complete work:**

```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task**: `bd update <id> --status in_progress`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`
6. **Commit together**: Always commit the `.beads/issues.jsonl` file together with the code changes so issue state stays in sync with code state

### Auto-Sync

bd automatically syncs with git:

- Exports to `.beads/issues.jsonl` after changes (5s debounce)
- Imports from JSONL when newer (e.g., after `git pull`)
- No manual export/import needed!

### MCP Server (Recommended)

If using Claude or MCP-compatible clients, install the beads MCP server:

```bash
pip install beads-mcp
```

Add to MCP config (e.g., `~/.config/claude/config.json`):

```json
{
  "beads": {
    "command": "beads-mcp",
    "args": []
  }
}
```

Then use `mcp__beads__*` functions instead of CLI commands.

### Managing AI-Generated Planning Documents

AI assistants often create planning and design documents during development:

- PLAN.md, IMPLEMENTATION.md, ARCHITECTURE.md
- DESIGN.md, CODEBASE_SUMMARY.md, INTEGRATION_PLAN.md
- TESTING_GUIDE.md, TECHNICAL_DESIGN.md, and similar files

#### Best Practice

Use a dedicated directory for these ephemeral files.

**Recommended approach:**

- Create a `history/` directory in the project root
- Store ALL AI-generated planning/design docs in `history/`
- Keep the repository root clean and focused on permanent project files
- Only access `history/` when explicitly asked to review past planning

**Example .gitignore entry (optional):**

```gitignore
# AI planning documents (ephemeral)
history/
```

**Benefits:**

- ✅ Clean repository root
- ✅ Clear separation between ephemeral and permanent documentation
- ✅ Easy to exclude from version control if desired
- ✅ Preserves planning history for archeological research
- ✅ Reduces noise when browsing the project

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ✅ Store AI planning docs in `history/` directory
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems
- ❌ Do NOT clutter repo root with planning documents

For more details, see README.md and QUICKSTART.md.
