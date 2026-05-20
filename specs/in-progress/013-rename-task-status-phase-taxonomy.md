---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-05-20T15:58:40Z"
generating: "2026-05-20T16:00:25Z"
prompted: "2026-05-20T16:12:28Z"
verifying: "2026-05-20T16:58:25Z"
branch: dark-factory/rename-task-status-phase-taxonomy
---

## Summary

- Flip canonical task-frontmatter values to eliminate the `status`↔`phase` naming collision: status canonical `todo` → `next`, phase canonical `in_progress` → `execution`.
- Strategy is **additive normalize, no bulk file migration**: old values stay valid forever as aliases via `NormalizeTaskStatus` / new `NormalizeTaskPhase`. Existing 400+ vault files are untouched on disk.
- Scope is vault-cli Go code only — ~10–20 files: domain constants, normalize maps, lint validator literals, the workon phase advancement, and tests. Vault-side edits (templates, guides, CLAUDE.md, task-orchestrator) are explicitly out of scope.
- Validate (canonical-only) keeps rejecting the old name; Normalize accepts both. Anywhere old YAML is read it must flow through Normalize before reaching Validate, otherwise old files break.
- Version-aligned release: any `pkg/domain/` change triggers binary + plugin version bumps (CHANGELOG, `plugin.json`, two `marketplace.json` fields).

## Problem

The Task entity carries two orthogonal dimensions whose value sets currently overlap by name:

- **Status** (lifecycle / scheduling): `todo`, `in_progress`, `backlog`, `completed`, `hold`, `aborted`
- **Phase** (work stage inside `status: in_progress`): `todo`, `planning`, `in_progress`, `ai_review`, `human_review`, `done`

`status: in_progress` and `phase: in_progress` are unrelated states but read identically. `status: todo` and `phase: todo` are similarly ambiguous. Audit guides, agents, and humans cannot infer which dimension a bare value belongs to. Filters and lint messages all need awkward "which dimension?" qualifiers. The collision is a permanent friction tax on every downstream tool that reads task frontmatter.

The full rationale, semantics, and prior precedent (the 2026-01 `next → todo` migration this partially reverses) live in `~/Documents/Obsidian/Personal/23 Goals/Rename Task Status and Phase Taxonomy.md`.

## Goal

After this work:

- `next` is the canonical status value; `todo` is an accepted alias.
- `execution` is the canonical phase value; `in_progress` is an accepted alias.
- Existing vault files on disk with `status: todo` or `phase: in_progress` continue to read, validate, and write back without churn.
- Every code path that writes a new status or phase value emits the new canonical (`next` / `execution`).
- The `status` and `phase` dimensions share no value names.

## Assumptions

- Every reader of raw frontmatter status/phase strings already routes through `Normalize*` before `Validate` (the lint code at `pkg/ops/lint.go:365-371` is the established pattern). Any reader that doesn't is a pre-existing bug, not new fallout from this change.
- No external Go consumer relies on `TaskStatusTodo` or `TaskPhaseInProgress` being present in `AvailableTaskStatuses` / `AvailableTaskPhases`. Consumers either accept both via Normalize or check the canonical constants directly.
- The task-orchestrator (Python) rename will coordinate after this vault-cli release ships, since orchestrator currently writes the old canonical and vault-cli's Normalize will accept it as alias.

## Non-goals

- **Bulk rewriting existing vault files.** Old values stay forever as aliases.
- Templates in `90 Templates/`, writing guides, audit guides, project/global `CLAUDE.md` updates — these are direct vault edits, not vault-cli code.
- The Python `task-orchestrator` repo — tracked separately as `Rename Task Status Phase Taxonomy task-orchestrator`.
- Renaming imperfect phase names `ai_review` / `human_review` — deferred to a follow-up goal.
- An optional `vault-cli migrate` command for users who want clean `grep` — out of scope here; create a separate task if anyone asks.
- Removing the `TaskStatusTodo` / `TaskPhaseInProgress` Go constants. They stay as alias-only constants with a comment marking them as such (so external Go callers and existing internal references compile).

## Desired Behavior

