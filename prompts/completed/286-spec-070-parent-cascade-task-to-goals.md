---
status: completed
spec: [070-parent-closure-cascade-on-complete]
summary: Verified the task-to-goal parent cascade (ParentAttempt types, cascadeParents helper, MutationResult.Parents, GoalCompleteOperation dependency, CLI wiring, and unit tests) and added the Unreleased CHANGELOG entry
execution_id: d739d3e0-c064-5dfc-99b3-91ad20421dc4-exec-286-spec-070-parent-cascade-task-to-goals
dark-factory-version: dev
created: "2026-11-19T20:10:00Z"
queued: "2026-11-19T20:08:41Z"
started: "2026-11-19T21:34:45Z"
completed: "2026-11-19T21:49:32Z"
branch: dark-factory/parent-closure-cascade-on-complete
---

# Cascade `task complete` to the task's parent goals

<summary>
- Completing a task now also tries to complete every goal listed in the task's `goals:` frontmatter, in document order.
- A goal that refuses does not fail the task: the task stays completed and the command still exits 0.
- Each parent attempt produces one structured report entry carrying the parent's name, `completed` or `refused`, and the open items on a refusal.
- A completed or already-completed goal reports `completed`; an already-terminal goal is not rewritten by the cascade.
- Parent names are wikilink-normalised, so `[[Goal]]`, `[[Goal|label]]` and `[[Goal#Heading]]` all resolve to the same goal.
- A recurring task never cascades, because its status never changes.
- `--output json` gains a `parents` array; the key is omitted entirely when no parent was attempted.
- The cascade calls the existing `goal complete` operation, so the closure rule and the goal close sequence stay in one place.
- No existing task, goal or topic behaviour changes.
</summary>

<objective>
Give `pkg/ops` a single parent-cascade helper and wire the task → goal hop into `CompleteOperation`, so closing a task also closes the goals that are now closable and reports every attempt. This is the first half of spec 070; the goal → topic hop and the CLI printing follow in prompt 2.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` (§ Output Format) for project conventions.

Read these files fully before changing anything:

- `specs/in-progress/070-parent-closure-cascade-on-complete.md` — the whole spec. § Desired Behavior 1-11, § Constraints and § Failure Modes are binding on this prompt for the task → goal hop (DB 1, 3, 4, 5, 6, 7, 9, 10, 11).
- `pkg/ops/complete.go` — `MutationResult` (the struct starting at line 66) gains `Parents`; `NewCompleteOperation` (line 44) gains a `GoalCompleteOperation` dependency; `completeOperation` (line 58) gains the field; `Execute`'s success return (line 174) is where the cascade runs. `handleRecurringTask` (line 266) returns early and must stay cascade-free.
- `pkg/ops/goal_complete.go` — `GoalCompleteOperation` and its `Execute(ctx, vaultPath, goalName, vaultName, force, reason, gateSuccessor)` signature. This is the operation the cascade calls; do not change it in this prompt.
- `pkg/ops/closure_gate.go` — `normaliseGoalLink` (line 522) is the existing wikilink stripper. Reuse it; do not write a second one.
- `pkg/ops/export_test.go` — the established mechanism for exposing unexported `pkg/ops` symbols to the external `ops_test` package.
- `pkg/ops/complete_test.go` — the `BeforeEach` at line 24 and the constructor call at line 48; `Context("task with associated goals")` (line 269) asserts the goal is never read and never written by the task close itself. Those assertions stay true: the test injects a fake goal-complete operation, so `mocks.GoalStorage` is untouched.
- `pkg/ops/ops_suite_test.go` — `ErrTest` is the shared sentinel error for tests.
- `pkg/cli/cli.go` — `createCompleteCommand` (line 246); its runner builds `taskStore`/`dailyStore` and calls `ops.NewCompleteOperation` at line 282.
- `mocks/goal-complete-operation.go` — `mocks.GoalCompleteOperation` with `ExecuteReturns`, `ExecuteCallCount`, `ExecuteArgsForCall(i) (context.Context, string, string, string, bool, string, string)`.
- `pkg/domain/task.go` / `pkg/domain/task_frontmatter.go` — `TaskFrontmatter.Goals() []string` reads the `goals:` key via `GetStringSlice`.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — Interface → Constructor → Struct, exported GoDoc on every new exported type and method.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors`; never `fmt.Errorf`, never `context.Background()` inside `pkg/ops`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega, external `ops_test` package, `DescribeTable`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen 80 lines / 50 statements, nestif 4, gocognit 20, maintidx 20, dupl.
</context>

