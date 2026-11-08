---
status: completed
summary: Added the non-fixable HOLD_WITH_DEFER_DATE lint finding (task-scoped, with recurring exemptions), promoted hold tasks on `task defer`, updated docs/changelog and tests; `make precommit` exits 0.
execution_id: vault-cli-hold-defer-lint-exec-275-hold-defer-lint
dark-factory-version: v0.198.17
created: "2026-11-04T15:47:35Z"
queued: "2026-11-04T15:47:35Z"
started: "2026-11-04T15:48:54Z"
completed: "2026-11-04T15:55:30Z"
---

# Reject hold + defer_date in task lint

<summary>
- `vault-cli task lint` gains a finding for a task parked at `status: hold` that still carries a `defer_date`
- The finding is not auto-fixable and names both resolutions: promote to `in_progress`, or clear the date
- Tasks whose `defer_date` is a scheduling date rather than a snooze are exempt, by two markers
- The existing `next` / `backlog` calendar-as-commitment rule is unchanged
- The task-writing rule documents the `hold` case beside the existing one
- Unit tests cover the firing case, both exemptions, and the `next`/`backlog` regression
- `task defer` on a `hold` task now promotes it to `in_progress` in the same write, so the command cannot create the state the new rule rejects
- The rule is task-scoped: goal, topic, theme, objective and vision pages are unaffected
</summary>

<objective>
Close the enforcement gap between the written `hold`-versus-`defer` rule and the lint that is meant to back it: a task parked at `status: hold` while carrying a snooze `defer_date` disappears from active rotations, and today nothing reports it. Make the rule mechanical instead of prose-only, without flagging the recurring tasks whose `defer_date` is a schedule date, and stop `task defer` from creating the state the rule rejects.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read `pkg/ops/lint.go`:
- the `IssueType` constant block, around the `IssueTypeStatusDateMismatch` declaration
- `collectLintIssues` — where every detector is wired in, next to the existing `detectStatusDateMismatch` arm
- `detectStatusDateMismatch` — the sibling detector this change sits beside
- `fixStatusDateMismatch` — the auto-fix the new finding deliberately does NOT get
- `detectMissingTaskIdentifier` — the existing example of parsing a frontmatter field with `yaml.Unmarshal` into a small struct

Read `pkg/domain/task_status.go` — `TaskStatusHold` already exists; no new status constant is needed.

Read `pkg/ops/lint_test.go` — the `STATUS_DATE_MISMATCH` contexts (trigger / no-trigger / auto-fix / lint-vs-validate parity) are the test shape to follow. `writeSDM` is the fixture helper and `countSDMIssues` is the counting helper.

Read `docs/task-writing.md` § "Calendar-as-commitment rule".

Read `pkg/ops/defer.go` `findAndDeferTask` — the existing `next` / `backlog` promotion guard and its "no-op on completed/aborted/hold" comment, which requirement 8 extends. Read `pkg/ops/defer_test.go` `Context("when current status is hold")` — the expectation requirement 8 inverts.
</context>

<requirements>
1. In the `IssueType` constant block in `pkg/ops/lint.go`, add a new constant `IssueTypeHoldWithDeferDate IssueType = "HOLD_WITH_DEFER_DATE"` immediately after `IssueTypeStatusDateMismatch`. Do not reuse `STATUS_DATE_MISMATCH`: that rule auto-promotes `next`/`backlog` to `in_progress`, while a `hold` may be a legitimate block, so this case must stay non-fixable and carry different resolution wording.

2. Add a new method `detectHoldWithDeferDate(frontmatterYAML string) (bool, string)` on `*lintOperation`, placed beside `detectStatusDateMismatch`. It returns `(false, "")` unless all of the following hold:
   - `status` is `hold` (compare against `domain.TaskStatusHold`)
   - `defer_date` is non-empty after trimming whitespace
   - `created_by` is not `recurring-task-creator`
   - the `recurring` key is empty or absent

   Parse the frontmatter with `yaml.Unmarshal` into a small anonymous struct with `status`, `defer_date`, `created_by` and `recurring` string fields — follow `detectMissingTaskIdentifier` for the shape, and return `(false, "")` on an unmarshal error rather than reporting a finding. Unquoted YAML timestamps (`defer_date: 2026-05-24T00:00:00Z`) decode into a string field correctly; do not reach for the shared `dateRegex` here.

   When it does fire, return a description that names the offending date value and both resolutions, in this shape:
   `status is hold but defer_date is <value> (a deferred task belongs at in_progress + defer_date; hold is for a block with no resume date — promote to in_progress, or clear defer_date)`

3. Document the two exemptions in the method's GoDoc comment, with the reason each is a scheduling date rather than a snooze:
   - `created_by: recurring-task-creator` — written by the recurring-task-creator pipeline, which is **external to this repo** (the field has no producer in this codebase, so do not treat its absence here as evidence it is unused). Instances it materializes are written with `defer_date` equal to their schedule date at creation, so the field is the materialization date, not a snooze. The key is present on 1747 task files in the personal vault this repo serves.
   - a non-empty `recurring:` key — vault-cli's own recurring roll rewrites `defer_date` to the next period's date on complete.

4. Wire the detector into `collectLintIssues` immediately after the existing `detectStatusDateMismatch` arm, adding it with `fixable = false`, and gate the arm on `pageType == PageTypeTask`. `collectLintIssues` is shared by six lint commands (`task`, `goal`, `topic`, `theme`, `objective`, `vision`), `hold` is a first-class goal status (`domain.GoalStatusHold`), and `goal defer` writes a `defer_date` without promoting — an ungated arm would report this finding, with task wording, on a legitimate goal state and make `vault-cli goal lint` exit non-zero. Do not touch `fixStatusDateMismatch`, and do not add a fix path for the new type. Gate at the call site rather than inside the detector: the detector's signature stays `(frontmatterYAML string)`, matching the other single-purpose detectors.