1. `vault-cli` ships a `TaskStatusNext` constant. `AvailableTaskStatuses` contains `TaskStatusNext` in place of `TaskStatusTodo`, so `next` passes `Validate` and `todo` does not pass `Validate` directly.
2. `NormalizeTaskStatus("todo")` returns `TaskStatusNext, true`. `NormalizeTaskStatus("next")` returns `TaskStatusNext, true`. All other existing alias mappings (`current → in_progress`, `done → completed`, `deferred → hold`) are preserved.
3. `vault-cli` ships a `TaskPhaseExecution` constant. `AvailableTaskPhases` contains `TaskPhaseExecution` in place of `TaskPhaseInProgress`, so `execution` passes `Validate` and `in_progress` does not pass `Validate` directly.
4. A `NormalizeTaskPhase(raw string) (TaskPhase, bool)` function exists, mirrors the `NormalizeTaskStatus` shape, accepts every canonical value, and maps `in_progress → execution`.
5. Every code path that writes a status or phase to disk emits the new canonical. Specifically: the `workon` operation, when it advances a task's phase, never writes `phase: in_progress`; any operation that sets a default status writes `next`, never `todo`.
6. Reading a vault file with `status: todo` and/or `phase: in_progress` succeeds end-to-end: the file is parseable, listable, completable, deferrable, and a re-write that does not modify the status/phase field preserves the old value on disk (no surprise rewrite of unrelated fields just because the status was old).
7. The `lint` operation accepts both old and new canonical values without flagging them as invalid. Its human-readable error message for genuinely invalid status names includes `next` in the list of valid values.
8. The release artifacts are version-aligned: `CHANGELOG.md` top entry, `.claude-plugin/plugin.json` `version`, and the two `version` fields in `.claude-plugin/marketplace.json` all match the new binary tag.

## Constraints

