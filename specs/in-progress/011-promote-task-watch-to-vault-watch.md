---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-05-10T20:51:12Z"
generating: "2026-05-10T20:54:26Z"
prompted: "2026-05-10T21:01:16Z"
verifying: "2026-05-10T22:15:19Z"
branch: dark-factory/promote-task-watch-to-vault-watch
---

## Summary

- Promote today's wide-scope `vault-cli task watch` (which already watches tasks, goals, themes, and objectives) to a properly-named top-level `vault-cli watch` command.
- Add a `type` field to every emitted JSON event identifying the entity kind (`task`, `goal`, `theme`, `objective`), derived from the file's parent directory.
- Add an optional `--types` filter on the new command for consumers that only care about a subset of entity kinds.
- Keep `vault-cli task watch` working unchanged (same wide scope, same output shape plus the new `type` field) but emit a one-time stderr deprecation warning pointing at `vault-cli watch`. Removal is a future spec.
- Driven by [[Eliminate Agent Task Rot]]: the immediate consumer is task-orchestrator's goal-cleanup-resolution loop replacement, which needs to subscribe to goal events alongside task events through a stable, properly-named interface.

## Problem

`vault-cli task watch` is misnamed. Its name implies it only watches tasks, but it actually watches tasks, goals, themes, and objectives in the same fsnotify loop. Consumers that read its output cannot tell which entity kind a given event belongs to without re-deriving it from the file path on their side. New consumers (notably task-orchestrator's goal-cleanup loop replacement) need to subscribe to goal events, but discovering that the existing `task watch` command silently emits them too is non-obvious and brittle to rely on. There is no top-level `watch` command surfacing this capability honestly.

## Goal

After this work:

- `vault-cli watch` is the canonical streaming-event command, documented in `--help`, with the same wide entity scope today's `task watch` has.
- Every event on stdout (from either command) carries a `type` field identifying the entity kind, derived from the file's parent directory at event time.
- Consumers can filter at the source via `--types task,goal` instead of parsing-then-discarding events they don't want.
- `vault-cli task watch` continues to function for current consumers but signals (on stderr) that it is deprecated in favor of `vault-cli watch`.

## Non-goals

- Removing `vault-cli task watch` entirely — deprecation only; removal is a future spec after consumers migrate.
- Changing what `vault-cli task watch` watches (it stays wide-scope so existing consumers don't regress).
- Adding new entity kinds beyond tasks, goals, themes, objectives.
- Filtering by event subtype (`created`/`modified`/`deleted`/`renamed`) — defer to consumer-side filtering.
- Reading frontmatter or file content to determine `type` — derivation is path-based only.
- Auto-restart, multi-process supervision, or any change to the existing single-fsnotify-watcher architecture.
- Changes to task-orchestrator's `vault_cli_watcher.py` — that migration is a separate prompt against the task-orchestrator repo after this ships.

## Desired Behavior

1. A new top-level subcommand `vault-cli watch` exists. Running it streams newline-delimited JSON events to stdout, one per debounced file change in the configured vault(s).
2. Its scope covers the same directories today's `task watch` covers: tasks, goals, themes, and objectives directories of every selected vault. (Directory names are vault-configurable; the new command consumes the same `Get*Dir()` accessors.)
3. Every emitted event includes a `type` field whose value is one of `task`, `goal`, `theme`, `objective`, determined by which configured directory the changed file lives in.
4. The pre-existing event keys (`event`, `name`, `vault`, `path`) remain present with unchanged shape and meaning. The new `type` key is added without renaming or removing any existing key.
5. A `--types` flag on `vault-cli watch` accepts a comma-separated list (e.g. `--types task,goal`). When set, only events whose derived type is in the list are emitted. When omitted, all four types are emitted. Unknown values in the list are rejected at startup with a clear error.
6. `vault-cli task watch` remains available with identical scope and output (now also carrying the `type` field). On startup it writes one deprecation warning to stderr naming `vault-cli watch` as the replacement. Stdout JSON output is unaffected by the warning.
7. `vault-cli watch --vault <name>` honors the standard vault-selection semantics already used by `task watch` (single vault when set, all configured vaults when omitted).
8. `vault-cli watch --help` describes the command, the wide scope, the `type` field meanings, and the `--types` filter with valid values.

## Assumptions

- Each entity kind has a single configured directory per vault (no overlap, no nesting of one kind inside another). The watcher already relies on this.
- Directory-to-type mapping is established at watch startup from the same `WatchTarget` already used by today's implementation; per-event derivation is a lookup, not a string-match against folder names. This keeps the mapping correct under custom vault configs (e.g. `21 Themes`, `22 Objectives`, `Custom Goals`).
- Existing consumers parse events as JSON objects by field name (not positionally). The one known consumer (`task-orchestrator/src/task_orchestrator/vault_cli_watcher.py`) reads `event`, `name`, `vault` by key — adding `type` is non-breaking.

## Constraints

- The watch loop, fsnotify wiring, debouncer, and error handling stay in `pkg/ops/watch.go`. The new CLI command in `pkg/cli/cli.go` is argv parsing → call into ops, mirroring the existing `task watch` wiring.
- `pkg/ops/` is a library layer — operations return structured results, never write to stdout directly. CLI layer owns all stdout formatting (per project rule in `CLAUDE.md`).
- The `WatchEvent` struct's existing JSON keys (`event`, `name`, `vault`, `path`) are preserved unchanged; `type` is added as an additional key.
- `type` MUST be derived from the file's parent directory via the watch-target → kind map built at startup. Frontmatter is never read for typing.
- The dir → kind mapping is plumbed from CLI (which knows the kind of each directory it asks the watcher to register) into ops. The implementation MUST extend the `WatchTarget` shape so each watched directory carries its kind alongside its path — either as `WatchDirs []struct{Dir, Kind string}` or via a parallel `Kinds []string` slice keyed by `WatchDirs` index. CLI is the source of truth for the mapping; ops never infers kind from folder-name string-matching.
- Both `vault-cli watch` and `vault-cli task watch` share one underlying ops implementation — no duplicated watch loops.
- The deprecation warning on `task watch` is emitted exactly once per process, on stderr, before any events are emitted. It must not be machine-mistaken for an event (stdout-only consumers ignore stderr already).
- All existing `task watch` tests continue to pass without modification (they assert on the existing four fields; adding `type` does not invalidate them).
- Per project rule (`CLAUDE.md` "Scenario-skip rule"), no new scenario is added — the behavior is reachable from unit + integration tests in `pkg/ops/` and `pkg/cli/`.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| `--types` contains an unknown value (e.g. `--types task,foo`) | Exit non-zero before starting the watcher with an error listing valid values | User corrects the flag |
| `--types` is empty string | Exit non-zero with usage error | User omits the flag or supplies values |
| File event arrives for a directory not in the watch-target → kind map | Event is ignored (defensive — should not happen given watcher only registers known dirs) | None needed |
| Configured entity directory does not exist on disk | Skipped at startup with a debug log (existing behavior); other directories still watched. `--types` filter referencing the missing kind is not an error — that kind simply produces no events. | User creates the directory or ignores |
| `vault-cli task watch` invoked | Deprecation warning to stderr, then runs identically to today plus the new `type` field on every event | User migrates to `vault-cli watch` at their convenience |

## Security / Abuse Cases

- The watcher only registers directories returned by the vault config's `Get*Dir()` accessors joined under the configured vault path. No user-supplied path enters the watch set, so path traversal is not introduced by this change.
- `--types` values are validated against a closed enum before any I/O — no injection surface.
- Stderr deprecation warning content is static; no user input flows into it.

## Acceptance Criteria

- [ ] `vault-cli watch --vault <name>` runs without error and streams JSON events on stdout for changes in tasks, goals, themes, and objectives directories.
- [ ] `vault-cli watch --help` documents the command, lists the four entity types, and documents `--types`.
- [ ] Editing a task file emits one JSON line containing `"type":"task"`.
- [ ] Editing a goal file emits one JSON line containing `"type":"goal"`.
- [ ] Editing a theme file emits one JSON line containing `"type":"theme"`.
- [ ] Editing an objective file emits one JSON line containing `"type":"objective"`.
- [ ] `vault-cli watch --types goal` emits goal events and suppresses task/theme/objective events.
- [ ] `vault-cli watch --types task,goal` emits both task and goal events and suppresses theme/objective events.
- [ ] `vault-cli watch --types unknown` exits non-zero with an error naming the valid values.
- [ ] `vault-cli watch` (no `--types`) emits all four kinds.
- [ ] `vault-cli task watch` continues to emit events for tasks, goals, themes, and objectives (unchanged scope).
- [ ] `vault-cli task watch` writes one stderr deprecation line on startup naming `vault-cli watch` as the replacement; stdout output is unaffected.
- [ ] `vault-cli task watch` events also include the `type` field.
- [ ] The `WatchEvent` struct's existing JSON keys (`event`, `name`, `vault`, `path`) are present and unchanged in shape; `type` appears as an additional key.
- [ ] All pre-existing `pkg/ops/watch*` and `pkg/cli` tests covering `task watch` pass without modification.
- [ ] `make precommit` passes.

## Verification

```
make precommit
```

Manual smoke test (release-gate equivalent, against `/tmp/new-vault-cli` per `CLAUDE.md`):

```
/tmp/new-vault-cli watch --vault personal --types task,goal &
# touch a task file → expect "type":"task"
# touch a goal file → expect "type":"goal"
# touch a theme file → expect no event
```

## Do-Nothing Option

Leave `vault-cli task watch` as the only entry point. Consumers continue to depend on the misnamed command and must derive the entity kind themselves from `path` by string-matching configured directory names — fragile under vault rename, and undocumented as a supported pattern. New consumers (task-orchestrator's goal-cleanup loop) discover the wide scope only by reading source code. Acceptable short-term but blocks a clean public API for the cross-repo subscription pattern.

## Verification Result

**Verified:** 2026-05-14T14:57:33Z (HEAD 0e930a2)
**Binary:** /Users/bborbe/Documents/workspaces/go/bin/vault-cli (15504642 bytes, version dev)
**Scenario:** Live `vault-cli watch` + `task watch` against `/tmp/spec011-vault` (Tasks/Goals/21 Themes/22 Objectives); --types filter and error cases exercised against the fresh binary.
**Evidence:**
- `vault-cli watch --vault smoke` stdout: `{"event":"created","name":"Alpha",...,"type":"task"}` + `goal/Beta` + `theme/Gamma` + `objective/Delta` (all four kinds, `type` populated from dir→kind map)
- `vault-cli watch --types goal`: only `{"...","name":"B","type":"goal"}`; task/theme/objective suppressed
- `vault-cli watch --types task,goal`: only `A2(task)` + `B2(goal)`; theme/objective suppressed
- `vault-cli watch --types unknown` → stderr `Error: unknown type "unknown" in --types; valid values: task, goal, theme, objective`, EXIT=1
- `vault-cli watch --types ""` → `Error: --types requires at least one value...`, EXIT=1
- `vault-cli task watch` stderr (1 line): `DEPRECATED: 'vault-cli task watch' is deprecated; use 'vault-cli watch' instead. See spec 011.`; stdout: 4 clean JSON events covering all four kinds, each with `type` field — no deprecation text on stdout
- `vault-cli watch --help` long description lists `type` field meanings and `--types` with valid values `task, goal, theme, objective`
- `go test ./pkg/ops/... ./pkg/cli/...` → both packages `ok` (includes pre-existing `task watch` tests + new `vault-cli watch --types` and deprecation tests in `pkg/cli/watch_test.go`)
- `make precommit` → `ready to commit` (gosec 0 issues, trivy clean, addlicense clean)
**Verdict:** PASS
