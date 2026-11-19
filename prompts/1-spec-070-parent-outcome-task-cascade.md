---
spec: ["070-parent-closure-cascade-on-complete"]
status: draft
created: "2026-11-19T18:55:00Z"
---

# Task completion cascades to its parent goals

<summary>
- Completing a task now also attempts to complete every goal named in the task's `goals:` frontmatter, in document order.
- Each attempt is recorded as a structured outcome: `completed` or `refused`, with the blocking items on a refusal.
- A goal that refuses never fails the task: the task stays completed and the command still exits 0.
- An already-terminal goal reports `completed` with no write.
- Goal names written as `[[Name]]`, `[[Name|label]]` or `[[Name#Heading]]` all resolve to the same goal.
- The outcome list rides on `MutationResult` as a new additive, `omitempty` `parents` field, so existing JSON consumers are unaffected.
- The task cascade delegates to the existing goal complete operation — it does not re-implement a goal close.
- The task's own completion behaviour is unchanged.
</summary>

<objective>
Introduce the shared parent-outcome type and the `parents` field on `MutationResult`, then make `vault-cli task complete` attempt each parent goal after the task itself is written, reporting every attempt without letting a refusal change the task's outcome or the exit code. This is the first of three prompts; it does not yet touch the goal→topic hop or CLI output formatting.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read these files fully before changing anything:

- `specs/in-progress/070-parent-closure-cascade-on-complete.md` — the whole spec. Desired Behavior 1, 3-7, 9-11, Constraints, and Failure Modes are binding. This prompt implements the task→goal half.
- `pkg/ops/complete.go` — `MutationResult` (the struct around line 66), `NewCompleteOperation`, `completeOperation`, and `Execute`. `Execute` returns the task result at the end of the non-recurring path; the cascade attaches there.
- `pkg/ops/closure_gate.go` — `normaliseGoalLink(link string) string` is the single wikilink-normalisation helper. Reuse it; do not write a second one.
- `pkg/ops/goal_complete.go` — `GoalCompleteOperation` and its frozen signature `Execute(ctx, vaultPath, goalName, vaultName, force, reason, gateSuccessor)`. A refused goal returns `MutationResult{Success: false, OpenItems: ...}` plus an error; an already-terminal goal returns `MutationResult{Success: true}` with no error.
- `pkg/domain/task_frontmatter.go` — `TaskFrontmatter.Goals()` reads `goals:` via `GetStringSlice`. `domain.Task` embeds it, so `task.Goals()` is available and returns raw entries (wikilinks included). `GetStringSlice` comma-splits a scalar value, which is the spec's malformed-input behaviour — no extra handling. `TaskFrontmatter.SetRecurring(string)` exists for tests.
- `pkg/domain/frontmatter_map.go` — `GetStringSlice` semantics (nil → nil, `[]any`/`[]string` coerced, scalar string comma-split).
- `pkg/cli/cli.go` — `createCompleteCommand` constructs `ops.NewCompleteOperation(...)` and calls `printTaskCompleteResult(os.Stdout, result)` on the plain path. `runMutation` already prints the result as JSON via `PrintJSON`.
- `pkg/ops/complete_test.go` — the `BeforeEach` constructor call, and the `Context("task with associated goals")` block whose comment and assertions must be updated (see requirement 7).
- `mocks/complete-operation.go` / `mocks/goal-complete-operation.go` — counterfeiter fakes. `mocks.GoalCompleteOperation` exists; do not add a new interface.
- `docs/development-patterns.md` — § Output Format and the interface → constructor → struct pattern.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega, external `_test` packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors`; never `fmt.Errorf`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — exported doc comments, Interface → Constructor → Struct.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen/nestif/gocognit/dupl limits.
</context>

<requirements>
## 1. New file `pkg/ops/parent_cascade.go`

BSD license header (`// Copyright (c) 2026 Benjamin Borbe All rights reserved.` plus the two standard lines), `package ops`. It imports nothing beyond the standard library unless a symbol below requires it. It must not import `pkg/cli` and must not write to stdout/stderr.

