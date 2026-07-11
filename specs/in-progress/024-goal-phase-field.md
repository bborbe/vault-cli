---
status: prompted
tags:
    - dark-factory
    - spec
approved: "2026-07-11T21:05:22Z"
generating: "2026-07-11T21:05:23Z"
prompted: "2026-07-11T21:12:18Z"
branch: dark-factory/goal-phase-field
---

## Summary

- Goals gain a validated `phase` frontmatter field with the four values `todo`, `planning`, `execution`, `done`.
- The field mirrors the shape of the existing task-side `Phase` type (newtype, canonical constant set, `Available…`, `Validate`) without touching or reusing the task type's storage.
- Setting an invalid phase on a goal fails loudly; a valid phase is written to the goal file's frontmatter and survives read-write cycles.
- Goals that predate this field keep parsing and operating with no error — no file is backfilled.
- This is the data-layer foundation only. The plan-goal / execute-goal gate commands that will consume the phase are explicitly out of scope.

## Problem

Tasks in vault-cli already carry a lifecycle `phase` (`todo → planning → execution → done`) that gate commands read to enforce a plan-before-execute workflow. Goals have no equivalent field, so the "Phase-Gated Goal Flow" workflow has nothing to read or write at the goal level. Before any goal-level gating command can exist, the goal domain type must be able to hold, validate, and surface a phase value — and it must do so without disturbing the millions-of-files-be-damned reality that most existing goal files have no `phase:` line at all.

## Goal

After this work, a goal file can carry a `phase` frontmatter field constrained to `todo` / `planning` / `execution` / `done`. Setting the field through the existing goal field-mutation command validates the value against that enum and persists it; reading the goal through the existing goal show command surfaces the value in both plain and JSON output. Goals with no `phase` field continue to parse, show, and mutate exactly as they do today. The task-side phase type and every task command behave identically to before.

## Non-goals

- Do NOT add plan-goal / execute-goal / any phase-transition or gating command — those are separate direct-markdown work that consumes this field.
- Do NOT add a new `goal update` or `goal status` command. The repo's goal surface is `goal set <name> <key> <value>` (validated write) and `goal show <name>` (surface). Phase rides those existing commands, exactly as task phase rides `task set` / `task show`.
- Do NOT backfill, rewrite, or migrate existing goal files that lack a phase.
- Do NOT modify, extend, or reuse the task-side `TaskPhase` type, its constants, or `NormalizeTaskPhase`.
- Do NOT add alias handling (e.g. an `in_progress` synonym) — the goal phase enum has no legacy values; if a future consumer needs one, that is a separate spec.
- Do NOT invent a "default to todo" read behavior — the task side returns an empty/nil phase for a missing key and the goal side mirrors that; a missing phase is empty, not `todo`.
- Do NOT add or extend goal-specific status/phase mismatch lint rules in this work.

## Desired Behavior

1. A goal-phase enum type exists, mirroring the shape of the task-side phase type: a string newtype, one canonical constant per value (`todo`, `planning`, `execution`, `done`), an `Available…` collection with a `Contains` check, a `String()` method, a `Validate(ctx)` method returning a validation error for any non-canonical value, and a `Ptr()` helper.
2. The goal frontmatter exposes a typed `phase` getter that returns the parsed phase when the key is present and an empty/nil result when the key is absent — no default substitution.
3. Setting the goal `phase` field to a canonical value through the existing goal field-set command writes `phase: <value>` into the goal file's frontmatter and preserves it through a read-write cycle.
4. Setting the goal `phase` field to any non-canonical value through the goal field-set command fails with a non-zero exit and an error message naming the offending phase; the goal file is left unchanged.
5. The existing goal show command surfaces the `phase` value in both plain output and `--output json` when the field is present.
6. A goal file with no `phase` field parses, shows, and accepts unrelated field mutations with no error, and its output contains no phase value.

## Suggested Decomposition

Single-layer (Domain-only) footprint — one prompt suffices. If the generator splits, this ordering holds:

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Goal-phase enum type (`goal_phase.go`) + `Validate` + `DescribeTable` unit test | #1 | #1, #2, #7 | — |
| 2 | Goal frontmatter typed getter + field-set case (`goal_frontmatter.go`) wiring into generic `goal set`/`goal show` | #2–#6 | #3, #4, #5, #6 | 1 |
| 3 | CHANGELOG `## Unreleased` entry + verification pass | — | #8 | 2 |

## Security / Abuse Cases

N/A — the only user input is a phase value validated against a closed 4-value enum before any write; the write target is an existing named goal file resolved through the standard vault path. No path-traversal, injection, or unvalidated-input surface introduced.

## Constraints

