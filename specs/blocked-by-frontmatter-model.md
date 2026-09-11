---
status: draft
created: 2026-09-11
---

## Blocked-by frontmatter model for tasks and goals

## Summary

- Tasks and goals can declare dependencies through a `blocked_by` frontmatter list naming other tasks/goals.
- A task or goal is blocked when any listed blocker is not completed; an empty or missing list means unblocked.
- vault-cli exposes both `blocked_by` (the raw list) and a computed `blocked` boolean in task and goal JSON output, so consumers (vault-ui, slash commands) get the derived block state without re-implementing it.
- Blocked state is derived and orthogonal to `status` — no automatic `hold` flip, no lifecycle coupling.
- The `next-task` slash command stops recommending tasks whose JSON `blocked` flag is true.

## Problem

The vault task system has no first-class dependency model. `blocked_by` is mentioned in the docs only as a trigger for `status: hold`, which conflates scheduling state with dependency state, and vault-cli itself has no typed accessor for it, no JSON emission, and no command that honors it. As a result, `next-task` can recommend work whose dependency is unmet, and downstream consumers that want to render "waiting on X" have to re-parse frontmatter themselves (vault-ui already does, with divergent semantics). The dependency graph that planning naturally produces ("build the library first, then the services that use it") cannot be expressed or enforced.

## Goal

A task or goal file can declare "I cannot start until these are done" as data, vault-cli surfaces that data and its derived blocked state in JSON output, and the `next-task` command respects it — while leaving `status` semantics untouched.

## Non-goals

