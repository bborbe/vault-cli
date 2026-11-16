---
status: completed
summary: Added the EXPIRED_DEFER_DATE lint rule (mirroring HOLD_WITH_DEFER_DATE) with an injected clock, tests, changelog and docs
execution_id: vault-cli-expired-defer-exec-282-expired-defer-date-lint
dark-factory-version: v0.199.1
created: "2026-11-15T22:20:00Z"
queued: "2026-11-16T00:55:58Z"
started: "2026-11-16T01:02:59Z"
completed: "2026-11-16T01:08:21Z"
---

# Report an expired defer_date on an in_progress task as EXPIRED_DEFER_DATE

<summary>
- Adds one lint rule, `EXPIRED_DEFER_DATE`, the mirror of the existing `HOLD_WITH_DEFER_DATE`.
- The rule reports a task left at `status: in_progress` with a `defer_date` earlier than today — a state that is not deferred (the date has passed), not running (nothing moves it), and cannot be closed (the session closer refuses it). It reads as live work while no rotation can reach it.
- The date test is date-only in local time, matching the vault-side regression probe (`defer_date < today`), so a defer later **today** is not expired.
- Recurring tasks are exempt — their defer date is a schedule date, not a snooze.
- Touches no other lint rule and no CLI surface. Adds one unreleased changelog bullet and one paragraph to the task-writing doc; does not bump version strings or tag.
- The full repository gate passes, so the rule ships with lint, vet and tests green.
</summary>

<objective>
`vault-cli task lint` reports a new non-fixable issue `EXPIRED_DEFER_DATE` for every task page whose frontmatter carries `status: in_progress` and a `defer_date` whose day is earlier than today, naming the task and the expired date, so an expired defer cannot persist unnoticed.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully:
- `pkg/ops/lint.go` — the whole file. The new rule mirrors the existing hold rule at three points: the `IssueType` const block, `holdWithDeferDateIssues` (the page-type gate) and `detectHoldWithDeferDate` (the detector), plus the append in `collectLintIssues`.
- `pkg/ops/lint_test.go` — the `Context("HOLD_WITH_DEFER_DATE", …)` block, to copy its fixture and assertion style.
- `pkg/ops/complete.go` — for the `libtime.CurrentDateTime` constructor-injection pattern and the `c.currentDateTime.Now().Format("2006-01-02")` read; `pkg/ops/complete_test.go` for the pinned-clock test setup.

Read for context only (do not modify):
- `CHANGELOG.md` — read the top ~14 lines. There is no `## Unreleased` section today; the newest versioned heading was `## v0.194.0` when this prompt was written. Resolve it at run time.
- `docs/task-writing.md` § "Calendar-as-commitment rule" — where the mirror `HOLD_WITH_DEFER_DATE` rule is documented at line ~527. Requirement 9 extends it. ⚠️ That same section states a `defer_date` *requires* `status: in_progress` — the exact shape this rule now reports as stale when the date passes, which is why the doc paragraph is part of this change rather than a follow-up.

⚠️ **Re-read every anchor below against the file before editing.** Line numbers and surrounding text drift; the anchors are quoted so they can be matched by content, not by number.

Background (why this rule, so it is not applied as a blind insert). The vault's task-status guidance already says a `defer_date` keeps a task `in_progress` and that `hold` is for a block with no scheduled resume date. It says nothing about a defer whose date has passed, and `HOLD_WITH_DEFER_DATE` lints the mirror case only. Measured 2026-11-08 across three vault task roots: 139 tasks read `status: in_progress` with a `defer_date` earlier than today; the population had drained to 1 by 2026-11-15, which is why the rule exists as a recurrence guard rather than a backlog repair.
</context>

<requirements>
1. **Add the issue type.** In `pkg/ops/lint.go`, inside the `const ( … )` block that declares `IssueTypeMissingFrontmatter` … `IssueTypeApprovedRecordInTodoInbox`, add one line immediately after `IssueTypeApprovedRecordInTodoInbox`:
   ```go
   	IssueTypeExpiredDeferDate            IssueType = "EXPIRED_DEFER_DATE"
   ```
   Keep the block gofmt-aligned (`gofmt` will fix the column; do not hand-align).

