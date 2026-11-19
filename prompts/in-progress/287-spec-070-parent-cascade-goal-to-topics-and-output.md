---
status: approved
spec: [070-parent-closure-cascade-on-complete]
created: "2026-11-19T20:10:00Z"
queued: "2026-11-19T20:08:41Z"
branch: dark-factory/parent-closure-cascade-on-complete
---

# Cascade `goal complete` to the goal's parent topics, and print both hops

<summary>
- Completing a goal now also tries to complete every topic listed in the goal's `topics:` frontmatter, in document order.
- A topic that refuses does not fail the goal: the goal stays completed and the command still exits 0.
- A goal that is already completed still attempts its topics, so a topic can close on a later run after its goal closed earlier.
- `vault-cli task complete` and `vault-cli goal complete` print one line per parent attempt: `parent <name>: completed` or `parent <name>: refused — open: <item>; <item>`.
- `--output json` carries the same attempts in a `parents` array, with `open_items` omitted on a completed entry.
- The goal page's `topics:` field becomes a documented relation, with its wikilink forms and the cascade rule.
- No change to the closure rule, to the task close itself, or to `topic complete`.
</summary>

<objective>
Wire the goal → topic hop into `GoalCompleteOperation` using the cascade helper from prompt 1, expose both hops' results on stdout and in JSON, and document the `topics:` relation. This completes spec 070's production behaviour; prompt 3 adds the integration and invariant test coverage.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` (§ Output Format) for project conventions.

Read these files fully before changing anything:

- `specs/in-progress/070-parent-closure-cascade-on-complete.md` — the whole spec. § Desired Behavior 2-11, § Constraints and § Failure Modes bind this prompt (DB 2, 3, 4, 5, 6, 7, 8, 9, 11).
- `pkg/ops/parent_cascade.go` — delivered by prompt 1. `cascadeParents(ctx, names, attempt)` normalises each wikilink, skips empty names, never returns an error, and returns one `ParentAttempt` per attempt. `MutationResult.Parents []ParentAttempt` already exists.
- `pkg/ops/goal_complete.go` — the whole file. `Execute` has three early returns and one final success return: the already-terminal goal (line 71), the gate's `closureOutcomeTerminal` (line 87), the gate refusal (line 90), and the write success (line 119). All three terminal paths must cascade; the refusal path must not.
- `pkg/ops/topic_complete.go` — `TopicCompleteOperation.Execute(ctx, vaultPath, topicName, vaultName) (MutationResult, error)`. This is the operation the goal cascade calls; do not change it.
- `pkg/domain/goal_frontmatter.go` — `GoalFrontmatter` is embedded by `domain.Goal`; `Tags()` (line 111) shows the `GetStringSlice` accessor shape to mirror.
- `pkg/domain/page.go` — `Page.Goals()` (line 51) is the accessor the spec names as the model.
- `pkg/cli/cli.go` — `printTaskCompleteResult` (line 312), `createCompleteCommand`'s runner (line 270), `createGoalCompleteCommand`'s runner (line 1544), `printOpenItems` (line 140). `fmt`, `io` and `strings` are already imported.
- `docs/goal-writing.md` — `### Dependencies (\`blocked_by\`)` ends at line 182 and `### Session (\`claude_session_id\`)` starts at line 184. The new `### Topics (\`topics\`)` section goes between them.
- `pkg/ops/goal_complete_test.go` — `BeforeEach` at line 41 and the constructor call at line 50.
- `mocks/topic-complete-operation.go` — `mocks.TopicCompleteOperation` with `ExecuteReturns`, `ExecuteCallCount`, `ExecuteArgsForCall(i) (context.Context, string, string, string)`.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — Interface → Constructor → Struct, exported GoDoc.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega, external `ops_test` package.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen 80 lines / 50 statements, nestif 4, dupl.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — not used in this prompt; prompt 3 owns the changelog.
</context>

<requirements>
## 1. `Topics()` accessor on the goal frontmatter

In `pkg/domain/goal_frontmatter.go`, next to `Tags()`:

```go
// Topics reads "topics" key via GetStringSlice. It is the goal->topic parent
// relation the completion cascade walks; each entry may be a wikilink.
func (f GoalFrontmatter) Topics() []string { return f.GetStringSlice("topics") }
```

The spec's § Assumptions describes the accessor as mirroring `Page.Goals()`. `domain.Goal` embeds `GoalFrontmatter`, not `Page`, so this is the type the cascade actually consumes; `GetStringSlice` comma-splits a scalar value, which is the spec's stated malformed-input behaviour (§ Failure Modes, last row).

Also add the mirrored accessor on `Page` so the relation is readable from any page-shaped read model, next to `Page.Goals()`:

```go
// Topics reads "topics" key via GetStringSlice.
func (p Page) Topics() []string { return p.GetStringSlice("topics") }
```

## 2. `GoalCompleteOperation` gains the topic-complete dependency

In `pkg/ops/goal_complete.go`:

1. Add the field:
   ```go
   type goalCompleteOperation struct {
   	goalStorage           storage.GoalStorage
   	taskStorage           storage.TaskStorage
   	currentDateTime       libtime.CurrentDateTime
   	topicCompleteOperation TopicCompleteOperation
   }
   ```
2. Add the parameter as the LAST parameter of `NewGoalCompleteOperation`:
   ```go
   func NewGoalCompleteOperation(
   	goalStorage storage.GoalStorage,
   	taskStorage storage.TaskStorage,
   	currentDateTime libtime.CurrentDateTime,
   	topicCompleteOperation TopicCompleteOperation,
   ) GoalCompleteOperation
   ```
   Assign it in the returned struct. Extend the constructor doc comment: the goal close runs through this operation for its own parent hop, so the topic close sequence exists once.
3. Add the helper:
   ```go
   // cascadeTopics attempts every topic declared in the goal's `topics:` list, in
   // document order. It runs only once the goal is terminal: a topic's gate
   // requires its member goals terminal, so an open goal could only refuse. The
   // topics of an already-terminal goal are still attempted, which is how a
   // topic closes on a later run after its goal closed earlier.
   func (g *goalCompleteOperation) cascadeTopics(
   	ctx context.Context,
   	vaultPath string,
   	vaultName string,
   	goal *domain.Goal,
   ) []ParentAttempt {
   	return cascadeParents(
   		ctx,
   		goal.Topics(),
   		func(ctx context.Context, topicName string) (MutationResult, error) {
   			return g.topicCompleteOperation.Execute(ctx, vaultPath, topicName, vaultName)
   		},
   	)
   }
   ```
4. Attach the cascade to all three terminal paths and nowhere else.

   The already-terminal early return becomes:
   ```go
   	if goal.Status() == domain.GoalStatusCompleted || goal.Status() == domain.GoalStatusAborted {
   		result := MutationResult{Success: true, Name: goal.Name, Vault: vaultName}
   		result.Parents = g.cascadeTopics(ctx, vaultPath, vaultName, goal)
   		return result, nil
   	}
   ```
   The gate's terminal verdict becomes:
   ```go
   	if verdict.Outcome == closureOutcomeTerminal {
   		result := MutationResult{Success: true, Name: goal.Name, Vault: vaultName}
   		result.Parents = g.cascadeTopics(ctx, vaultPath, vaultName, goal)
   		return result, nil
   	}
   ```
   The final success return after `WriteGoal` becomes:
   ```go
   	result := MutationResult{Success: true, Name: goal.Name, Vault: vaultName}
   	result.Parents = g.cascadeTopics(ctx, vaultPath, vaultName, goal)
   	return result, nil
   ```
   The refusal return (`closureOutcomeOpen` / `closureOutcomeUnmeasurable`) stays exactly as it is — it carries no `Parents`, because a non-terminal goal's topics cannot close.

   The topic operation re-reads the goal from disk, so the topic gate sees the goal's just-written `status: completed`. Do not pass the in-memory goal or a status override.

## 3. Update every `NewGoalCompleteOperation` call site

`grep -rn 'NewGoalCompleteOperation' pkg/ integration/` must afterwards show exactly three files: `pkg/ops/goal_complete.go`, `pkg/ops/goal_complete_test.go`, `pkg/cli/cli.go`.

In `pkg/cli/cli.go`:

- `createCompleteCommand`'s runner (the call prompt 1 added): add the topic store and pass it:
  ```go
  					goalCompleteOp := ops.NewGoalCompleteOperation(
  						goalStore,
  						taskStore,
  						currentDateTime,
  						ops.NewTopicCompleteOperation(
  							storage.NewTopicStorage(storageConfig),
  							goalStore,
  						),
  					)
  ```
- `createGoalCompleteCommand`'s runner (around line 1549-1556): build the topic store and pass the fourth argument:
  ```go
  					storageConfig := storage.NewConfigFromVault(vault)
  					goalStore := storage.NewGoalStorage(storageConfig)
  					taskStore := storage.NewTaskStorage(storageConfig)
  					topicStore := storage.NewTopicStorage(storageConfig)
  					completeOp := ops.NewGoalCompleteOperation(
  						goalStore,
  						taskStore,
  						currentDateTime,
  						ops.NewTopicCompleteOperation(topicStore, goalStore),
  					)
  ```

## 4. CLI printing for both hops

In `pkg/cli/cli.go`, next to `printOpenItems`:

```go
// printParents writes one stdout line per cascade attempt. A completed parent
// prints "parent <name>: completed"; a refusal prints the parent's open items.
// The child's own outcome line is printed by the command before this runs.
func printParents(w io.Writer, attempts []ops.ParentAttempt) {
	for _, attempt := range attempts {
		if attempt.Result == ops.ParentResultCompleted {
			fmt.Fprintf(w, "parent %s: completed\n", attempt.Name)
			continue
		}
		fmt.Fprintf(
			w,
			"parent %s: refused — open: %s\n",
			attempt.Name,
			strings.Join(attempt.OpenItems, "; "),
		)
	}
}
```

The dash is the literal em dash `—` (U+2014), matching the spec's output shape.

- In `createCompleteCommand`'s runner, extend the plain-mode block:
  ```go
  					if !OutputFormat(*outputFormat).IsJSON() {
  						printTaskCompleteResult(os.Stdout, result)
  						printParents(os.Stdout, result.Parents)
  					}
  ```
- In `createGoalCompleteCommand`'s runner, extend the success block:
  ```go
  					if !OutputFormat(*outputFormat).IsJSON() {
  						fmt.Printf("✅ Goal completed: %s\n", result.Name)
  						printParents(os.Stdout, result.Parents)
  					}
  ```

Do not print parents on an error return: the only error paths are the child's own refusal, where no cascade ran. `runMutation` already prints the whole `MutationResult` (including `parents`) as JSON in `--output json` mode.

## 5. Unit tests

### 5.1 Update `pkg/ops/goal_complete_test.go`

1. Declare `mockTopicCompleteOperation *mocks.TopicCompleteOperation` in the `var` block.
2. In `BeforeEach`, before constructing the operation:
   ```go
   		mockTopicCompleteOperation = &mocks.TopicCompleteOperation{}
   		mockTopicCompleteOperation.ExecuteReturns(ops.MutationResult{Success: true}, nil)
   ```
   and pass it as the fourth argument to `ops.NewGoalCompleteOperation`.
3. Add a `Context("parent cascade", ...)` with:

   - `It("does not cascade a goal with no topics")` — default fixture; assert `mockTopicCompleteOperation.ExecuteCallCount() == 0` and `result.Parents` is empty.
   - `It("cascades to the declared topic")` — build the goal with `mockGoalStorage.FindGoalByNameReturns(goalWithTopics("active", []string{"My Topic"}), nil)` (do not change the shared `BeforeEach` fixture — `It("does not cascade a goal with no topics")` depends on it having no topics); assert one call with args `(ctx, vaultPath, "My Topic", vaultName)` and `result.Parents[0].Result == ops.ParentResultCompleted`.
   - `It("reports a refused topic and still completes the goal")` — topic fake returns `(ops.MutationResult{Success: false, OpenItems: []string{"gate item"}}, ErrTest)`; assert `err` is nil, `result.Success` is true, `result.Parents[0].Result == ops.ParentResultRefused`, `result.Parents[0].OpenItems == []string{"gate item"}`, `mockGoalStorage.WriteGoalCallCount() == 1`.
   - `It("normalises a wikilink topic name")` — topics `[]string{"[[My Topic|label]]"}`; assert the fake was called with `"My Topic"`.
   - `It("cascades from an already-completed goal")` — goal `status: completed`, topics `[]string{"My Topic"}`; assert `mockTopicCompleteOperation.ExecuteCallCount() == 1`, `result.Parents[0].Result == ops.ParentResultCompleted`, and `mockGoalStorage.WriteGoalCallCount() == 0`.
   - `It("does not cascade when the goal refuses")` — goal with one open SC box and topics `[]string{"My Topic"}`; assert the error is non-nil and `mockTopicCompleteOperation.ExecuteCallCount() == 0`.

   For the topics fixture, add a package-level helper beside `completeGoalContent`:
   ```go
   func goalWithTopics(status string, topics []string) *domain.Goal {
   	return domain.NewGoal(
   		map[string]any{"status": status, "topics": topics},
   		domain.FileMetadata{Name: "my-goal"},
   		completeGoalContent(status),
   	)
   }
   ```
   Use it as `mockGoalStorage.FindGoalByNameReturns(goalWithTopics("active", []string{"My Topic"}), nil)` in the topic contexts. Do not build the list by string concatenation. Keep `completeGoalContent` unchanged.

### 5.2 New file `pkg/cli/cli_parents_test.go` (package `cli_test`)

Follow the existing external test package style used by `pkg/cli/output_test.go` (`package cli_test`). Expose the helper from `pkg/cli/export_test.go` — either `var PrintParents = printParents` or a `PrintParentsForTest(w io.Writer, attempts []ops.ParentAttempt)` wrapper matching the file's existing `...ForTest` convention; then add one `Describe("printParents", ...)` in the new file:

- `It("prints one completed line per completed attempt")` — two completed attempts; assert stdout is exactly `parent Goal A: completed\nparent Goal B: completed\n`.
- `It("prints a refused line with every open item joined by semicolons")` — one refused attempt with `[]string{"box one", "box two"}`; assert the line is `parent Goal A: refused — open: box one; box two\n`.
- `It("prints nothing for an empty attempt list")` — assert the writer stays empty.

Use `bytes.Buffer` as the `io.Writer`.

## 6. Documentation: `docs/goal-writing.md` gains a `topics:` section

Insert a new `### Topics (\`topics\`)` section between `### Dependencies (\`blocked_by\`)` (ends line 182) and `### Session (\`claude_session_id\`)` (starts line 184). Content requirements:

- The field is a YAML list of topic names; each entry may be a plain name or a wikilink, and `[[Name]]`, `[[Name|label]]` and `[[Name#Heading]]` all resolve to the same topic.
- A scalar value is malformed and comma-splits on read; correct it to a list.
- `vault-cli goal complete "<name>"` closes the goal and then attempts every declared topic with the same closure gate, in document order. A topic that refuses does not fail the goal: the goal stays completed, the refusal is reported as `parent <topic>: refused — open: <item>`, and the command exits 0.
- A topic that is already `completed` reports `parent <topic>: completed` and is not rewritten; its own attempt still happens, so a topic can close on a later run after its goal closed earlier.
- `vault-cli task complete` does the same one level down: it closes the task, then every goal in the task's `goals:`, and each goal then attempts its `topics:`. Cascading stops at topic level — a topic's own parents are not attempted.
- `--output json` carries a `parents` array of `{name, result, open_items}` entries; `result` is `completed` or `refused`, and `open_items` is omitted on a completed entry.
- A recurring task never cascades.
- The cascade is in-process and fires only on a new complete; nothing retroactively closes pages that are already open.

Match the document's existing voice (present tense, second-person-free, concrete command examples in fenced blocks). Do not restructure any other section.
</requirements>

<constraints>
- **The cascade never changes the child's outcome.** A topic refusal leaves `result.Success == true` for the goal, the goal written, and `err == nil`.
- **The cascade never re-implements a close.** It calls `TopicCompleteOperation.Execute(ctx, vaultPath, topicName, vaultName)` — the frozen call shape. `checkClosureGate` remains the only closure rule.
- **Cascade only from a terminal goal.** The refusal path returns no `Parents`.
- **`pkg/ops` stays a library layer** — structured results only; all printing lives in `pkg/cli`.
- **The text line shape is frozen**: `parent <name>: completed` and `parent <name>: refused — open: <item>; <item>`. Use the literal em dash.
- **The JSON shape is frozen**: `parents` is an array of `{name, result, open_items}`; `open_items` is `omitempty`; the key is omitted when no attempt ran.
- **Do not change** `topic complete`, `checkClosureGate`, the task close, or any existing command output.
- **Do not touch** `CHANGELOG.md`, `integration/`, or `agents/`. Prompt 3 owns integration tests and the changelog.
- **Every new exported type and method carries GoDoc.**
- **Error wrapping** goes through `github.com/bborbe/errors` — never `fmt.Errorf`, never `context.Background()` inside `pkg/ops`.
- **Do not hand-bump version manifests or tag.** The releaser owns the tag post-merge.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `make test` — must exit 0. Re-run it iteratively while implementing.

Focused runs (confirm the names appear; a zero-match focus exits 0 while running nothing):

```
go test -mod=mod ./pkg/ops -run TestSuite -ginkgo.focus='GoalCompleteOperation' -ginkgo.v
go test -mod=mod ./pkg/cli -run TestSuite -ginkgo.focus='printParents' -ginkgo.v
```

The `-ginkgo.v` output must contain every `It` name from requirement 5.1 and 5.2.

Evidence greps:

```
grep -rn 'NewGoalCompleteOperation' pkg/ integration/          # cli.go, goal_complete.go, goal_complete_test.go only
grep -c 'cascadeTopics' pkg/ops/goal_complete.go               # >= 4: doc comment, definition, three call sites
grep -c 'func (f GoalFrontmatter) Topics' pkg/domain/goal_frontmatter.go   # 1
grep -c 'func (p Page) Topics' pkg/domain/page.go                          # 1
grep -n '^### Topics' docs/goal-writing.md                                 # 1, between blocked_by and Session
```

Finally run `make precommit` once — must exit 0.

If `make precommit` exits non-zero: fix, re-run only the failing target (`make lint`, `make test`, …) until it passes, then run `make precommit` once more. A non-zero `make precommit` exit code must be reported as `"status":"failed"` in the completion report — never rationalised as success.
</verification>