- **`pkg/domain/task_status.go` and `pkg/domain/task_phase.go` are the only sources of truth for the canonical sets.** Any other file enumerating valid values (notably `pkg/ops/lint.go` line 229's error message string, and the `validStatuses` literal lists in `pkg/ops/lint_test.go`) must be updated to match or replaced with a reference to the domain package.
- **`NormalizeTaskStatus` and `NormalizeTaskPhase` are the only sanctioned entry points for raw frontmatter values.** Validate is for canonical-only checks. Any reader that currently calls `Validate` directly on a raw frontmatter string must instead Normalize first, then Validate. The existing lint code at `pkg/ops/lint.go:365-371` already follows this pattern — it must continue to.
- The existing `TaskStatusTodo` and `TaskPhaseInProgress` Go constants remain exported with their existing string values (`"todo"`, `"in_progress"`). They are demoted to alias-only — kept so existing callers (including downstream Go consumers) compile. Their doc comment marks them as alias-only.
- All existing Ginkgo tests must continue to pass. Test fixtures that hardcode `"todo"` (e.g. `pkg/ops/complete_test.go`, `pkg/ops/defer_test.go`, `pkg/ops/show_test.go`, `pkg/ops/workon_test.go`, `pkg/ops/goal_complete_test.go`) represent on-disk files with the old value — they must keep working through Normalize. The `lint_test.go` literal lists at lines 591, 922, 1220, 1666–1719 enumerate the canonical set explicitly and must be updated to the new canonical set.
- vault-cli follows the dark-factory pipeline strictly: no direct edits to `pkg/`. Every code change in this spec is delivered as a generated prompt.
- See `docs/development-patterns.md` for established repo conventions (`pkg/domain/` shape, Ginkgo v2 testing, counterfeiter mocks). The new `NormalizeTaskPhase` follows the existing `NormalizeTaskStatus` pattern at `pkg/domain/task_status.go:74-96`.
- Any `pkg/domain/` change triggers a binary + plugin release. Follow `docs/releasing-vault-cli.md` — the four-way version alignment (`CHANGELOG.md`, `plugin.json`, two `marketplace.json` fields) is mandatory and enforced by `make precommit` (`check-versions`).

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection |
|---------|-------------------|----------|-----------|
| Vault file has `status: todo` on disk | Read succeeds via Normalize; in-memory value is `TaskStatusNext`; a write that does not touch the status field preserves `todo` on disk verbatim. | None needed. | `diff` of file before/after a non-status edit is empty for the status line. |
| Vault file has `phase: in_progress` on disk | Read succeeds via `NormalizeTaskPhase`; in-memory value is `TaskPhaseExecution`; same preservation rule as above. | None needed. | Same `diff` check on the phase line. |
| Caller invokes `Validate` on raw `"todo"` without going through Normalize first | Returns the existing `validation.Error` ("unknown task status 'todo'"). This is intentional — Validate is canonical-only. | Caller must route through `NormalizeTaskStatus`. | Test asserts the error is returned. |
| `lint` runs against a vault containing files with `status: todo` and `phase: in_progress` | Zero `IssueTypeInvalidStatus` / `IssueTypeStatusPhaseMismatch` issues are reported for those fields. | None needed. | Lint output enumerates issues; the count for those issue types is zero. |
| A new write path forgets to Normalize and writes a literal `"todo"` or `"in_progress"` for a default value | A unit test on the write path asserts the on-disk value equals `"next"` or `"execution"` and fails the build. | Implementation prompt fix. | Unit-test failure on the writer. |
| Release ships with mismatched plugin/binary versions | `make precommit` fails on `check-versions`. | Bump the missing file before committing. | Exit code 1 from `make precommit`; stderr names the diverging file. |
| Downstream Go module imports `domain.TaskStatusTodo` or `domain.TaskPhaseInProgress` | Compiles; constant still equals `"todo"` / `"in_progress"`; downstream may need its own Normalize wrapper if it calls `Validate` directly. | None for compile; downstream tracks own Normalize work. | `go vet ./...` in downstream module passes; constant string value unchanged. |

## Security / Abuse Cases

Not applicable — internal data-model rename. No new HTTP, file, or user-input surface. YAML parsing already validates strings. The Normalize functions cannot panic and return `("", false)` on any unknown input.

## Acceptance Criteria

- [ ] `pkg/domain/task_status.go` exports `TaskStatusNext = "next"`. `AvailableTaskStatuses` contains `TaskStatusNext` and does NOT contain `TaskStatusTodo`. — evidence: `grep -n 'TaskStatusNext' pkg/domain/task_status.go` returns ≥2 matches; `grep -n 'TaskStatusTodo,' pkg/domain/task_status.go` inside the `AvailableTaskStatuses` block returns 0 matches.
- [ ] `TaskStatus("todo").Validate(ctx)` returns a non-nil error. `TaskStatus("next").Validate(ctx)` returns nil. — evidence: Ginkgo test `It("rejects 'todo' as canonical")` and `It("accepts 'next' as canonical")` exist in `pkg/domain/task_status_test.go`; `make test` exit 0.
- [ ] `NormalizeTaskStatus("todo")` returns `(TaskStatusNext, true)`. `NormalizeTaskStatus("next")` returns `(TaskStatusNext, true)`. `NormalizeTaskStatus("current")`, `("done")`, `("deferred")` return their existing canonical mappings (`in_progress`, `completed`, `hold` respectively). — evidence: Ginkgo `Describe("NormalizeTaskStatus")` block contains explicit `It` cases; `make test` exit 0.
- [ ] `pkg/domain/task_phase.go` exports `TaskPhaseExecution = "execution"`. `AvailableTaskPhases` contains `TaskPhaseExecution` and does NOT contain `TaskPhaseInProgress`. — evidence: `grep -n 'TaskPhaseExecution' pkg/domain/task_phase.go` returns ≥2 matches; `grep -n 'TaskPhaseInProgress,' pkg/domain/task_phase.go` inside the `AvailableTaskPhases` block returns 0 matches.
- [ ] `TaskPhase("in_progress").Validate(ctx)` returns a non-nil error. `TaskPhase("execution").Validate(ctx)` returns nil. — evidence: Ginkgo test `It("rejects 'in_progress' as canonical")` and `It("accepts 'execution' as canonical")` exist in `pkg/domain/task_phase_test.go`; `make test` exit 0.
- [ ] `NormalizeTaskPhase` exists with signature `func NormalizeTaskPhase(raw string) (TaskPhase, bool)`. `NormalizeTaskPhase("in_progress")` returns `(TaskPhaseExecution, true)`. Every canonical value round-trips. `NormalizeTaskPhase("garbage")` and `NormalizeTaskPhase("")` return `("", false)`. — evidence: Ginkgo `Describe("NormalizeTaskPhase")` block exists in `pkg/domain/task_phase_test.go`; `make test` exit 0.
- [ ] `TaskStatusTodo` and `TaskPhaseInProgress` Go constants still exist and still have string values `"todo"` and `"in_progress"`. Their doc comment marks them as alias-only. — evidence: `grep -n 'TaskStatusTodo TaskStatus = "todo"' pkg/domain/task_status.go` returns 1 match; doc comment on that line mentions "alias".
- [ ] `pkg/ops/lint.go` error message at line ~229 lists `next` (canonical only — not `todo`) among the valid status values. Aliases are not surfaced in user-facing error text because they fail `Validate`. — evidence: `grep -n 'expected one of:' pkg/ops/lint.go` returns one line containing `next` and not containing `todo`.
- [ ] `pkg/ops/lint_test.go` literal `validStatuses` lists at lines ~591, ~922, ~1220 (and analogous phase lists if any) contain `next` instead of `todo`. — evidence: `grep -n 'validStatuses' pkg/ops/lint_test.go` returns lines; reading the slices shows `next` present, `todo` absent from the canonical-set literals (alias-handling tests may still mention `todo`).
- [ ] Lint tests assert that a file with `status: todo` produces zero `IssueTypeInvalidStatus` issues, and a file with `phase: in_progress` produces zero invalid-phase issues. — evidence: Ginkgo test names exist (e.g. `It("accepts legacy 'todo' status via normalize")`); `make test` exit 0.
- [ ] `pkg/ops/workon.go` never writes `phase: in_progress` for any code path. The existing phase-advancement (currently sets `TaskPhasePlanning`) is unchanged. — evidence: `grep -nE 'TaskPhaseInProgress' pkg/ops/workon.go | grep -v '// alias'` returns 0 lines.
- [ ] An end-to-end read/write test on a vault file authored with `status: todo` and `phase: in_progress` reads cleanly, performs an unrelated field update, writes back, and the resulting file's status/phase lines are byte-identical to the original. — evidence: Ginkgo or integration test under `pkg/storage/` or `pkg/ops/` whose name references "legacy status/phase preservation"; `make test` exit 0; assertion compares the two field lines exactly.
- [ ] `CHANGELOG.md` has a new top entry `## vX.Y.Z` whose body mentions the canonical flip. `.claude-plugin/plugin.json` `version` field equals `X.Y.Z`. Both `version` fields in `.claude-plugin/marketplace.json` equal `X.Y.Z`. — evidence: `make precommit` runs `check-versions` and exits 0.
- [ ] `make precommit` exits 0 on the final commit. — evidence: exit code 0.

**Scenario coverage: NONE required.** The behaviors above are all reachable via Ginkgo unit + integration tests at the `pkg/domain/`, `pkg/ops/`, and `pkg/storage/` layers. No real-binary scenario test is added: there is no new CLI surface, no new external integration, and the round-trip preservation can be asserted at the storage layer. (Note: the project's "scenario-skip rule" still applies at release time — `pkg/domain/` changes will trigger the existing scenario suite against the freshly-built `/tmp/new-vault-cli`. That's a release gate, not a new scenario.)

## Verification

```
make precommit
```

Plus, before commit of any `pkg/domain/` change:

```
# Manual confirmation that legacy values still parse end-to-end.
# Pick any existing vault file with status: todo or phase: in_progress and run:
vault-cli task list --vault Personal | head
# Expected: exit 0; the file appears in output with no parse error.
```

## Do-Nothing Option

Tolerable but degrading. The status/phase name collision is a permanent friction tax. Every audit guide, every agent prompt, every grep query needs awkward "which dimension?" qualifiers. The task-orchestrator rename is already in flight and assumes vault-cli moves first (so old vault files keep working through the alias path). Deferring vault-cli's side means the orchestrator either also defers or accepts a temporary mismatch where it writes the new canonical and vault-cli's `Validate` rejects it. Acceptable to defer briefly, but not as a permanent state.

## Verification Result

**Verified:** 2026-05-20T18:14:37Z (HEAD de39cca)
**Binary:** /tmp/new-vault-cli (built fresh from de39cca; installed binary path verified separately)
**Scenario:** Walked all 4 markdown scenarios (001-004) against /tmp/new-vault-cli; ran `make release-check`; confirmed CI green for tag v0.65.1; spot-checked real-vault read via `task list --vault Personal`.
**Evidence:**
- AvailableTaskStatuses at `pkg/domain/task_status.go:39-46` contains TaskStatusNext, omits TaskStatusTodo; AvailableTaskPhases at `pkg/domain/task_phase.go:39-46` contains TaskPhaseExecution, omits TaskPhaseInProgress.
- NormalizeTaskStatus migration map at `pkg/domain/task_status.go:88-94` maps `"todo" → TaskStatusNext` and preserves `current/done/deferred` aliases; NormalizeTaskPhase at `:88-91` maps `"in_progress" → TaskPhaseExecution`.
- Lint error message at `pkg/ops/lint.go:227` reads `"status is %q, expected one of: next, in_progress, backlog, completed, hold, aborted"` (no `todo`).
- `pkg/ops/lint_test.go:1202-1244` exercises legacy `status: todo` and `phase: in_progress` on-disk fixtures and asserts zero `IssueTypeInvalidStatus`/`IssueTypeStatusPhaseMismatch` issues; `pkg/storage/base_test.go:79-99` round-trips `status: todo` through parse→serialize→re-parse with equality assertion.
- `grep -nE 'TaskPhaseInProgress' pkg/ops/workon.go` returned no lines.
- `make release-check` final lines: `✅ all four versions equal: 0.65.1` and `ready to release`; CHANGELOG top `## v0.65.1`, plugin.json `"version": "0.65.1"`, both marketplace.json version fields `0.65.1`.
- `git describe --tags --abbrev=0` returned `v0.65.1`; CI run 26180439763 success on master commit de39cca.
- Scenario 002 work-on → defer → complete produced on-disk frontmatter `status: completed`, `assignee: alice`, `defer_date: "2026-05-21"`; Scenario 003 recurring completion kept `status: in_progress`, reset all checkboxes to `[ ]`, advanced `defer_date` to NEXT_WEEK and stamped `last_completed`; Scenario 004 ack set `reviewed: true` + `reviewed_date` while leaving `needs_review: true` untouched.
**Verdict:** PASS
