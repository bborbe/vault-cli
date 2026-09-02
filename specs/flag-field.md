---
status: draft
created: "2026-09-02T21:55:00+02:00"
---

## Summary

- Task objects carry a boolean `flag` field that round-trips through frontmatter
- `vault-cli task set "T" flag true|false|yes|no` writes it; invalid values error
- `vault-cli task clear "T" flag` removes it; `vault-cli task get "T" flag` reads it
- Un-flagged tasks carry no `flag:` key (round-trip is omitempty-clean)
- The Vault UI board can then sort flagged cards to the top (separate repo, direct flow)

## Problem

The Vault UI board orders cards by `due_date`, `planned_date` and `priority`. There is no way to mark a task as "picked for today" without corrupting one of those fields — `planned_date: today` abuses a sort field to mean "focus", and `priority: 1` collides with genuine urgency. The daily boot therefore has no frontmatter channel to express a focus pick, and the board cannot render one.

## Goal

A task can carry a boolean `flag` in frontmatter, writable, readable and clearable through the vault-cli typed surface with validation — the field the Vault UI will later sort to the top. The flag is independent of dates and priority.

## Non-goals

- Rendering or sorting the flag in the Vault UI — separate repo, direct flow, tracked in the same task
- Multiple flag colours / categories — one boolean flag
- Flagging goals, themes or objectives — task frontmatter only
- Deciding which tasks get flagged each morning — that is the boot's job, not this CLI's
- A dated `flagged_on` value — deliberately rejected: boolean only (see Assumptions)

## Assumptions

- `FrontmatterMap.GetBool` already coerces YAML bools and the strings `true`/`yes`/`false`/`no` — the read path needs no new coercion
- `task set`/`clear`/`get` generic infrastructure auto-discovers known keys through the `SetField` dispatch — adding `flag` to the typed surface is sufficient for CLI operation
- Boolean (not dated) shape is the agreed design — the task's Out of Scope records "one boolean flag first"

## Acceptance Criteria

- [ ] **AC1 — set writes the field:** `vault-cli task set "T" flag true` on a real task, then `grep -n '^flag: true' tasks/T.md` returns ≥ 1 line, and no other frontmatter key in that file changed. Same observable for the coerced inputs: `set flag yes` writes `^flag: true`, `set flag FALSE` writes `^flag: false`.
  - Evidence shape: vault/file artifact — frontmatter line present; `git diff tasks/T.md` shows only the `flag` line added
- [ ] **AC2 — invalid value errors:** `vault-cli task set "T" flag banana` exits non-zero and stderr contains an error naming the value and the accepted set (`true`/`yes`/`false`/`no`).
  - Evidence shape: exit code + stderr match
- [ ] **AC3 — get reads it:** after `set flag true`, `vault-cli task get "T" flag` prints `true`; after `set flag TRUE` (case-insensitive), it also prints `true`; after `set flag false`, it prints `false`; on a task never flagged, it prints empty.
  - Evidence shape: stdout match
- [ ] **AC4 — clear removes it:** `vault-cli task clear "T" flag` then `grep -c '^flag:' tasks/T.md` returns 0 and `vault-cli task get "T" flag` prints empty.
  - Evidence shape: file absence + stdout match
- [ ] **AC5 — round-trip preserves other fields:** set `flag true` on a task that already carries `status`, `priority`, `planned_date`, `themes` and an unknown custom key; after the write, every pre-existing key is byte-identical (`git diff tasks/T.md` contains only the added `flag` line).
  - Evidence shape: negative — diff of pre-existing keys empty
- [ ] **AC6 — suite green:** `make precommit` exits 0, including new unit tests for the accessor, the validated setter, the clear and the omitempty round-trip.
  - Evidence shape: exit code

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — shellcheck / lint / typecheck clean
- `make test` — unit + integration suite passes, including the new flag-field tests
- `grep -n 'func (f \*TaskFrontmatter) SetFlag' pkg/domain/task_frontmatter.go` — accessor landed
- `grep -n 'setFlagField' pkg/domain/task_frontmatter.go` — validated setter wired into the dispatch

No operator-executable rung: this is a pure code change shipped by a merged PR; verification is container-executable.

## Desired Behavior

1. A task file with `flag: true` in frontmatter reads back as `Flag() == true`; with `flag: false`, `yes`, `no` or absent, `Flag() == false`. Absent and false are distinct at the YAML level (absent = no key, false = `flag: false`) but both read as false.

2. `vault-cli task set "T" flag <value>` accepts `true`, `yes`, `false`, `no` (case-insensitive, matching `GetBool` coercion) and writes the canonical `flag: true` / `flag: false` form. Any other value exits non-zero with an error naming the accepted set. No other frontmatter key is touched.

3. `vault-cli task clear "T" flag` removes the `flag` key entirely; the task reads back as un-flagged.

4. `vault-cli task get "T" flag` prints the current value: `true` / `false` when set, empty when absent.

5. Writing a task never emits a `flag` key that was not explicitly set — the field round-trips with the same omitempty semantics as the rest of the map; a task that was never flagged gains no `flag:` line on any write.

6. The flag is orthogonal to `status`, `phase`, `priority`, `planned_date`, `due_date` and `defer_date` — setting it changes none of them and none of them changes it.

## Constraints

- New typed accessors: `Flag() bool` (GetBool-backed), `SetFlag(ctx, bool) error` (validated, mirroring the `SetPriority` pattern at `task_frontmatter.go:295`), `ClearFlag()`
- `SetField` dispatch gains a `flag` case routed to a validated setter (`setFlagField`), so the generic `task set` path validates instead of passing through as a free-form key
- `task get` and `task clear` work on `flag` through the existing generic infrastructure
- Missing `flag` must not cause an empty/false key to be written — keep the omitempty round-trip invariant
- Existing tests must pass; the change must not break serialization of any existing field
- Per `docs/development-patterns.md`: operations inject the storage interface, no direct file I/O in ops, `libtime` for dates, counterfeiter mocks, no `encoding/json` in command files

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| User passes `flag banana` | Non-zero exit, stderr names the accepted set, no write occurs | Operator re-runs with a valid value |
| Task file already has `flag: banana` from an external writer | `Flag()` reads false (GetBool coercion), set/clear operate normally, write preserves the raw value until explicitly changed | Operator runs `set flag true` / `clear flag` to fix |
| Concurrent write while setting flag | Same file-write race as every other field — no new hazard, existing storage semantics apply | Re-run the set |

## Security / Abuse

Not applicable — CLI-only tool operating on local files, no network input; `flag` input is restricted to a validated enum (AC2).

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `flag` typed accessor + validated setter + dispatch + tests | 1-6 | 1-6 | — |

Rationale: single-layer change (domain frontmatter + its tests); one prompt covers it. The Vault UI rendering half is a separate repo on the direct flow and is explicitly out of this spec's scope.

## Do-Nothing Option

The `flag` field never exists, so the daily boot keeps having no frontmatter channel for a focus pick. The workaround is corrupting `planned_date` to mean focus, which breaks the board's urgency sort and silently re-introduces the staleness the flag was designed to remove. The whole [[Add a Flag Field to Tasks and Sort Flagged Cards to the Top of Vault UI Columns]] task (and the [[Rethink Start-Day to Drive the Vault UI Board Instead of the Daily Note]] morning pass that depends on it) stays blocked.
