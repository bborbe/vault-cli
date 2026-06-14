---
status: completed
tags:
    - dark-factory
    - spec
approved: "2026-06-14T14:27:11Z"
generating: "2026-06-14T14:27:12Z"
prompted: "2026-06-14T14:41:38Z"
verifying: "2026-06-14T15:55:20Z"
completed: "2026-06-14T19:27:28Z"
branch: dark-factory/enforce-status-in-progress-on-calendar-date
---

## Summary

- A task with a calendar date (`planned_date`, `defer_date`, `due_date`) is a commitment to execute, so its status must be `in_progress`.
- The Obsidian Kanban board filters by `status=in_progress|completed`; tasks with future dates but `status: next` / `backlog` are invisible and silently miss their cadence day.
- Three confirmed silent-miss instances this quarter (two weekly trading reviews, one SSL cert renewal). Missed reviews feed scale-up gates blind — capital relevance is real.
- Three reinforcing fixes prevent the violation: lint detects + auto-fixes existing files, defer auto-promotes at write-time, task-creator agent emits `in_progress` when a date is set.
- One detector function powers both `vault-cli task lint` and `vault-cli task validate` — single source of truth, no rule duplication.

## Problem

The Obsidian Kanban board surfaces tasks by status (`in_progress` and `completed`). When a task is authored or deferred with a future calendar date but a non-active status (`next` or `backlog`), the board cannot see it, so the cadence day arrives with no visible reminder. Three silent misses this quarter (ORB GBPJPY Weekly Review W24, ORB DE40 Sunday Review W23, Renew Quant SSL Certificate) confirm the pattern. Some of these feed downstream scale-up gates; a missed review means decisions get made on stale evidence. The vault has no enforcement of the invariant "a calendar date is a commitment", so the violation re-occurs every time a task is created or deferred without conscious status discipline.

## Goal

After this work, the following invariant holds across the vault tooling: **if a task has any of `planned_date`, `defer_date`, or `due_date` set, its status is `in_progress` (or terminal — `completed` / `aborted`).** The invariant is enforced at three points — file creation (task-creator agent), date assignment (`task defer` command), and audit (`task lint` / `task validate`) — with auto-fix promoting `next` / `backlog` to `in_progress` and leaving the date untouched.

## Non-goals

- Do NOT flag tasks whose status is `completed` or `aborted` with a stale date — terminal status takes precedence; the calendar-as-commitment rule only fires for unstarted work.
- Do NOT strip the calendar date as an alternative fix path — invariant; the auto-fix direction is always "promote status", never "remove date". If a future caller wants the inverse, that's a separate spec.
- Do NOT add a one-time vault sweep / remediation of pre-existing violations in this spec — separate task once the rule is live.
- Do NOT change `task-auditor.md` — semantic/judgment checks only; deterministic field-combo rules belong in the Go lint, not in an LLM prompt.
- Do NOT add a `task set` warn-and-offer-promote flag — out of scope; revisit once the rule is live and the field-set path surfaces in real use.
- Do NOT add a CLI bypass / `--allow-status-date-mismatch` flag — invariant; the rule has no exceptions for unstarted tasks. If a future consumer demands an exception, that's a separate spec.
- Do NOT touch Personal vault guides (`Task Writing Guide.md`, Boss Memory) — those are direct vault edits, not vault-cli changes.
- Do NOT change `work-on-task` agent — already implies `in_progress`, no bypass risk.

## Desired Behavior

1. `vault-cli task lint` reports a `status_date_mismatch` issue for any task file whose status is `next` or `backlog` AND any of `planned_date` / `defer_date` / `due_date` is set.
2. `vault-cli task lint --fix` rewrites the status field to `in_progress` for such files; all other frontmatter fields (including the date that triggered the issue) are byte-identical before and after.
3. `vault-cli task validate <path>` surfaces the same issue with the same description string — the detector is invoked by both subcommands, not re-implemented.
4. `vault-cli task defer <task> <date>` on a task whose current status is `next` or `backlog` writes back both the new `defer_date` AND `status: in_progress` in a single file write.
5. `vault-cli task defer` on a task whose status is already `in_progress` leaves status untouched (idempotent — the defer write does not flip status redundantly).
6. `vault-cli task defer` on a task whose status is `completed`, `aborted`, or `hold` leaves status untouched — only `next` and `backlog` are promoted.
7. The `task-creator` agent emits `status: in_progress` (not `status: todo`) in the new task's frontmatter whenever any of `planned_date`, `defer_date`, or `due_date` is being written to that task.
8. The `task-creator` agent continues to emit `status: next` (canonical replacement for legacy `todo`) when no date field is being written — non-dated tasks default to the queue, not the board.

## Constraints

