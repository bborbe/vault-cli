---
status: completed
spec: [070-parent-closure-cascade-on-complete]
summary: 'Added the parent-cascade verification surface: JSON-envelope and single-write-path unit tests, 14 end-to-end integration cases for task→goal and goal→topic cascades including refusal, ordering, wikilink, recurring, and JSON paths, flattened nested parent reports, and the Unreleased changelog entry'
execution_id: vault-cli-parent-cascade-exec-288-spec-070-parent-cascade-tests-changelog
dark-factory-version: v0.198.17
created: "2026-11-19T20:10:00Z"
queued: "2026-11-19T20:08:41Z"
started: "2026-11-19T21:48:36Z"
completed: "2026-11-19T21:55:24Z"
branch: dark-factory/parent-closure-cascade-on-complete
---

# Cover the parent cascade end to end and record the change

<summary>
- Integration tests drive the real binary through task → goal and goal → topic cascades, including the refusal paths.
- The `parents` JSON envelope is pinned by tests: array shape, `result` values, and `open_items` omitted on a completed entry.
- A task with no `goals:` and a goal with no `topics:` produce no `parent` line and no `parents` key.
- A recurring task is proven never to cascade.
- A parent refusal is proven never to write the parent page.
- The single-close-path invariant is asserted both structurally (no goal/topic write in the child operation files) and behaviourally (mock call counts).
- The change is recorded under `## Unreleased` in the changelog.
</summary>

<objective>
Close spec 070's verification surface: integration coverage for every acceptance criterion that depends on the real binary, unit coverage for the JSON envelope and the single-write-path invariant, and the changelog entry. Prompts 1 and 2 deliver the production behaviour; this prompt proves it and records it.
</objective>

<context>
Read these files fully before adding anything:

- `specs/in-progress/070-parent-closure-cascade-on-complete.md` — § Acceptance Criteria is the checklist this prompt implements; § Failure Modes names the behaviours the tests must exercise.
- `pkg/ops/parent_cascade.go`, `pkg/ops/complete.go`, `pkg/ops/goal_complete.go` — delivered by prompts 1 and 2. `ops.ParentAttempt`, `ops.ParentResultCompleted`, `ops.ParentResultRefused` are exported.
- `pkg/ops/complete_test.go` / `pkg/ops/goal_complete_test.go` — both already inject `mocks.GoalCompleteOperation` / `mocks.TopicCompleteOperation`; this prompt adds new files rather than rewriting them.
- `pkg/ops/ops_suite_test.go` — `ErrTest`; Ginkgo v2, external `ops_test` package.
- `integration/cli_test.go` — helpers at the top of the file:
  - `createTempVaultWithGoals(tasks map[string]string, goals map[string]string) (vaultPath, configPath string, cleanup func())` — line 47.
  - `createTempVaultWithTopicPages(topicsDir string, topics map[string]string, goals map[string]string) (vaultPath, configPath string, cleanup func())` — line 188. It creates an **empty** `Tasks/` directory; write task fixtures into `filepath.Join(vaultPath, "Tasks", name+".md")` yourself.
  - The existing `Describe("vault-cli goal and topic closure gate", ...)` block at line 2295 shows the local-closure style (`runCommand`, `readFile`) this file uses. Do not add package-level helpers; copy the local-closure pattern per `It`, as the surrounding code does.
- `CHANGELOG.md` — the preamble ends at the `* PATCH version …` line; the top versioned section is `## v0.198.2`.
- `docs/goal-writing.md` — prompt 2 added `### Topics (\`topics\`)`; do not edit it here.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega, external `_test` packages, `gexec`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen 80 lines / 50 statements, nestif 4, dupl.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — prefix, one bullet per logical change, `## Unreleased` placement.
</context>

<requirements>
## 1. New file `pkg/ops/mutation_result_parents_test.go` (package `ops_test`)

One `Describe("MutationResult parents JSON", ...)`, marshalling with `encoding/json` and unmarshalling into `map[string]any`:

- `It("omits the parents key when no attempt ran")` — `ops.MutationResult{Success: true, Name: "t"}`; assert the decoded map has no `"parents"` key.
- `It("omits open_items on a completed entry")` — `Parents: []ops.ParentAttempt{{Name: "Goal A", Result: ops.ParentResultCompleted}}`; assert `parents` is a `[]any` of length 1, the entry's `"name"` is `Goal A`, its `"result"` is `completed`, and the entry has no `"open_items"` key.
- `It("carries open_items on a refused entry")` — `Parents: []ops.ParentAttempt{{Name: "Goal A", Result: ops.ParentResultRefused, OpenItems: []string{"box one"}}}`; assert the entry's `"result"` is `refused` and `"open_items"` is `[]any{"box one"}`.
- `It("keeps both entries in document order")` — one completed and one refused entry; assert `parents[0].name` and `parents[1].name` in the constructed order.

## 2. New file `pkg/ops/parent_cascade_single_path_test.go` (package `ops_test`)

One `Describe("parent cascade single close path", ...)`. Build the **real** operations over mocks so the assertions are behavioural, not structural:

```go
mockTopicStorage := &mocks.TopicStorage{}
mockGoalStorage := &mocks.GoalStorage{}
mockTaskStorage := &mocks.TaskStorage{}
mockDailyNoteStorage := &mocks.DailyNoteStorage{}
mockInteractionCounter := &mocks.InteractionCounter{}
currentDateTime := libtime.NewCurrentDateTime()
currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))

topicOp := ops.NewTopicCompleteOperation(mockTopicStorage, mockGoalStorage)
goalOp := ops.NewGoalCompleteOperation(mockGoalStorage, mockTaskStorage, currentDateTime, topicOp)
completeOp := ops.NewCompleteOperation(
	mockTaskStorage,
	mockDailyNoteStorage,
	currentDateTime,
	mockInteractionCounter,
	goalOp,
)
```

Cases:

- `It("does not write the goal when the parent goal refuses")` —
  task: `domain.NewTask(map[string]any{"status": "todo", "goals": []string{"my-goal"}}, domain.FileMetadata{Name: "my-task"}, domain.Content(""))`; `mockTaskStorage.FindTaskByNameReturns(task, nil)`, `mockTaskStorage.WriteTaskReturns(nil)`, `mockDailyNoteStorage.ReadDailyNoteReturns("", nil)`.
  goal: `domain.NewGoal(map[string]any{"status": "in_progress"}, domain.FileMetadata{Name: "my-goal"}, domain.Content("---\nstatus: in_progress\n---\n# Success Criteria\n- [ ] an open criterion\n# Definition of Done\n- [x] done\n"))`; `mockGoalStorage.FindGoalByNameReturns(goal, nil)`.
  Run `completeOp.Execute(ctx, "/vault", "my-task", "test-vault", false, "", "")`.
  Assert: `err` is nil, `result.Success` is true, `result.Parents[0].Result == ops.ParentResultRefused`, `result.Parents[0].OpenItems` contains `an open criterion`, `mockGoalStorage.WriteGoalCallCount() == 0`, `mockTaskStorage.WriteTaskCallCount() == 1`.
- `It("does not write the topic when the parent topic refuses")` —
  goal: `domain.NewGoal(map[string]any{"status": "in_progress", "topics": []string{"my-topic"}}, domain.FileMetadata{Name: "my-goal"}, domain.Content("---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n# Definition of Done\n- [x] done\n"))`; `mockGoalStorage.FindGoalByNameReturns(goal, nil)`, `mockGoalStorage.WriteGoalReturns(nil)`, `mockTaskStorage.ListTasksReturns(nil, nil)`.
  topic: `domain.NewTopic(map[string]any{"status": "in_progress"}, domain.FileMetadata{Name: "my-topic"}, domain.Content("---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n# Completion Gate\n- [ ] an open gate item\n"))`; `mockTopicStorage.FindTopicByNameReturns(topic, nil)`.
  Run `goalOp.Execute(ctx, "/vault", "my-goal", "test-vault", false, "", "")`.
  Assert: `err` is nil, `result.Success` is true, `result.Parents[0].Result == ops.ParentResultRefused`, `mockTopicStorage.WriteTopicCallCount() == 0`, `mockGoalStorage.WriteGoalCallCount() == 1`.