```go
// ParentOutcome reports one parent-completion attempt made by a cascade.
//
// Result is ParentResultCompleted when the parent closed or was already
// terminal, and ParentResultRefused when the parent's closure check refused.
// OpenItems is populated only on a refusal and always carries at least one
// item; a completed entry omits the key from JSON.
type ParentOutcome struct {
	Name      string   `json:"name"`
	Result    string   `json:"result"`
	OpenItems []string `json:"open_items,omitempty"`
}

const (
	// ParentResultCompleted marks a parent that closed or was already terminal.
	ParentResultCompleted = "completed"
	// ParentResultRefused marks a parent whose closure check refused.
	ParentResultRefused = "refused"
)

// parentOutcomeFrom builds the report entry for one parent attempt.
// Success records completed; anything else records refused, carrying the
// operation's OpenItems or, when it has none, the returned error text.
func parentOutcomeFrom(name string, result MutationResult, err error) ParentOutcome {
	if err == nil && result.Success {
		return ParentOutcome{Name: name, Result: ParentResultCompleted}
	}
	items := result.OpenItems
	if len(items) == 0 {
		if err != nil {
			items = []string{err.Error()}
		} else {
			items = []string{"unknown error"}
		}
	}
	return ParentOutcome{Name: name, Result: ParentResultRefused, OpenItems: items}
}
```

The JSON tags are frozen: `name`, `result`, `open_items` (with `omitempty`). `ParentResultCompleted` and `ParentResultRefused` are exported because `pkg/cli` compares against them.

## 2. `MutationResult` gains `Parents`

In `pkg/ops/complete.go`, add to `MutationResult` after the `OpenItems` field:

```go
	// Parents reports every parent-completion attempt made by a cascade, in
	// document order. Omitted when no attempt ran.
	Parents []ParentOutcome `json:"parents,omitempty"`
```

The field is additive and `omitempty`; a nil slice is omitted from JSON, so `jq 'has("parents")'` prints `false` when no attempt ran.

## 3. `CompleteOperation` gains a `GoalCompleteOperation` dependency

In `pkg/ops/complete.go`:

1. Change the constructor signature (append the new parameter last):
   ```go
   func NewCompleteOperation(
       taskStorage storage.TaskStorage,
       dailyNoteStorage storage.DailyNoteStorage,
       currentDateTime libtime.CurrentDateTime,
       interactionCounter InteractionCounter,
       goalComplete GoalCompleteOperation,
   ) CompleteOperation
   ```
   Add `goalComplete: goalComplete` to the returned struct literal.
2. Add the field to `completeOperation`:
   ```go
   goalComplete GoalCompleteOperation
   ```
   Do **not** add `storage.GoalStorage` or `storage.TopicStorage` — the cascade delegates; the deliberate absence of a goal write documented in `Execute` stands untouched.
3. Update the `NewCompleteOperation` doc comment to state that `goalComplete` is the cascade dependency and that the operation still holds no goal storage.

## 4. Cascade to goals in `completeOperation.Execute`

In `Execute`, after the daily-note update block and before the final `return`, compute and attach the cascade:

```go
	parents := c.cascadeToGoals(ctx, vaultPath, vaultName, task)
	return MutationResult{
		Success:  true,
		Name:     task.Name,
		Vault:    vaultName,
		Warnings: warnings,
		Parents:  parents,
	}, nil
```

The recurring-task branch returns earlier via `handleRecurringTask`, so a recurring task never cascades — leave that path untouched.

Add the method to `pkg/ops/complete.go`:

```go
// cascadeToGoals attempts each goal named in the task's `goals:` frontmatter,
// in document order, with force=false and no close-out fields. It never
// re-implements a goal close: each attempt calls the goal complete operation.
// Every attempt is reported, and a refusal never fails the task.
//
// A goal's own report entries (its topic attempts) are appended after that
// goal's entry, so one task run surfaces the whole chain it reached.
func (c *completeOperation) cascadeToGoals(
	ctx context.Context,
	vaultPath string,
	vaultName string,
	task *domain.Task,
) []ParentOutcome {
	var parents []ParentOutcome
	for _, link := range task.Goals() {
		name := normaliseGoalLink(link)
		if name == "" {
			continue
		}
		result, err := c.goalComplete.Execute(ctx, vaultPath, name, vaultName, false, "", "")
		parents = append(parents, parentOutcomeFrom(name, result, err))
		parents = append(parents, result.Parents...)
	}
	return parents
}
```

Rules:
- The frozen call shape is exactly `Execute(ctx, vaultPath, name, vaultName, false, "", "")`. Never pass the task's `force` through; the parent attempt is always `force=false`.
- Every error from the parent is swallowed: `parentOutcomeFrom` consumes it and the loop continues. The task's `err` is never set by the cascade.
- An empty `normaliseGoalLink` result (e.g. a stray `[[]]`) is skipped, producing no entry.
- `result.Parents` is the nested goal→topic report added in prompt 2. It is nil today; appending it is forward-compatible and must not be removed.

