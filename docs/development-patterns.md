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
- **Map-based frontmatter** — all entity frontmatter is stored in `map[string]any`; unknown fields survive read-write cycles; known fields have typed accessors

## Adding a New Command

Every new `vault-cli <noun> <verb>` command follows this layered approach:

1. **Domain** (`pkg/domain/`) — entity struct embeds `XxxFrontmatter` (a `FrontmatterMap`-backed typed wrapper), `FileMetadata`, and `Content string`. Typed getters/setters for known fields; `GetField`/`SetField`/`ClearField` for arbitrary keys. Unknown fields survive read-write cycles.
2. **Storage** (`pkg/storage/markdown.go`) — add methods to `Storage` interface, implement on `markdownStorage`
3. **Operation** (`pkg/ops/`) — interface + factory + struct, one file per operation
4. **CLI** (`pkg/cli/cli.go`) — Cobra command wired into root, uses `getVaults` for multi-vault

## Entity Structure

Each entity (Task, Goal, Theme, Objective, Vision) cleanly separates three concerns:

**Frontmatter** (`pkg/domain/<entity>_frontmatter.go`)
- Embeds `FrontmatterMap` (a `map[string]any` wrapper)
- Typed getter methods for known fields (e.g., `Status() TaskStatus`, `Priority() Priority`)
- Typed setter methods that validate known fields (e.g., `SetStatus(TaskStatus) error`)
- Generic `GetField(key) string` / `SetField(ctx, key, value) error` / `ClearField(key)` for
  arbitrary keys — unknown fields pass through without validation
- All fields in the map (known and unknown) are preserved through read-write cycles

**Filesystem metadata** (`pkg/domain/file_metadata.go`)
- `FileMetadata` struct: `Name`, `FilePath`, `ModifiedDate`
- Embedded in every entity; never stored in YAML

**Markdown content**
- `Content string` field on the entity struct
- The full markdown file content including the frontmatter block
- Used by the storage layer to extract the body on write

**Storage** (`pkg/storage/`)
- `parseToFrontmatterMap` parses the YAML frontmatter block into `map[string]any`
- `serializeMapAsFrontmatter` marshals the map back to YAML; unknown fields are preserved
- Entity-specific read helpers call `NewXxx(data, meta, content)` constructors

**Operations** (`pkg/ops/`)
- Inject `XxxStorage` interfaces (never file I/O directly)
- Use entity accessor methods (`goal.Status()`, `task.SetField(ctx, key, val)`)
- No reflection; no hardcoded field switches

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