- The task-side phase type, its constants, `NormalizeTaskPhase`, and every task command must be byte-for-byte unchanged. Frozen: existing task-phase behavior and its test suite.
- Frontmatter remains map-based (`FrontmatterMap`); unknown keys must continue to survive read-write cycles. That map is the lazy-migration mechanism — no separate migration code.
- Follow the layered pattern in `docs/development-patterns.md` (Domain → Storage → Ops → CLI). The expected footprint is Domain-only (new goal-phase type + goal frontmatter getter/setter/field-case); Storage, Ops, and CLI reuse the existing generic goal set/show wiring and require no new command.
- The existing generic `status/phase mismatch` lint keys off the presence of a `phase:` line and will begin evaluating goals that carry a phase. Legacy goals (no phase) must remain lint-clean, and this work must not add new false-positive lint output for the four canonical goal phases on an otherwise-consistent goal.
- `make precommit` must pass in the repo root.
- A `## Unreleased` (or top-of-file dated) CHANGELOG entry describing the new goal phase field is required.

## Assumptions

- The caller's shorthand `goal update <name> --phase <value>` maps to the repo's actual `goal set <name> phase <value>`; `goal status <name>` maps to `goal show <name>`. The spec is written against the real command surface.
- The four goal phases are a deliberate subset of the seven task phases (goal has no `ai_review` / `human_review` / `in_progress`). This is intended, not an omission.
- Relevant coding guides are available in-container: `go-enum-type-pattern.md`, `go-parse-pattern.md`, `go-cli-guide.md`, `go-testing-guide.md`.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility |
|---------|-------------------|----------|-----------|---------------|
| `goal set <name> phase bogus` | Non-zero exit; error names the invalid phase; file unchanged | Re-run with a canonical value | Command exit code + stderr message | Reversible (no write occurred) |
| Goal file has no `phase` key | Parses and operates normally; phase reads as empty | None needed | `goal show` exits 0 with no phase value | N/A |
| Goal file has a legacy/hand-typed `phase: in_progress` (not in the goal enum) | Reading tolerates the raw value in show output; validation on an explicit re-set rejects it | Set a canonical value | `goal show` displays raw value; `goal set` rejects | Reversible |
| Goal carries `phase: execution` with a terminal status | Existing generic status/phase mismatch lint may flag it; core get/set/show still function | Fix status or phase to a consistent pair | `goal lint` output | Reversible |
| Concurrent `goal set phase` on the same file | Last writer wins (existing whole-file write semantics; no new locking introduced) | Re-read and re-set if clobbered | File content after both writes | Reversible |

## Acceptance Criteria

- [ ] A goal-phase enum type declares canonical constants for `todo`, `planning`, `execution`, `done` and a matching `Available…` collection — evidence: `grep -nE '"todo"|"planning"|"execution"|"done"' pkg/domain/goal_phase.go` returns ≥4 lines.
- [ ] The goal-phase type rejects a non-canonical value via `Validate` and accepts each canonical value — evidence: a `DescribeTable` unit test covering the 4 canonical values plus ≥1 invalid value passes under `go test ./pkg/domain/...` (exit 0).
- [ ] `goal set <name> phase execution` on a real goal file writes `phase: execution` to that file's frontmatter — evidence: `git diff` (or file read) of the goal file shows an added `phase: execution` line.
- [ ] `goal set <name> phase bogus` exits non-zero and leaves the file unchanged — evidence: shell exit code ≠ 0 and stderr contains a message naming the invalid goal phase; `git status` shows the goal file unmodified.
- [ ] `goal show <name> --output json` on a goal whose phase is set includes the phase value — evidence: JSON output contains `"phase":"execution"` (under the fields map).
- [ ] A goal file with no `phase` key runs `goal show` and an unrelated `goal set` cleanly — evidence: both commands exit 0; `goal show --output json` output contains no `phase` value; the round-tripped file still has no `phase:` line.
- [ ] The task-side phase type file is unchanged — evidence: `git diff --stat pkg/domain/task_phase.go` shows no changes.
- [ ] `make precommit` exits 0 in the repo root — evidence: exit code.
- [ ] A CHANGELOG entry for the goal phase field exists under `## Unreleased` (or the current top dated section) — evidence: `grep -n 'goal phase' CHANGELOG.md` returns ≥1 line.

Scenario coverage: NO new scenario. Unit tests (domain enum + frontmatter getter/setter) plus integration-level exercise of the existing goal set/show commands reach every behavior; no real Docker / cluster / external tool is involved.

## Verification

Behavioral check against a temporary goal file, plus the standard build gate:

```
# 1. Build gate
make precommit

# 2. Valid phase flips frontmatter (against a scratch vault/goal)
vault-cli goal set <goal> phase execution
#   -> exit 0; goal file frontmatter now contains: phase: execution

# 3. JSON surfaces the phase
vault-cli goal show <goal> --output json
#   -> output includes "phase":"execution"

# 4. Invalid phase is rejected, file untouched
vault-cli goal set <goal> phase bogus
#   -> non-zero exit; stderr names the invalid goal phase; git status shows file unmodified

# 5. Legacy goal (no phase) runs clean
vault-cli goal show <legacy-goal> --output json
#   -> exit 0; no phase value in output
```

## Do-Nothing Option

If we do nothing, the "Phase-Gated Goal Flow" workflow cannot begin — the gate commands would have no goal-level field to read or write, and would have to either invent an ad-hoc key (diverging from the task-phase convention) or track goal phase outside the vault files. The current state (goals with no phase concept) is acceptable only for as long as goal-level gating is not pursued; this spec is the minimal unblocking step.
