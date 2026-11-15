---
status: draft
created: "2026-11-15T22:20:00Z"
---

# Report an expired defer_date on an in_progress task as EXPIRED_DEFER_DATE

<summary>
- Adds one lint rule, `EXPIRED_DEFER_DATE`, the mirror of the existing `HOLD_WITH_DEFER_DATE`.
- The rule reports a task left at `status: in_progress` with a `defer_date` earlier than today — a state that is not deferred (the date has passed), not running (nothing moves it), and cannot be closed (`close-worker.py` refuses it `task-not-terminal`). It reads as live work while no rotation can reach it.
- The date test is date-only in local time, matching the vault-side regression probe (`defer_date < today`), so a defer later **today** is not expired.
- Exempts exactly the rows `HOLD_WITH_DEFER_DATE` exempts — `created_by: recurring-task-creator` and a non-empty `recurring:` key — because their `defer_date` is a schedule date, not a snooze.
- Touches no other lint rule and no CLI surface. Adds one unreleased changelog bullet; does not bump version strings or tag.
- Runs `make precommit` as final validation and records the gate's output.
</summary>

<objective>
`vault-cli task lint` reports a new non-fixable issue `EXPIRED_DEFER_DATE` for every task page whose frontmatter carries `status: in_progress` and a `defer_date` whose day is earlier than today, naming the task and the expired date, so an expired defer cannot persist unnoticed.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully:
- `pkg/ops/lint.go` — the whole file. The new rule mirrors the existing hold rule at three points: the `IssueType` const block, `holdWithDeferDateIssues` (the page-type gate) and `detectHoldWithDeferDate` (the detector), plus the append in `collectLintIssues`.
- `pkg/ops/lint_test.go` — the `Context("HOLD_WITH_DEFER_DATE", …)` block, to copy its fixture and assertion style.
- `pkg/ops/complete.go` — for the `libtime.CurrentDateTime` injection pattern (`c.currentDateTime.Now().Format("2006-01-02")` at the `today` read) and `pkg/ops/complete_test.go` for the pinned-clock test setup.

Read for context only (do not modify):
- `CHANGELOG.md` — read the top ~14 lines. There is no `## Unreleased` section today; the newest versioned heading was `## v0.194.0` when this prompt was written. Resolve it at run time.

⚠️ **Re-read every anchor below against the file before editing.** Line numbers and surrounding text drift; the anchors are quoted so they can be matched by content, not by number.

Background (why this rule, so it is not applied as a blind insert). The vault's task-status guidance already says a `defer_date` keeps a task `in_progress` and that `hold` is for a block with no scheduled resume date. It says nothing about a defer whose date has passed, and `HOLD_WITH_DEFER_DATE` lints the mirror case only. Measured 2026-11-08 across three vault task roots: 139 tasks read `status: in_progress` with a `defer_date` earlier than today; the population had drained to 1 by 2026-11-15, which is why the rule exists as a recurrence guard rather than a backlog repair. `close-worker.py`'s `task-not-terminal` detail is also shape-identical for an expired defer and for no defer at all, so neither instrument named the real reason.
</context>

<requirements>
1. **Add the issue type.** In `pkg/ops/lint.go`, inside the `const ( … )` block that declares `IssueTypeMissingFrontmatter` … `IssueTypeApprovedRecordInTodoInbox`, add one line immediately after `IssueTypeApprovedRecordInTodoInbox`:
   ```go
   	IssueTypeExpiredDeferDate            IssueType = "EXPIRED_DEFER_DATE"
   ```
   Keep the block gofmt-aligned (`gofmt` will fix the column; do not hand-align).

2. **Inject a clock without changing the existing constructor's callers.** Replace
   ```go
   // NewLintOperation creates a new lint operation.
   func NewLintOperation() LintOperation {
   	return &lintOperation{}
   }

   type lintOperation struct{}
   ```
   with
   ```go
   // NewLintOperation creates a new lint operation using the system clock.
   func NewLintOperation() LintOperation {
   	return NewLintOperationWithCurrentDateTime(libtime.NewCurrentDateTime())
   }

   // NewLintOperationWithCurrentDateTime creates a lint operation whose "today" is
   // supplied by the caller, so a date-sensitive rule is deterministic under test.
   func NewLintOperationWithCurrentDateTime(currentDateTime libtime.CurrentDateTime) LintOperation {
   	return &lintOperation{currentDateTime: currentDateTime}
   }

   type lintOperation struct {
   	currentDateTime libtime.CurrentDateTime
   }
   ```
   ⚠️ `NewLintOperation()` keeps its zero-argument signature on purpose: it has callers in `pkg/cli/cli.go` (two) and several `pkg/ops/*_test.go` files. Do not change those call sites.

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

