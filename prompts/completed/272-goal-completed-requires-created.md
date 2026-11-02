---
status: completed
execution_id: vault-cli-topic-complete-exec-272-goal-completed-requires-created
dark-factory-version: v0.198.7
created: "2026-11-01T16:45:00Z"
queued: "2026-11-01T18:09:18Z"
started: "2026-11-01T18:12:52Z"
completed: "2026-11-01T18:22:05Z"
lastFailReason: 'validate completion report: completion report status: partial'
reconciled: "2026-11-03"
reconciledReason: 'Work shipped in 9641ef7 "Lint reports a completed goal with no created key, and goal-creator writes it" (2026-11-01), an ancestor of origin/master; only the completion report failed validation (status partial). Verified on master: pkg/ops/lint.go and pkg/ops/lint_test.go both carry COMPLETED_GOAL_MISSING_CREATED. Status reconciled to completed — the prompt was never re-run. Surfaced by spec 065 completing prompts 273/274, which opened a missing-completed-prompt gap at 272.'
---

# Require a Start Date on Every Completed Goal

<summary>
- A goal that reaches `status: completed` without a `created:` key becomes a lint issue instead of a silent gap
- The issue is `COMPLETED_GOAL_MISSING_CREATED`, raised by `vault-cli goal lint` and never auto-fixed
- The goal-creation agent now writes `created: "YYYY-MM-DD"` on every new goal, so the issue fires only on goals that genuinely lost their start date
- Because a `created` date brings a goal under the existing DoD audit gate, the same agent change adds the `# Definition of Done` section that gate already requires
- The goal writing guide documents `created` as a required frontmatter field for the first time
- Motivation: a completed goal with no `created:` key has a completion date and no duration, so its cycle time cannot be read at all
- Existing goals stay as they are, and every other lint rule and page type is untouched
</summary>

<objective>
Make "a completed goal always carries a start date" an enforced invariant rather than a convention, so goal cycle time stays measurable: add a `COMPLETED_GOAL_MISSING_CREATED` lint issue for goals, have the goal creator write `created:` and the Definition-of-Done section that field pulls it under, and document the field.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read these files before implementing:
- `pkg/ops/lint.go`:
  - The `IssueType` const block (~line 83) — every issue type is declared here with a stable string value.
  - `detectBlockedByScalar` — the closest template: a single-key detector returning `(bool, string)`, wired into `collectLintIssues` with `add(<IssueType>, <description>, false)`.
  - `detectQuotedBooleans` — shows the multi-match detector shape.
  - `collectLintIssues` (~line 266) — where every detector is wired; add the new one next to the `detectBlockedByScalar` call.
  - `statusRegex` (~line 28) — `(?m)^status:\s*['"]?([a-z_]+)['"]?\s*$`; reuse it rather than writing a second status parser.
  - `dateRegex` (~line 34) — the precedent for a field-value regex; note it is scoped to three specific keys, so a new `created` regex is needed.
- `pkg/ops/lint_test.go`:
  - `Describe("LintOperation - QUOTED_BOOLEAN detection and fix")` (~line 2680) — the template for a per-rule Describe block, including the `lintOp.Execute(ctx, "goal", vaultPath, goalsDir, goalsDir, false)` call that lints a goal page.
  - `writeFile(dir, name, content)` — the fixture helper, declared as a closure *inside* that Describe block; redeclare it in the new block.
  - The status/phase-mismatch Describe block (~line 1990) — its `writeGoal` closure writes a `status: completed` goal with no `created:`, and two entries assert the issue list is empty or has length 1. That fixture must gain `created:` (see requirement 7).
- `integration/cli_test.go`:
  - The `Describe` block for `vault-cli goal lint` status/phase (~line 1580) and its `goalFixture(status, phase)` closure — the AC9 entries assert exit 0 for `status: completed` with no `created:` (see requirement 7).
  - `AC10a` (~line 1508) — the template for asserting a named issue type appears in plain stdout with exit 1.