## 5. Update the production call site in `pkg/cli/cli.go`

In `createCompleteCommand`'s runner, build a goal complete operation and pass it in. The goal complete constructor still has its current three-parameter signature at this prompt:

```go
					storageConfig := storage.NewConfigFromVault(vault)
					taskStore := storage.NewTaskStorage(storageConfig)
					dailyStore := storage.NewDailyNoteStorage(storageConfig)
					goalStore := storage.NewGoalStorage(storageConfig)
					interactionCounter, err := completeInteractionCounter(ctx, vault)
					if err != nil {
						return ops.MutationResult{}, err
					}
					goalCompleteOp := ops.NewGoalCompleteOperation(goalStore, taskStore, currentDateTime)
					completeOp := ops.NewCompleteOperation(
						taskStore,
						dailyStore,
						currentDateTime,
						interactionCounter,
						goalCompleteOp,
					)
```

Do not change the goal complete constructor in this prompt; prompt 2 adds its topic dependency.

## 6. Add `ParentOutcome` to `pkg/ops/export_test.go` only if a test needs the unexported helper

`ParentOutcome` is exported, so the external `ops_test` package can build and read it directly. Do not add an alias unless a test needs `parentOutcomeFrom`, which it should not — tests drive the cascade through `Execute`.

## 7. Tests in `pkg/ops/complete_test.go`

1. In `BeforeEach`, declare and wire the goal fake:
   ```go
   mockGoalComplete = &mocks.GoalCompleteOperation{}
   mockGoalComplete.ExecuteReturns(ops.MutationResult{Success: true}, nil)
   completeOp = ops.NewCompleteOperation(
       mockTaskStorage,
       mockDailyNoteStorage,
       currentDateTime,
       mockInteractionCounter,
       mockGoalComplete,
   )
   ```
   Add `mockGoalComplete *mocks.GoalCompleteOperation` to the `var (...)` block.
2. Update `Context("task with associated goals")`: the task now delegates instead of writing. Keep the existing negative storage assertions (`mockGoalStorage.FindGoalByNameCallCount() == 0`, `mockGoalStorage.WriteGoalCallCount() == 0`) — they still hold because `CompleteOperation` holds no goal storage — and rewrite the comment to say the cascade delegates to `GoalCompleteOperation` rather than writing the goal's `# Tasks` list. Add:
   - `It("delegates to the goal complete operation", ...)` asserting `mockGoalComplete.ExecuteCallCount() == 1`, and that the call args are `(ctx, vaultPath, "Test Goal", "test-vault", false, "", "")` via `ExecuteArgsForCall(0)`.
   - `It("reports the goal as completed", ...)` asserting `result.Parents` has length 1, `Parents[0].Name == "Test Goal"`, `Parents[0].Result == ops.ParentResultCompleted`, and `Parents[0].OpenItems` is empty.
3. Add `Context("cascade to goals", ...)` with these cases (each sets `task.SetGoals(...)` before `JustBeforeEach`):
   - **Two goals in document order**: `SetGoals([]string{"[[Goal A]]", "[[Goal B]]"})`; `ExecuteReturnsOnCall(0, MutationResult{Success: true}, nil)` and `ExecuteReturnsOnCall(1, MutationResult{Success: false, OpenItems: []string{"open box"}}, errors.New("refused"))`. Assert `len(result.Parents) == 2`, `Parents[0].Result == ParentResultCompleted`, `Parents[1].Result == ParentResultRefused`, `Parents[1].OpenItems == []string{"open box"}`, and the two `ExecuteArgsForCall` names are `Goal A` then `Goal B`.
   - **Refusal does not fail the task**: one goal returning `(MutationResult{Success: false, OpenItems: []string{"x"}}, errors.New("refused"))`; assert `err` is nil, `result.Success` is true, `mockTaskStorage.WriteTaskCallCount() == 1`.
   - **Refusal with no OpenItems falls back to the error text**: goal returns `(MutationResult{Success: false}, errors.New("find goal: not found"))`; assert `Parents[0].OpenItems == []string{"find goal: not found"}`.
   - **Wikilink forms normalise**: `DescribeTable` over `"[[Goal A]]"`, `"[[Goal A|label]]"`, `"[[Goal A#Heading]]"`, `"Goal A"`, each asserting `ExecuteArgsForCall(0)`'s goal name is `Goal A`.
   - **Nested topic entries are flattened**: goal returns `(MutationResult{Success: true, Parents: []ops.ParentOutcome{{Name: "Topic A", Result: ops.ParentResultCompleted}}}, nil)`; assert `len(result.Parents) == 2`, `Parents[0].Name == "Goal A"`, `Parents[1].Name == "Topic A"`.
   - **No goals field**: leave `SetGoals` unset; assert `mockGoalComplete.ExecuteCallCount() == 0` and `result.Parents` is empty.
   - **Recurring task never cascades**: call `task.SetRecurring("1d")` (exists on `TaskFrontmatter`, embedded by `domain.Task`) and `SetGoals([]string{"Goal A"})`; assert `mockGoalComplete.ExecuteCallCount() == 0` and `result.Parents` is empty.
   - **Task-level write failure does not cascade**: `mockTaskStorage.WriteTaskReturns(errors.New("boom"))`; assert the error is returned, `result.Success` is false, and `mockGoalComplete.ExecuteCallCount() == 0`.

