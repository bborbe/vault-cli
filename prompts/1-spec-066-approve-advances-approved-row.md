---
status: draft
spec: [066-bug-approved-record-todo-inbox-deadlock]
created: "2026-11-08T18:04:30Z"
branch: dark-factory/bug-approved-record-todo-inbox-deadlock
---

# Make `task approve` advance an already-approved row instead of refusing it (spec 066, prompt 1 of 3)

<summary>
- Approving a task that already carries an approval record now succeeds and moves it out of the approval inbox to planning, instead of refusing.
- The existing approver identity and timestamp are preserved exactly as they are: no second approval is recorded, and no missing key is invented.
- A partially recorded approval (only one of the two keys, or one key with an empty value) is advanced the same way.
- The repair does not resolve, require, or write an owner, so an ownerless row that was already approved can leave the inbox.
- Every existing refusal is unchanged: a row past the inbox, a blank approver, and a genuinely unowned row that carries no approval record.
- A row past the inbox that carries a record is still refused by the phase guard, so the repair only applies inside the inbox.
- Concurrent approvals stay safe: the last write wins, the phase ends at planning, and the record is never touched.
- A failed write surfaces an error and leaves the row unchanged on disk.
- No new CLI verb, flag, or output line appears; the printed success line and result shape are unchanged.
</summary>

<objective>
Make `vault-cli task approve` idempotent on a `phase: todo` task that already carries `approved_by` and/or `approved_at`: advance the phase to `planning`, preserve the existing record byte-for-byte, write no new approval, and require no owner. This closes the deadlock where the row is approved but unopenable — `approve` refuses it, `set phase planning` refuses it, and `work-on` points the operator at the refusing verb. This is spec 066 prompt 1 of 3: it covers Desired Behaviors 1–3 and Acceptance Criteria 1–6.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then `docs/dod.md` for the definition of done this change is graded against.

Read these files fully before changing anything:

