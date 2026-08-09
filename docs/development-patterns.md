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
- `FrontmatterMap` provides the raw accessor family: `Get`, `GetString`, `GetBool`, `GetTime`,
  `GetStringSlice`, `Set`, `Delete`, `Keys`, `RawMap`. Getters **coerce** rather than type-assert —
  `GetBool` accepts a YAML bool and the strings `true` / `yes` / `false` / `no` (case-insensitive),
  returning `false` for a missing key or an unrecognised value.
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

**Decision — the one hybrid**
- `domain.Decision` embeds `FrontmatterMap` directly (tagged `yaml:"-"`) rather than a `DecisionFrontmatter` wrapper, and keeps typed struct fields for its six managed keys — `needs_review`, `reviewed`, `reviewed_date`, `status`, `type`, `page_type` — because those fields are the mutation surface `pkg/ops/decision_ack.go` writes. `WriteDecision` copies the preserved map and overlays those six last, so a managed value wins on a name collision and every other key round-trips untouched.

**Storage** (`pkg/storage/`)
- `parseToFrontmatterMap` parses the YAML frontmatter block into `map[string]any`
- `serializeMapAsFrontmatter` marshals the map back to YAML; unknown fields are preserved
- Entity-specific read helpers call `NewXxx(data, meta, content)` constructors
- **Rendering caveat**: a bare YAML date (`review_date: 2026-08-15`) parses to `time.Time` and re-serializes as RFC3339 (`review_date: 2026-08-15T00:00:00Z`). The instant is preserved; only the rendering is normalized. This applies to every entity — `WriteGoal` behaves the same way today.

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