- `agents/goal-creator.md` § 8 `Compose frontmatter` (the required-field list), § 9 `Compose body` (the numbered section list), and § 12 `Audit` (the self-check line naming required frontmatter fields).
- `agents/goal-auditor.md` (~line 111) — the `DOD_REQUIRED_AS_OF` rule that treats a goal created on or after 2026-06-26 with no `# Definition of Done` section as an audit failure, and grandfathers a goal with no `created` at all.
- `docs/goal-writing.md` § `Frontmatter` (~line 127) and § `Definition of Done` (~line 243) — the frontmatter block and the "both Success Criteria and Definition of Done are required" rule this change documents.

Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` wrapping with `ctx`, never `fmt.Errorf`.
</context>

<requirements>
1. In `pkg/ops/lint.go`, add to the `IssueType` const block:
   ```go
   IssueTypeCompletedGoalMissingCreated IssueType = "COMPLETED_GOAL_MISSING_CREATED"
   ```
   The new name is longer than any existing one, so gofmt re-columns the whole block — let it. Do not hand-pad the surrounding lines.

2. Add a detector to `pkg/ops/lint.go`, placed next to `detectBlockedByScalar`, with this contract:
   ```go
   func (l *lintOperation) detectCompletedGoalMissingCreated(
       pageType string,
       frontmatterYAML string,
   ) (bool, string)
   ```
   - Returns `(false, "")` unless `pageType == "goal"`. The rule is goal-only: tasks carry `created_date`, themes and objectives are out of scope. There is no `PageTypeGoal` constant — the codebase uses the `"goal"` literal, so use it.
   - Returns `(false, "")` unless the frontmatter's `status` is exactly `completed`. Read it with the existing `statusRegex`, comparing the raw capture as `detectStatusPhaseMismatch` does. `completed` is the canonical goal status (`domain.GoalStatusCompleted`); the legacy goal aliases `active` / `on_hold` are not completed states, and the task-only alias `done` is out of scope here — do not normalize through `domain.NormalizeTaskStatus`.
   - Returns `(false, "")` when a `created:` key is present with a non-empty value. A bare `created:` with no value counts as missing.
   - Otherwise returns `(true, <description>)`. The description names the consequence, not just the absence — for example: `goal is completed but carries no created: key (cycle time is unmeasurable without a start date)`.
   - The rule is **not** fixable: there is no derivable value for a missing start date, and a bulk date stamped across a set is worse than a visible gap.

3. Wire the detector into `collectLintIssues` immediately after the `detectBlockedByScalar` block, passing `pageType` through:
   ```go
   if missingIssue, missingDesc := l.detectCompletedGoalMissingCreated(pageType, frontmatterYAML); missingIssue {
       add(IssueTypeCompletedGoalMissingCreated, missingDesc, false)
   }
   ```

4. In `agents/goal-creator.md` § 8 `Compose frontmatter`, add `created: "YYYY-MM-DD"` to the required-field list and state that the value is today's date obtained with `date +%F`. Keep it quoted and date-only. Adjust the closing "Do NOT set fields the user did not ask for" sentence so `created` is not read as forbidden — `created` is required, everything else stays opt-in. Also update § 12 `Audit`'s checklist line `Frontmatter has required fields (status, page_type)` to name `created`, so the creator's self-audit catches a missing start date, and extend § 12's body checklist line `Body has Success Criteria + Tasks sections` to name `# Definition of Done`.

   This has a second consequence that must land in the same change. `agents/goal-auditor.md` treats a goal whose `created` is `DOD_REQUIRED_AS_OF` (2026-06-26) or later, and which has no `# Definition of Done` section, as an audit failure; today goals escape that gate only because they carry no `created` at all. So § 9 `Compose body` of `agents/goal-creator.md` must also write a `# Definition of Done` section, placed after `# Success Criteria` and before `# Non-goals`, containing at least two binary checkboxes covering closure (for example `- [ ] PRs merged` and `- [ ] Goal page synced`). `docs/goal-writing.md` § Definition of Done requires exactly this: both `# Success Criteria` and `# Definition of Done` are required, and the DoD carries ≥2 binary closure checkboxes. Note this in the § 9 numbered list so the section is not optional. The section is required even when step 7 loaded a `goal_template` body: if the template has no `# Definition of Done`, append one — a template must not silently reintroduce the audit failure the section exists to prevent.

5. In `docs/goal-writing.md` § `Frontmatter`, add `created: "YYYY-MM-DD"        # required — the date the goal was created; the start of its cycle time` to the YAML block, and add one short paragraph after the block explaining why: a completed goal without `created` has a completion date and no duration, so `vault-cli goal lint` reports `COMPLETED_GOAL_MISSING_CREATED` and cycle time cannot be read from it. State in that paragraph that existing goals without `created` stay grandfathered by the goal auditor — the doc must not read as an instruction to backdate history.