- Cross-kind blockers (task blocked by a goal, goal blocked by a task) — same-kind `blocked_by` only; a task's blockers resolve in the tasks directory, a goal's in the goals directory.
- Auto-setting `status: hold` from `blocked_by` — block state stays derived and orthogonal to status.
- Migrating existing vault tasks to add `blocked_by` — no bulk rewrite of existing task/goal files.
- A new vault-cli subcommand for dependency resolution — blocked state is a computed list-output field; slash commands consume the JSON.
- Rendering "blocked by" in the UI — separate vault-ui spec (show-don't-hide).

## Acceptance Criteria

- [ ] `vault-cli task list --output json` on a vault containing a task with `blocked_by: [A, B]` in frontmatter emits a `blocked_by` array `["A", "B"]` for that task (stdout match).
- [ ] `vault-cli goal list --output json` emits `blocked_by` and the computed `blocked` field for goals the same way (stdout match).
- [ ] A task with no `blocked_by` field emits neither a `blocked_by` key nor a `blocked` key, and every other JSON field is byte-identical to pre-change output (negative evidence: field-set diff empty).
- [ ] `/vault-cli:next-task` output greps for the blocked task's name return 0 matches while an unblocked alternative is available (negative evidence: grep of command output).
- [ ] `vault-cli task list --output json` on a fixture vault where a task's `blocked_by` names a blocker whose file is missing shows `"blocked": true` for that task, and `"blocked": false` once the blocker file is created with `status: completed` (stdout match, state transition).
- [ ] `go test ./pkg/ops/...` passes with a unit test asserting the blocked-state resolver: blocked when any blocker's status is not `completed`; unblocked when all are completed; a wikilink-form blocker name (`[[A]]`) and a case-mismatched name both resolve to the same file; a missing blocker file counts as blocked (exit code 0).

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint + format + generate + tests + version checks pass
- `make test` — unit suite passes, including accessor and blocked-state resolver tests (empty/missing list, wikilink vs plain names, missing blocker file, all-completed)
- `vault-cli task list --output json` against a fixture vault (created in the test fixture dir) shows `blocked_by` and `blocked` for a blocked task
- `grep -n 'blocked_by' pkg/domain/task_frontmatter.go pkg/domain/goal_frontmatter.go` returns lines ≥1 (smoke check — ACs 1-2 carry the functional proof)

### Operator-executable (runs on the host after merge, spec verification ladder)

- `vault-cli task list --output json` on the Personal vault shows `blocked_by` and `blocked` on a fixture task with the field
- `/vault-cli:next-task` invoked in a session reports no blocked task while an unblocked one exists

## Desired Behavior

1. `TaskFrontmatter` and `GoalFrontmatter` each expose a `BlockedBy()` accessor returning the `blocked_by` frontmatter value as a string slice, mirroring the existing `Goals()` accessor pattern (missing or non-list value → empty slice).
2. Task and goal JSON output includes `blocked_by` as a JSON array of strings when present, and a computed `blocked` boolean (true iff the task/goal is blocked per behavior 4). Additive-only: absent `blocked_by` in frontmatter emits neither key; all existing fields keep their names and values.
3. A blocker name is matched to its entity file case-insensitively, wikilink brackets stripped. Task blockers resolve in the tasks directory, goal blockers in the goals directory (same-kind only).
4. A task/goal is blocked iff at least one listed blocker's entity has `status != completed`. A blocker whose file is missing, unreadable, or has no parseable status counts as not completed (blocked) — the safe default for "cannot verify it is done".
5. The blocked-state determination lives in a Go resolver in `pkg/ops`, unit-tested, and is what computes the `blocked` field on list output.
6. `next-task` (worker and boss mode) filters out tasks whose JSON `blocked` flag is true from its recommendations; it does not reorder or hide unblocked tasks. This **replaces** the command template's existing Step-5 blocker heuristic (content-scanning for `**Blocker:**` / `Blocked by:` / `Prerequisites` patterns) — the typed JSON field is the single source of truth, no duplicate content-scanning semantics.
7. `docs/task-writing.md` and `docs/goal-writing.md` describe `blocked_by` as a dependency list whose derived block state is orthogonal to `status` — replacing the current "hold is triggered by `blocked_by:`" phrasing.

## Assumptions

- The vault directories are readable at list time (vault-cli already reads every task file for `list`, so resolver reads add no new I/O class).
- Blocker names refer to files within the same vault.
- A blocker whose entity cannot be confirmed completed blocks — operators prefer "cannot verify, don't start" over "missing, proceed".

## Constraints

- `blocked_by` frontmatter key is a YAML list of strings (plain names or `[[wikilinks]]`); both forms parse to the same list.
- Additive-only JSON change: existing task/goal output fields keep their names and values; no field is removed or re-typed.
- No changes to `status` / `phase` transitions, `task set`, `task defer`, or `task complete` semantics.
- Follow vault-cli conventions (see `docs/development-patterns.md`): accessor via `FrontmatterMap` getters, Ginkgo/Gomega tests, `pkg/ops` returns structured results (no stdout).

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Blocker renamed or removed after `blocked_by` was set | Task stays blocked (unknown blocker = not completed) | Operator edits `blocked_by` or completes/aborts the blocker; `task set <name> blocked_by ""` clears the list |
| Circular `blocked_by` (A blocked by B, B blocked by A) | Both blocked; no recursion or hang (blocked state is a single-level status read, never a transitive walk) | Operator breaks the cycle by clearing one `blocked_by` |
| Blocker file exists but has unparseable/absent frontmatter | Treated as blocked (cannot verify status) | Operator repairs the blocker file |
| Malformed `blocked_by` (not a YAML list, e.g. a scalar string) | Treated as empty → unblocked; no error | Operator rewrites the field as a list |

## Security / Abuse

- `blocked_by` values are plain names used only to locate files within the vault directories — no shell interpolation, no path traversal outside the configured vault dirs (matching existing entity-name resolution).
- `next-task` and the resolver read entity files read-only; no writes are performed as part of blocked-state computation.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `BlockedBy()` accessor (task + goal) + blocked-state resolver in `pkg/ops` + JSON emission of `blocked_by`/`blocked` on task+goal list, unit tests, fixture vault | 1, 2, 3, 4, 5 | 1, 2, 3, 5, 6 | — |
| 2 | `next-task` filters tasks whose JSON `blocked` is true (worker + boss mode) | 6 | 4 | prompt 1 |
| 3 | Docs rewrite: `task-writing.md` + `goal-writing.md` blocked_by semantics | 7 | — | — |

Rationale: prompt 1 establishes the field contract and the computed blocked state; prompt 2 consumes the JSON flag in the command; prompt 3 is doc-only and independent once the semantics are fixed.

## Do-Nothing Option

vault-cli keeps ignoring `blocked_by`: `next-task` can recommend work whose dependency is unmet, consumers keep divergent ad-hoc parsing, and the dependency graph stays invisible in the CLI. The cost grows as more consumers (vault-ui, slash commands) each re-implement blocked-state with their own semantics.