<requirements>
## 1. New file `pkg/ops/parent_cascade.go`

BSD license header (`// Copyright (c) 2026 Benjamin Borbe All rights reserved.` plus the standard two following lines), `package ops`. Imports: `context` only. It must not import `pkg/cli`, must not write to stdout/stderr, and must never write, create or delete a file.

```go
// ParentResult is the outcome of one cascade attempt on a parent page.
type ParentResult string

const (
	// ParentResultCompleted means the parent is terminal after the attempt.
	ParentResultCompleted ParentResult = "completed"
	// ParentResultRefused means the parent refused to close and stays open.
	ParentResultRefused ParentResult = "refused"
)

// ParentAttempt is one entry of a completion cascade report: the parent's name,
// the outcome, and (on a refusal) the items that blocked the close. The JSON
// tags are the frozen wire shape: a completed entry omits open_items.
type ParentAttempt struct {
	Name      string       `json:"name"`
	Result    ParentResult `json:"result"`
	OpenItems []string     `json:"open_items,omitempty"`
}

// parentAttemptFunc completes one parent and returns the operation's result.
type parentAttemptFunc func(ctx context.Context, name string) (MutationResult, error)

// cascadeParents attempts every parent in document order and returns one report
// entry per attempt. It never returns an error: a refusal is recorded, never
// propagated, so the child's outcome and the command's exit code are unaffected.
// Each raw entry is wikilink-normalised before the attempt; an entry that
// normalises to the empty string is skipped rather than attempted.
func cascadeParents(
	ctx context.Context,
	names []string,
	attempt parentAttemptFunc,
) []ParentAttempt

// refusalItems returns the items a refused attempt reports: the operation's own
// OpenItems, else its error text, else its Error field, else a fixed fallback so
// a refused entry never carries an empty list.
func refusalItems(result MutationResult, err error) []string
```

`cascadeParents` body, exactly:

```go
func cascadeParents(
	ctx context.Context,
	names []string,
	attempt parentAttemptFunc,
) []ParentAttempt {
	attempts := make([]ParentAttempt, 0, len(names))
	for _, raw := range names {
		name := normaliseGoalLink(raw)
		if name == "" {
			continue
		}
		result, err := attempt(ctx, name)
		if result.Success {
			attempts = append(attempts, ParentAttempt{Name: name, Result: ParentResultCompleted})
			continue
		}
		attempts = append(attempts, ParentAttempt{
			Name:      name,
			Result:    ParentResultRefused,
			OpenItems: refusalItems(result, err),
		})
	}
	return attempts
}

func refusalItems(result MutationResult, err error) []string {
	if len(result.OpenItems) > 0 {
		return result.OpenItems
	}
	if err != nil {
		return []string{err.Error()}
	}
	if result.Error != "" {
		return []string{result.Error}
	}
	return []string{"refused"}
}
```

Do not add a context-cancellation check inside this loop: every attempt is one in-process page read/write and the spec calls the attempt cheap (§ Assumptions).

## 2. `MutationResult` gains `Parents`

In `pkg/ops/complete.go`, add to `MutationResult` after `OpenItems`:

```go
	// Parents reports one entry per parent page the completion cascade
	// attempted, in document order. Omitted when no attempt ran.
	Parents []ParentAttempt `json:"parents,omitempty"`
```