- `It("writes the parent goal exactly once on a successful close")` — same task fixture, goal body fully checked; assert `mockGoalStorage.WriteGoalCallCount() == 1`, `result.Parents[0].Result == ops.ParentResultCompleted`, and `_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)` yields a goal whose `Status()` is `domain.GoalStatusCompleted`.
- `It("reports a missing parent goal as refused and still completes the task")` — `mockGoalStorage.FindGoalByNameReturns(nil, ErrTest)`; assert `err` is nil, `result.Success` is true, `result.Parents[0].Result == ops.ParentResultRefused`, `result.Parents[0].OpenItems[0]` contains `test error`, `mockGoalStorage.WriteGoalCallCount() == 0`, `mockTaskStorage.WriteTaskCallCount() == 1`. This is the Failure Modes row 1 path; row 2 (`closureOutcomeUnmeasurable`) takes the same `!Success` branch, so one case covers both.

Imports for this file: `context`, `libtime`, `libtimetest`, `. "github.com/onsi/ginkgo/v2"`, `. "github.com/onsi/gomega"`, `"github.com/bborbe/vault-cli/mocks"`, `"github.com/bborbe/vault-cli/pkg/domain"`, `"github.com/bborbe/vault-cli/pkg/ops"`.

## 3. Integration tests: new `Describe` block in `integration/cli_test.go`

Add `Describe("vault-cli parent closure cascade", ...)` immediately after the existing `Describe("vault-cli goal and topic closure gate", ...)` block. Each `It` gets its own temp vault and its own local `runCommand(args ...string) *gexec.Session` closure, matching the surrounding style. Use `gexec.Exit(0)` / `gexec.Exit(1)` and read files with `os.ReadFile` plus `//#nosec G304 -- test file`.

Fixture bodies to reuse (declare them as local string constants inside the block):

- task linked to a goal, no checkboxes (so the task close does not block):
  `"---\nstatus: todo\ngoals:\n    - '[[my-goal]]'\n---\n# Linked\n"`
- closure-complete goal:
  `"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n# Definition of Done\n- [x] done\n"`
- goal with a topic parent: prepend `topics:\n    - '[[my-topic]]'\n` to the goal frontmatter.
- closure-complete topic:
  `"---\nstatus: in_progress\n---\n# My Topic\n## Goals\n- [[my-goal]]\n# Success Criteria\n- [x] done\n# Completion Gate\n- [x] done\n"`

Cases (each `It` name must appear in `go test ./integration -run TestIntegration -ginkgo.v -v` output):

