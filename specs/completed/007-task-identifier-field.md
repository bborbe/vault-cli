---
status: completed
approved: "2026-03-27T18:29:12Z"
prompted: "2026-03-27T18:33:51Z"
verifying: "2026-03-27T21:20:45Z"
completed: "2026-03-27T21:21:15Z"
branch: dark-factory/task-identifier-field
---

## Summary

- Task objects carry a `task_identifier` field that round-trips through frontmatter
- Every task-writing operation auto-generates a UUIDv4 when `task_identifier` is missing
- Validation reports missing `task_identifier` as an error
- Frontmatter get/set/clear for `task_identifier` works automatically (no new CLI code needed)
- External consumers (e.g. automation agents) can use vault-cli's domain type instead of duplicating frontmatter parsing

## Problem

External tools that need stable task identity (surviving file renames/moves) currently duplicate vault-cli functionality: manual frontmatter extraction via string splitting, manual UUID injection via string manipulation, and manual file I/O. The `domain.Task` struct lacks `task_identifier`, so consumers maintain parallel structs and custom injection code. This duplication means multiple codebases parse the same frontmatter format differently.

## Goal

After this work, `domain.Task` carries `TaskIdentifier`, validation errors on missing identifiers, and every task-writing operation auto-generates UUIDs. External consumers can use vault-cli's domain type directly instead of duplicating frontmatter handling.

## Non-goals

- Updating external consumers to use the new field (separate repos, separate follow-up)
- Adding a CLI subcommand for task_identifier (it's a library operation consumed by agent, not a user-facing command)
- Validating UUID format on read (accept any non-empty string)
- Deduplication of task_identifiers across files (agent's responsibility)

## Assumptions

- Generic frontmatter get/set/clear infrastructure auto-discovers new struct fields — adding `task_identifier` to the domain struct is sufficient for CLI get/set/clear to work
- Adding `github.com/google/uuid` as a dependency is acceptable (crypto/rand-backed UUIDv4)
- Task storage is file-based with YAML frontmatter; no database or remote storage involved

## Desired Behavior

1. Task objects carry a `task_identifier` field. Reading a task file with `task_identifier: abc-123` in frontmatter populates this field. Writing a task preserves it.

2. Every operation that writes a task (set, clear, complete, defer, etc.) auto-generates a UUIDv4 for `task_identifier` if it is missing before writing. This ensures identifiers are populated organically as tasks are touched.

3. Validation reports an error when a task has no `task_identifier`. This surfaces tasks that haven't been touched by any write operation yet.

4. The existing generic get/set/clear infrastructure automatically discovers `task_identifier` — no new CLI subcommands needed. These work out of the box:
   - `vault-cli task get "T" task_identifier` returns the stored identifier (empty if unset)
   - `vault-cli task set "T" task_identifier some-uuid` sets the field
   - `vault-cli task clear "T" task_identifier` removes the field from frontmatter

5. All tasks in a vault can be backfilled with identifiers in a single operation. Tasks missing `task_identifier` get a generated UUID written back. Unparseable task files are skipped with a warning. The caller receives the list of modified files (for batch-commit).

## Constraints

- Existing tests must pass — adding the field must not break existing serialization
- Tasks without `task_identifier` must NOT get an empty field written (use `omitempty` YAML tag)
- Auto-generation must be idempotent — once set, subsequent writes preserve the existing identifier
- Auto-generation happens at the write layer, not the read layer — reading a task without `task_identifier` returns empty, not a generated UUID
- The operation layer returns structured results, never writes to stdout (per CLAUDE.md design decision)
- No new CLI subcommands needed — get/set/clear work via existing generic infrastructure
- Field name must follow existing snake_case convention (`page_type`, `planned_date`, `defer_date`)
- UUID generation must be crypto/rand-backed (same quality as agent uses today)
- Empty `task_identifier` must not appear in written frontmatter

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Task file has no frontmatter | `EnsureTaskIdentifier` returns error (existing parse error path) | Caller skips file |
| Task file is read-only | `WriteTask` returns OS permission error | Caller handles error |
| `EnsureAllTaskIdentifiers` encounters one bad file | Logs warning, continues with remaining files | Returns partial results + errors |
| Disk full during write-back | Write fails atomically (write to temp file, then rename) | No partial/truncated files; caller receives error |
| Concurrent writes to same file (batch + external edit) | Last writer wins (no file locking) | Acceptable — CLI is single-user, not a server |

## Acceptance Criteria

- [ ] Task domain type has `task_identifier` field with omitempty behavior
- [ ] Reading a task with `task_identifier: uuid` in frontmatter populates the field
- [ ] Writing a task with `task_identifier` set preserves it in frontmatter
- [ ] Any task-writing operation auto-generates `task_identifier` if missing before writing
- [ ] Auto-generation preserves existing `task_identifier` (never overwrites)
- [ ] Validation errors when `task_identifier` is missing
- [ ] Generic get/set/clear works for `task_identifier` (no new CLI code)
- [ ] Batch backfill processes all tasks, skips errors, returns modified file list
- [ ] All existing tests pass
- [ ] `make precommit` passes

## Verification

```
make precommit
```

## Security / Abuse

Not applicable — CLI-only tool operating on local files, no network input. UUID generation uses crypto/rand-backed `github.com/google/uuid`.

## Do-Nothing Option

External consumers continue to duplicate vault-cli frontmatter parsing with custom string manipulation. Multiple codebases parse the same file format differently, increasing risk of divergence. Any change to frontmatter format requires updates in multiple places.
