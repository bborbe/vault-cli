# Development Patterns

Project-specific patterns for vault-cli. For general Go patterns see [coding-guidelines](https://github.com/bborbe/coding-guidelines).

## Architecture

- `main.go` — CLI entry point, delegates to `pkg/cli`
- `pkg/cli/` — Cobra command definitions, output formatting
- `pkg/config/` — Config file parsing, `Loader` interface, vault resolution
- `pkg/domain/` — Domain types (Goal, Task, Theme, Vision, Objective)
- `pkg/ops/` — Business operations (complete, defer, lint, list, search, update, workon)
- `pkg/storage/` — Markdown file read/write, frontmatter parsing
- `mocks/` — Counterfeiter-generated mocks
- `integration/` — Integration tests

## Key Design Decisions

- **Cobra CLI** — all commands under `vault-cli <noun> <verb>` pattern
- **Config via YAML** — `~/.vault-cli/config.yaml`, `Loader` interface for testability
- **Plain output default** — `--output plain` (default) or `--output json`
- **Factory functions are pure composition** — no conditionals, no I/O, no `context.Background()`

## Adding a New Command

Every new `vault-cli <noun> <verb>` command follows this layered approach:

1. **Domain** (`pkg/domain/`) — struct with YAML tags for frontmatter fields, metadata fields tagged `yaml:"-"`
2. **Storage** (`pkg/storage/markdown.go`) — add methods to `Storage` interface, implement on `markdownStorage`
3. **Operation** (`pkg/ops/`) — interface + factory + struct, one file per operation
4. **CLI** (`pkg/cli/cli.go`) — Cobra command wired into root, uses `getVaults` for multi-vault

## Multi-Vault Pattern

All commands use `getVaults()` to resolve vaults:

- `--vault NAME` → single vault
- No flag → all configured vaults

Commands iterate vaults and call operations per vault. For mutation commands (complete, defer, ack), try each vault until the item is found.

## Output Format

- `--output plain` (default) — human-readable lines
- `--output json` — structured JSON via `PrintJSON(v)` from `output.go`
- Never import `encoding/json` in command files — use the `PrintJSON` helper

## Testability

- Inject `libtime.CurrentDateTime` for date/time (never call `time.Now()` directly)
- Inject `storage.Storage` interface (never read files directly in ops)
- Factory functions are pure composition — no conditionals, no I/O, no `context.Background()`

## Mocks

- Counterfeiter with `//counterfeiter:generate` comments on interfaces
- Mocks go in `mocks/` directory
- Regenerate: `go generate ./...`

## Naming

- Operations: `<Noun><Verb>Operation` (e.g., `DecisionListOperation`, `DecisionAckOperation`)
- Files: `pkg/ops/<noun>_<verb>.go` + `_test.go`
- CLI: `create<Noun>Commands()` returns parent, `create<Noun><Verb>Command()` returns leaf