- New detector function MUST mirror the existing `detectStatusPhaseMismatch` pattern in `pkg/ops/lint.go` (regex-based frontmatter parsing, same return-tuple shape, wired into `collectLintIssues` next to its sibling).
- Both `task lint` and `task validate` MUST share the single detector implementation — proven by AC 3 (same issue, same description, same wiring).
- Auto-fix direction is fixed: promote status, never strip date. Any future caller asking for the inverse fix requires a new spec.
- `task-creator` agent change MUST NOT introduce new frontmatter fields — only flip the existing `status:` default based on date-field presence.
- Tests use Ginkgo v2 / Gomega per project convention. No new interface seams expected; no Counterfeiter mocks required.
- `make precommit` MUST stay green in `pkg/ops` (and any other touched module).
- DoD (`docs/dod.md`) applies — enforced by `.dark-factory.yaml` validation prompt after each generated prompt.
- Existing lint issue types and their wire-format names MUST NOT change — additive only.
- Referenced docs: `docs/task-writing.md` (canonical task structure, amended by this spec); `pkg/ops/lint.go` lines 509-563 (pattern to mirror); `pkg/ops/defer.go` lines 134-151 (auto-promote injection point).

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Task file has malformed YAML frontmatter | Detector returns "no issue" rather than crashing; existing YAML-parse lint issue (if any) fires separately | User fixes YAML; rerun lint |
| Task file has `status: in_progress` and a date | No issue reported (invariant already satisfied) | None needed |
| Task file has `status: completed` + stale `defer_date` | No issue reported (out of scope per Non-goals) | None needed |
| Task file has a date field present but empty (e.g. `defer_date:` with no value) | Treated as "no date set"; no issue | None needed |
| `task defer` invoked on a `completed` / `aborted` task | Status untouched; defer_date written as before | None — terminal status preserved |
| `task lint --fix` mid-write crash (process killed between file rewrite and fsync) | Filesystem-level partial write; same risk profile as existing lint --fix paths — no new risk introduced | Rerun `task lint --fix`; idempotent |
| Two concurrent `task lint --fix` invocations on the same file | Last write wins; same concurrency profile as existing lint paths — no new locking introduced | Rerun lint to confirm fixed state |
| `task-creator` agent emits `in_progress` for a task with all date fields removed during composition | Agent re-evaluates default at emit time; trailing-edge state, not first-pass intent, drives the default | Author re-reviews emitted file |

## Security / Abuse Cases

Not applicable — feature operates on locally-owned vault files via existing read/write paths. No HTTP, no user-supplied paths beyond what existing commands already accept, no new trust boundary.

## Acceptance Criteria

- [ ] `vault-cli task lint <fixture-dir>` on a synthetic fixture with `status: next` + `defer_date: 2026-12-01` prints a line containing `status_date_mismatch` — evidence: stdout grep `status_date_mismatch` returns ≥1 line
- [ ] Same lint invocation also reports the same issue for fixtures with `status: backlog` + `planned_date`, `status: next` + `due_date`, and `status: backlog` + `due_date` — evidence: 4 separate fixtures, each lint run prints `status_date_mismatch` ≥1 line
- [ ] `vault-cli task lint --fix` on the `next` + `defer_date` fixture rewrites `status: next` to `status: in_progress` — evidence: `grep -c '^status: in_progress' fixture.md` returns 1 after fix; `grep -c '^status: next' fixture.md` returns 0
- [ ] Same `--fix` run leaves the `defer_date` line byte-identical — evidence: `grep '^defer_date:' fixture.md` returns identical line before and after
- [ ] `vault-cli task validate <fixture>` on the same `next` + `defer_date` fixture surfaces an issue whose description string is identical to the lint output — evidence: substring match between `lint` stdout and `validate` stdout for the issue description
- [ ] `vault-cli task defer "<task name>" 2026-12-01` on a `status: next` task results in a file containing both `status: in_progress` and `defer_date: 2026-12-01` — evidence: both `grep '^status: in_progress'` and `grep '^defer_date: 2026-12-01'` return 1 hit after defer
- [ ] `vault-cli task defer` on a `status: in_progress` task leaves the status line byte-identical — evidence: `grep '^status:' file.md` returns identical content before and after
- [ ] `vault-cli task defer` on a `status: completed` task leaves the status line byte-identical — evidence: `grep '^status: completed' file.md` returns 1 hit after defer
- [ ] `vault-cli task defer` on a `status: backlog` task results in `status: in_progress` — evidence: `grep '^status: in_progress'` returns 1 hit after defer
- [ ] A task file authored by following the updated `task-creator.md` instructions with any date field set contains `status: in_progress` — evidence: file content inspection; `grep '^status: in_progress' new-task.md` returns 1 hit, `grep '^status: todo'` returns 0 hits
- [ ] A task file authored by following the updated `task-creator.md` instructions with NO date field set contains `status: next` — evidence: `grep '^status: next' new-task.md` returns 1 hit
- [ ] A task with `status: completed` + a `defer_date` set produces no `status_date_mismatch` from lint — evidence: `grep -c 'status_date_mismatch' lint-output` returns 0
- [ ] Ginkgo tests cover: lint detection for each (date-field × inactive-status) combination, lint auto-fix promotion, lint no-op on terminal status, defer auto-promote from `next` / `backlog`, defer no-op on `in_progress` / `completed` / `aborted` / `hold` — evidence: `make test` in `pkg/ops` exits 0 with new `It(...)` blocks present in `lint_test.go` and `defer_test.go`
- [ ] `make precommit` exits 0 in `pkg/ops` — evidence: exit code
- [ ] `CHANGELOG.md` `## Unreleased` section contains a bullet naming the rule and its auto-fix direction — evidence: `grep -A20 '^## Unreleased' CHANGELOG.md` returns a line mentioning "status" and "date" and "in_progress"
- [ ] `docs/task-writing.md` Lifecycle section names the calendar-as-commitment rule and references the auto-fix direction — evidence: `grep -i 'calendar' docs/task-writing.md` returns ≥1 line; `grep -i 'in_progress' docs/task-writing.md` shows the rule context