- `pkg/ops/task_approve.go` (204 lines) — the whole file. `Execute` is the operation under change; `refuseExistingApprovalRecord` is the guard being replaced; `resolveOwner` stays exactly as it is.
- `pkg/ops/task_approve_test.go` (290 lines) — the whole file. The `DescribeTable("refuses a todo task that already carries an approval record", ...)` block at the end of the suite is the behaviour being inverted.
- `pkg/ops/frontmatter.go` lines 330–430 — `checkPhaseRegression` and `todoPlanningSetRefusal`. Both are read-only for this prompt; they must stay unchanged.
- `pkg/ops/complete.go` lines 65–80 — the `MutationResult` struct. Its shape is unchanged.
- `pkg/domain/task_frontmatter.go` — `Get`, `GetString`, `SetPhase`, `Assignee` semantics. `Get` returns the raw map value and returns `nil` only when the key is absent, so a key holding an empty string is present.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega suite files, Counterfeiter mocks, external test packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` wrapping (`errors.Wrap`, `errors.Errorf`); never `fmt.Errorf`, never `context.Background()` inside the package.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — public interface + private struct + `New*` constructor, which this file already follows.
</context>

<requirements>
The current `Execute` flow is: reject an empty `approvedBy`; find the task; reject a zero clock; reject a phase other than `todo`; refuse when an approval record is present (`refuseExistingApprovalRecord`); resolve the owner; write phase + `approved_by` + `approved_at` + optional assignee in one `WriteTask`. The refusal is what this prompt replaces.

1. **Add a presence check that replaces the refusal.** Delete `refuseExistingApprovalRecord` from `pkg/ops/task_approve.go` (its only caller is `Execute` — verified by `grep -rn 'refuseExistingApprovalRecord' pkg/`). Add in its place:

   ```go
   // hasApprovalRecord reports whether the task already carries an approved_by
   // or an approved_at key. Presence, not value, is the test: a key carrying an
   // empty value still means an approval was recorded, and the repair advances
   // the phase without rewriting or completing that record.
   func hasApprovalRecord(task *domain.Task) bool {
   	return task.Get("approved_by") != nil || task.Get("approved_at") != nil
   }
   ```

   After deleting `refuseExistingApprovalRecord`, the `"strings"` import becomes unused (`strings.Join` was its only use in this file). Remove the import or the build fails.

2. **Branch to the repair immediately after the phase guard and before `resolveOwner`.** Replace the `refuseExistingApprovalRecord` call site (currently `if err := refuseExistingApprovalRecord(ctx, taskName, task); err != nil { ... }`) with:

   ```go
   	// A todo row that already carries an approval record has already been
   	// approved; only the phase half of the transition is missing. Advance it and
   	// preserve the record exactly as it is: no approval is written, no missing
   	// key is invented, and no owner is resolved or required, because the owner
   	// rule governs the approval act, which already happened.
   	if hasApprovalRecord(task) {
   		task.SetPhase(domain.TaskPhasePlanning.Ptr())
   		return o.write(ctx, task, vaultName)
   	}
   ```

   The zero-clock refusal stays where it is (before the phase guard). Do not move it, and do not weaken it.

3. **Extract the shared write path.** Add this method and call it from BOTH the repair branch and the existing end-of-function write:

   ```go
   // write persists the task and returns the success result, so the approval
   // write and the already-approved repair share exactly one write path.
   func (o *taskApproveOperation) write(
   	ctx context.Context,
   	task *domain.Task,
   	vaultName string,
   ) (MutationResult, error) {
   	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
   		return MutationResult{
   			Success: false,
   			Error:   err.Error(),
   		}, errors.Wrap(ctx, err, "write task")
   	}
   	return MutationResult{Success: true, Name: task.Name, Vault: vaultName}, nil
   }
   ```

   The existing block at the end of `Execute` that calls `WriteTask` and builds the success result is replaced by `return o.write(ctx, task, vaultName)`. The `"write task"` wrapping text must survive — a test asserts it. Extracting the helper also keeps `Execute` under the `funlen` limit of 80 lines / 50 statements (`.golangci.yml`).

4. **Update the interface doc comment.** The `TaskApproveOperation` interface comment contains this clause, verbatim:

   ```
   any task that already carries an approval record, is refused with nothing written.
   ```

   Replace exactly that clause with:

   ```
   any task that already carries `approved_by` and/or `approved_at` is advanced to planning with the record preserved and is not re-approved; no owner is resolved or written on that path.
   ```

   Keep the rest of the comment — the owner precedence paragraph — intact.

5. **Rewrite the inverted test table.** In `pkg/ops/task_approve_test.go`, replace the `DescribeTable("refuses a todo task that already carries an approval record", ...)` block with an advance table plus the extra cases below. The seeded task is `domain.NewTask(fields, domain.FileMetadata{Name: "Alpha"}, domain.Content(""))` via the existing `seedTask` helper; `mockStorage` is `mocks.Storage`; `pinned` is 2026-03-03T12:00:00Z.

   ```go
   	DescribeTable("advances a todo task that already carries an approval record",
   		func(fields map[string]any) {
   			seedTask(fields)
   			// Snapshot the seeded record BEFORE Execute. domain.NewTask stores the
   			// caller's map by reference, so reading fields[...] after the call would
   			// compare the written map against itself and the assertion could never
   			// fail — a tautology, not a preservation check.
   			seededBy := fields["approved_by"]
   			seededAt := fields["approved_at"]

   			result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "someone", "current")
   			Expect(execErr).To(BeNil())
   			Expect(result.Success).To(BeTrue())
   			Expect(result.Name).To(Equal("Alpha"))
   			Expect(result.Vault).To(Equal("vault-a"))

   			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
   			_, written := mockStorage.WriteTaskArgsForCall(0)
   			Expect(written.Phase()).To(Equal(domain.TaskPhasePlanning.Ptr()))
   			// Byte-identical preservation: the written value is the seeded value,
   			// including nil for a key that was never there.
   			Expect(written.Get("approved_by")).To(Equal(seededBy))
   			Expect(written.Get("approved_at")).To(Equal(seededAt))
   			// The repair is not an approval: no owner is resolved or written even
   			// though --assignee and current_user are both non-empty here.
   			Expect(written.Get("assignee")).To(BeNil())
   		},
   		Entry("both keys", map[string]any{
   			"status": "next", "phase": "todo",
   			"approved_by": "someone", "approved_at": "2026-01-01T00:00:00Z",
   		}),
   		Entry("only approved_by", map[string]any{
   			"status": "next", "phase": "todo", "approved_by": "someone",
   		}),
   		Entry("only approved_at", map[string]any{
   			"status": "next", "phase": "todo", "approved_at": "2026-01-01T00:00:00Z",
   		}),
   		Entry("an empty approved_by value counts as present", map[string]any{
   			"status": "next", "phase": "todo", "approved_by": "",
   		}),
   	)
   ```

   Add these cases:

   - "still refuses an approved row that is past the inbox": seed `{"status": "in_progress", "phase": "execution", "approved_by": "someone", "approved_at": "2026-01-01T00:00:00Z"}`; expect an error containing `not "todo"`, zero writes. This pins that the phase guard runs before the repair.
   - "refuses an empty approver even on an already-approved row": seed `{"status": "next", "phase": "todo", "approved_by": "someone"}`, call with `approvedBy` `""`; expect an error containing `approved_by`, `FindTaskByNameCallCount() == 0`, zero writes.
   - "wraps a write failure on the repair path": seed `{"status": "next", "phase": "todo", "approved_by": "someone"}`, `mockStorage.WriteTaskReturns(liberrors.New(ctx, "boom"))`; expect an error containing `write task`.
   - "advances a terminal row that is still at phase todo": seed `{"status": "completed", "phase": "todo", "approved_by": "someone"}`; expect success, one write, `written.Phase()` equal to `planning`, and `approved_by` unchanged. `approve` inspects only `phase`, so the spec's Failure Modes say it advances here even though the lint fix (prompt 2) deliberately refuses the same row.

6. **Rewrite the integration test that asserts the old refusal.** This is not optional: `make test` runs `integration/`, and that test would fail the whole prompt. In `integration/cli_test.go`, the test named `"AC6: a task carrying any approval record is refused, per fixture"` (around line 4348) seeds three approved fixtures (`approvedByOnlyFrontmatter`, `approvedAtOnlyFrontmatter`, `approvedBothFrontmatter`), runs `task approve` on each, and asserts `gexec.Exit(1)` plus an unchanged file hash. Replace it with an advance assertion over the same three fixtures:

   ```go
   	It("AC6: a task carrying any approval record is advanced, per fixture", func() {
   		vaultPath, configPath, cleanup = createTempVault(map[string]string{
   			"Alpha": approvedByOnlyFrontmatter,
   			"Beta":  approvedAtOnlyFrontmatter,
   			"Gamma": approvedBothFrontmatter,
   		})

   		for _, name := range []string{"Alpha", "Beta", "Gamma"} {
   			taskFile := filepath.Join(vaultPath, "Tasks", name+".md")
   			before := frontmatterOf(taskFile)

   			session := runEntityCommand("task", "approve", name)
   			Eventually(session).Should(gexec.Exit(0))

   			after := frontmatterOf(taskFile)
   			Expect(after["phase"]).To(Equal("planning"), "fixture %s phase", name)
   			Expect(after["approved_by"]).To(Equal(before["approved_by"]), "fixture %s approved_by", name)
   			Expect(after["approved_at"]).To(Equal(before["approved_at"]), "fixture %s approved_at", name)
   		}
   	})
   ```

   `frontmatterOf` is already defined and used in this suite. Leave the goal and topic approve integration tests alone — `goal approve` and `topic approve` keep their refusal (`refuseExistingEntityApprovalRecord`) and are out of scope for this spec.

7. **Do not change any other file.** `pkg/ops/frontmatter.go` (`checkPhaseRegression`, `todoPlanningSetRefusal`), `pkg/ops/workon.go`, `pkg/cli/cli.go`, `pkg/ops/complete.go`, `pkg/ops/goal_approve.go` and `pkg/ops/topic_approve.go` stay untouched. The goal and topic siblings carry their own `refuseExistingEntityApprovalRecord` guard in `pkg/ops/approval_record.go`, so deleting `refuseExistingApprovalRecord` does not affect them. The CLI already prints `✅ Approved <name>: phase planning` from `MutationResult.Name`, which the repair path populates, so no CLI edit is needed.
</requirements>

<constraints>
- Copied from the spec: `todoPlanningSetRefusal` and `checkPhaseRegression` are unchanged. `--force` still does not bypass the approval-inbox refusal for a row carrying no approval record.
- Copied from the spec: `task approve`'s existing refusals are unchanged: a row not at `phase: todo`, an empty `--by`, and an unowned row with no `assignee` / `--assignee` / configured `current_user`.
- Copied from the spec: `task work-on` is unchanged. Its refusal message stays exactly as it is.
- Copied from the spec: the repair path writes no `approved_by` / `approved_at` value — not even to complete a partial record.
- Copied from the spec: `MutationResult`'s shape and the CLI's printed success line are unchanged.
- Copied from the spec: `task set` semantics are unchanged. No new CLI verb, subcommand or flag.
- Copied from the spec: no `go.mod` / `go.sum` change.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass, apart from the two inverted assertions this prompt rewrites: the unit `DescribeTable` in `pkg/ops/task_approve_test.go` and `AC6` in `integration/cli_test.go`. The goal and topic approve tests must keep passing untouched.
- Use `github.com/bborbe/errors` for every error; never `fmt.Errorf`, never `context.Background()` inside `pkg/`.
</constraints>

<verification>
Run `make test` — the Ginkgo suite must pass, including the new table and the three added cases.

Run `make precommit` once at the end — it must exit 0 (format, generate, lint, tests, changelog check, license headers).

Then confirm the guard is gone:

```
! grep -q 'refuseExistingApprovalRecord' pkg/ops/task_approve.go   # must find nothing
grep -c 'hasApprovalRecord' pkg/ops/task_approve.go                # expect >= 3 (declaration, doc comment, call site)
! grep -q '"strings"' pkg/ops/task_approve.go                      # must find nothing
```

Emit no `git` command; this prompt is verified by build and test only.
</verification>