1. `It("task complete cascades a fully checked parent goal")` — `createTempVaultWithGoals` with the linked task and the complete goal. Run `task complete my-task`; exit 0; stdout contains `parent my-goal: completed`; the goal file reads `status: completed`.
2. `It("goal complete cascades a fully checked parent topic")` — `createTempVaultWithTopicPages("Topics", topics, goals)` with the complete goal carrying `topics: ['[[my-topic]]']` and the complete topic. Run `goal complete my-goal`; exit 0; stdout contains `parent my-topic: completed`; the topic file reads `status: completed`.
3. `It("task complete reports a refused goal and still exits 0")` — goal with `- [ ] an open criterion` in `# Success Criteria`. Run `task complete my-task`; exit 0; the task file reads `status: completed`; the goal file still reads `status: in_progress`; stdout contains `parent my-goal: refused — open: an open criterion`.
4. `It("goal complete reports a refused topic and still exits 0")` — complete goal with a topic parent whose `# Completion Gate` holds `- [ ] an open gate item`. Run `goal complete my-goal`; exit 0; the goal reads `status: completed`; the topic still reads `status: in_progress`; stdout contains `parent my-topic: refused — open: an open gate item`.
5. `It("task complete attempts two goals in document order")` — task `goals:` lists `'[[goal-one]]'` then `'[[goal-two]]'`; `goal-one` complete, `goal-two` with one open SC box. Run `task complete my-task`; exit 0; stdout has exactly two lines matching `^parent ` (count lines with `regexp.MustCompile(`(?m)^parent `).FindAllString`); the index of `parent goal-one: completed` is lower than the index of `parent goal-two: refused`.
6. `It("task complete cascades the topics of an already-completed goal")` — build the vault with `createTempVaultWithTopicPages("Topics", topics, goals)` (the `topics_dir` default is `23 Topics`, so `createTempVaultWithGoals` would not resolve a topic under `Topics/`); goal fixture `status: completed` carrying `topics: ['[[my-topic]]']`, complete topic `status: in_progress`, then write the linked task into `filepath.Join(vaultPath, "Tasks", "my-task.md")`. Plain run: exit 0, stdout contains `parent my-goal: completed` and `parent my-topic: completed`. JSON run: exit 0; unmarshal stdout into `map[string]any`; `parents` is a `[]any` of length 2 (requirement 6's flattening is what makes the nested topic attempt appear); the entry named `my-goal` has `result == "completed"` and no `open_items` key; the entry named `my-topic` has `result == "completed"`; the topic file reads `status: completed`.
7. `It("task complete with no goals prints no parent line and omits parents in JSON")` — task with no `goals:` key. Plain run: exit 0, stdout does not match `(?m)^parent `. JSON run: exit 0, decoded map has no `parents` key.
8. `It("goal complete with no topics prints no parent line and omits parents in JSON")` — complete goal with no `topics:` key. Plain run: exit 0, no `^parent ` line. JSON run: decoded map has no `parents` key.
9. `It("task complete closes task, goal and topic in one run")` — build the vault with `createTempVaultWithTopicPages("Topics", topics, goals)` (topic holds the goal, goal carries `topics: ['[[my-topic]]']`), then write the linked task into `filepath.Join(vaultPath, "Tasks", "my-task.md")`. Run `task complete my-task`; exit 0; task, goal and topic files all read `status: completed`.
10. `It("task complete reports the parents array shape in JSON")` — task with one complete and one refused goal. Run `task complete my-task --output json`; exit 0; `parents` is a `[]any` of length 2; the completed entry has `name` and `result` and no `open_items` key; the refused entry's `open_items` is a `[]any` whose first element is exactly `an open criterion`.
11. `It("goal complete reports the parents array shape in JSON")` — build the vault with `createTempVaultWithTopicPages("Topics", topics, goals)`; complete goal with one complete topic and one refused topic (two entries in `topics:`). Run `goal complete my-goal --output json`; exit 0; `parents` has length 2; one `completed` with no `open_items`, one `refused` with `open_items[0] == "an open gate item"`.
12. `It("task complete resolves a labelled wikilink goal")` — task `goals:` is `'[[my-goal|label]]'`. Run `task complete my-task`; exit 0; stdout contains `parent my-goal: completed`; goal reads `status: completed`.
13. `It("task complete resolves a heading wikilink goal")` — task `goals:` is `'[[my-goal#Heading]]'`; same assertions as case 12.
14. `It("task complete on a recurring task does not cascade")` — task fixture `"---\nstatus: todo\nrecurring: 1d\ngoals:\n    - '[[my-goal]]'\n---\n# Recurring\n"`, goal fully checked `in_progress`. Run `task complete my-task`; exit 0; the task file does not read `status: completed`; the goal still reads `status: in_progress`; stdout does not match `(?m)^parent `.

Do not relax, rename or delete any existing integration case. If a helper is needed to decode JSON, write a local closure inside the `It`.

## 4. CHANGELOG

`CHANGELOG.md` has no `## Unreleased` section. Create it below the preamble (after the `* PATCH version …` line) and above `## v0.198.2`, with one bullet:

```
## Unreleased

- feat: `task complete` and `goal complete` cascade one level up — a completed task attempts each goal in its `goals:`, a completed goal attempts each topic in its `topics:`, every attempt is reported on stdout as `parent <name>: completed|refused` and as a `parents` array in `--output json`, and a refusing parent never fails the child or changes its exit code
```

Do not bump any version manifest and do not tag.

## 5. Do not change production code

This prompt adds tests and one changelog section only. If a test exposes a real defect in prompts 1-2, fix the production code minimally and note the fix in the completion report; do not restructure the cascade — requirement 6's two-line flattening is the only production change this prompt may make.

## 6. Flatten nested parent reports (minimal production fix)

`cascadeParents` in `pkg/ops/parent_cascade.go` must append each attempt's own `Parents` after the attempt entry, so one `task complete` run reports the goal attempt and the goal's topic attempt in the same list. Replace the loop body with:

```go
		result, err := attempt(ctx, name)
		if result.Success {
			attempts = append(attempts, ParentAttempt{Name: name, Result: ParentResultCompleted})
			attempts = append(attempts, result.Parents...)
			continue
		}
		attempts = append(attempts, ParentAttempt{
			Name:      name,
			Result:    ParentResultRefused,
			OpenItems: refusalItems(result, err),
		})
		attempts = append(attempts, result.Parents...)
```

Without this, a task's `parents` array and stdout carry only the goal entry, and the spec's already-terminal AC ("stdout carries `parent <topic-name>: completed`" in the same run) cannot be met. This is the genuine defect requirement 5's escape hatch permits — do not weaken case 6's length-2 assertion instead.
</requirements>

<constraints>
- **Every acceptance criterion in the spec must map to at least one test here.** The Post-Deploy ACs are the operator's rung; the integration tests are their container-executable equivalent.
- **Do not delete or relax existing tests.** The existing `Describe("vault-cli goal and topic closure gate", ...)` cases and the task-close cases must keep passing unchanged.
- **Tests must assert state transitions and stdout text, not just exit codes.** A run that exits 0 with the parent still open is a failure of the feature, not of the test.
- **The JSON assertions are structural** — decode with `encoding/json` into `map[string]any`; do not depend on `jq` being installed.
- **A refused entry always carries at least one open item**, and a completed entry never carries the `open_items` key. Assert both.
- **`pkg/ops` tests stay in the external `ops_test` package**; integration tests stay in the existing suite.
- **Do not touch** `docs/`, `agents/`, or any prompt-1/prompt-2 source file beyond a genuine defect fix.
- **Do not hand-bump version manifests or tag.** The releaser owns the tag post-merge.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `make test` — must exit 0. Re-run it iteratively while implementing.

Focused runs (confirm the names appear; a zero-match focus exits 0 while running nothing):

```
go test -mod=mod ./pkg/ops -run TestSuite -ginkgo.focus='MutationResult parents JSON|parent cascade single close path' -ginkgo.v
go test -mod=mod ./integration -run TestIntegration -ginkgo.focus='parent closure cascade' -ginkgo.v
```

The `-ginkgo.v` output must contain every `It` name from requirements 1, 2 and 3. Also run the full integration suite once, because the new block shares helpers and fixtures with the rest of the file:

```
go test -mod=mod ./integration -run TestIntegration -v
```

Evidence greps (each count is exact):

```
grep -c 'func checkClosureGate' pkg/ops/closure_gate.go        # 1
grep -c 'WriteGoal' pkg/ops/complete.go                        # 0
grep -c 'WriteTopic' pkg/ops/goal_complete.go                  # 0
grep -n '^## Unreleased' CHANGELOG.md                          # 1 line, above v0.198.2
```

Finally run `make precommit` once — must exit 0.

If `make precommit` exits non-zero: fix, re-run only the failing target (`make lint`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. A non-zero `make precommit` exit code must be reported as `"status":"failed"` in the completion report — never rationalised as success.
</verification>

<!--
Open questions surfaced for the reviewer (resolved in the prompts, not asked):

1. `Topics()` placement. The spec says the accessor "mirrors the existing `Goals()`
   accessor on `Page`". `domain.Goal` embeds `GoalFrontmatter`, not `Page`, so the
   cascade consumes `GoalFrontmatter.Topics()`. Prompt 2 adds both that accessor and
   the mirrored `Page.Topics()` so the spec's wording is satisfied literally.
2. Whether a refused goal should still attempt its topics. The spec only states that
   an already-terminal parent still cascades. Prompt 2 cascades only from terminal
   goals: a topic gate requires its member goals terminal, so an attempt under an
   open goal could only refuse and would add noise to stdout.
3. Cascade-read failure. The spec's Failure Modes list "cascade read of `goals:`/
   `topics:` fails → warning recorded". Both reads are `GetStringSlice` on the
   in-memory page and cannot fail; an unreadable parent page surfaces through the
   parent operation as a `refused` entry with the read error as its open item, which
   is the same observable outcome. No separate warning path is added.
-->
