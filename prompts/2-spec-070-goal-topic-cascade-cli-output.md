---
spec: ["070-parent-closure-cascade-on-complete"]
status: draft
created: "2026-11-19T18:56:00Z"
---

# Goal completion cascades to its parent topics, and both hops print their report

<summary>
- Completing a goal now also attempts to complete every topic named in the goal's `topics:` frontmatter, in document order.
- A goal that is already `completed` or `aborted` still attempts its topics, so a topic can close on a later run.
- A topic that refuses never fails the goal: the goal stays completed and the command still exits 0.
- Both `task complete` and `goal complete` print one `parent …` line per attempt in plain output.
- A completed parent prints `parent <name>: completed`; a refused parent prints `parent <name>: refused — open: <item>; <item>`.
- `--output json` carries the same attempts in the `parents` array; a completed entry omits `open_items`.
- The goal cascade delegates to the existing topic complete operation — it does not re-implement a topic close.
- Topics gain no parent cascade of their own.
</summary>

<objective>
Add a `Topics()` accessor to goals, give `GoalCompleteOperation` a `TopicCompleteOperation` dependency, cascade goal completion to the goal's topics on every terminal and successful path, and add plain-text parent reporting for both `task complete` and `goal complete`. This is the second of three prompts and depends on the `ParentOutcome` type and `MutationResult.Parents` field added in prompt 1.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read these files fully before changing anything:

- `specs/in-progress/070-parent-closure-cascade-on-complete.md` — the whole spec. Desired Behavior 2-9 and 11, Constraints, and Failure Modes are binding. This prompt implements the goal→topic half and the CLI reporting.
- `pkg/ops/parent_cascade.go` — the `ParentOutcome` type, the two result constants, and `parentOutcomeFrom` added by prompt 1. Reuse them.
- `pkg/ops/goal_complete.go` — the whole file. `Execute` currently returns early on an already-terminal goal and again on a `closureOutcomeTerminal` verdict, and returns a bare success after `WriteGoal`. All three return points must now attach the topic cascade.
- `pkg/ops/topic_complete.go` — `TopicCompleteOperation.Execute(ctx, vaultPath, topicName, vaultName)`. It returns `MutationResult{Success: false, OpenItems: ...}` plus an error on a refusal and `MutationResult{Success: true}` on success or an already-terminal topic.
- `pkg/ops/closure_gate.go` — `normaliseGoalLink(link string) string`; reuse it for topic names too. Topic names carry the same wikilink forms.
- `pkg/domain/goal_frontmatter.go` — add `Topics()` next to the existing accessors. `domain.Goal` embeds `GoalFrontmatter`, so `goal.Topics()` resolves.
- `pkg/domain/frontmatter_map.go` — `GetStringSlice` semantics, including the scalar comma-split (the spec's malformed-`topics:` behaviour).
- `pkg/cli/cli.go` — `printTaskCompleteResult(w io.Writer, result ops.MutationResult)`, `createGoalCompleteCommand`'s success line, `createCompleteCommand`'s runner, and `createGoalCompleteCommand`'s runner. `strings` and `io` are already imported.
- `pkg/ops/goal_complete_test.go` — the `BeforeEach` constructor call and the terminal/refusal contexts. Add the topic fake there.
- `mocks/topic-complete-operation.go` / `mocks/topic-storage.go` — counterfeiter fakes; both exist.
- `docs/development-patterns.md` — § Output Format (`--output json` via `PrintJSON`, never `encoding/json` in command files).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega, external `_test` packages, `DescribeTable`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors`; never `fmt.Errorf`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — exported doc comments, Interface → Constructor → Struct.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen/nestif/gocognit/dupl limits.
</context>

<requirements>
## 1. `GoalFrontmatter` gains `Topics()`

In `pkg/domain/goal_frontmatter.go`, add next to the other list accessors:

```go
// Topics reads "topics" key via GetStringSlice.
func (f GoalFrontmatter) Topics() []string { return f.GetStringSlice("topics") }
```

Do not add a `SetTopics` — nothing in this spec writes the field.

## 2. `GoalCompleteOperation` gains a `TopicCompleteOperation` dependency

In `pkg/ops/goal_complete.go`:

1. Change the constructor signature (append the new parameter last):
   ```go
   func NewGoalCompleteOperation(
       goalStorage storage.GoalStorage,
       taskStorage storage.TaskStorage,
       currentDateTime libtime.CurrentDateTime,
       topicComplete TopicCompleteOperation,
   ) GoalCompleteOperation
   ```
   Add `topicComplete: topicComplete` to the returned struct literal.
2. Add the field to `goalCompleteOperation`:
   ```go
   topicComplete TopicCompleteOperation
   ```
   Do **not** add `storage.TopicStorage` — the cascade delegates; `grep -c 'WriteTopic' pkg/ops/goal_complete.go` must print `0`.
3. Update the constructor doc comment to name `topicComplete` as the cascade dependency.

## 3. Cascade to topics in `goalCompleteOperation.Execute`

Every path that reports the goal `completed` must attach the topic cascade. Restructure `Execute` so the two terminal returns and the successful-close return all carry it:

```go
	if goal.Status() == domain.GoalStatusCompleted || goal.Status() == domain.GoalStatusAborted {
		return MutationResult{
			Success: true,
			Name:    goal.Name,
			Vault:   vaultName,
			Parents: g.cascadeToTopics(ctx, vaultPath, vaultName, goal),
		}, nil
	}
```

and, for the gate's terminal verdict:

```go
	if verdict.Outcome == closureOutcomeTerminal {
		return MutationResult{
			Success: true,
			Name:    goal.Name,
			Vault:   vaultName,
			Parents: g.cascadeToTopics(ctx, vaultPath, vaultName, goal),
		}, nil
	}
```

and, after `WriteGoal` succeeds:

```go
	return MutationResult{
		Success: true,
		Name:    goal.Name,
		Vault:   vaultName,
		Parents: g.cascadeToTopics(ctx, vaultPath, vaultName, goal),
	}, nil
```

The refusal path (`verdict.Outcome != closureOutcomeClose`) is unchanged: a refused goal does **not** cascade, and it carries no `Parents`.

Add the method:

```go
// cascadeToTopics attempts each topic named in the goal's `topics:` frontmatter,
// in document order, with the topic complete operation. Every attempt is
// reported, and a refusal never fails the goal. An already-terminal goal still
// cascades, so a topic can close on a later run even when its goal closed earlier.
func (g *goalCompleteOperation) cascadeToTopics(
	ctx context.Context,
	vaultPath string,
	vaultName string,
	goal *domain.Goal,
) []ParentOutcome {
	var parents []ParentOutcome
	for _, link := range goal.Topics() {
		name := normaliseGoalLink(link)
		if name == "" {
			continue
		}
		result, err := g.topicComplete.Execute(ctx, vaultPath, name, vaultName)
		parents = append(parents, parentOutcomeFrom(name, result, err))
	}
	return parents
}
```

Rules:
- The frozen call shape is exactly `Execute(ctx, vaultPath, name, vaultName)`.
- Every error from the topic is swallowed by `parentOutcomeFrom`; the goal's own `err` is never set by the cascade.
- `goal.Topics()` is read from the in-memory goal object, so it works on both the already-terminal branch and the just-written branch.

## 4. CLI: print one line per parent attempt

In `pkg/cli/cli.go`, add near `printTaskCompleteResult`:

```go
// printParentOutcomes writes one line per parent-completion attempt made by a
// cascade. A completed parent prints "parent <name>: completed"; a refused
// parent prints "parent <name>: refused — open: <item>; <item>".
func printParentOutcomes(w io.Writer, parents []ops.ParentOutcome) {
	for _, parent := range parents {
		if parent.Result == ops.ParentResultCompleted {
			fmt.Fprintf(w, "parent %s: completed\n", parent.Name)
			continue
		}
		fmt.Fprintf(
			w,
			"parent %s: refused — open: %s\n",
			parent.Name,
			strings.Join(parent.OpenItems, "; "),
		)
	}
}
```

The em dash is U+2014 (`—`), exactly as written in the spec. Join refusal items with `"; "`.

Then:
1. In `printTaskCompleteResult`, after the warnings loop, add `printParentOutcomes(w, result.Parents)`.
2. In `createGoalCompleteCommand`'s success branch, add the parent print after the existing success line:
   ```go
   if !OutputFormat(*outputFormat).IsJSON() {
       fmt.Printf("✅ Goal completed: %s\n", result.Name)
       printParentOutcomes(os.Stdout, result.Parents)
   }
   ```
3. Do not add parent printing to `createTopicCompleteCommand`: topics have no parent cascade, so `result.Parents` is always nil there.

The JSON path needs no change: `runMutation` already calls `PrintJSON(result)` and `MutationResult.Parents` is serialised with `omitempty`.

## 5. Wire the constructors in `pkg/cli/cli.go`

### 5.1 `createCompleteCommand`'s runner

The goal complete operation now needs a topic complete operation. Replace the wiring added by prompt 1 with:

```go
					storageConfig := storage.NewConfigFromVault(vault)
					taskStore := storage.NewTaskStorage(storageConfig)
					dailyStore := storage.NewDailyNoteStorage(storageConfig)
					goalStore := storage.NewGoalStorage(storageConfig)
					topicStore := storage.NewTopicStorage(storageConfig)
					interactionCounter, err := completeInteractionCounter(ctx, vault)
					if err != nil {
						return ops.MutationResult{}, err
					}
					topicCompleteOp := ops.NewTopicCompleteOperation(topicStore, goalStore)
					goalCompleteOp := ops.NewGoalCompleteOperation(
						goalStore,
						taskStore,
						currentDateTime,
						topicCompleteOp,
					)
					completeOp := ops.NewCompleteOperation(
						taskStore,
						dailyStore,
						currentDateTime,
						interactionCounter,
						goalCompleteOp,
					)
```

### 5.2 `createGoalCompleteCommand`'s runner

```go
					storageConfig := storage.NewConfigFromVault(vault)
					goalStore := storage.NewGoalStorage(storageConfig)
					taskStore := storage.NewTaskStorage(storageConfig)
					topicStore := storage.NewTopicStorage(storageConfig)
					topicCompleteOp := ops.NewTopicCompleteOperation(topicStore, goalStore)
					completeOp := ops.NewGoalCompleteOperation(
						goalStore,
						taskStore,
						currentDateTime,
						topicCompleteOp,
					)
```

Leave `createTopicCompleteCommand` untouched.

## 6. Tests in `pkg/ops/goal_complete_test.go`

1. Add to the `var (...)` block: `mockTopicComplete *mocks.TopicCompleteOperation` and `mockTopicStorage *mocks.TopicStorage`.
2. In `BeforeEach`, wire them:
   ```go
   mockTopicComplete = &mocks.TopicCompleteOperation{}
   mockTopicComplete.ExecuteReturns(ops.MutationResult{Success: true}, nil)
   mockTopicStorage = &mocks.TopicStorage{}
   op = ops.NewGoalCompleteOperation(
       mockGoalStorage,
       mockTaskStorage,
       currentDateTime,
       mockTopicComplete,
   )
   ```
   The default goal fixture has no `topics:`, so existing contexts call the cascade zero times and need no other change. `mockTopicStorage` is deliberately not wired into the goal operation; it exists so the single-write-path assertion below has a target.
3. Add `Context("cascade to topics", ...)`. Each case sets the goal's `topics:` via `domain.NewGoal(map[string]any{"status": ..., "topics": []any{"[[Topic A]]"}, ...}, ...)` (or `goal.Set("topics", []any{...})` — grep for the existing fixture style and follow it).
   - **Closes a topic and reports it**: one `topics:` entry; assert `err` nil, `result.Success` true, `mockTopicComplete.ExecuteCallCount() == 1`, the call args are `(ctx, vaultPath, "Topic A", "test-vault")`, `len(result.Parents) == 1`, `Parents[0].Result == ops.ParentResultCompleted`, and `mockTopicStorage.WriteTopicCallCount() == 0`.
   - **Two topics in document order**: `["[[Topic A]]", "[[Topic B]]"]`; assert `len(result.Parents) == 2` and the two `ExecuteArgsForCall` names are `Topic A` then `Topic B`.
   - **Refused topic does not fail the goal**: `mockTopicComplete.ExecuteReturns(ops.MutationResult{Success: false, OpenItems: []string{"open gate"}}, errors.New("refused"))`; assert `err` nil, `result.Success` true, `Parents[0].Result == ops.ParentResultRefused`, `Parents[0].OpenItems == []string{"open gate"}`, and the goal was still written (`mockGoalStorage.WriteGoalCallCount() == 1`).
   - **Already-terminal goal still cascades**: set the goal status to `completed` (and a second case `aborted`) with one `topics:` entry; assert `err` nil, `result.Success` true, `mockGoalStorage.WriteGoalCallCount() == 0`, `mockTopicComplete.ExecuteCallCount() == 1`, and `Parents[0].Result == ops.ParentResultCompleted`.
   - **No topics field**: assert `mockTopicComplete.ExecuteCallCount() == 0` and `result.Parents` is empty.
   - **Wikilink forms normalise**: `DescribeTable` over `"[[Topic A]]"`, `"[[Topic A|label]]"`, `"[[Topic A#Heading]]"`, `"Topic A"`, each asserting `ExecuteArgsForCall(0)`'s topic name is `Topic A`.
   - **A refused goal does not cascade**: goal with one open SC box and a `topics:` entry; assert the error is returned and `mockTopicComplete.ExecuteCallCount() == 0`.

## 7. CLI output test

Add a spec (in `pkg/cli/force_tty_test.go` or a new `pkg/cli/parent_output_test.go`) that calls the parent printer through an exported test hook and asserts the exact strings:
- a completed parent renders `parent Goal A: completed\n`;
- a refused parent with two items renders `parent Goal B: refused — open: first; second\n`.

Add `PrintParentOutcomesForTest(w io.Writer, parents []ops.ParentOutcome)` to `pkg/cli/export_test.go`, following the existing `PrintTaskCompleteResultForTest` pattern. Write into a `bytes.Buffer`; never swap `os.Stdout`.
</requirements>

<constraints>
- **`pkg/ops` stays a library layer**: it returns structured results and never writes to stdout. No `fmt.Print*` in `pkg/ops`.
- **The closure rule is not duplicated**: `checkClosureGate` in `pkg/ops/closure_gate.go` remains the single implementation. This prompt adds no closure logic.
- **`GoalCompleteOperation` holds no `TopicStorage`.** The cascade delegates to `TopicCompleteOperation`; `grep -c 'WriteTopic' pkg/ops/goal_complete.go` must print `0`.
- **Frozen call shape**: `TopicCompleteOperation.Execute(ctx, vaultPath, name, vaultName)`.
- **A topic refusal never changes the goal's outcome or exit code.** The cascade swallows every topic error.
- **A refused goal does not cascade.** Only the completed/terminal paths attach `Parents`.
- **No cascade above topic level.** Do not add a theme or objective attempt.
- **The text format is exact**: `parent <name>: completed` and `parent <name>: refused — open: <item>; <item>` (em dash U+2014, items joined with `"; "`).
- **`MutationResult.Parents` stays additive and `omitempty`.** Do not rename or re-tag it.
- **Every new exported type, constant and method carries GoDoc.**
- **Error wrapping in production code** uses `github.com/bborbe/errors` with `ctx`; never `fmt.Errorf`, never `context.Background()` inside `pkg/ops`.
- **Existing tests must still pass**; update the `NewGoalCompleteOperation` call site in `pkg/ops/goal_complete_test.go` and the two call sites in `pkg/cli/cli.go`.
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
Run `make test` — must exit 0. Re-run it iteratively while implementing.

Focused runs (a focus matching zero specs exits 0 while running nothing, so confirm the names appear):

```
go test -mod=mod ./pkg/ops -run TestSuite -ginkgo.focus='cascade to topics' -ginkgo.v
go test -mod=mod ./pkg/cli/... -run TestSuite -ginkgo.v
```

Structural greps (each count is exact):

```
grep -c 'WriteTopic' pkg/ops/goal_complete.go                        # 0
grep -c 'topicComplete.Execute' pkg/ops/goal_complete.go             # 1
grep -c 'func (f GoalFrontmatter) Topics' pkg/domain/goal_frontmatter.go  # 1
grep -c 'func printParentOutcomes' pkg/cli/cli.go                    # 1
grep -c 'printParentOutcomes(' pkg/cli/cli.go                       # 3 (definition + 2 call sites)
grep -c 'func checkClosureGate' pkg/ops/closure_gate.go              # 1
```

`grep -rn 'NewGoalCompleteOperation' pkg/` must show only `pkg/ops/goal_complete.go`, `pkg/ops/goal_complete_test.go`, and `pkg/cli/cli.go`.

`go vet -mod=mod ./pkg/ops/... ./pkg/cli/... ./pkg/domain/...` must exit 0.

Finally run `make precommit` once — must exit 0. If it exits non-zero, fix and re-run the failing target, then `make precommit` again. A non-zero `make precommit` is a failure, never rationalised as success.
</verification>