2. **Inject the clock through the constructor.** Replace
   ```go
   // NewLintOperation creates a new lint operation.
   func NewLintOperation() LintOperation {
   	return &lintOperation{}
   }

   type lintOperation struct{}
   ```
   with
   ```go
   // NewLintOperation creates a new lint operation whose "today" comes from the supplied
   // clock, so a date-sensitive rule is deterministic under test.
   func NewLintOperation(currentDateTime libtime.CurrentDateTime) LintOperation {
   	return &lintOperation{currentDateTime: currentDateTime}
   }

   type lintOperation struct {
   	currentDateTime libtime.CurrentDateTime
   }
   ```
   Then supply the clock at every call site, matching `pkg/ops/complete.go`'s constructor and `pkg/cli/cli.go`'s per-command pattern:
   - `pkg/cli/cli.go` — at the two `ops.NewLintOperation()` sites (in the `task validate` handler and in the `lint` handler), add `currentDateTime := libtime.NewCurrentDateTime()` in the same scope immediately before the call and pass it. `cli.go` already uses exactly this form elsewhere, so follow it rather than inventing a shared variable.
   - every `ops.NewLintOperation()` in `pkg/ops/*_test.go` (21 sites: 14 in `lint_test.go`, 6 in `lint_validate_exit_test.go`, 1 in `blocked_by_lint_test.go`) becomes `ops.NewLintOperation(libtime.NewCurrentDateTime())`, **except** the `BeforeEach` of `Describe("LintOperation - Status and Date Rules", …)` (lint_test.go ~line 2421): its fixtures hardcode `defer_date: 2026-12-01`, which stops being future-dated on 2026-12-01 and would start emitting `EXPIRED_DEFER_DATE` inside unrelated tests — pin it with `libtimetest.ParseDateTime("2026-11-15T12:00:00Z")` exactly as the new `Context` does.
   ⚠️ Do **not** keep a zero-argument `NewLintOperation()` that calls `libtime.NewCurrentDateTime()` internally. The `go-time/inject-getter-not-create` rule forbids a factory creating the clock, and `pkg/ops` contains no such call today — a factory-made clock would also make the new rule's tests non-deterministic for every other lint test.

3. **Add the page-type gate, mirroring `holdWithDeferDateIssues`.** Place it immediately after `holdWithDeferDateIssues` in `pkg/ops/lint.go`:
   ```go
   // expiredDeferDateIssues returns the EXPIRED_DEFER_DATE issue for a task page left at
   // in_progress with a defer_date whose day has already passed. The page-type gate lives
   // here for the same reason as the hold rule's: collectLintIssues is shared by six lint
   // commands, and the wording is task-specific.
   func (l *lintOperation) expiredDeferDateIssues(
   	pageType string,
   	filePath string,
   	frontmatterYAML string,
   ) []LintIssue {
   	if pageType == PageTypeTask {
   		if expired, desc := l.detectExpiredDeferDate(frontmatterYAML); expired {
   			return []LintIssue{{
   				FilePath:    filePath,
   				IssueType:   IssueTypeExpiredDeferDate,
   				Description: desc,
   				Fixable:     false,
   				Fixed:       false,
   			}}
   		}
   	}
   	return nil
   }
   ```