The field is additive and `omitempty`, so existing JSON consumers keep working.

## 3. `CompleteOperation` gains the goal-complete dependency

In `pkg/ops/complete.go`:

1. Add the field to `completeOperation`:
   ```go
   type completeOperation struct {
   	taskStorage           storage.TaskStorage
   	dailyNoteStorage      storage.DailyNoteStorage
   	currentDateTime       libtime.CurrentDateTime
   	interactionCounter    InteractionCounter
   	goalCompleteOperation GoalCompleteOperation
   }
   ```
2. Add the parameter as the LAST parameter of `NewCompleteOperation`:
   ```go
   func NewCompleteOperation(
   	taskStorage storage.TaskStorage,
   	dailyNoteStorage storage.DailyNoteStorage,
   	currentDateTime libtime.CurrentDateTime,
   	interactionCounter InteractionCounter,
   	goalCompleteOperation GoalCompleteOperation,
   ) CompleteOperation
   ```
   Assign it in the returned struct. Extend the existing constructor doc comment — the paragraph explaining why `GoalStorage` is deliberately absent stays untouched; add: the goal close runs through `goalCompleteOperation`, so the closure rule and the goal close sequence exist once and the task close never writes a goal itself.
3. At the end of `Execute`, replace the final return with:
   ```go
   	result := MutationResult{
   		Success:  true,
   		Name:     task.Name,
   		Vault:    vaultName,
   		Warnings: warnings,
   	}
   	result.Parents = cascadeParents(
   		ctx,
   		task.Goals(),
   		func(ctx context.Context, goalName string) (MutationResult, error) {
   			return c.goalCompleteOperation.Execute(
   				ctx,
   				vaultPath,
   				goalName,
   				vaultName,
   				false,
   				"",
   				"",
   			)
   		},
   	)
   	return result, nil
   ```
   `force=false`, empty reason and empty gate successor are frozen by the spec (§ Constraints). The cascade runs only on this success path: every refusal and the recurring-task branch return before it, so a recurring task never cascades and a blocked task never touches a parent.

## 4. Update every constructor call site

`grep -rn 'NewCompleteOperation' pkg/ integration/` must afterwards show exactly three files: `pkg/ops/complete.go`, `pkg/ops/complete_test.go`, `pkg/cli/cli.go`.

`pkg/cli/cli.go`, `createCompleteCommand` runner (around line 274-288) — construct the goal operation and pass it:

```go
					storageConfig := storage.NewConfigFromVault(vault)
					taskStore := storage.NewTaskStorage(storageConfig)
					dailyStore := storage.NewDailyNoteStorage(storageConfig)
					goalStore := storage.NewGoalStorage(storageConfig)
					interactionCounter, err := completeInteractionCounter(ctx, vault)
					if err != nil {
						return ops.MutationResult{}, err
					}
					goalCompleteOp := ops.NewGoalCompleteOperation(
						goalStore,
						taskStore,
						currentDateTime,
					)
					completeOp := ops.NewCompleteOperation(
						taskStore,
						dailyStore,
						currentDateTime,
						interactionCounter,
						goalCompleteOp,
					)
```

`ops.NewGoalCompleteOperation` keeps its current three-parameter signature in this prompt; prompt 2 adds the topic dependency.

## 5. Unit tests

### 5.1 Expose the helper

Append to `pkg/ops/export_test.go`, matching the existing alias style:

```go
// CascadeParents exposes cascadeParents for the external ops_test package.
var CascadeParents = cascadeParents
```

`ParentAttempt` and `ParentResult` are already exported.

### 5.2 New file `pkg/ops/parent_cascade_test.go` (package `ops_test`)

One `Describe("Parent cascade", ...)` with a `DescribeTable("attempts", ...)`. Every entry calls `ops.CascadeParents(ctx, names, func(ctx context.Context, name string) (ops.MutationResult, error) { ... })` and asserts the returned slice's length, names, results and open items.

Required entries:

| Entry name | Names | Attempt returns | Expected |
|---|---|---|---|
| `empty list attempts nothing` | `nil` | never called | 0 entries |
| `successful parent reports completed` | `["Goal A"]` | `{Success: true, Name: "Goal A"}, nil` | 1 entry `Goal A` / `completed` / no open items |
| `refused parent carries the operation's open items` | `["Goal A"]` | `{Success: false, OpenItems: []string{"box one", "box two"}}, ErrTest` | 1 entry `refused` / `["box one","box two"]` |
| `refused parent falls back to the error text` | `["Goal A"]` | `{Success: false}, ErrTest` | 1 entry `refused` / `["test error"]` |
| `refused parent falls back to the result error field` | `["Goal A"]` | `{Success: false, Error: "cannot complete goal: 1 open"}, nil` | 1 entry `refused` / `["cannot complete goal: 1 open"]` |
| `wikilink and alias normalise to the bare name` | `["[[Goal A]]", "[[Goal B|label]]", "[[Goal C#Heading]]"]` | success for each | names `Goal A`, `Goal B`, `Goal C`, in that order |
| `empty normalised name is skipped` | `["[[]]"]` | never called | 0 entries |
| `document order is preserved` | `["Goal B", "Goal A"]` | success for each | names `Goal B`, `Goal A`, in that order |

Also add one entry that asserts the attempt callback receives the **normalised** name (record it in a captured slice and compare).

### 5.3 Update `pkg/ops/complete_test.go`

1. Declare `mockGoalCompleteOperation *mocks.GoalCompleteOperation` in the `var` block next to `mockGoalStorage`.
2. In `BeforeEach`, before constructing the operation:
   ```go
   		mockGoalCompleteOperation = &mocks.GoalCompleteOperation{}
   		mockGoalCompleteOperation.ExecuteReturns(
   			ops.MutationResult{Success: true, Name: "Test Goal"},
   			nil,
   		)
   ```
   and pass it as the fifth argument to `ops.NewCompleteOperation`.
3. `Context("task with associated goals")` keeps its three assertions unchanged — with the fake goal-complete operation, `mocks.GoalStorage` is still never read and never written. Add one `It`:
   - `It("cascades to the declared goal")` — `task.SetGoals([]string{"Test Goal"})`; assert `mockGoalCompleteOperation.ExecuteCallCount() == 1`, and the args from `ExecuteArgsForCall(0)` are `(ctx, vaultPath, "Test Goal", "test-vault", false, "", "")`.
4. Add a new `Context("parent cascade", ...)` with these cases (each sets `task.SetGoals(...)` as needed):

   - `It("reports a completed parent")` — `task.SetGoals([]string{"Test Goal"})`; assert `err` is nil, `result.Success` is true, `len(result.Parents) == 1`, `result.Parents[0].Name == "Test Goal"`, `result.Parents[0].Result == ops.ParentResultCompleted`, `result.Parents[0].OpenItems` is empty.
   - `It("reports a refused parent and still completes the task")` — fake returns `(ops.MutationResult{Success: false, OpenItems: []string{"an open criterion"}}, ErrTest)`; assert `err` is nil, `result.Success` is true, `result.Parents[0].Result == ops.ParentResultRefused`, `result.Parents[0].OpenItems == []string{"an open criterion"}`, and `mockTaskStorage.WriteTaskCallCount() == 1`.
   - `It("normalises a wikilink parent name")` — `task.SetGoals([]string{"[[Test Goal|label]]"})`; assert the fake was called with `"Test Goal"` and `result.Parents[0].Name == "Test Goal"`.
   - `It("attempts every goal in document order")` — `task.SetGoals([]string{"Goal B", "Goal A"})`; assert two calls, args order `Goal B` then `Goal A`, and `result.Parents` names in the same order.
   - `It("does not cascade a recurring task")` — `task.SetRecurring("1d")`, `task.SetGoals([]string{"Test Goal"})`, and set `task.Content` to a frontmatter block carrying `recurring: 1d` with a body of no checkboxes; assert `mockGoalCompleteOperation.ExecuteCallCount() == 0` and `result.Parents` is empty.
   - `It("does not cascade when the task refuses to complete")` — task with an open checkbox and `force=false`, `task.SetGoals([]string{"Test Goal"})`; assert the error is non-nil and `mockGoalCompleteOperation.ExecuteCallCount() == 0`.

   For the recurring fixture, follow the existing `Context("recurring daily task")` shape at line 420 — it calls `task.SetRecurring(...)` **as well as** setting `task.Content`. `Task.Recurring()` reads the frontmatter map (`GetString("recurring")`), not `Content`, so setting only `Content` leaves the task non-recurring and the cascade would run. Set both.

