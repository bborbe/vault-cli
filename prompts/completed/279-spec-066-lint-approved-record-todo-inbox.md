---
status: completed
spec: [066-bug-approved-record-todo-inbox-deadlock]
summary: Added the fixable APPROVED_RECORD_IN_TODO_INBOX lint rule, its phase todo→planning fixer, terminal-row non-fixable handling, tests, and a changelog entry.
execution_id: vault-cli-approved-todo-exec-279-spec-066-lint-approved-record-todo-inbox
dark-factory-version: v0.198.17
created: "2026-11-08T18:04:30Z"
queued: "2026-11-08T18:15:49Z"
started: "2026-11-08T18:24:29Z"
completed: "2026-11-08T18:33:09Z"
branch: dark-factory/bug-approved-record-todo-inbox-deadlock
---

# Report and repair an approval record stuck at phase todo in `task lint` (spec 066, prompt 2 of 3)

<summary>
- `vault-cli task lint` reports one new finding, `APPROVED_RECORD_IN_TODO_INBOX`, for a task that carries an approval record while its phase is still todo.
- The finding is marked fixable on a non-terminal row, so the plain-output line is prefixed `WARN` and `task lint --fix` is offered as the repair.
- The fix advances the phase to planning and leaves the approval record exactly as it is, matching what `task approve` now does on the same row.
- A second lint run after the fix reports zero such findings, because the phase is no longer todo.
- A `completed` or `aborted` row is reported as non-fixable (`ERROR`) and the description names the terminal status, so a finished row is never silently re-opened by a bulk fix.
- The rule is task-scoped: goal and topic lint runs do not report it, and the JSON lint contract is untouched.
- No other lint rule's behaviour, wording, or fixability changes.
</summary>

<objective>
Add a fixable `APPROVED_RECORD_IN_TODO_INBOX` lint rule that reports a task carrying `approved_by` and/or `approved_at` at `phase: todo` and, when non-terminal, repairs it by advancing the phase to `planning` while preserving the record verbatim. This makes the deadlocked state both visible and repairable in bulk, reusing the phase-advance behaviour established in prompt 1. This is spec 066 prompt 2 of 3: it covers Desired Behaviors 4–5 and Acceptance Criteria 7–9.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then `docs/dod.md` for the definition of done this change is graded against.

Read these sections of `pkg/ops/lint.go` (1250 lines — do not read the whole file in one call):

- lines 22–49 — the package-level compiled regexes (`phaseRegex`, `statusRegex`, `completedStatusRegex`).
- lines 94–134 — the `IssueType` constants, `PageTypeTask` / `PageTypeGoal`, and the `LintIssue` struct.
- lines 285–383 — `collectLintIssues`, the shared rule list. Note the existing pattern at line 337 (`issues = append(issues, l.holdWithDeferDateIssues(...)...)`) and line 380 (`l.taskIdentifierIssues(...)`): a rule that is task-scoped and needs a page-type gate is its own helper returning `[]LintIssue`, appended at the call site.
- lines 732–800 — `detectStatusPhaseMismatch`, the existing phase/status rule, for the phase-normalisation idiom.
- lines 820–860 — `holdWithDeferDateIssues`, the helper shape to mirror (page-type gate inside the helper, one issue, `Fixable: false`).
- lines 965–1025 — `fixIssues`, the fixer dispatch switch that the new fixable issue type must be added to.

Read `pkg/cli/cli.go` lines 748–810 — `lintIssueTypeName` maps `Fixable && !fix` to `WARN` and everything else to `ERROR`. This is why `Fixable` on the issue is the only thing the rule needs to set; no CLI change is required.

Read `pkg/ops/lint_test.go` lines 1–70 for the top-level suite setup, and lines 2755–2800 for a recent rule's test shape (a rule that asserts on `Fixable` and on substrings of `Description`).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega suite files and external test packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface/constructor/struct layout already used by this file.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-linting-guide.md` — `funlen` 80 lines / 50 statements, `nestif` 4; this is why the new rule is a helper rather than inline code in `collectLintIssues`.
</context>