Test files may use the standard library `errors`/`fmt` freely; only production `pkg/ops` code must use `github.com/bborbe/errors` with `ctx`. Check what the existing test file imports and follow it.

## 8. Do not touch the goal→topic hop or CLI formatting

The goal complete operation's signature and body are out of scope here. `printTaskCompleteResult` and the goal complete success line are out of scope here; prompt 2 adds parent printing for both hops.
</requirements>

<constraints>
- **`pkg/ops` stays a library layer**: it returns structured results and never writes to stdout. No `fmt.Print*` in `pkg/ops`.
- **The closure rule is not duplicated**: `checkClosureGate` in `pkg/ops/closure_gate.go` remains the single implementation. This prompt adds no closure logic.
- **`CompleteOperation` holds no `GoalStorage` or `TopicStorage`.** The cascade delegates to `GoalCompleteOperation`; `grep -c 'WriteGoal' pkg/ops/complete.go` must print `0`.
- **Frozen call shape**: `GoalCompleteOperation.Execute(ctx, vaultPath, name, vaultName, false, "", "")`. Never pass the child's `force`, reason or successor.
- **A parent refusal never changes the child's outcome or exit code.** The cascade swallows every parent error.
- **Recurring tasks never cascade.** The `handleRecurringTask` early return is the guard; do not move the cascade above it.
- **`MutationResult.Parents` is additive and `omitempty`.** Do not rename or re-tag it.
- **Every new exported type, constant and method carries GoDoc.**
- **Error wrapping in production code** uses `github.com/bborbe/errors` with `ctx`; never `fmt.Errorf`, never `context.Background()` inside `pkg/ops`.
- **Existing tests must still pass**, except the `Context("task with associated goals")` block explicitly updated above.
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
Run `make test` — must exit 0. Re-run it iteratively while implementing.

Focused run (the focus must match the new specs; a focus matching zero specs exits 0 while running nothing):

```
go test -mod=mod ./pkg/ops -run TestSuite -ginkgo.focus='cascade|associated goals' -ginkgo.v
```

The `-ginkgo.v` output must contain the new spec names from requirement 7.

Structural greps (each count is exact):

```
grep -c 'WriteGoal' pkg/ops/complete.go                     # 0
grep -c 'goalComplete.Execute' pkg/ops/complete.go          # 1
grep -c 'Parents \[\]ParentOutcome' pkg/ops/complete.go     # 1
grep -c 'func parentOutcomeFrom' pkg/ops/parent_cascade.go  # 1
grep -c 'func checkClosureGate' pkg/ops/closure_gate.go     # 1
```

`go vet -mod=mod ./pkg/ops/... ./pkg/cli/...` must exit 0.

Finally run `make precommit` once — must exit 0. If it exits non-zero, fix and re-run the failing target, then `make precommit` again. A non-zero `make precommit` is a failure, never rationalised as success.

<!--
Reviewer note — one spec Failure Modes row is structurally impossible here:
"Cascade read of `goals:`/`topics:` fails | Warning recorded, child still completes".
`task.Goals()` and `goal.Topics()` are in-memory `FrontmatterMap` reads with no
error return, so there is no failure path to warn about. The row's real failure
(a parent page that cannot be read) is covered by the "Parent goal file missing or
unreadable" row: the goal complete operation returns an error, and
`parentOutcomeFrom` records it as `refused` with the error text. No synthetic
warning is added.
-->
</verification>