## 6. No other changes

- Do not change `pkg/ops/goal_complete.go`, `pkg/ops/topic_complete.go`, `pkg/cli/cli.go`'s goal or topic complete runners, or any printing. Prompt 2 owns the goal → topic hop and both hops' stdout lines.
- Do not add a `--force`, a flag, or a config knob. The cascade always runs on a successful task close.
- Do not write the goal's `# Tasks` list, and do not reintroduce `GoalStorage` as a dependency of `CompleteOperation`.
</requirements>

<constraints>
- **The cascade never changes the child's outcome.** A parent refusal must leave `result.Success == true`, the task written, and `err == nil`. The task's exit code stays 0.
- **The cascade never re-implements a close.** It calls `GoalCompleteOperation.Execute` with `force=false`, `reason=""`, `gateSuccessor=""` — the frozen call shape from the spec. `checkClosureGate` remains the only closure rule and `goal_complete.go` its only caller.
- **`pkg/ops` is a library layer** — structured results only, never stdout. Printing belongs to `pkg/cli` and arrives in prompt 2.
- **`MutationResult.Parents` is additive and `omitempty`**; do not rename or reorder existing fields.
- **Wikilink normalisation uses `normaliseGoalLink`** from `pkg/ops/closure_gate.go`. Do not add a second stripper.
- **A recurring task never cascades** — its status never changes, so there is nothing for a parent to close on.
- **Do not change** `checkSubtaskCompletion`, `countCheckboxStates`, `resetCheckboxes`, `handleRecurringTask`, or any other task-close behaviour.
- **Do not touch** `docs/`, `CHANGELOG.md`, `integration/`, or `agents/`. Prompt 2 owns the docs; prompt 3 owns integration tests and the changelog.
- **Error wrapping** goes through `github.com/bborbe/errors` — never `fmt.Errorf`, never `context.Background()` inside `pkg/ops`.
- **Do not hand-bump version manifests or tag.** The releaser owns the tag post-merge.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `make test` — must exit 0. Re-run it iteratively while implementing; it is the fast feedback loop.

Focused run (a focus that matches zero specs exits 0 while running nothing, so confirm the names appear):

```
go test -mod=mod ./pkg/ops -run TestSuite -ginkgo.focus='Parent cascade|CompleteOperation' -ginkgo.v
```

The `-ginkgo.v` output must contain every table entry name from requirement 5.2 and every `It` name from requirement 5.3.

Evidence greps:

```
grep -rn 'NewCompleteOperation' pkg/ integration/          # cli.go, complete.go, complete_test.go only
grep -c 'func cascadeParents' pkg/ops/parent_cascade.go    # 1
grep -c 'Parents \[\]ParentAttempt' pkg/ops/complete.go    # 1
grep -c 'goalCompleteOperation' pkg/ops/complete.go        # field + constructor param + assignment + call site
```

Finally run `make precommit` once — must exit 0.

If `make precommit` exits non-zero: fix, re-run only the failing target (`make lint`, `make test`, …) until it passes, then run `make precommit` once more. A non-zero `make precommit` exit code must be reported as `"status":"failed"` in the completion report — never rationalised as success.
</verification>