4. **Extract the shared exemption helper, then add the detector.** The two rules must not drift on what counts as a schedule date, so the exemption test lives in one place rather than being copied. (The 4-field frontmatter struct and its `yaml.Unmarshal` call are deliberately repeated from `detectHoldWithDeferDate`: that shared run is well under `dupl`'s default 150-token threshold, and a shared parser helper would obscure which fields each rule reads.) First add this package-level helper beside the two detectors:
   ```go
   // deferDateExempt reports whether a task's defer_date is a schedule date rather than a
   // snooze. Both defer-date rules share it: a recurring task's date is when the next
   // instance is due, so a past one is not a lapsed park.
   func deferDateExempt(createdBy, recurring string) bool {
   	return createdBy == "recurring-task-creator" || strings.TrimSpace(recurring) != ""
   }
   ```
   Then refactor `detectHoldWithDeferDate` to call it in place of its two inline conditions (same behaviour, no wording change), and add the new detector immediately after `detectHoldWithDeferDate`:
   ```go
   // detectExpiredDeferDate detects a task left at status in_progress with a defer_date
   // that has already passed. The state is a dead park: the date has gone, nothing moves
   // the task, and the session closer refuses to close it — so it reads as live work
   // while no rotation can reach it.
   //
   // The date test is date-only in local time, matching the vault-side regression probe
   // (`defer_date < today`), so a defer later TODAY is not expired. An unparseable value
   // is not reported: the rule cannot vouch for it, and STATUS_DATE_MISMATCH already
   // covers the malformed-date class.
   func (l *lintOperation) detectExpiredDeferDate(frontmatterYAML string) (bool, string) {
   	var fm struct {
   		Status    string `yaml:"status"`
   		DeferDate string `yaml:"defer_date"`
   		CreatedBy string `yaml:"created_by"`
   		Recurring string `yaml:"recurring"`
   	}
   	if err := yaml.Unmarshal([]byte(frontmatterYAML), &fm); err != nil {
   		return false, ""
   	}
   	if domain.TaskStatus(fm.Status) != domain.TaskStatusInProgress {
   		return false, ""
   	}
   	if deferDateExempt(fm.CreatedBy, fm.Recurring) {
   		return false, ""
   	}
   	deferDate := strings.TrimSpace(fm.DeferDate)
   	if len(deferDate) < 10 {
   		return false, ""
   	}
   	// A full RFC3339 instant is compared on its day, which is the probe's own rule.
   	parsed, err := time.Parse("2006-01-02", deferDate[:10])
   	if err != nil {
   		return false, ""
   	}
   	today, err := time.Parse("2006-01-02", l.currentDateTime.Now().Format("2006-01-02"))
   	if err != nil {
   		return false, ""
   	}
   	if !parsed.Before(today) {
   		return false, ""
   	}
   	return true, fmt.Sprintf(
   		"status is in_progress but defer_date %s has already passed — the task is neither deferred nor running; finish it, re-defer to a future date, or hold it with a reason",
   		deferDate,
   	)
   }
   ```

5. **Wire it into `collectLintIssues`.** Immediately after the line
   ```go
   	issues = append(issues, l.holdWithDeferDateIssues(pageType, filePath, frontmatterYAML)...)
   ```
   add
   ```go
   	issues = append(issues, l.expiredDeferDateIssues(pageType, filePath, frontmatterYAML)...)
   ```
   ⚠️ This one edit covers both `Execute` (the `task lint` path) and `ExecuteFile` (the `task validate` path) — both funnel through `collectLintIssues`. Do not add the call in two places.

6. **Add the imports** `"time"` (stdlib) and `libtime "github.com/bborbe/time"` to `pkg/ops/lint.go`'s import block, matching the alias `pkg/ops/complete.go` already uses.

7. **Add tests** in `pkg/ops/lint_test.go`, in a new `Context("EXPIRED_DEFER_DATE", …)` placed immediately after the existing `Context("HOLD_WITH_DEFER_DATE", …)` block. Use a pinned clock, following `complete_test.go`:
   ```go
   	currentDateTime := libtime.NewCurrentDateTime()
   	currentDateTime.SetNow(libtimetest.ParseDateTime("2026-11-15T12:00:00Z"))
   	lintOp = ops.NewLintOperation(currentDateTime)
   ```
   Add `libtime "github.com/bborbe/time"` and `libtimetest "github.com/bborbe/time/test"` to `pkg/ops/lint_test.go`'s import block (both already used by `pkg/ops/complete_test.go`).

   Write every fixture with `page_type: task` and a **UUID** `task_identifier` (e.g. `11111111-1111-4111-a111-111111111111`) so `INVALID_TASK_IDENTIFIER` cannot fire — the HOLD block's fixtures use non-UUID identifiers, and copying that shape here would make every count assertion below fail for an unrelated reason. Count by `IssueType` the way `countHoldIssues` does, never by total issue count.

   Cover, at minimum:
   - fires on `status: in_progress` + `defer_date: 2026-11-13` (past) — exactly 1 `EXPIRED_DEFER_DATE` issue, `Fixable` false, description contains `in_progress`, `defer_date`, `2026-11-13` and `re-defer`;
   - fires on a quoted full timestamp `defer_date: "2026-11-13T18:10:00+01:00"` (the real-world shape) — exactly 1;
   - does NOT fire on `defer_date: 2026-11-15` (today, the boundary) — 0;
   - does NOT fire on `defer_date: 2026-11-15T23:00:00+01:00` (later today) — 0;
   - does NOT fire on `defer_date: 2026-12-01` (future) — 0;
   - does NOT fire on a value shorter than 10 characters (`defer_date: 2026`) — 0;
   - does NOT fire on an unparseable 10-character value (`defer_date: 2026-13-45`) — 0;
   - does NOT fire when `created_by: recurring-task-creator` or when `recurring:` is non-empty, even with a past date — 0 each;
   - **`ExecuteFile`** (the `task validate` path) surfaces the same issue with the identical description string `Execute` produces;
   - **task-scoping:** `Execute` with `ops.PageTypeGoal` over the same firing fixture reports 0 `EXPIRED_DEFER_DATE`, while `PageTypeTask` reports 1;
   - **mutual exclusion:** `status: hold` + a past `defer_date` reports `HOLD_WITH_DEFER_DATE` and 0 `EXPIRED_DEFER_DATE`;
   - **non-fixable in practice:** the file is byte-identical after `Execute(..., fix=true)`;
   - **`--fix` can create the state:** `status: next` + a past `defer_date` is promoted to `in_progress` by the `STATUS_DATE_MISMATCH` fix, and a second `Execute` then reports exactly 1 `EXPIRED_DEFER_DATE` — assert both halves so the pair is locked rather than "fixed" by weakening the promotion.
   ⚠️ If the existing `Context("HOLD_WITH_DEFER_DATE", …)` setup already assigns `lintOp`, do not duplicate the variable declaration — read the surrounding `BeforeEach` first and adapt rather than shadowing.

8. **Record the change in the changelog.** `CHANGELOG.md` has no `## Unreleased` heading today. Add one directly above the **newest versioned heading** — resolve it at run time (it was `## v0.194.0` when this prompt was written), never a hardcoded literal. Under it add:
   ```markdown
   ## Unreleased

   - feat: report `EXPIRED_DEFER_DATE` when a task sits at `in_progress` with a past `defer_date`

   - feat: `ops.NewLintOperation` now takes a `libtime.CurrentDateTime` so the new rule's "today" is injectable
   ```
   ⚠️ If a `## Unreleased` section already exists when you run, append the bullet as the last bullet inside the **existing** section — never create a second `## Unreleased` heading. Do NOT bump the plugin version strings and do NOT create a tag.

9. **Document the rule where the mirror rule is documented.** In `docs/task-writing.md` § "Calendar-as-commitment rule", immediately after the existing `HOLD_WITH_DEFER_DATE` paragraph (the one ending "…does not apply to goal / topic / theme / objective / vision pages."), add one paragraph:
   ```markdown
   A task left at `status: in_progress` with a `defer_date` earlier than today is reported by `task lint` / `task validate` as `EXPIRED_DEFER_DATE`. The date test is date-only in local time, so a defer later today is not expired. The finding is not auto-fixable — the disposition is a judgement (finish the task, re-defer to a future date, or hold it with a reason) — and it is task-scoped, like the hold rule. The same two exemptions apply: `created_by: recurring-task-creator` and a non-empty `recurring:` key, whose `defer_date` is a schedule date rather than a snooze.
   ```

10. **Run the repository's full gate** as final validation; see `<verification>`. **Self-check before you finish:** re-run `<verification>` and confirm every line passes, then walk each test case in requirement 7 against the code you wrote and confirm each one is actually asserted — do not report success from reading the diff.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- The change is additive: do not renumber or reword any existing lint rule, do not change `HOLD_WITH_DEFER_DATE`'s wording or behaviour (the shared-helper refactor in requirement 4 must leave its output byte-identical), and do not keep a clock-creating zero-argument constructor.
- Do NOT touch the CLI surface beyond the two `NewLintOperation` call sites in `pkg/cli/cli.go`, and do NOT touch `commands/`, `agents/`, or the plugin manifests. `docs/task-writing.md` **is** in scope — requirement 9 extends it.
- Do NOT bump the plugin version strings in `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT create a tag — the release agent owns both after merge.
- Do NOT add a `--fix` path: the disposition is a judgement (finish / re-defer / hold), so the issue is non-fixable by construction.
- If `make test` now fails because an existing fixture sits at `status: in_progress` with a non-exempt past `defer_date`, that is a consequence of this change — update that fixture's expectation. It is not an unrelated pre-existing failure.
</constraints>

<verification>
Run each; record the output verbatim in the report.

- `grep -c 'IssueTypeExpiredDeferDate' pkg/ops/lint.go` → 2 (the const declaration and the gate's `IssueType:` reference)
- `grep -n 'func (l \*lintOperation) detectExpiredDeferDate' pkg/ops/lint.go` → exactly 1 hit
- `grep -n 'l.expiredDeferDateIssues(pageType' pkg/ops/lint.go` → exactly 1 hit, on the line immediately after the `holdWithDeferDateIssues` append
- `grep -c 'deferDateExempt(' pkg/ops/lint.go` → 3 (the declaration plus both detectors' call sites)
- `grep -c 'Context("EXPIRED_DEFER_DATE"' pkg/ops/lint_test.go` → exactly 1
- `! grep -q 'ops.NewLintOperation()' pkg/ops/*_test.go pkg/cli/cli.go` → succeeds (every call site now passes a clock)
- `grep -c '^## Unreleased' CHANGELOG.md` → exactly 1
- `grep -c 'EXPIRED_DEFER_DATE' docs/task-writing.md` → exactly 1
- `make precommit` → exit 0

If the gate fails, STOP and report `"status":"failed"` with the exact failing command and its output. Do not attempt a partial fix of unrelated pre-existing failures — report them and stop.
</verification>