<requirements>
Before editing, confirm prompt 1 shipped:

```
grep -c 'hasApprovalRecord' pkg/ops/task_approve.go   # must be >= 3
```

If the check fails, STOP and report `"status":"failed"` with message `"spec-066 prompt 2 precondition missing: prompt 1 not yet deployed"`. This prompt's fixer reproduces the phase advance at the file level and does not call the approve operation, so the code is independent — but the issue description this prompt ships asserts `task approve`'s behaviour, so it must not ship ahead of it.

1. **Add the issue type.** In the `IssueType` const block in `pkg/ops/lint.go` (lines 97–112), add:

   ```go
   	IssueTypeApprovedRecordInTodoInbox    IssueType = "APPROVED_RECORD_IN_TODO_INBOX"
   ```

   Keep the existing gofmt alignment of the block.

2. **Add the key-presence regex.** In the package-level regex var block (lines 23–49), add:

   ```go
   	// approvedRecordRegex matches the key line, not its value: an empty
   	// `approved_by:` still records an approval, so presence is what matters.
   	approvedRecordRegex = regexp.MustCompile(`(?m)^approved_(by|at):`)
   ```

3. **Add the rule helper, mirroring `holdWithDeferDateIssues`.** Place it next to `holdWithDeferDateIssues` (around line 843) and implement exactly:

   ```go
   // approvedRecordInTodoInboxIssues returns the APPROVED_RECORD_IN_TODO_INBOX
   // issue for a task carrying an approval record while its phase is still todo —
   // the state in which `task approve` used to refuse and every other verb refused
   // the phase. The rule is task-scoped: goal and topic approval carry the same
   // guard pair, but they are out of scope for this spec.
   //
   // A terminal row (status completed or aborted) is reported non-fixable: advancing
   // its phase would produce the `status: completed` + `phase: planning` pair that
   // checkPhaseRegression exists to refuse. The description names the status so the
   // operator can decide whether to re-open the row deliberately.
   func (l *lintOperation) approvedRecordInTodoInboxIssues(
   	pageType string,
   	filePath string,
   	frontmatterYAML string,
   ) []LintIssue {
   	if pageType != PageTypeTask {
   		return nil
   	}

   	phaseMatches := phaseRegex.FindStringSubmatch(frontmatterYAML)
   	if len(phaseMatches) < 2 {
   		return nil
   	}
   	phase, ok := domain.NormalizeTaskPhase(phaseMatches[1])
   	if !ok || phase != domain.TaskPhaseTodo {
   		return nil
   	}

   	if !approvedRecordRegex.MatchString(frontmatterYAML) {
   		return nil
   	}

   	// completedStatusRegex is used instead of statusRegex because it tolerates a
   	// trailing YAML comment. A terminal row must never be classified as fixable,
   	// so the tolerant matcher is the safe one here.
   	statusMatches := completedStatusRegex.FindStringSubmatch(frontmatterYAML)
   	if len(statusMatches) >= 2 {
   		status := domain.TaskStatus(statusMatches[1])
   		if status == domain.TaskStatusCompleted || status == domain.TaskStatusAborted {
   			return []LintIssue{{
   				FilePath:  filePath,
   				IssueType: IssueTypeApprovedRecordInTodoInbox,
   				Description: fmt.Sprintf(
   					"task carries an approval record at phase %q but status is %q; advancing the phase would re-open terminal work — re-open it deliberately with `vault-cli task set <name> phase %s`",
   					string(domain.TaskPhaseTodo), status, string(domain.TaskPhasePlanning),
   				),
   				Fixable: false,
   				Fixed:   false,
   			}}
   		}
   	}

   	return []LintIssue{{
   		FilePath:  filePath,
   		IssueType: IssueTypeApprovedRecordInTodoInbox,
   		Description: fmt.Sprintf(
   			"task carries an approval record at phase %q (the approval inbox); `vault-cli task approve` advances it to %q",
   			string(domain.TaskPhaseTodo), string(domain.TaskPhasePlanning),
   		),
   		Fixable: true,
   		Fixed:   false,
   	}}
   }
   ```