## Verification

```
cd pkg/ops && make precommit
```

Plus replay scenarios for the AC matrix — each scenario exercises one synthetic fixture and checks the evidence shape declared above.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Add `detectStatusDateMismatch` + `IssueTypeStatusDateMismatch` to `pkg/ops/lint.go`; wire into `collectLintIssues`; auto-fix promotes `next`/`backlog` → `in_progress`; Ginkgo tests in `lint_test.go` covering each (date-field × inactive-status) combination, terminal-status no-op, and fix idempotence | 1, 2, 3 | lint-related ACs (1-5, 12, 13, 14) | — |
| 2 | Inject auto-promote in `findAndDeferTask` in `pkg/ops/defer.go` for `next` / `backlog` → `in_progress`; Ginkgo tests in `defer_test.go` covering promote, no-op on `in_progress`, no-op on terminal/hold | 4, 5, 6 | defer-related ACs (6-9, 13, 14) | prompt 1 (shared test fixture conventions) |
| 3 | Update `agents/task-creator.md` step 8 to emit `status: in_progress` when any date field is set, `status: next` otherwise; update `docs/task-writing.md` Lifecycle table + Critical Rule paragraph; add `CHANGELOG.md` `## Unreleased` bullet | 7, 8 | 10, 11, 15, 16 | prompts 1 + 2 (rule must be live before docs claim it) |

Rationale: prompt 1 lands the detector (single source of truth for lint + validate); prompt 2 closes the create-side leak via defer using the same status semantics; prompt 3 ships the prevention-at-authoring path and the docs/changelog only after the runtime rule exists, so the docs are never ahead of the code.

## Do-Nothing Option

Status quo: the Kanban filter remains blind to `next` / `backlog` tasks with future dates. Three silent misses this quarter is the observed rate; one feeds a scale-up gate. Discipline-by-memory has already failed three times this quarter. Doing nothing means accepting that future cadence misses will continue at the same rate, with capital decisions made on stale evidence. Not acceptable.

## Verification Result

**Verified:** 2026-06-14T19:17:01Z (HEAD 8441e82)
**Binary:** /tmp/new-vault-cli (built from worktree HEAD)
**Scenario:** Built `vault-cli` from HEAD, ran fixture-based CLI replays for lint detection (4 status×date combos), lint --fix promotion (status flipped, date byte-identical), validate description parity, and defer on each of next/backlog/in_progress/completed/aborted/hold.
**Evidence:**
- `WARN Tasks/next-defer.md: STATUS_DATE_MISMATCH status is next but defer_date is set (calendar dates are commitments; expected in_progress)` (lint and validate emit byte-identical description)
- After `task lint --fix`: `grep -c '^status: in_progress' next-defer.md` = 1, `grep -c '^status: next'` = 0, `defer_date: 2026-12-01` line byte-identical
- After `task defer backlog-only 2026-12-25`: file has both `status: in_progress` and `defer_date: "2026-12-25"`; on `completed-defer` defer preserves `status: completed`; on `aborted-only` preserves aborted; on `hold-only` preserves hold; on `in_progress-defer` leaves status untouched
- `completed-defer.md` (status: completed + defer_date) produced 0 `STATUS_DATE_MISMATCH` lines (terminal-status carve-out works)
- `agents/task-creator.md:114` carries the conditional emit rule (`status: in_progress` IF any date field, `status: next` OTHERWISE)
- `make precommit` exit 0; `go test ./pkg/ops` exit 0 with new `It(...)` blocks in `lint_test.go` (each date×status combo, terminal no-op, fix promotion) and `defer_test.go` (promote next/backlog, no-op in_progress/completed/aborted/hold)
- `CHANGELOG.md` `## Unreleased` carries three bullets naming the rule, auto-fix direction, and `in_progress`; `docs/task-writing.md:182` "Calendar-as-commitment rule" lives under `## Lifecycle`
**Verdict:** PASS
