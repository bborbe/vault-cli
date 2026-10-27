---
status: approved
spec: [064-bug-completed-goal-keeps-non-terminal-phase]
created: "2026-10-27T23:05:49Z"
queued: "2026-10-27T23:19:44Z"
branch: dark-factory/bug-completed-goal-keeps-non-terminal-phase
---

# Accept `phase: done` on an aborted goal, and on nothing else

<summary>
- A goal at `status: aborted` with `phase: done` is no longer reported as a status/phase mismatch.
- A goal at `status: completed` with `phase: done` stays clean, exactly as before.
- A goal at `status: completed` or `status: aborted` with any other phase is still reported, with the same issue type and a message naming both values.
- A goal with no `phase` key stays exempt, exactly as before.
- A goal at `status: in_progress` with `phase: done` is still reported, so the widening does not disable the detector.
- The five status/phase combinations are pinned by a table-driven test written against goal files, not just task files.
- Nothing else about the lint rule moves: the issue type, its fixability, and the other rules are unchanged.
</summary>

<objective>
Widen lint Rule 2 so `phase: done` is accepted on a terminal status (`completed` or `aborted`) and on nothing else, so an operator who repairs an aborted goal to `phase: done` no longer trades a Rule-1 error for a Rule-2 error. The CLI now produces that state (prompt 1) and the lint must accept it, or the invariant is self-contradictory. This is spec 064 prompt 2 of 3: it covers Desired Behavior 3 and Acceptance Criteria 8–10.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then `docs/dod.md` for the definition of done this change is graded against.

Read these files before changing anything:

- `pkg/ops/lint.go` — `detectStatusPhaseMismatch(frontmatterYAML string) (bool, string)` is the shared status/phase rule. Its structure is:
  1. early return when the `phase` key is absent (the absent-phase exemption);
  2. Rule 1: `completed` / `aborted` requires `phase == done`, message `"status is %s but phase is %s (expected done or no phase)"`;
  3. Rule 2: `phase == done` requires `status == completed`, message `"phase is done but status is %s (expected completed)"`;
  4. Rule 3: `backlog` / `hold` with an active phase.

  `IssueTypeStatusPhaseMismatch IssueType = "STATUS_PHASE_MISMATCH"` is the issue type; `collectLintIssues` adds the rule's result with `fixable` set to `false`. The rule reads `phaseRegex` and `statusRegex` and casts to `domain.TaskPhase` / `domain.TaskStatus`; it is applied to every page type, goals included (`createGenericLintCommand` in `pkg/cli/cli.go` passes the page type and the page directory to the same `LintOperation`).
- `pkg/ops/lint_test.go` — `Context("STATUS_PHASE_MISMATCH — should trigger")` and `Context("STATUS_PHASE_MISMATCH — should NOT trigger")` are the existing rule tables. They write task files with the local `writeTask` helper and call `lintOp.Execute(ctx, ops.PageTypeTask, vaultPath, tasksDir, "", false)`. Note that the existing "should NOT trigger" table only asserts that `err` is nil — it does not assert the issue list is empty. Your new table must assert the issue list itself.
- `pkg/ops/lint.go` `LintIssue` — fields `FilePath`, `IssueType`, `Description`, `Fixable`, `Fixed`.
- `pkg/ops/lint.go` `LintOperation.Execute(ctx, pageType, vaultPath, tasksDir, goalsDir, fix)` — the third argument is the directory to walk (for a goal run it is the goals directory); the fourth is the goals directory used by orphan-goal detection. Pass the goals directory for both when linting goals.
- `pkg/cli/cli.go` `printLintIssuesPlain` — the CLI exits non-zero when any issue is unfixed, which is how Acceptance Criteria 8–10's exit codes are produced. That layer is unchanged by this prompt.
- `specs/in-progress/064-bug-completed-goal-keeps-non-terminal-phase.md` — read Desired Behavior 3, Acceptance Criteria 8–10, Non-goals (the two "Do NOT" bullets), Constraints and Failure Modes.

Coding-plugin docs (in-container paths):

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, `DescribeTable` / `Entry`, external test packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — error wrapping and package conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits (funlen 80, nestif 4, golines 100).
</context>

<requirements>
1. **Widen Rule 2 only — `pkg/ops/lint.go` `detectStatusPhaseMismatch`.** Change the Rule 2 condition so `phase == domain.TaskPhaseDone` is accepted when the status is `completed` **or** `aborted`:

   ```go
   // Rule 2: phase=done requires a terminal status (completed or aborted)
   if phase == domain.TaskPhaseDone &&
       status != domain.TaskStatusCompleted && status != domain.TaskStatusAborted {
       return true, fmt.Sprintf(
           "phase is done but status is %s (expected completed or aborted)",
           status,
       )
   }
   ```

   Update the comment above the branch to say "terminal status (completed or aborted)". The message keeps naming both values and keeps the `phase is done but status is %s` prefix — Acceptance Criterion 10 matches on `phase is done but status is in_progress`.

2. **Everything else in the rule is unchanged.** Do not touch the absent-phase early return, Rule 1 (its condition and its `"status is %s but phase is %s (expected done or no phase)"` message), Rule 3, `IssueTypeStatusPhaseMismatch`, or the `false` fixability passed by `collectLintIssues`. Do not add a second issue type for the same condition.

3. **Do not add a blanket `phase == done ⇒ no issue` short-circuit.** A goal at `status: in_progress` with `phase: done` must still be reported. This is the scoping lock the spec calls out: a short-circuit would satisfy the aborted and absent-phase cases and silently stop flagging a real inconsistency.