4. **Register the rule in `collectLintIssues`.** Immediately after the `holdWithDeferDateIssues` append at line 337, add:

   ```go
   	// A task carrying an approval record at phase todo is approved but stuck in
   	// the approval inbox — see approvedRecordInTodoInboxIssues.
   	issues = append(issues, l.approvedRecordInTodoInboxIssues(pageType, filePath, frontmatterYAML)...)
   ```

   Do not add the check inline: `collectLintIssues` is already near the `funlen` threshold.

5. **Add the fixer and dispatch it.** Add next to the other fixers (e.g. after `fixStatusDateMismatch`, around line 1185):

   ```go
   // fixApprovedRecordInTodoInbox advances phase todo -> planning in place,
   // leaving every other byte — including the approval record — untouched. It
   // rewrites every line matching the phase key and never re-serialises the
   // block; like fixInvalidStatus it is not frontmatter-scoped, so a body line
   // reading `phase: todo` is rewritten too.
   func (l *lintOperation) fixApprovedRecordInTodoInbox(content string) (string, bool) {
   	matches := phaseRegex.FindStringSubmatch(content)
   	if len(matches) < 2 {
   		return content, false
   	}
   	phase, ok := domain.NormalizeTaskPhase(matches[1])
   	if !ok || phase != domain.TaskPhaseTodo {
   		return content, false
   	}
   	return phaseRegex.ReplaceAllString(
   		content,
   		"phase: "+string(domain.TaskPhasePlanning),
   	), true
   }
   ```

   In the `fixIssues` switch (lines 1003–1014), add:

   ```go
   		case IssueTypeApprovedRecordInTodoInbox:
   			apply(i, l.fixApprovedRecordInTodoInbox)
   ```

   The terminal arm is never fixed because `fixIssues` skips `!issues[i].Fixable` before the switch.

6. **Add tests as a new independent suite at the end of `pkg/ops/lint_test.go`.** The file's existing helper closures are scoped to other `Context` blocks, so create a self-contained `Describe`:

   ```go
   var _ = Describe("LintOperation APPROVED_RECORD_IN_TODO_INBOX", func() {
   	var (
   		ctx       context.Context
   		lintOp    ops.LintOperation
   		vaultPath string
   		tasksDir  string
   	)

   	BeforeEach(func() {
   		ctx = context.Background()
   		lintOp = ops.NewLintOperation()
   		var err error
   		vaultPath, err = os.MkdirTemp("", "vault-approved-record-*")
   		Expect(err).To(BeNil())
   		tasksDir = "Tasks"
   		Expect(os.MkdirAll(filepath.Join(vaultPath, tasksDir), 0755)).To(Succeed())
   	})

   	AfterEach(func() {
   		if vaultPath != "" {
   			_ = os.RemoveAll(vaultPath)
   		}
   	})

   	writeTask := func(name, frontmatter string) string {
   		path := filepath.Join(vaultPath, tasksDir, name)
   		Expect(os.WriteFile(path, []byte("---\n"+frontmatter+"---\n# Test\n"), 0600)).To(Succeed())
   		return path
   	}

   	countIssues := func(issues []ops.LintIssue) int {
   		n := 0
   		for _, i := range issues {
   			if i.IssueType == ops.IssueTypeApprovedRecordInTodoInbox {
   				n++
   			}
   		}
   		return n
   	}

   	// ... the It blocks below ...
   })
   ```

   Required `It` blocks (each task fixture needs a `task_identifier` only if you want to avoid unrelated identifier issues — the count helper ignores them either way, so keep fixtures minimal and assert only on this issue type):

   Every "one issue" below means **one issue of this type** — the `countIssues` helper filters by `IssueTypeApprovedRecordInTodoInbox`, because these minimal fixtures also fire other rules (`MISSING_TASK_IDENTIFIER` on every one, and `STATUS_PHASE_MISMATCH` on the `completed` fixture). Do not assert on the total issue count.

   - "reports a fixable issue for approved_by at phase todo": frontmatter `status: next\nphase: todo\napproved_by: someone\n`; expect exactly one issue of this type, `Fixable` true, `Fixed` false, description containing `approval record` and `planning`.
   - "reports a fixable issue for approved_at at phase todo": frontmatter `status: next\nphase: todo\napproved_at: 2026-01-01T00:00:00Z\n`; one fixable issue.
   - "treats an empty approved_by value as present": frontmatter `status: next\nphase: todo\napproved_by:\n`; one fixable issue.
   - "does not fire at phase planning": frontmatter `status: next\nphase: planning\napproved_by: someone\napproved_at: 2026-01-01T00:00:00Z\n`; zero issues.
   - "does not fire with no approval record": frontmatter `status: next\nphase: todo\n`; zero issues.
   - "does not fire on a goal page": write the same file under `tasksDir`, call `lintOp.Execute(ctx, ops.PageTypeGoal, vaultPath, tasksDir, tasksDir, false)`, expect zero of this issue type.
   - "reports a completed row as non-fixable and names the status": frontmatter `status: completed\nphase: todo\napproved_by: someone\n`; one issue, `Fixable` false, description containing `completed`.
   - "reports an aborted row as non-fixable and names the status": frontmatter `status: aborted\nphase: todo\napproved_by: someone\n`; one issue, `Fixable` false, description containing `aborted`.
   - "fix advances phase to planning and preserves the record": write `status: next\nphase: todo\napproved_by: someone\napproved_at: 2026-01-01T00:00:00Z\n`; run `Execute(..., true)`; expect the issue `Fixed` true; read the file back and assert it contains `phase: planning`, `approved_by: someone`, and `approved_at: 2026-01-01T00:00:00Z`, and no longer contains `phase: todo`.
   - "a second lint run reports nothing after the fix": after the fix above, run `Execute(..., false)` again and expect zero of this issue type.
   - "fix leaves a terminal row untouched": write `status: completed\nphase: todo\napproved_by: someone\n`, capture the file bytes, run `Execute(..., true)`, expect one non-fixed issue and the file bytes identical to before.

