---
status: approved
spec: [037-mandatory-abort-reason-and-gate-successor]
created: "2026-08-24T19:00:00Z"
queued: "2026-08-24T19:00:06Z"
---

# Ops layer honors the close-out guard (no silent completion)

<summary>
- The three ops paths that today swallow the status-change error — task complete, goal complete, and the task update checkbox sync — now return it instead, so a rejected close-out never reaches the file write.
- A task whose completion is blocked by the reason guard is not written: no status change, no phase change, no completed-date stamp, no goal or daily-note updates.
- A goal whose completion is blocked by the reason guard is not written, and the "already completed" short-circuit is untouched.
- The `task update` checkbox sync aborts without a status change when its computed status is `completed` and the close-out fields are missing — no silent completion through the sync path.
- Recurring task completion is unaffected: it never sets status to `completed`, so it never trips the guard.
- Objective completion is untouched — the spec's guard covers tasks and goals only.
- The two set-field ops (task set, goal set) already propagate `SetField` errors and need no change here; the new guard flows through them for free.
</summary>

<objective>
`complete.go`, `goal_complete.go`, and `update.go` must stop discarding the `SetStatus` error from prompt 1 (the `_ = task.SetStatus(...)` / `_ = goal.SetStatus(...)` holes). When the close-out guard rejects the transition, each operation must return a `MutationResult{Success: false, Error: ...}` plus a wrapped error and must not write the entity — no partial close-out, no silent completion.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, and `/workspace/docs/development-patterns.md` for the ops/CLI layering (ops returns structured results, never writes to stdout).