4. **Goal-flavored table tests — `pkg/ops/lint_test.go`.** Add a new top-level `Describe` (for example `Describe("LintOperation - STATUS_PHASE_MISMATCH on goal pages")`) with its own `BeforeEach`/`AfterEach` that creates a temp vault with a `Goals` directory via `os.MkdirTemp` and `os.MkdirAll`, mirroring the setup of the existing mismatch suite. Add a local helper that writes `Goals/Goal.md` from a status and an optional phase:

   - with a phase: `"---\nstatus: " + status + "\nphase: " + phase + "\npage_type: goal\n---\n# Goal\n"`
   - without a phase: `"---\nstatus: " + status + "\npage_type: goal\n---\n# Goal\n"`

   Call `lintOp.Execute(ctx, "goal", vaultPath, "Goals", "Goals", false)` and assert on the returned `[]ops.LintIssue`, not on the error alone.

   Add a `DescribeTable` for the cases that must stay clean, asserting `Expect(issues).To(BeEmpty())`:

   - `Entry("aborted + done", "aborted", "done")` — Acceptance Criterion 8, first half: this is the Rule-2 widening.
   - `Entry("completed + no phase", "completed", "")` — Acceptance Criterion 9, first half: the absent-phase exemption is unchanged.

   Add a `DescribeTable` for the cases that must still report, asserting `Expect(issues).To(HaveLen(1))`, `Expect(issues[0].IssueType).To(Equal(ops.IssueTypeStatusPhaseMismatch))`, `Expect(issues[0].Fixable).To(BeFalse())`, and `Expect(issues[0].Description).To(ContainSubstring(<expected>))`:

   - `Entry("aborted + execution", "aborted", "execution", "status is aborted but phase is execution")` — Acceptance Criterion 8, second half: Rule 1 still fires, so the widening cannot disable the detector.
   - `Entry("completed + execution", "completed", "execution", "status is completed but phase is execution")` — Acceptance Criterion 9, second half.
   - `Entry("in_progress + done", "in_progress", "done", "phase is done but status is in_progress")` — Acceptance Criterion 10: the scoping lock.

   Use the two-table shape so the "must stay clean" rows assert emptiness rather than only a nil error.

5. **CLI-level coverage for Acceptance Criteria 8–10 — `integration/cli_test.go`.** The ACs are worded on `goal lint` **exit codes** and stdout lines, so detector-level tests alone do not evidence them. Add one `Describe("vault-cli goal lint — status/phase on goals", ...)` beside the existing goal-lint suite, reusing that file's own helpers (`createTempVaultWithGoals`, `cleanup()` in `AfterEach`, `gexec.Start`/`Eventually(session).Should(gexec.Exit(<code>))`, and `session.Out.Contents()`), exactly as the neighbouring suites do. One `It` per case, each with a single-goal vault:
   - `status: aborted` / `phase: done` → exit **0**, and the output contains no `STATUS_PHASE_MISMATCH` line (AC 8, first half).
   - `status: aborted` / `phase: execution` → exit **1**, exactly one line matching `status is aborted but phase is execution` (AC 8, second half).
   - `status: completed` with **no** `phase` key → exit **0**, no `STATUS_PHASE_MISMATCH` line (AC 9, first half).
   - `status: completed` / `phase: execution` → exit **1**, exactly one line matching `status is completed but phase is execution` (AC 9, second half).
   - `status: in_progress` / `phase: done` → exit **1**, exactly one line matching `phase is done but status is in_progress` (AC 10).
   The suite runs under `go test ./...`, so `make precommit` executes it — no `<verification>` change is needed.

6. **Leave the existing task tables as they are.** None of them contains `aborted + done`, so no existing row changes meaning — but add `Entry("aborted + done", "aborted", "done")` to the task "should NOT trigger" table and switch that table's assertion from `_, err := ...` to `issues, err := ...` with `Expect(issues).To(BeEmpty())`, because the rule is shared and a task at that pair becomes lint-clean through this prompt. Without the emptiness assertion the row proves nothing — that table only checks `err` today. Do not add task-specific code for it; it follows from the shared rule.

7. **Self-check.** Before finishing, re-run `<verification>` and confirm it passes; walk each requirement above and Acceptance Criteria 8–10 of the spec against the change. Confirm the writer-side change is out of scope here — `pkg/ops/goal_complete.go` and `pkg/ops/frontmatter_entity.go` belong to prompt 1.
</requirements>

<constraints>
- Do NOT widen Rule 2 beyond `aborted`: a blanket "phase done ⇒ no issue" short-circuit would stop flagging `phase: done` on an `in_progress` goal, which is a real inconsistency.
- Rule 1 and the absent-phase early return are unchanged; only Rule 2's status test widens, and only for `aborted`.
- `STATUS_PHASE_MISMATCH` keeps its existing issue type and message vocabulary — no second issue type for the same condition.
- Do NOT modify `pkg/ops/goal_complete.go` or `pkg/ops/frontmatter_entity.go` in this prompt — the write-path change is prompt 1's scope.
- Do NOT change anything about topic phase, and do not change the `todo → planning` refusal or the approval record.
- Tests follow repository convention: Ginkgo v2 / Gomega, `DescribeTable` / `Entry` for table-driven rows, external test package (`package ops_test`).
- Existing tests must still pass. Do NOT commit — dark-factory handles git.
</constraints>

<verification>
make precommit

# Rule 2 now treats aborted as terminal
grep -q 'status != domain.TaskStatusAborted' pkg/ops/lint.go
grep -q 'expected completed or aborted' pkg/ops/lint.go

# the widening is present in both terminal checks (Rule 1 + widened Rule 2)
test "$(grep -c 'domain.TaskStatusAborted' pkg/ops/lint.go)" -ge 2

# the absent-phase early return is intact
grep -q 'No phase key — no validation' pkg/ops/lint.go
</verification>

<!-- DARK-FACTORY-REPORT -->