7. **Do not widen the JSON contract.** `LintIssueJSON` and `ValidateIssueJSON` stay as they are — the spec's Non-goal forbids adding `issue_type` to the lint JSON output. The rule is verified through plain output and through `Fixable`/`Fixed` in the operation's result.

   Note: `task validate` calls `lintOp.ExecuteFile`, which runs this same rule list, so the new finding also appears in `task validate` plain and JSON output automatically through the existing `ValidateIssueJSON.issue_type` field. That is expected and needs no code change — do not suppress it.
</requirements>

<constraints>
- Copied from the spec: `todoPlanningSetRefusal` and `checkPhaseRegression` are unchanged.
- Copied from the spec: the fix writes no `approved_by` / `approved_at` value and never completes a partial record.
- Copied from the spec: the rule applies only to a non-terminal row; a `status: completed` or `aborted` row is reported non-fixable with its status named.
- Copied from the spec: no `issue_type` in the lint JSON output; no new CLI verb, subcommand or flag.
- Copied from the spec: no `go.mod` / `go.sum` change.
- Goal and topic rows are out of scope (spec Non-goal) — the page-type gate must keep them out of this rule.
- Do NOT commit — dark-factory handles git.
- Existing lint tests must still pass; the new rule must not change any other rule's output.
- Use `github.com/bborbe/errors` for errors in `pkg/`; never `fmt.Errorf` for error construction (`fmt.Sprintf` for descriptions is fine and is already the file's idiom).
</constraints>

<verification>
Run `make test` — the Ginkgo suite must pass, including the new `Describe`.

Run `make precommit` once at the end — it must exit 0.

Then confirm the rule is present:

```
grep -n 'APPROVED_RECORD_IN_TODO_INBOX' pkg/ops/lint.go        # the IssueType constant
grep -c 'approvedRecordInTodoInboxIssues' pkg/ops/lint.go      # expect >= 3 (declaration, doc comment, call site)
grep -c 'IssueTypeApprovedRecordInTodoInbox' pkg/ops/lint.go   # expect >= 4 (const + two returns + switch case)
```

Emit no `git` command; this prompt is verified by build and test only.
</verification>