6. Add tests to `pkg/ops/lint_test.go` in a new `Describe("LintOperation - COMPLETED_GOAL_MISSING_CREATED")` block covering, at minimum:
   - a completed goal with no `created:` key produces exactly one `COMPLETED_GOAL_MISSING_CREATED` issue;
   - a completed goal with `created: "2026-01-05"` produces none;
   - a completed goal with a bare `created:` and no value produces the issue;
   - an `in_progress` goal with no `created:` key produces none;
   - linting the same content with `pageType` `ops.PageTypeTask` produces no `COMPLETED_GOAL_MISSING_CREATED` issue — filter by issue type here, because the task page type also yields one `MISSING_TASK_IDENTIFIER` for this fixture (the identifier checks are task-only; see `taskIdentifierIssues`);
   - the issue is not fixable — assert `Fixable` is false on the returned issue.
   Drive at least the positive and the completed-with-date cases through `lintOp.Execute(ctx, "goal", vaultPath, goalsDir, goalsDir, false)`, not only the detector, so the wiring is exercised.

7. Three existing fixtures already assert that a `status: completed` goal produces no other issue; the new rule makes each of them fail. Repair them by giving the fixture a `created:` value — do NOT relax the new rule, do NOT delete an entry, and do NOT weaken an existing assertion:
   - `pkg/ops/lint_test.go`, the status/phase-mismatch Describe block: the `writeGoal` closure builds the frontmatter for both the `"should NOT trigger"` entry `"completed + no phase"` (asserts `BeEmpty()`) and the `"should trigger"` entry `"completed + execution"` (asserts `HaveLen(1)`). Add `created: "2026-01-05"` in both branches it writes, so each entry still proves only the phase rule.
   - `integration/cli_test.go`, the `vault-cli goal lint` status/phase Describe: `AC9: accepts completed with no phase` runs `goalFixture("completed", "")` and asserts exit 0. Add `created: "2026-01-05"` to that fixture — the simplest correct repair is inside the `goalFixture` closure itself, since a `created` key is harmless for the `aborted` and `in_progress` fixtures too.

8. Add one integration test in `integration/cli_test.go`, next to the status/phase `Describe`, proving the plain-output path: a `status: completed` goal with no `created:` key makes `goal lint` exit 1 and print `COMPLETED_GOAL_MISSING_CREATED`; the same goal with `created: "2026-01-05"` exits 0 and prints nothing about it. Model it on `AC10a` — write both fixtures inline as literal YAML strings. Do NOT reuse the status/phase block's `goalFixture` closure: requirement 7 gives it a `created:` key, so a fixture taken from it would never trigger the new rule.

9. Add a `## Unreleased` section to `CHANGELOG.md` (it does not exist yet — create it below the `* PATCH version…` preamble line and above `## v0.184.4`) with one `- feat:` bullet stating that `vault-cli goal lint` reports `COMPLETED_GOAL_MISSING_CREATED` on a completed goal with no `created:` key, and that `goal-creator` now writes `created`. Follow `docs/dod.md` § Documentation.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT change the behaviour of any existing lint rule, issue type, or fix path.
- Do NOT add a `--fix` path for the new issue: it must always report `Fixable: false`.
- Do NOT touch `pkg/ops/topic_complete.go` or the topic completion behaviour.
- Do NOT modify anything under `scripts/` or the `Makefile` to work around a container-environment failure. Several `scripts/*.sh` tests fail inside this container because its mawk lacks `{m,n}` interval expressions and its HTTP proxy intercepts loopback requests; those failures are pre-existing and environmental, and they are out of scope.
- Do NOT add an equivalent finding to `agents/goal-auditor.md`. The auditor is deliberately out of scope here: it already reads `created` for grandfathering, and making it fail on a missing start date would flag every goal written before this change. The lint issue is the enforcement surface.
- Existing tests must still pass.
- Follow `pkg/ops/lint.go`'s existing detector style: a method on `*lintOperation` returning `(bool, string)`, wired through `collectLintIssues`'s `add` helper.
</constraints>

<verification>
Run `ROOTDIR=/workspace go vet -mod=mod ./...` -- must pass.
Run `ROOTDIR=/workspace go test -mod=mod -count=1 ./pkg/ops/... ./integration/...` -- must pass.
Confirm the CHANGELOG entry landed in the right section: `awk '/^## /{sec=$0} /COMPLETED_GOAL_MISSING_CREATED/{print "sits under: " sec}' CHANGELOG.md` must print `sits under: ## Unreleased`.

`make precommit` is deliberately not the verification command: its `make test` step runs `scripts/*.sh`, and this container's mawk 1.3.4 and loopback proxy interception fail several of those scripts for reasons unrelated to any Go change. The operator runs the full `make precommit` on the host after the prompt lands.
</verification>

Before you finish, re-run `<verification>` and confirm it passes; walk each acceptance criterion against the change.