4. **Add the detector.** Place it immediately after `detectHoldWithDeferDate` in `pkg/ops/lint.go`:
   ```go
   // detectExpiredDeferDate detects a task left at status in_progress with a defer_date
   // that has already passed. The state is a dead park: the date has gone, nothing moves
   // the task, and close-worker.py refuses to close it — so it reads as live work while
   // no rotation can reach it.
   //
   // The date test is date-only in local time, matching the vault-side regression probe
   // (`defer_date < today`), so a defer later TODAY is not expired. An unparseable value
   // is not reported: the rule cannot vouch for it, and STATUS_DATE_MISMATCH already
   // covers the malformed-date class.
   //
   // The two exemptions are copied deliberately from detectHoldWithDeferDate: a
   // recurring task's defer_date is a schedule date, not a snooze.
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
   	deferDate := strings.TrimSpace(fm.DeferDate)
   	if len(deferDate) < 10 {
   		return false, ""
   	}
   	if fm.CreatedBy == "recurring-task-creator" {
   		return false, ""
   	}
   	if strings.TrimSpace(fm.Recurring) != "" {
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

6. **Add the imports** `"time"` (stdlib) and `libtime "github.com/bborbe/time"` to `pkg/ops/lint.go`'s import block, matching the alias `pkg/ops/complete.go` already uses.

7. **Add tests** in `pkg/ops/lint_test.go`, in a new `Context("EXPIRED_DEFER_DATE", …)` placed immediately after the existing `Context("HOLD_WITH_DEFER_DATE", …)` block. Use a pinned clock, following `complete_test.go`:
   ```go
   	currentDateTime := libtime.NewCurrentDateTime()
   	currentDateTime.SetNow(libtimetest.ParseDateTime("2026-11-15T12:00:00Z"))
   	lintOp = ops.NewLintOperationWithCurrentDateTime(currentDateTime)
   ```
   Cover, at minimum, these six cases — each writes one task page with `writeSDM` and asserts on `ops.IssueTypeExpiredDeferDate`:
   - fires on `status: in_progress` + `defer_date: 2026-11-13` (past) — exactly 1 issue, `Fixable` false, description contains `in_progress`, `defer_date`, `2026-11-13` and `re-defer`;
   - fires on a quoted full timestamp `defer_date: "2026-11-13T18:10:00+01:00"` (the real-world shape) — exactly 1 issue;
   - does NOT fire on `defer_date: 2026-11-15` (today, the boundary) — 0 issues;
   - does NOT fire on `defer_date: 2026-11-15T23:00:00+01:00` (later today) — 0 issues;
   - does NOT fire on `defer_date: 2026-12-01` (future) — 0 issues;
   - does NOT fire when `created_by: recurring-task-creator` or when `recurring:` is non-empty, even with a past date — 0 issues.
   ⚠️ If the existing `Context("HOLD_WITH_DEFER_DATE", …)` setup already assigns `lintOp`, do not duplicate the variable declaration — read the surrounding `BeforeEach` first and adapt rather than shadowing.

8. **Record the change in the changelog.** `CHANGELOG.md` has no `## Unreleased` heading today. Add one directly above the **newest versioned heading** — resolve it at run time (it was `## v0.194.0` when this prompt was written), never a hardcoded literal. Under it add:
   ```markdown
   ## Unreleased

   - feat: report `EXPIRED_DEFER_DATE` when a task sits at `in_progress` with a past `defer_date`
   ```
   ⚠️ If a `## Unreleased` section already exists when you run, append the bullet as the last bullet inside the **existing** section — never create a second `## Unreleased` heading. Do NOT bump the plugin version strings and do NOT create a tag.

9. **Run the repository's full gate** as final validation; see `<verification>`.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- The change is additive: do not renumber or reword any existing lint rule, do not change `HOLD_WITH_DEFER_DATE`'s wording or behaviour, and do not alter `NewLintOperation()`'s signature.
- Do NOT touch the CLI surface, `commands/`, `agents/`, `docs/`, or the plugin manifests.
- Do NOT bump the plugin version strings in `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT create a tag — the release agent owns both after merge.
- Do NOT add a `--fix` path: the disposition is a judgement (finish / re-defer / hold), so the issue is non-fixable by construction.
</constraints>

<verification>
Run each; record the output verbatim in the report.

- `grep -c 'EXPIRED_DEFER_DATE' pkg/ops/lint.go` → 2 (the const and the `IssueTypeExpiredDeferDate` reference in the gate)
- `grep -n 'func (l \*lintOperation) detectExpiredDeferDate' pkg/ops/lint.go` → exactly 1 hit
- `grep -n 'l.expiredDeferDateIssues(pageType' pkg/ops/lint.go` → exactly 1 hit, on the line immediately after the `holdWithDeferDateIssues` append
- `grep -c 'Context("EXPIRED_DEFER_DATE"' pkg/ops/lint_test.go` → exactly 1
- `grep -c 'NewLintOperation()' pkg/cli/cli.go` → unchanged from before the edit (2 today)
- `grep -c '^## Unreleased' CHANGELOG.md` → exactly 1
- `make precommit` → exit 0

If `make precommit` fails, STOP and report `"status":"failed"` with the exact failing command and its output. Do not attempt a partial fix of unrelated pre-existing failures — report them and stop.
</verification>