5. Add tests in `pkg/ops/lint_test.go` in a new `Context("HOLD_WITH_DEFER_DATE", ...)` block inside the existing `Describe` that owns `writeSDM`, so the helper is in scope (rename that `Describe` so it covers both rules). Reuse `writeSDM` and a counting helper modelled on `countSDMIssues`. Cover:
   - fires on `status: hold` + `defer_date`, asserting `Fixable` is false, the description contains `hold`, contains `defer_date`, and contains both `in_progress` and `clear defer_date`
   - fires on `status: hold` + an unquoted timestamp `defer_date` (the shape real vault files carry)
   - does not fire on `status: hold` + `defer_date` + `created_by: recurring-task-creator`
   - does not fire on `status: hold` + `defer_date` + a non-empty `recurring: weekly`
   - for each of the two exemptions, the same frontmatter with the exemption key removed produces exactly one finding — the pair proves the exemption is what silences it, not the fixture shape
   - does not fire on `status: hold` with no `defer_date`
   - does not fire on `status: hold` with a present but valueless `defer_date:`
   - does not fire on `status: hold` + `planned_date` only (the rule is about the snooze field, not every date)
   - does not fire on `status: in_progress` + `defer_date`
   - the regression: `status: next` + `defer_date` still produces exactly one `STATUS_DATE_MISMATCH` and zero `HOLD_WITH_DEFER_DATE`
   - `Execute(..., fix=true)` on a firing fixture that also carries `page_type: task` and `task_identifier` (so no other fixable issue rewrites the file) leaves the file byte-identical — the finding stays non-fixable
   - `ExecuteFile` (the `task validate` path) surfaces `HOLD_WITH_DEFER_DATE` with the same description `Execute` produces
   - the arm is task-scoped: `Execute` with `PageTypeGoal` over the same firing fixture produces zero `HOLD_WITH_DEFER_DATE`, while `PageTypeTask` still produces the finding

6. Extend `docs/task-writing.md` § "Calendar-as-commitment rule" with a short paragraph after the existing text: a task at `status: hold` carrying a `defer_date` is reported by `task lint` / `task validate` as `HOLD_WITH_DEFER_DATE`; it is not auto-fixable because both resolutions are legitimate — promote to `in_progress` (the date is a resume date) or clear the date (the block is real and open-ended). Name the two exemptions and state the coverage limit plainly: instances created by the recurring task creator (`created_by: recurring-task-creator`) and tasks carrying a non-empty `recurring:` key are exempt because their `defer_date` is a schedule date, so the rule is **not** mechanically enforced for those tasks. State that the rule is task-scoped (`task lint` / `task validate` only) and does not apply to goal / topic / theme / objective / vision pages. Also correct the sentence in that same section that enumerates the `task defer` promotion — it currently reads "date assignment (`task defer` auto-promotes `next` / `backlog` to `in_progress` in the same write)" — so it includes `hold`, matching requirement 8.

7. Add one `## Unreleased` bullet to `CHANGELOG.md`, in the existing `## Unreleased` section if master already carries one, otherwise as a new section above the first version heading. Use a `feat:` bullet naming `HOLD_WITH_DEFER_DATE`, the `hold` + `defer_date` pair, the recurring exemption, and the `task defer` status promotion from requirement 8. Do not rename or add any version heading.

8. Close the write path that produces the state requirement 2 rejects. In `pkg/ops/defer.go` `findAndDeferTask`, extend the existing calendar-as-commitment promotion — the guard currently reading `status == domain.TaskStatusNext || status == domain.TaskStatusBacklog` — to include `domain.TaskStatusHold`, and update the comment that currently says "no-op on completed/aborted/hold (out of scope)". Leave `completed` and `aborted` untouched. Then update `pkg/ops/defer_test.go` `Context("when current status is hold")`: its `It("leaves status as hold")` expectation and the byte-level `status` assertion must now expect `in_progress`, and the test name must say so.

Before finishing: re-run `<verification>` and confirm it passes, then walk each requirement against the change.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT bump any version string (`CHANGELOG.md` version heading, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`). The repo auto-releases (`.maintainer.yaml` → `release.autoRelease: true`; `.dark-factory.yaml`'s `autoRelease` is a different switch and is false); the releaser owns version bumps and tags.
- Do NOT change the `completed` / `aborted` arms of `findAndDeferTask` — only `hold` is added to the promotion.
- Do NOT create a git tag.
- Do NOT change the `next` / `backlog` arm of `STATUS_DATE_MISMATCH`, its auto-fix, or its wording — that behaviour shipped and is out of scope.
- Existing tests must still pass.
- Repo-relative paths only.
</constraints>

<verification>
```bash
ROOTDIR=/workspace make precommit
```
Then confirm the finding is wired and documented:
```bash
grep -q 'HOLD_WITH_DEFER_DATE' pkg/ops/lint.go
grep -q 'HOLD_WITH_DEFER_DATE' pkg/ops/lint_test.go
grep -q 'HOLD_WITH_DEFER_DATE' docs/task-writing.md
grep -q 'HOLD_WITH_DEFER_DATE' CHANGELOG.md
grep -q 'TaskStatusHold' pkg/ops/defer.go
! grep -q 'leaves status as hold' pkg/ops/defer_test.go
grep -q 'pageType == PageTypeTask' pkg/ops/lint.go
! grep -q 'auto-promotes `next` / `backlog` to `in_progress` in the same write' docs/task-writing.md
```
</verification>