Relevant coding-plugin guides:

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, ...)` with `github.com/bborbe/errors`, never `fmt.Errorf`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, mock-fake based ops tests
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — the `github.com/bborbe/vault-cli/mocks` counterfeiter fakes used in ops tests

**Precondition — prompt 1 (`spec-037` domain close-out guard) must have shipped.** Verify before writing anything:

```
grep -c 'missingCloseOutFields(f.FrontmatterMap)' pkg/domain/task_frontmatter.go
```

If that prints `0`, prompt 1 has not landed. STOP, report `"status":"failed"` with the message `domain close-out guard not yet deployed (prompt 1)`, and do NOT re-implement the guard yourself.

Read these files fully before making changes:

- `/workspace/pkg/ops/complete.go` — the `_ = task.SetStatus(domain.TaskStatusCompleted)` hole at line 109, immediately followed by `task.SetPhase(domain.TaskPhaseDone.Ptr())`, the completed-date stamp, and the single `c.taskStorage.WriteTask(ctx, task)` call. The recurring path (`handleRecurringTask`) returns before this point and never sets status to completed — leave it alone. The subtask guard (`checkSubtaskCompletion`) returns before this point too.
- `/workspace/pkg/ops/goal_complete.go` — the `_ = goal.SetStatus(domain.GoalStatusCompleted)` hole at line 80, after the already-completed short-circuit (`goal.Status() == domain.GoalStatusCompleted`, lines 69-72) and the open-tasks check, and before `goal.SetCompleted(...)` and the single `g.goalStorage.WriteGoal(ctx, goal)` call.
- `/workspace/pkg/ops/update.go` — the `_ = task.SetStatus(u.statusFromProgress(completed, total))` hole at line 78, between the checkbox parse and the single `u.taskStorage.WriteTask(ctx, task)` call. `statusFromProgress` returns `domain.TaskStatusCompleted` only when `completed == total`.
- Test files to extend, mirroring the existing fixture style (mock storage fakes + real `domain.NewTask` / `domain.NewGoal`): `/workspace/pkg/ops/complete_test.go`, `/workspace/pkg/ops/goal_complete_test.go`, `/workspace/pkg/ops/update_test.go`. Note prompt 1 already added `"aborted_reason": "test reason", "gate_successor": "none"` to the default `complete_test.go` and `goal_complete_test.go` fixtures and to the `update_test.go` "with all checkboxes checked" fixture — so the guard-positive paths in those files already pass; your new specs must construct field-less fixtures explicitly for the guard-negative cases.

Verified existing signatures that must NOT change in this prompt: `CompleteOperation.Execute(ctx, vaultPath, taskName, vaultName, force)`, `GoalCompleteOperation.Execute(ctx, vaultPath, goalName, vaultName, force)`, `UpdateOperation.Execute(ctx, vaultPath, taskName, vaultName)`. The CLI flag surface and the one-step field writes land in prompt 3; do not add parameters here.

Out of scope — do NOT touch: `/workspace/pkg/ops/objective_complete.go` (objective completion is not part of spec 037; it keeps its current `_ =` swallow), `/workspace/pkg/ops/frontmatter.go`, `/workspace/pkg/ops/frontmatter_entity.go` (both already return `SetField` errors — the guard flows through them for free), and `/workspace/pkg/cli/`.
</context>

<requirements>
1. In `/workspace/pkg/ops/complete.go`, replace the line
   ```go
   	_ = task.SetStatus(domain.TaskStatusCompleted)
   ```
   (line 109) with
   ```go
   	if err := task.SetStatus(domain.TaskStatusCompleted); err != nil {
   		return MutationResult{
   			Success: false,
   			Error:   err.Error(),
   		}, errors.Wrap(
   			ctx,
   			err,
   			"set status",
   		)
   	}
   ```
   This must appear before `task.SetPhase(...)` / `SetCompletedDate` / `WriteTask`, so a rejected close-out writes nothing. Do NOT change anything else in `complete.go`, including `handleRecurringTask` (recurring completion never sets status to `completed`, so it never hits the guard).

2. In `/workspace/pkg/ops/goal_complete.go`, replace the line
   ```go
   	_ = goal.SetStatus(domain.GoalStatusCompleted)
   ```
   (line 80) with
   ```go
   	if err := goal.SetStatus(domain.GoalStatusCompleted); err != nil {
   		return MutationResult{
   			Success: false,
   			Error:   err.Error(),
   		}, errors.Wrap(
   			ctx,
   			err,
   			"set status",
   		)
   	}
   ```
   This must appear before `goal.SetCompleted(...)` / `WriteGoal`. Keep the already-completed short-circuit (lines 69-72) and the open-tasks check exactly as they are.

3. In `/workspace/pkg/ops/update.go`, replace the line
   ```go
   	_ = task.SetStatus(u.statusFromProgress(completed, total))
   ```
   (line 78) with
   ```go
   	target := u.statusFromProgress(completed, total)
   	if err := task.SetStatus(target); err != nil {
   		return MutationResult{
   			Success: false,
   			Error:   err.Error(),
   		}, errors.Wrap(
   			ctx,
   			err,
   			"set status",
   		)
   	}
   ```
   This aborts before `u.taskStorage.WriteTask(ctx, task)` and before `syncGoals`, so the task file's status stays as it was and no goal checkboxes are touched. Do NOT change `statusFromProgress` or anything else in `update.go`.

4. Add a `Context("task without close-out fields", ...)` block to `/workspace/pkg/ops/complete_test.go`, inside the existing `Describe("CompleteOperation", ...)`, with its own `BeforeEach` that overrides the default fixture with a field-less task:
   ```go
   task = domain.NewTask(
   	map[string]any{"status": "todo"},
   	domain.FileMetadata{Name: taskName},
   	domain.Content(""),
   )
   ```
   followed by `mockTaskStorage.FindTaskByNameReturns(task, nil)` — the enclosing `BeforeEach` wired `FindTaskByNameReturns` to the default (with-fields) fixture, so the inner override must re-stub it, otherwise the mock still returns the with-fields task and the guard never fires. Mirror the sibling "task with incomplete checkboxes" `Context` at complete_test.go:146-153. Assert: `err` is non-nil; `err.Error()` contains `"aborted_reason"`; `result.Error` contains `"aborted_reason"`; and `mockTaskStorage.WriteTaskCallCount()` is `0` — the task is never written. (Spec Failure Modes row 1: no write occurred.) Keep `force` defaulting to `false`.

5. Add a `Context("goal without close-out fields", ...)` block to `/workspace/pkg/ops/goal_complete_test.go` with a field-less goal fixture:
   ```go
   goal = domain.NewGoal(
   	map[string]any{"status": "active"},
   	domain.FileMetadata{Name: goalName},
   	domain.Content(""),
   )
   ```
   followed by `mockGoalStorage.FindGoalByNameReturns(goal, nil)` — the enclosing `BeforeEach` wired the mock to the default (with-fields) goal fixture, so the inner override must re-stub it, otherwise the guard never fires. Assert: `err` is non-nil; `err.Error()` contains `"aborted_reason"`; `mockGoalStorage.WriteGoalCallCount()` is `0`.

6. Add a `Context("all checkboxes checked but no close-out fields", ...)` block to `/workspace/pkg/ops/update_test.go` with a task fixture whose content is all `- [x]` and whose map is `map[string]any{"status": "todo"}` (no close-out fields), so `statusFromProgress` computes `TaskStatusCompleted`. Mutate `task.Content` in place (mirroring the existing "with all checkboxes checked" `Context`, which needs no re-wire); if instead you construct a fresh `domain.NewTask`, re-wire `mockTaskStorage.FindTaskByNameReturns(task, nil)` afterwards. Assert: `err` is non-nil; `err.Error()` contains `"aborted_reason"`; `mockTaskStorage.WriteTaskCallCount()` is `0`. (Spec AC 5 negative arm — no silent completion through the checkbox sync.)

7. Confirm the guard-positive paths are already green from prompt 1 (default fixtures now carry the fields): the existing "success" contexts in `complete_test.go`, `goal_complete_test.go`, and the "with all checkboxes checked" context in `update_test.go` must pass unchanged. Do not add duplicate positive-path specs.

8. Run `make test`; iterate until green, then `make precommit` once. If `make precommit` fails on a target, re-run only the failing target until it passes, then `make precommit` once more.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- The `SetStatus` chokepoints are the frozen enforcement points from prompt 1. This prompt does NOT add a guard anywhere — it only stops swallowing the error the guard already raises.
- No partial write: when `SetStatus` returns the guard error, the operation must return before any `WriteTask` / `WriteGoal` call. The error goes into both the returned error and `MutationResult.Error` (so `--output json` carries it — the JSON shape is `{success:false, error:...}`).
- Do NOT change any operation signature, interface, or constructor in this prompt — the CLI flag surface (prompt 3) will add the parameters. Do NOT regenerate mocks here (no interface changed).
- Recurring task completion must stay exactly as it is: it never sets status to `completed`, so it must not trip the guard and must not be altered.
- Objective completion (`objective_complete.go`) is OUT OF SCOPE — do not touch it.
- `frontmatter.go` / `frontmatter_entity.go` already propagate `SetField` errors — do not add redundant handling.
- Do NOT touch `pkg/domain` (prompt 1 owns the guard), `pkg/cli`, `pkg/storage`, or the integration suite in this prompt.
- Existing tests must still pass; `make precommit` must exit 0.
</constraints>

<verification>
Run all of the following from `/workspace`.

The three swallow holes are closed — each propagated-error block exists exactly once:

```
grep -c 'if err := task.SetStatus(domain.TaskStatusCompleted); err != nil' pkg/ops/complete.go
grep -c 'if err := goal.SetStatus(domain.GoalStatusCompleted); err != nil' pkg/ops/goal_complete.go
grep -c 'target := u.statusFromProgress(completed, total)' pkg/ops/update.go
grep -c '_ = task.SetStatus' pkg/ops/complete.go pkg/ops/update.go pkg/ops/goal_complete.go
grep -c '_ = goal.SetStatus' pkg/ops/goal_complete.go
```

The first three must each print exactly `1`; the `_ =` greps must each print exactly `0`.

The new guard-negative specs exist:

```
grep -c 'task without close-out fields' pkg/ops/complete_test.go
grep -c 'goal without close-out fields' pkg/ops/goal_complete_test.go
grep -c 'all checkboxes checked but no close-out fields' pkg/ops/update_test.go
grep -c 'WriteTaskCallCount()).To(Equal(0))' pkg/ops/complete_test.go
grep -c 'WriteGoalCallCount()).To(Equal(0))' pkg/ops/goal_complete_test.go
grep -c 'WriteTaskCallCount()).To(Equal(0))' pkg/ops/update_test.go
```

Each must print a number `>= 1`.

No signature changed and nothing outside the intended surface moved (these signatures are multi-line in this repo, so count the distinguishing tokens — each of the three interfaces and their method receivers must both carry the listed params, i.e. count 2):

```
grep -c 'force bool' pkg/ops/complete.go       # must be 2 (interface + method)
grep -c 'force bool' pkg/ops/goal_complete.go  # must be 2
grep -c 'vaultName string' pkg/ops/update.go   # must be 2
```

A param added or removed shifts the count, so this still detects an accidental signature change.

The ops package is green:

```
go test -mod=mod -count=1 ./pkg/ops/...
```

Must exit 0.

Finally, run the full gate once:

```
make precommit
```

Must exit 0. A non-zero exit code means `"status":"failed"` — no exceptions.
</verification>
