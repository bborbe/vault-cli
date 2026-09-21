---
status: completed
spec: [051-topic-command-ladder]
execution_id: vault-cli-topic-ladder-exec-223-spec-051-topic-ops-and-command-group
dark-factory-version: v0.196.0
created: "2026-09-20T20:29:34Z"
queued: "2026-09-20T21:28:57Z"
started: "2026-09-20T21:40:10Z"
completed: "2026-09-20T21:53:41Z"
branch: dark-factory/topic-command-ladder
---

# The `topic` command family: twelve leaves, the ops layer behind them, and root registration

<summary>
- A vault-cli user can address topic pages through a `topic` command family at the root of the binary.
- The family has exactly twelve leaves — the same twelve the goal family has, member for member — and no others.
- None of the eleven goal slash-command names appears anywhere in the topic help output.
- Listing shows the topic pages in the vault's configured topics directory; showing one emits its full detail including its ordered frontmatter fields and content.
- Showing a topic in JSON reports an already-present phase value at the same place and in the same shape the goal command reports one, reports no phase key at all when the page carries none, and reads an absent field as an empty line with a successful exit.
- Setting, getting, clearing, adding to and removing from a topic page round-trip real frontmatter on disk.
- Completing and deferring a topic produce observable state transitions, including a refusal for a second completion and a refusal for a past date that leaves the file untouched.
- Linting reports a seeded defect and exits non-zero for it; searching is scoped to the topics directory.
- Working on a topic moves it into its in-progress state and reports a session outcome rather than a usage error.
- The goal command family, the task command family and every existing test behave exactly as they do today.
</summary>

<objective>
Build the ops layer for topics and register the `topic` command family at the binary's root with exactly the twelve leaves the goal family has, each performing the real operation its name promises, so a topic page can be listed, inspected and mutated through supported commands instead of hand-edited markdown. This is spec 051's prompt 3 of 4: it covers Desired Behaviors 4, 5 and 6 and Acceptance Criteria 1, 2, 3, 6, 7, 8, 9 and 10. It depends on prompt 1's `domain.Topic` entity and prompt 2's `storage.TopicStorage` — it will not compile without them.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then these files:

- `pkg/cli/cli.go` — the file you edit most. Read `NewRootCommand` (the `rootCmd.AddCommand(...)` block, in particular the `createGoalCommands` line and its neighbours), `createGoalCommands` (the twelve-leaf reference), `createGoalCompleteCommand`, `createGoalDeferCommand`, `createWorkOnGoalCommand`, and the generic command builders they call: `createGenericListCommand`, `createGenericLintCommand`, `createGenericSearchCommand`, `createEntityGetCommand`, `createEntitySetCommand`, `createEntityClearCommand`, `createEntityShowCommand`, `createEntityListAddCommand`, `createEntityListRemoveCommand`. Also read `getVaults`, `runMutation`, `resolveSessionMode`, `formatWorkOnResult` and `PrintJSON`'s callers. This file is ~2500 lines — read the goal-family region and the generic builders, not the whole file.
- `pkg/ops/frontmatter_entity.go` — the generic entity ops. Read `FrontmatterEntity` (the interface `*domain.Topic` must satisfy), `entityGetOperation`, `goalSetOperation`, `goalClearOperation`, `goalTagsListOperation`, `themeTagsListOperation`, `EntityShowResult`, and `entityShowOperation.Execute`'s `switch e := entity.(type)` block. This is where the six generic topic ops and the show type-switch case go.
- `pkg/ops/goal_complete.go` — `GoalCompleteOperation`, its interface, its constructor, its `Execute` shape, and `MutationResult` (declared in `pkg/ops/complete.go`).
- `pkg/ops/goal_defer.go` — `GoalDeferOperation` and its constructor taking `libtime.CurrentDateTime`.
- `pkg/ops/goal_workon.go` — `GoalWorkOnOperation`, `applyGoalAssigneeMatrix`, `persistGoalSessionID`, `handleClaudeSession`. This is the file you mirror most closely.
- `pkg/ops/defer_date_parser.go` — `parseDeferDate(ctx, dateStr, now)` and `isDeferDateInPast(targetDate, now)`. Package-scoped; call them unqualified.
- `pkg/ops/errors.go` — `ErrStarterUnavailable` (soft failure: keep as a warning, CLI exits 0) and `ErrSessionBusy`.
- `pkg/ops/claude_session.go` — `NewClaudeSessionStarter(claudeScript, locker)` **returns nil when the claude script is not on `PATH`**. That nil starter is the `ErrStarterUnavailable` path.
- `pkg/ops/vault_dispatcher.go` — `FirstSuccess`: single vault calls the callback directly; multiple vaults continue only on `errors.Is(err, storage.ErrNotFound)`.
- `pkg/ops/frontmatter_entity_test.go`, `pkg/ops/goal_complete_test.go`, `pkg/ops/goal_defer_test.go`, `pkg/ops/goal_workon_test.go` — the test shapes you copy. Note `libtimetest.ParseDateTime` + `currentDateTime.SetNow(...)` for a pinned clock, `mocks.GoalStorage`, `mocks.ClaudeSessionStarter`, `mocks.ClaudeResumer`, and `pinnedSessionID`.
- `pkg/storage/storage.go` — `TopicStorage` and `NewTopicStorage`, added by prompt 2.
- `pkg/domain/topic.go` and `pkg/domain/topic_frontmatter.go` — `Topic`, `NewTopic`, `TopicStatusInProgress`, `TopicStatusCompleted`, and the `GetField` / `SetField` / `ClearField` / `Tags` / `SetTags` / `DeferDate` / `SetDeferDate` accessors, added by prompt 1.
- `integration/cli_test.go` — the real-binary harness. Read `createTempVault`, `createTempVaultWithGoals`, `createTempVaultWithTopics` (added under spec 048), the `Describe("command registration", ...)` `DescribeTable` and its `Entry(...)` list, and two or three behavioural blocks (e.g. the `config list --output json topics_dir` block) for the `gexec.Start` + `Eventually(session).Should(gexec.Exit(N))` style.
- `integration/integration_suite_test.go` — `binPath` is built once in `BeforeSuite` with `gexec.Build("github.com/bborbe/vault-cli")`; the specs run the real binary as a subprocess.
- `docs/development-patterns.md` § "Adding a New Command" step 3 and 4, § "Multi-Vault Pattern", § "Output Format" and § "Naming" — the ops-layer shape, `getVaults` vs `getWatchVaults`, "Never import `encoding/json` in command files — use the `PrintJSON` helper", and the naming rules `create<Noun>Commands()` / `create<Noun><Verb>Command()`.
- `docs/dod.md` — this repo's `validationPrompt`. Its Testing section requires: "New CLI commands/subcommands have an entry in the integration test command registration table (`integration/cli_test.go`)".
- `specs/in-progress/051-topic-command-ladder.md` — the spec. Read its Goal, Non-goals, all fifteen Acceptance Criteria, Constraints, the whole Failure Modes table and the Security section. Every requirement below comes from them.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — Interface → Constructor → Struct → Method.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-factory-pattern.md` — `Create*` naming, zero-logic factories.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors`; never `fmt.Errorf`; never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter directives; `make generate` wipes and regenerates `mocks/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits: `funlen` 80 lines / 50 statements, `nestif` 4, `gocognit` 20, `golines` 100 columns.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-cli-guide.md` — cobra conventions.

**The twelve leaves, frozen, in the order the goal family registers them.** `createGoalCommands` registers exactly these twelve `AddCommand` calls, and the topic group mirrors them one for one:

| # | leaf | goal builder | topic builder |
|---|---|---|---|
| 1 | `list` | `createGenericListCommand(…, "goals", GoalsDir, …)` | `createGenericListCommand(…, "topics", TopicsDir, …)` |
| 2 | `lint` | `createGenericLintCommand(…, "goal", GoalsDir, GoalsDir, …)` | `createGenericLintCommand(…, "topic", TopicsDir, TopicsDir, …)` |
| 3 | `search` | `createGenericSearchCommand(…, "goals", GoalsDir, …)` | `createGenericSearchCommand(…, "topics", TopicsDir, …)` |
| 4 | `get` | `createEntityGetCommand(…, "goal", NewGoalGetOperation)` | `createEntityGetCommand(…, "topic", ops.NewTopicGetOperation)` |
| 5 | `set` | `createEntitySetCommand(…, "goal", NewGoalSetOperation)` | `createEntitySetCommand(…, "topic", ops.NewTopicSetOperation)` |
| 6 | `clear` | `createEntityClearCommand(…, "goal", NewGoalClearOperation)` | `createEntityClearCommand(…, "topic", ops.NewTopicClearOperation)` |
| 7 | `show` | `createEntityShowCommand(…, "goal", NewGoalShowOperation)` | `createEntityShowCommand(…, "topic", ops.NewTopicShowOperation)` |
| 8 | `add` | `createEntityListAddCommand(…, "goal", NewGoalListAddOperation)` | `createEntityListAddCommand(…, "topic", ops.NewTopicListAddOperation)` |
| 9 | `remove` | `createEntityListRemoveCommand(…, "goal", NewGoalListRemoveOperation)` | `createEntityListRemoveCommand(…, "topic", ops.NewTopicListRemoveOperation)` |
| 10 | `complete` | `createGoalCompleteCommand` | `createTopicCompleteCommand` |
| 11 | `defer` | `createGoalDeferCommand` | `createTopicDeferCommand` |
| 12 | `work-on` | `createWorkOnGoalCommand` | `createWorkOnTopicCommand` |

**Environment facts that shape this prompt:**

1. **Make no git calls, anywhere — every check in `<verification>` is git-free.** The daemon does not check `<verification>` exit codes, so a git command that dies (`fatal: not a git repository`) reports a false pass.
2. **`.dark-factory.yaml` sets `GOFLAGS=-buildvcs=false`.** Keep it — `gexec.Build` in the integration suite runs `go build`, which otherwise tries to read a masked `.git` and fails with a VCS status error.
3. **`semantic-search-mcp` is not guaranteed on `PATH` in this container.** `ops.NewSearchOperation().Execute` returns `errors.Wrap(ctx, err, "semantic-search-mcp not found on PATH")` when the binary is missing. Do NOT write an integration spec that requires a real semantic-search result — see § 6.
4. **`claude` is not guaranteed on `PATH` either, and must not be invoked.** The integration vault's config sets `claude_script` to a name that is deliberately not installed, so `NewClaudeSessionStarter` returns nil, the work-on path takes `ErrStarterUnavailable` as a soft warning, and the command still exits 0 with the status written. See § 6.
5. **`make generate` regenerates `mocks/` from scratch.** You add `//counterfeiter:generate` directives; you do not hand-write fakes.
</context>

<requirements>

## 0. Scope — ten files, plus three generated mocks

- `pkg/ops/frontmatter_entity.go` — edited: six topic constructors, the show type-switch case, one list-operation type.
- `pkg/ops/topic_complete.go` — NEW.
- `pkg/ops/topic_defer.go` — NEW.
- `pkg/ops/topic_workon.go` — NEW.
- `pkg/ops/frontmatter_entity_test.go` — extended with the six generic topic ops' specs.
- `pkg/ops/topic_complete_test.go` — NEW.
- `pkg/ops/topic_defer_test.go` — NEW.
- `pkg/ops/topic_workon_test.go` — NEW.
- `pkg/cli/cli.go` — edited: four command builders and one root registration line.
- `integration/cli_test.go` — edited: one helper, twelve `Entry(...)` lines, one `Describe` block.
- `mocks/topic-complete-operation.go` — GENERATED, never hand-written: the `//counterfeiter:generate` directive in § 2 produces it.
- `mocks/topic-defer-operation.go` — GENERATED: the directive in § 3 produces it.
- `mocks/topic-workon-operation.go` — GENERATED: the directive in § 4a produces it.

`make generate` runs `rm -rf mocks` then `go generate ./...`, and `mocks/` is tracked in git (it is not gitignored), so these three land in the change alongside the ten above.

Nothing else. `pkg/ops/goal_workon.go`, `pkg/ops/goal_complete.go`, `pkg/ops/goal_defer.go`, `pkg/ops/goal_workon_test.go`, `pkg/ops/goal_complete_test.go`, `pkg/ops/goal_defer_test.go`, `pkg/ops/lint.go`, `pkg/ops/list.go`, `pkg/ops/search.go`, `pkg/ops/show.go`, `pkg/ops/watch.go`, `pkg/ops/resolve.go`, `pkg/storage/**`, `pkg/domain/**`, `pkg/config/**`, `main.go`, `go.mod`, `go.sum`, `CHANGELOG.md`, `README.md`, `docs/**` and `scenarios/**` are all untouched. No new dependency. No new scenario file — the spec's `## Scenario coverage` section records that decision; `ls scenarios/*.md | wc -l` must still print `5`.

## 1. `pkg/ops/frontmatter_entity.go` — the six generic topic ops and the show case

### 1a. `NewTopicGetOperation`

Add immediately after `NewVisionGetOperation`:

```go
// NewTopicGetOperation creates an EntityGetOperation for topics.
func NewTopicGetOperation(topicStorage storage.TopicStorage) EntityGetOperation {
	return &entityGetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (FrontmatterEntity, error) {
			return topicStorage.FindTopicByName(ctx, vaultPath, name)
		},
		entityType: "topic",
	}
}
```

- Frozen signature. `entityType` is `"topic"` — it appears in the error message `find topic` and in the CLI's `Short` text.
- The `findFn` returns `(*domain.Topic, error)` from a function whose declared return type is `(FrontmatterEntity, error)`. That compiles only because `*domain.Topic` satisfies `FrontmatterEntity`; if it does not, prompt 1 is incomplete — stop and report `"status":"failed"` naming prompt 1 rather than adding methods to make it fit.

### 1b. `NewTopicSetOperation`

Mirror `NewGoalSetOperation`, but **without** the goal-specific machinery. `goalSetOperation.Execute` runs `blockedBySetRefusal`, `writeGoalCloseOutFieldsIfCloseOut`, and a `strings.Contains(err.Error(), "missing close-out field(s)")` hint rewrite. None of those applies to a topic: there is no `blocked_by` list-field contract for topics, no close-out status, and no `--reason` / `--gate-successor` flag on `topic set`.

```go
type topicSetOperation struct {
	topicStorage storage.TopicStorage
}

// Execute sets the value of a frontmatter field on the named topic.
func (o *topicSetOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key, value, reason, gateSuccessor string,
) error {
	topic, err := o.topicStorage.FindTopicByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find topic")
	}
	if err := topic.SetField(ctx, key, value); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("set field %q", key))
	}
	if err := o.topicStorage.WriteTopic(ctx, topic); err != nil {
		return errors.Wrap(ctx, err, "write topic")
	}
	return nil
}

// NewTopicSetOperation creates an EntitySetOperation for topics.
func NewTopicSetOperation(topicStorage storage.TopicStorage) EntitySetOperation {
	return &topicSetOperation{topicStorage: topicStorage}
}
```

- The `Execute` signature is frozen by the `EntitySetOperation` interface: it takes `reason` and `gateSuccessor` even though the topic path ignores both. Do NOT change the interface. Do NOT invent a topic close-out. Both parameters are unused — `revive`'s `unused-parameter` rule is excluded repo-wide, so no `_` rename and no `//nolint` is needed; mirror the `themeSetOperation` / `visionSetOperation` shape, which has the same unused pair.
- Do NOT add `blockedBySetRefusal`, `writeGoalCloseOutFieldsIfCloseOut`, or any `aborted_reason` / `gate_successor` handling. The spec's Non-goals forbid reusing the goal close-out machinery for topics.
- The body is deliberately identical in shape to `themeSetOperation.Execute` — reference it.

### 1c. `NewTopicClearOperation`

Mirror `NewGoalClearOperation`:

```go
type topicClearOperation struct {
	topicStorage storage.TopicStorage
}

// Execute clears the value of a frontmatter field on the named topic.
func (o *topicClearOperation) Execute(ctx context.Context, vaultPath, entityName, key string) error {
	topic, err := o.topicStorage.FindTopicByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find topic")
	}
	topic.ClearField(key)
	if err := o.topicStorage.WriteTopic(ctx, topic); err != nil {
		return errors.Wrap(ctx, err, "write topic")
	}
	return nil
}

// NewTopicClearOperation creates an EntityClearOperation for topics.
func NewTopicClearOperation(topicStorage storage.TopicStorage) EntityClearOperation {
	return &topicClearOperation{topicStorage: topicStorage}
}
```

### 1d. `NewTopicShowOperation` and the type-switch case

Add `NewTopicShowOperation` immediately after `NewVisionShowOperation`:

```go
// NewTopicShowOperation creates an EntityShowOperation for topics.
func NewTopicShowOperation(topicStorage storage.TopicStorage) EntityShowOperation {
	return &entityShowOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return topicStorage.FindTopicByName(ctx, vaultPath, name)
		},
		entityType: "topic",
	}
}
```

Then add a case to `entityShowOperation.Execute`'s type switch, immediately after the `case *domain.Vision:` block and immediately before the `default:` block:

```go
	case *domain.Topic:
		nameVal = e.Name
		filePathVal = e.FilePath
		contentVal = string(e.Content)
		for _, k := range e.Keys() {
			fields[k] = e.GetField(k)
			fieldOrder = append(fieldOrder, k)
		}
```

This case is what makes Acceptance Criteria 4 and 5 hold at the CLI level:

- `fields` is a `map[string]string` keyed by every key present in the topic's frontmatter map, and `EntityShowResult.Fields` is tagged `json:"fields"`. So `topic show … --output json` emits `.fields.phase` with the raw on-disk phase string — the same key path and the same value shape (`string`) that `goal show … --output json` emits.
- A topic page with no `phase` line has no `phase` entry in the map, so `Keys()` does not include it and the JSON `fields` object has **no** `phase` member at all — not an empty one. Do NOT add a fallback that inserts `"phase": ""`; that would break Acceptance Criterion 5.
- Do NOT reorder or edit the four existing cases, do NOT change `EntityShowResult`, and do NOT add a `case *domain.Task:` — tasks have their own `ShowOperation` and are out of scope.

### 1e. The topic list-field operation

Add after `NewVisionListRemoveOperation` and before `NewTaskListAddOperation`:

```go
// knownTopicScalarFields are topic fields that hold a scalar (not a list).
var knownTopicScalarFields = map[string]bool{
	"status": true, "page_type": true, "phase": true,
	"assignee": true, "defer_date": true, "claude_session_id": true,
}

type topicTagsListOperation struct {
	topicStorage storage.TopicStorage
	mode         string
}

func (o *topicTagsListOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key, value string,
) error {
	topic, err := o.topicStorage.FindTopicByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find topic")
	}
	if knownTopicScalarFields[key] {
		return errors.Errorf(ctx, "not a list field: %q", key)
	}
	if key != "tags" {
		return errors.Errorf(ctx, "unknown field: %q", key)
	}
	current := topic.Tags()
	updated, err := applyListMutation(ctx, current, value, o.mode)
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("%s field %q", o.mode, key))
	}
	topic.SetTags(updated)
	if err := o.topicStorage.WriteTopic(ctx, topic); err != nil {
		return errors.Wrap(ctx, err, "write topic")
	}
	return nil
}

// NewTopicListAddOperation creates an EntityListAddOperation for topics.
func NewTopicListAddOperation(topicStorage storage.TopicStorage) EntityListAddOperation {
	return &topicTagsListOperation{topicStorage: topicStorage, mode: "add"}
}

// NewTopicListRemoveOperation creates an EntityListRemoveOperation for topics.
func NewTopicListRemoveOperation(topicStorage storage.TopicStorage) EntityListRemoveOperation {
	return &topicTagsListOperation{topicStorage: topicStorage, mode: "remove"}
}
```

- This mirrors `themeTagsListOperation`, `objectiveTagsListOperation` and `visionTagsListOperation` — three of the four existing list operations — rather than the goal/task variants.
- **The list-field allowlist is `tags` and only `tags`.** Do NOT add `blocked_by` and do NOT add `goals`. Both would drag in machinery this spec never mentions: `blocked_by` on a topic would need the `blockedByAppendRefusal` / `BlockedByIsScalar` / `BLOCKED_BY_SCALAR` lint contract, and `goals` is a task-specific field. Acceptance Criterion 6 says only "`topic add` appends one entry to a list field and `topic remove` drops it" — it does not name the field, and the theme/objective/vision precedent is a `tags`-only allowlist.
- `applyListMutation` already errors on a duplicate add and on a missing remove; reuse it, do not reimplement it.

## 2. `pkg/ops/topic_complete.go`

Standard BSD license header (copy from `pkg/ops/goal_complete.go`), `package ops`. Imports: `context`, `fmt`, `github.com/bborbe/errors`, `github.com/bborbe/vault-cli/pkg/domain`, `github.com/bborbe/vault-cli/pkg/storage`. No `libtime` — see below.

```go
//counterfeiter:generate -o ../../mocks/topic-complete-operation.go --fake-name TopicCompleteOperation . TopicCompleteOperation
type TopicCompleteOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		topicName string,
		vaultName string,
	) (MutationResult, error)
}

// NewTopicCompleteOperation creates a new topic complete operation.
func NewTopicCompleteOperation(topicStorage storage.TopicStorage) TopicCompleteOperation {
	return &topicCompleteOperation{topicStorage: topicStorage}
}

type topicCompleteOperation struct {
	topicStorage storage.TopicStorage
}

// Execute marks a topic as completed.
func (o *topicCompleteOperation) Execute(
	ctx context.Context,
	vaultPath string,
	topicName string,
	vaultName string,
) (MutationResult, error) {
	topic, err := o.topicStorage.FindTopicByName(ctx, vaultPath, topicName)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "find topic")
	}

	if topic.GetField("status") == domain.TopicStatusCompleted {
		msg := fmt.Sprintf("topic %q is already completed", topicName)
		return MutationResult{Success: false, Error: msg}, errors.Errorf(ctx, "%s", msg)
	}

	if err := topic.SetField(ctx, "status", domain.TopicStatusCompleted); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "set status")
	}

	if err := o.topicStorage.WriteTopic(ctx, topic); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "write topic")
	}

	return MutationResult{Success: true, Name: topic.Name, Vault: vaultName}, nil
}
```

Non-negotiable properties:

- **The constructor takes only the storage.** Do NOT take a `libtime.CurrentDateTime`. The goal's complete sets `completed: <date>` and therefore needs a clock; the topic's complete writes only `status`, per Acceptance Criterion 7, whose evidence is a status transition and nothing more. Do NOT write a `completed` key, a `completed_date` key, or any other date onto a topic page. If you believe the goal's `completed` date should be mirrored, do NOT add it — it is recorded as an open question in `<constraints>` for the human auditor.
- **No open-task gate.** The goal's `checkOpenTasks` blocks completion when open tasks link to the goal. Topics have no task-linkage concept anywhere in this spec, so there is no gate, no `--force`, no `--reason`, no `--gate-successor`, and no `taskStorage` parameter.
- The already-completed refusal mirrors the goal's message shape: `topic %q is already completed`, wrapped in `errors.Errorf(ctx, "%s", msg)` with a `MutationResult{Success: false, Error: msg}`. It is raised **before** any write, so the file is byte-identical afterwards — that is Acceptance Criterion 7's second half.
- The status value comes from `domain.TopicStatusCompleted`, never a bare literal. The comparison reads through `topic.GetField("status")`, which returns the raw on-disk string; a page with no `status` key reads as `""` and is not "already completed", so it transitions normally.
- Do NOT validate the prior status against any enum, and do NOT reject a non-canonical prior value. A topic page carrying `status: whatever` must still be completable.

## 3. `pkg/ops/topic_defer.go`

Standard BSD license header (copy from `pkg/ops/goal_defer.go`), `package ops`. Imports: `context`, `fmt`, `github.com/bborbe/errors`, `libtime "github.com/bborbe/time"`, `github.com/bborbe/vault-cli/pkg/storage`. No `domain` import is needed unless you reference a topic constant — you do not.

```go
//counterfeiter:generate -o ../../mocks/topic-defer-operation.go --fake-name TopicDeferOperation . TopicDeferOperation
type TopicDeferOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		topicName string,
		dateStr string,
		vaultName string,
	) (MutationResult, error)
}

// NewTopicDeferOperation creates a new topic defer operation.
func NewTopicDeferOperation(
	topicStorage storage.TopicStorage,
	currentDateTime libtime.CurrentDateTime,
) TopicDeferOperation {
	return &topicDeferOperation{
		topicStorage:    topicStorage,
		currentDateTime: currentDateTime,
	}
}
```

`Execute` is `goalDeferOperation.Execute` with `goalStorage` → `topicStorage`, `goalName` → `topicName`, `goal` → `topic`, `goal.SetDeferDate(targetDate.Ptr())` → `topic.SetDeferDate(targetDate.Ptr())`, and `g.goalStorage.WriteGoal(ctx, goal)` → `o.topicStorage.WriteTopic(ctx, topic)`. Everything else is copied verbatim, in this order:

1. `now := o.currentDateTime.Now().Time()`
2. `targetDate, err := parseDeferDate(ctx, dateStr, now)` — on error return `MutationResult{Success: false, Error: err.Error()}` and `errors.Wrap(ctx, err, "parse date")`
3. `if isDeferDateInPast(targetDate, now)` — return the exact message `cannot defer to past date: %s` formatted with `targetDate.Time().Format("2006-01-02")`, as both the `MutationResult.Error` and the wrapped `errors.Errorf`. The past-date check runs **before** the topic is read, so the file is never touched — that is Acceptance Criterion 7's "leaves the file byte-identical".
4. find the topic, wrapping a failure as `errors.Wrap(ctx, err, "find topic")`
5. `topic.SetDeferDate(targetDate.Ptr())`
6. `o.topicStorage.WriteTopic(ctx, topic)`, wrapping as `errors.Wrap(ctx, err, "write topic")`
7. return `MutationResult{Success: true, Name: topic.Name, Vault: vaultName, Message: targetDate.Time().Format("2006-01-02")}`

- Do NOT reimplement date parsing. `parseDeferDate` already handles `+Nd`, weekday names, `YYYY-MM-DD` and RFC3339, and the spec's Failure Modes row for a relative date or a non-UTC timezone is satisfied by reusing it unchanged.
- Do NOT add a daily-note update, a status change, or a `defer_date` validation of your own.
- `TopicFrontmatter.SetDeferDate` takes `*libtime.DateOrDateTime`; `libtime.DateOrDateTime` has a `Ptr()` method (the goal defer uses it). Do not build the pointer by hand.

## 4. `pkg/ops/topic_workon.go`

Standard BSD license header (copy from `pkg/ops/goal_workon.go`), `package ops`. Imports: `context`, `fmt`, `log/slog`, `github.com/bborbe/errors`, `github.com/bborbe/vault-cli/pkg/config`, `github.com/bborbe/vault-cli/pkg/domain`, `github.com/bborbe/vault-cli/pkg/storage`.

### 4a. The interface and constructor

```go
//counterfeiter:generate -o ../../mocks/topic-workon-operation.go --fake-name TopicWorkOnOperation . TopicWorkOnOperation
type TopicWorkOnOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		topicName string,
		assignee string,
		vaultName string,
		isInteractive bool,
		sessionDir string,
		vault *config.Vault,
	) (MutationResult, error)
}

// NewTopicWorkOnOperation creates a new topic work-on operation.
func NewTopicWorkOnOperation(
	topicStorage storage.TopicStorage,
	uuidGenerator func() string,
	starter ClaudeSessionStarter,
	resumer ClaudeResumer,
) TopicWorkOnOperation {
	return &topicWorkOnOperation{
		topicStorage:  topicStorage,
		uuidGenerator: uuidGenerator,
		starter:       starter,
		resumer:       resumer,
	}
}
```

Both frozen — they are `GoalWorkOnOperation`'s with `Goal` → `Topic` and `goalStorage` → `topicStorage`.

### 4b. `Execute`

`goalWorkOnOperation.Execute` with the goal-specific pieces replaced, in this order:

1. `var warnings []string`
2. `topic, err := o.topicStorage.FindTopicByName(ctx, vaultPath, topicName)` — on error return `MutationResult{Success: false, Error: err.Error()}` and `errors.Wrap(ctx, err, "find topic")`
3. `if err := topic.SetField(ctx, "status", domain.TopicStatusInProgress); err != nil` — return `MutationResult{Success: false, Error: err.Error()}` and `errors.Wrap(ctx, err, "set status")`. The goal calls `goal.SetStatus(domain.GoalStatusInProgress)`, which validates against the goal enum; the topic path deliberately does **not** validate, because there is no topic status enum (prompt 1 defines only the two plain string constants). The in-progress value comes from `domain.TopicStatusInProgress`, never a bare literal.
4. `if w, err := applyTopicAssigneeMatrix(ctx, topic, assignee); err != nil { return MutationResult{Success: false, Error: err.Error()}, err } else if w != "" { warnings = append(warnings, w) }`
5. `if err := o.topicStorage.WriteTopic(ctx, topic); err != nil` — return `MutationResult{Success: false, Error: err.Error()}` and `errors.Wrap(ctx, err, "write topic")`
6. `sessionID, sessionErr := o.handleClaudeSession(ctx, topic, vaultPath, sessionDir, vault, isInteractive)` — the `ErrStarterUnavailable` branch appends `fmt.Sprintf("claude session: %v", sessionErr)` to `warnings`, calls `slog.Warn("workon warning", "warning", warning)`, and continues; any other error returns `MutationResult{Success: false, Name: topic.Name, Vault: vaultName, Warnings: warnings, SessionID: sessionID, Error: sessionErr.Error()}` and `errors.Wrap(ctx, sessionErr, "start work-on session")`. **Copy both branches exactly**, including the `slog.Warn` and the deliberate absence of a second `slog` line on the hard-failure branch — the goal file carries a comment explaining that the error is printed by the caller and a second stack-carrying copy would bury the child's reason.
7. `if isInteractive && o.resumer != nil && sessionID != ""` → return `MutationResult{Success: true, Name: topic.Name, Vault: vaultName, Warnings: warnings, SessionID: sessionID}` and `o.resumer.ResumeSession(ctx, sessionID, sessionDir, "")`. Copy the goal's comment about the empty resumed-turn argument verbatim in substance: the goal work-on carries the same resumed-turn defect and fixing it is a separate spec, so passing `""` keeps argv byte-identical.
8. return `MutationResult{Success: true, Name: topic.Name, Vault: vaultName, Warnings: warnings, SessionID: sessionID}, nil`

Do NOT add a daily-note update and do NOT advance a phase. The goal work-on has neither (its doc comment says so explicitly); the topic work-on has neither for the same reason, and the spec's Non-goals forbid writing a phase onto a topic page.

### 4c. `applyTopicAssigneeMatrix`

The goal's `applyGoalAssigneeMatrix` is a pure function returning a warning string, because `GoalFrontmatter.SetAssignee` has no error return. `TopicFrontmatter` has no typed `SetAssignee` (prompt 1 deliberately omits it), so the write goes through `SetField`, which returns an error — the helper therefore returns `(string, error)`.

```go
// applyTopicAssigneeMatrix updates the topic's assignee per the blank/equal/different
// rule so `topic work-on` never silently overrides a teammate's assignment.
//
// Returns a warning string when the topic already belongs to a different non-blank
// user (and the assignee is left unchanged); returns "" for the blank and
// already-self-assigned cases.
func applyTopicAssigneeMatrix(ctx context.Context, topic *domain.Topic, assignee string) (string, error) {
	switch existing := topic.GetField("assignee"); existing {
	case "":
		if err := topic.SetField(ctx, "assignee", assignee); err != nil {
			return "", errors.Wrap(ctx, err, "set assignee")
		}
		return "", nil
	case assignee:
		return "", nil
	default:
		return fmt.Sprintf(
			"assignee not updated: topic owned by %s (current user: %s)",
			existing,
			assignee,
		), nil
	}
}
```

- The three-way rule is frozen: blank → assign, equal → no-op, different non-blank → leave it and warn. Do NOT collapse it to an unconditional assignment.
- The warning text mirrors the goal's with `goal` → `topic`.
- `ctx` is threaded through to `SetField`; do NOT call `context.Background()`.

### 4d. `persistTopicSessionID`

`persistGoalSessionID` with `topicStorage` / `topicName` / `topic`, and `refreshed.SetClaudeSessionID(sessionID)` → `refreshed.SetField(ctx, "claude_session_id", sessionID)`:

```go
func persistTopicSessionID(
	ctx context.Context,
	vaultPath string,
	topicName string,
	sessionID string,
	topicStorage storage.TopicStorage,
) (string, error) {
	refreshed, err := topicStorage.FindTopicByName(ctx, vaultPath, topicName)
	if err != nil {
		return "", errors.Wrap(ctx, err, "re-read topic after claude session")
	}
	if err := refreshed.SetField(ctx, "claude_session_id", sessionID); err != nil {
		return "", errors.Wrap(ctx, err, "save session id to topic")
	}
	if err := topicStorage.WriteTopic(ctx, refreshed); err != nil {
		return "", errors.Wrap(ctx, err, "save session id to topic")
	}
	return sessionID, nil
}
```

- **The re-read is load-bearing and must not be optimised away.** The headless turn mutates the same file and always finishes before this runs, so writing the stale in-memory copy would revert the session's own frontmatter changes. Keep the `FindTopicByName` call, and carry the goal's explanatory comment in substance.
- **On failure it returns an empty id, never the one it was handed.** The id is the Vault UI's signal that Resume will work; reporting an id whose write did not land would advertise a session that is not on disk.

### 4e. `handleClaudeSession`

`goalWorkOnOperation.handleClaudeSession` with:

- `if existing := topic.GetField("claude_session_id"); existing != "" { return existing, nil }`
- `if o.starter == nil { return "", ErrStarterUnavailable }`
- the prompt: `fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnGoalCommand(), topic.FilePath)` — **use `GetWorkOnGoalCommand()`**, the same accessor the goal work-on uses, not `GetWorkOnCommand()`. There is no topic-specific work-on command: the spec's Non-goals forbid shipping a topic slash-command file, and adding a third config key would be a new config surface the spec never asks for. See the open question in `<constraints>`.
- `sessionID := o.uuidGenerator()`, `slog.Info("starting claude session", "topic", topic.Name)`
- the interactive branch: `o.starter.StartSession(ctx, sessionID, prompt, sessionDir, topic.Name, isInteractive)`, then `persistTopicSessionID(...)`
- the non-interactive branch: the same `StartSession` call, then `persistTopicSessionID(...)` — with the goal's comments preserved in substance (the id is persisted only after the turn finished; on any failure nothing is persisted, so no compensating clear is needed)
- both branches wrap a `StartSession` failure as `errors.Wrap(ctx, err, "start claude session")`

Do NOT add a session lock of your own — `ops.NewSessionLocker()` is constructed by the CLI and passed into the starter and resumer (§ 5d), exactly as `createWorkOnGoalCommand` does it.

## 5. `pkg/cli/cli.go` — the command family

### 5a. `createTopicCommands`

Add this function immediately after `createGoalCommands` and before `createGoalCompleteCommand`:

```go
//nolint:dupl // Command groups are structurally similar but manage distinct entity types
func createTopicCommands(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topic",
		Short: "Manage topics in the vault",
	}
	cmd.AddCommand(
		createGenericListCommand(
			ctx, configLoader, vaultName, "topics",
			func(c *storage.Config) string { return c.TopicsDir },
			outputFormat,
		),
	)
	cmd.AddCommand(
		createGenericLintCommand(
			ctx, configLoader, vaultName, "topic",
			func(c *storage.Config) string { return c.TopicsDir },
			func(c *storage.Config) string { return c.TopicsDir },
			outputFormat,
		),
	)
	cmd.AddCommand(
		createGenericSearchCommand(
			ctx, configLoader, vaultName, "topics",
			func(c *storage.Config) string { return c.TopicsDir },
			outputFormat,
		),
	)
	cmd.AddCommand(createEntityGetCommand(ctx, configLoader, vaultName, outputFormat, "topic",
		func(cfg *storage.Config) ops.EntityGetOperation {
			return ops.NewTopicGetOperation(storage.NewTopicStorage(cfg))
		},
	))
	cmd.AddCommand(createEntitySetCommand(ctx, configLoader, vaultName, outputFormat, "topic",
		func(cfg *storage.Config) ops.EntitySetOperation {
			return ops.NewTopicSetOperation(storage.NewTopicStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityClearCommand(ctx, configLoader, vaultName, outputFormat, "topic",
		func(cfg *storage.Config) ops.EntityClearOperation {
			return ops.NewTopicClearOperation(storage.NewTopicStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityShowCommand(ctx, configLoader, vaultName, outputFormat, "topic",
		func(cfg *storage.Config) ops.EntityShowOperation {
			return ops.NewTopicShowOperation(storage.NewTopicStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityListAddCommand(ctx, configLoader, vaultName, outputFormat, "topic",
		func(cfg *storage.Config) ops.EntityListAddOperation {
			return ops.NewTopicListAddOperation(storage.NewTopicStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityListRemoveCommand(ctx, configLoader, vaultName, outputFormat, "topic",
		func(cfg *storage.Config) ops.EntityListRemoveOperation {
			return ops.NewTopicListRemoveOperation(storage.NewTopicStorage(cfg))
		},
	))
	cmd.AddCommand(createTopicCompleteCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createTopicDeferCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createWorkOnTopicCommand(ctx, configLoader, vaultName, outputFormat))
	return cmd
}
```

- **Exactly twelve `AddCommand` calls.** Not eleven, not thirteen, no alias. In particular: no `validate` (the task family's single-file validate is task-only), no `watch` (the spec's Non-goals forbid adding a topic kind to the watch surface), no `update` (it is a task-specific checkbox rewriter), no `backfill-identifiers` (task-only).
- **No `--reason` / `--gate-successor` on `set`.** `createEntitySetCommand` already gates those two flags behind `if entityType == "goal"`, so passing `"topic"` omits them. Do NOT change that condition and do NOT pass `"goal"` here to get them.
- The lint page type is `"topic"`. It must **not** be `"task"` — `pkg/ops/lint.go`'s `taskIdentifierIssues` runs only when `pageType == PageTypeTask`, and passing `"task"` would add two identifier checks to every topic page.
- The lint's second directory argument is `TopicsDir` again, mirroring the goal registration's `GoalsDir, GoalsDir`. `createGenericLintCommand` uses it as the goals-directory for the orphan-goal check; topics have no orphan-goal relationship, and passing the topics directory keeps the registration honest without inventing a new parameter. Do NOT change `createGenericLintCommand`'s signature.
- The list/search page-type strings are `"topics"` (plural), matching the goal registration's `"goals"`; the entity type strings passed to the entity builders are `"topic"` (singular). Copy the goal registration's casing exactly.

### 5b. `createTopicCompleteCommand`

Add immediately after `createTopicCommands`:

```go
func createTopicCompleteCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "complete <topic-name>",
		Short: "Mark a topic as complete",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			topicName := args[0]

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			return runMutation(
				ctx,
				vaults,
				*outputFormat,
				func(ctx context.Context, vault *config.Vault) (ops.MutationResult, error) {
					storageConfig := storage.NewConfigFromVault(vault)
					topicStore := storage.NewTopicStorage(storageConfig)
					completeOp := ops.NewTopicCompleteOperation(topicStore)
					result, err := completeOp.Execute(ctx, vault.Path, topicName, vault.Name)
					if err != nil {
						return result, err
					}
					if !OutputFormat(*outputFormat).IsJSON() {
						fmt.Printf("✅ Topic completed: %s\n", result.Name)
					}
					return result, nil
				},
			)
		},
	}
}
```

- **No flags.** The goal's `--force`, `--reason` and `--gate-successor` are close-out machinery: `--force` overrides the open-task gate (which topics do not have), and `--reason` / `--gate-successor` populate the `aborted_reason` / `gate_successor` fields consulted by the goal status guard (which topics do not have). An inert flag would violate Desired Behavior 4's "each performing the real operation its name promises rather than a placeholder". See the open question in `<constraints>`.
- `runMutation` gives first-success-across-vaults semantics and prints the `MutationResult` as JSON when `--output json` is set; the plain branch prints the `✅` line. Copy the goal complete command's shape exactly, minus the flags and minus the `currentDateTime`.
- `Args: cobra.ExactArgs(1)` — the argument shape mirrors the goal's `<goal-name>`.

### 5c. `createTopicDeferCommand`

Add immediately after `createTopicCompleteCommand`. It is `createGoalDeferCommand` with `goal` → `topic` throughout:

- `Use: "defer <topic-name> [date]"`, `Short: "Defer a topic to a specific date"`
- the same `Long` help text with `a goal` → `a topic` in the first line; keep the date-format block verbatim (`+Nd`, weekday, ISO date, RFC3339)
- `Args: cobra.RangeArgs(1, 2)`; `dateStr := "+1d"` when only the name is given
- `currentDateTime := libtime.NewCurrentDateTime()`
- `ops.NewTopicDeferOperation(storage.NewTopicStorage(storageConfig), currentDateTime)`
- `deferOp.Execute(ctx, vault.Path, topicName, dateStr, vault.Name)`
- the plain branch prints `fmt.Printf("📅 Topic deferred to %s: %s\n", result.Message, result.Name)`
- no flags

### 5d. `createWorkOnTopicCommand`

Add immediately after `createTopicDeferCommand`. It is `createWorkOnGoalCommand` with `goal` → `topic`:

- `Use: "work-on <topic-name>"`, `Short: "Mark a topic as in_progress and start a Claude session"`, `Args: cobra.ExactArgs(1)`
- the `--mode` flag: `cmd.Flags().StringVar(&mode, "mode", "auto", "Session mode: auto, interactive, or headless")` — this flag **is** mirrored, because it controls the session's own behaviour and has real meaning here
- `resolveSessionMode(ctx, mode)` first, then `(*configLoader).GetCurrentUser(ctx)` wrapped as `errors.Wrap(ctx, err, "get current user")`, then `getVaults`
- inside `dispatcher.FirstSuccess`: `locker := ops.NewSessionLocker()`, `starter := ops.NewClaudeSessionStarter(vault.GetClaudeScript(), locker)`, `resumer := ops.NewClaudeResumer(vault.GetClaudeScript(), locker)`, `topicStore := storage.NewTopicStorage(storage.NewConfigFromVault(vault))`, `workOnOp := ops.NewTopicWorkOnOperation(topicStore, uuid.NewString, starter, resumer)`, the `sessionDir` resolution (`vault.Path`, overridden by `vault.GetSessionProjectDir()` when non-empty), `workOnOp.Execute(ctx, vault.Path, topicName, currentUser, vault.Name, isInteractive, sessionDir, vault)`, then `return formatWorkOnResult(result, err, currentUser, *outputFormat)`
- no daily-note update, no `--assignee` flag, no `--force`

### 5e. Root registration

In `NewRootCommand`, add exactly this line immediately after the `rootCmd.AddCommand(createGoalCommands(...))` line:

```go
	rootCmd.AddCommand(createTopicCommands(ctx, &configLoader, &vaultName, &outputFormat))
```

Do NOT touch `createTaskCommands`, `createWatchCommand`, `createResolveCommand`, `buildWatchTargets`, or the `validKinds := []string{"task", "goal", "theme", "objective"}` list. The spec's Non-goals name all three watch-side lists explicitly.

## 6. `integration/cli_test.go` — the real-binary evidence

### 6a. The helper

Add a fourth temp-vault helper next to `createTempVault` / `createTempVaultWithGoals` / `createTempVaultWithTopics`, following their shape (`os.MkdirTemp` for the vault, `os.MkdirAll` for each directory, `os.CreateTemp` for the config, a cleanup that removes both):

```go
// createTempVaultWithTopicPages creates a temporary vault whose config sets
// topics_dir, current_user and a claude_script that is deliberately not installed,
// and whose topics and goals directories contain the given pages.
func createTempVaultWithTopicPages(
	topicsDir string,
	topics map[string]string,
	goals map[string]string,
) (vaultPath string, configPath string, cleanup func())
```

Its config content is this YAML, written with `fmt.Sprintf` (vault path into `path: %s`, `topicsDir` into `topics_dir: "%s"`):

```yaml
default_vault: test
current_user: tester@example.com
vaults:
  test:
    name: test
    path: <vaultPath>
    tasks_dir: Tasks
    goals_dir: Goals
    topics_dir: "<topicsDir>"
    claude_script: "claude-not-installed-for-tests"
```

- The `claude_script` value is the load-bearing part: `ops.NewClaudeSessionStarter` calls `exec.LookPath` on it, which fails, so the starter is nil and the work-on path takes `ErrStarterUnavailable` as a soft warning. That makes `topic work-on` deterministic and fast. **Never** leave `claude_script` unset on a vault used by a work-on spec — the default is `claude`, and if that happens to be on `PATH` the spec would spawn a real headless turn.
- `createTempVault`, `createTempVaultWithGoals` and `createTempVaultWithTopics` must NOT gain a `topics_dir`, `current_user` or `claude_script` line — every other spec in the file depends on their configs staying as they are.
- Create the `Tasks` directory as well as `Topics` and `Goals`: the existing helpers do, and `createGenericLintCommand`/`ListPages` behaviour on an absent directory is already covered elsewhere.

### 6b. The command-registration entries

In the `Describe("command registration", ...)` `DescribeTable`, add a `// Topic subcommands` block with exactly these twelve entries, immediately after the `// Goal subcommands` block:

```go
			// Topic subcommands
			Entry("topic list", "topic", "list"),
			Entry("topic lint", "topic", "lint"),
			Entry("topic search", "topic", "search"),
			Entry("topic show", "topic", "show"),
			Entry("topic get", "topic", "get"),
			Entry("topic set", "topic", "set"),
			Entry("topic clear", "topic", "clear"),
			Entry("topic complete", "topic", "complete"),
			Entry("topic defer", "topic", "defer"),
			Entry("topic add", "topic", "add"),
			Entry("topic remove", "topic", "remove"),
			Entry("topic work-on", "topic", "work-on"),
```

`docs/dod.md` requires this: "New CLI commands/subcommands have an entry in the integration test command registration table". The count is frozen at twelve.

### 6c. The behavioural `Describe` block

Add a new inner `Describe` block inside the outer `Describe("vault-cli integration tests", …)`, alongside the existing blocks (after the `config list --output json topics_dir` block). It drives the real binary through `gexec.Start` and every spec calls the helper's `cleanup` via `defer`. Its spec names are frozen and are grep targets in `<verification>`.

**Twelve-leaf set equality and the slash-name exclusion — Acceptance Criteria 1, 2 and 3:**

```
It("topic --help lists exactly the twelve leaves")
It("topic --help leaf set equals the goal --help leaf set")
It("topic --help contains none of the eleven goal slash-command names")
```

- Parse the leaf block from the help output in Go: run `<binPath> topic --help`, take `session.Out.Contents()`, cut from the `Available Commands:` line to the next blank line, keep lines matching `^  [a-z]`, and take the first whitespace-separated field of each as the leaf name. Build a `map[string]bool` or a sorted `[]string`.
- The first spec asserts the resulting set has exactly 12 members and equals the frozen set `{add, clear, complete, defer, get, lint, list, remove, search, set, show, work-on}`.
- The second spec runs `<binPath> goal --help` the same way and asserts `reflect.DeepEqual` (or an equivalent sorted-slice comparison) between the two leaf sets. **A count comparison is not sufficient** — Acceptance Criterion 1 says so explicitly ("equal counts alone are not sufficient"). Assert set equality.
- The third spec asserts `Expect(out).NotTo(ContainSubstring(name))` for each of the eleven names, in a loop or as eleven assertions: `plan-goal`, `execute-goal`, `verify-goal`, `audit-goal`, `create-goal`, `update-goal`, `goal-status`, `launch-goal`, `work-on-goal`, `complete-goal`, `defer-goal`. This is Acceptance Criterion 3's negative evidence; do NOT weaken it to a check on the leaf set alone — the names could appear in a `Long` or `Short` string.
- For the first and second specs, assert the help output's exit code is 0 **and** that the parsed leaf set is non-empty before comparing. An empty set would otherwise satisfy a naive equality check against another empty set if the help format ever changed.

**Phase observability — Acceptance Criteria 4 and 5:**

```
It("topic show emits phase at .fields.phase, equal to what goal show emits")
It("topic show omits the phase key entirely when the page carries none")
It("topic get prints an empty line and exits 0 for an absent phase")
It("topic get prints the on-disk phase value and exits 0 when the page carries one")
```

- Seed two topic pages: `With Phase.md` carrying `phase: planning` in its frontmatter, and `Without Phase.md` carrying no `phase` line. Seed one goal `With Phase` carrying `phase: planning` in `Goals/`.
- Run `<binPath> --config <cfg> topic show "With Phase" --vault test --output json`, `json.Unmarshal` stdout into `map[string]any`, take `m["fields"].(map[string]any)`, and assert `fields["phase"]` is present and equals the string `"planning"`. Do the same for `goal show "With Phase" --output json` and assert the two extracted values are **equal** — that is Acceptance Criterion 4's "the value equals the same on-disk phase string that `goal show` reports at `.fields.phase`".
- For the no-phase page, assert the JSON parses, that `fields` is present and non-empty (anchor the assertion on a key that *is* there, e.g. `status`, so the negative check cannot pass vacuously on empty output), and that `_, ok := fields["phase"]; Expect(ok).To(BeFalse())`. **Absent, not empty** — do NOT write `Expect(fields["phase"]).To(BeEmpty())`, which would pass on a present-but-empty key and would not distinguish the two.
- `topic get "Without Phase" phase` → `Eventually(session).Should(gexec.Exit(0))` and `Expect(string(session.Out.Contents())).To(Equal("\n"))` — the command prints one empty line. `topic get "With Phase" phase` → exit 0 and stdout equal to `"planning\n"`. Both are required: the positive one stops the negative clause from passing vacuously.
- Do NOT pipe output through `jq` — the Go parse is the stronger assertion (it checks structure and types, not just presence).

**Round-trip — Acceptance Criterion 6:**

```
It("topic set, get and clear round-trip a frontmatter key on disk")
It("topic add and topic remove round-trip a list field on disk")
```

- `topic set "Round Trip" owner alice` → exit 0; then read the file from disk with `os.ReadFile` and assert its bytes contain `owner: alice`; then `topic get "Round Trip" owner` → stdout `"alice\n"`; then `topic clear "Round Trip" owner` → exit 0 and the file's bytes no longer contain `owner`.
- `topic add "Round Trip" tags alpha` → exit 0; count the `tags` entries on disk; `topic add "Round Trip" tags beta` → exit 0; assert one more entry than after the first add; then `topic remove "Round Trip" tags alpha` → exit 0 and assert one fewer entry than after the second add. Asserting against the on-disk file, not against the command's own output, is what makes this a state-transition test.

**Complete and defer — Acceptance Criterion 7:**

```
It("topic complete moves the status to completed and refuses a second complete")
It("topic defer writes defer_date for a relative and an absolute date")
It("topic defer refuses a past date and leaves the page byte-identical")
```

- `topic complete "Round Trip"` → exit 0; the file's frontmatter now carries `status: completed`. A second `topic complete "Round Trip"` → `gexec.Exit(1)`, and the file's bytes are **unchanged** between the two runs (capture before and after and compare).
- `topic defer "Round Trip" +7d` → exit 0 and the file carries a `defer_date`. Because the run date is not pinnable in a subprocess, assert the date is exactly 7 days after today computed in Go (`time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")`) rather than asserting a literal. Then `topic defer "Round Trip" 2027-03-19` → exit 0 and the file carries `2027-03-19`.
- `topic defer "Round Trip" 2000-01-01` → `gexec.Exit(1)`, and the file's bytes are identical before and after. That is Acceptance Criterion 7's byte-identical requirement.
- For the two refusals, assert the exit code **and** the file comparison. An exit code alone would pass on a command that failed after writing.

**Lint — Acceptance Criterion 8:**

```
It("topic lint reports a seeded duplicate key and passes a clean page")
```

- Seed `Dupe.md` with a frontmatter block carrying the same key twice (for example `status` twice, both `in_progress`), and `Clean.md` with a clean block whose `status` is one of the values the lint's status check accepts (`next`, `in_progress`, `backlog`, `completed`, `hold`, `aborted`) — use `status: in_progress`. `topic lint --vault test` → `gexec.Exit(1)`; stdout names `Dupe.md` and reports `DUPLICATE_KEY`; stdout does not name `Clean.md`. Then remove `Dupe.md` and re-run → exit 0 and stdout contains `No lint issues found`. Both directions are required. Note that the generic lint's status and priority checks are page-type-independent — `collectLintIssues` runs them for every page type, so `topic lint` validates a topic page's `status` against the same value list `goal lint` uses. That is the existing shared behaviour, it is not gated by the `"topic"` page type you pass, and Acceptance Criterion 8 does not ask you to change it. Do NOT modify `pkg/ops/lint.go` to make topic statuses validate differently.

**Search — Acceptance Criterion 9, container-executable part only:**

```
It("topic search dispatches to the semantic search operation scoped to the topics directory")
```

- `topic search "attention routing" --vault test` → the session must not report a usage error: assert `Expect(string(session.Out.Contents())).NotTo(ContainSubstring("unknown command"))` and the same for `session.Err.Contents()`. Do NOT assert exit 0 — `semantic-search-mcp` may be absent from this container, in which case the command exits non-zero with `semantic-search-mcp not found on PATH`, which is the correct behaviour and still proves the leaf is registered and reaches the search operation.
- **Acceptance Criterion 9's "returns a topic page that matches a seeded query" needs a real `semantic-search-mcp` and is operator-side** on the spec's verification ladder. Say so in your completion report; do not fake it with a stub binary, and do not add a test that skips itself conditionally.

**Show and list — Acceptance Criterion 11's listing half:**

```
It("topic list returns exactly the pages on disk in the configured directory")
```

- Seed N topic pages (N ≥ 2, plus at least one non-`.md` file and one subdirectory in the topics directory to prove they are filtered), run `topic list --vault test --output json`, `json.Unmarshal` the array, and assert its length equals the `.md` file count on disk in the configured directory. Then re-run with a config whose `topics_dir` names a directory that does not exist and assert exit 0 **and** that the unmarshalled list is empty — that is the "configured-but-absent directory yields an empty result and exit 0, never a silent fallback to the default" half, and it also asserts the default directory was not created. Unmarshal into a `[]ops.TaskListItem` (or `[]map[string]any`) and assert `BeEmpty()`; do NOT assert a literal `[]` body, because `createGenericListCommand` passes a nil slice to `PrintJSON`, which `encoding/json` encodes as `null` — `null` unmarshals into a nil slice and `BeEmpty()` passes, whereas a `ContainSubstring("[]")` assertion never would.

**Traversal refusal — Acceptance Criterion 12:**

```
It("topic show and topic set refuse a traversal name and touch nothing outside the topics directory")
```

- Write `<vaultPath>/outside-file.md` and capture its bytes. Run `topic show "../outside-file" --vault test` → `gexec.Exit(1)`; run `topic set "../outside-file" owner alice --vault test` → `gexec.Exit(1)`. Assert the outside file's bytes are identical afterwards. That is Acceptance Criterion 12's container-executable half; its `git status` half is operator-side.
- Do NOT assert a specific error string here — the storage's message is prompt 2's contract and is pinned there. Assert the exit code and the file's unchanged bytes.

**Work-on — Acceptance Criterion 10:**

```
It("topic work-on moves the page into its in-progress state and reports a session outcome")
```

- `topic work-on "Round Trip" --vault test --mode headless` → `Eventually(session).Should(gexec.Exit(0))`. Read the file from disk and assert its frontmatter carries `status: in_progress` and `assignee: tester@example.com`. Assert stdout does **not** contain `Usage:` (that would be a cobra usage error, which Acceptance Criterion 10 excludes) and does contain `Now working on:`.
- Pass `--mode headless` explicitly rather than relying on `auto`: it removes the TTY-detection dependency and matches what the config's non-installed `claude_script` implies.
- Do NOT assert a `session_id` is present — the starter is nil on this path, so no session id is minted, and that is the correct soft-failure behaviour.

## 7. Failure modes and security — the mapping

Map the spec's Failure Modes table onto this change and state the mapping in your completion report:

- **Topics directory absent from the vault.** `topic list` prints nothing and exits 0 (`createGenericListCommand` → `ListOperations.Execute` → `ListPages`'s `fs.ErrNotExist` branch). Covered by § 6c's list spec's second half.
- **Topic page named by `show` does not exist.** `createEntityShowCommand` → `dispatcher.FirstSuccess` → `FindTopicByName` returns `ErrNotFound` → non-zero exit, message naming the topic and the directory (prompt 2's wraps). Add a spec asserting `topic show "Nonexistent" --vault test` exits 1 — put it in the show block.
- **Configuration names a topics directory that does not exist.** Used verbatim, empty list, exit 0. Covered by § 6c's list spec.
- **Topic page carries a non-canonical `phase` value.** `show` surfaces the raw string and the page stays readable — the type-switch case in § 1d reads `GetField`, which prompt 1 made raw. Add a spec: a page carrying `phase: whatever-the-vault-holds` shows without error and reports that exact value. Put it in the show block.
- **Topic page carries a duplicate frontmatter key.** Covered by § 6c's lint spec.
- **`defer` with a relative date or a non-UTC timezone.** Covered by § 6c's defer spec; `parseDeferDate` and `libtime.DateOrDateTime` are reused unchanged.
- **`defer` with a past date.** Covered by § 6c's defer spec (non-zero exit, byte-identical file).
- **Two writers mutate the same topic page at once / crash mid-write.** Last write wins, no partial frontmatter block; prompt 2's round-trip specs and § 6c's complete/defer specs (write, then re-read and compare bytes) are the evidence. Name them as prompt 2's, with this prompt's specs as the CLI-level corroboration.
- **`topic work-on` with no claude script on `PATH`.** `ErrStarterUnavailable` is a soft warning, the status is still written, the command exits 0 — covered by § 6c's work-on spec.

Security properties to preserve, not to build: the topic name is user-supplied and resolves a file path; the containment lives in prompt 2's storage and § 6c's traversal spec is the CLI-level corroboration. Do NOT add a second validation in the CLI — a name check in `pkg/cli` would be a second source of truth for the same rule. Malformed YAML on a topic page fails through `parseToFrontmatterMap` with an error naming the file and leaves the file untouched; the ops layer must not swallow it — every `FindTopicByName` failure is returned, not logged and continued. The `work-on` path reuses the existing session starter and locker, so it inherits that path's existing gating rather than adding a new one. No network, no subprocess and no credential access is introduced by this prompt.

## 8. Flags — what is mirrored and what is deliberately not

Desired Behavior 4 says each leaf mirrors its goal counterpart's "purpose, argument shape, and flags". The argument shapes are mirrored exactly. The flags are mirrored where they carry meaning for a topic and deliberately omitted where they do not:

| leaf | goal flags | topic flags | why |
|---|---|---|---|
| `list` | `--status`, `--all`, `--assignee` | all three | `createGenericListCommand` supplies them; the topic page's `status`/`assignee` are read through the same `Page` path the goal list uses |
| `lint` | `--fix` | `--fix` | `createGenericLintCommand` supplies it |
| `search` | `--top-k` | `--top-k` | `createGenericSearchCommand` supplies it |
| `get` / `clear` | none | none | — |
| `set` | none for non-goal entities | none | `createEntitySetCommand` gates `--reason` / `--gate-successor` behind `entityType == "goal"` |
| `show` | none | none | — |
| `add` / `remove` | none | none | — |
| `complete` | `--force`, `--reason`, `--gate-successor` | **none** | all three are goal close-out machinery for a gate and a status guard topics do not have; an inert flag would be a placeholder |
| `defer` | none | none | — |
| `work-on` | `--mode` | `--mode` | it controls the session's own behaviour and has real meaning here |

Do NOT add a flag that does nothing. Do NOT add an opt-out, a tunable threshold, or a config key that this table does not list.

## 9. Self-check before finishing

- Re-read the changed hunks and confirm: `createTopicCommands` has exactly twelve `AddCommand` calls and they are the twelve in the frozen table; `NewRootCommand` gained exactly one line; `entityShowOperation.Execute`'s type switch in `pkg/ops/frontmatter_entity.go` gained exactly one `case *domain.Topic:` (`createEntityShowCommand` in `pkg/cli/cli.go` is the CLI builder and carries no type switch); the six `NewTopic*Operation` constructors exist; `pkg/ops/goal_workon.go`, `pkg/ops/goal_complete.go`, `pkg/ops/goal_defer.go` and their tests are untouched; no `pkg/ops/watch.go`, `pkg/cli/cli.go` `buildWatchTargets` or `validKinds` change.
- Walk spec 051's Acceptance Criteria 1, 2, 3, 6, 7, 8, 9 and 10 and state in your completion report which requirement and which spec satisfies each, plus which evidence covers each Failure Modes row listed in § 7. For Acceptance Criterion 9, state explicitly that only the dispatch half is container-executable and the semantic-result half is operator-side.
- Walk `docs/dod.md`: every exported type, function and interface has a doc comment; no `fmt.Print*` and no `os.Stdout` in `pkg/ops/`; the CLI prints and the ops layer returns structured results; every error goes through `github.com/bborbe/errors` with a `ctx`; no `context.Background()` in `pkg/`; no new `go.mod` dependency; the twelve command-registration entries exist; tests use Ginkgo v2 / Gomega with counterfeiter mocks in an external test package.
- Confirm each check in `<verification>` passes by **running** it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 051 — non-goals.** Do NOT add any binary leaf beyond the twelve listed — the goal binary surface is exactly twelve leaves and the topic surface mirrors it one for one. Do NOT turn the eleven goal *slash commands* into binary subcommands; `plan-goal`, `execute-goal`, `verify-goal`, `audit-goal`, `create-goal`, `update-goal`, `goal-status`, `launch-goal`, `work-on-goal`, `complete-goal` and `defer-goal` are plugin-level prompt files, not `vault-cli goal` subcommands, and none of those names appears in `vault-cli topic --help`. Do NOT write, backfill, default or normalize a `phase` field onto any topic page. Do NOT invent a default phase (for example falling back to `todo`) when a topic page has no `phase` line. Do NOT modify, extend or reuse the goal phase type, the task phase type, or their normalizers for the topic entity. Do NOT change the goal command family's output, flags, exit codes, or the goal-side frontmatter allowlists. Do NOT extend the hardcoded entity-kind lists elsewhere in the binary — the watch command's accepted type list (`validKinds := []string{"task", "goal", "theme", "objective"}`), its watch-directory set (`buildWatchTargets`) and the resolve/type set each name task, goal, theme and objective by hand; adding topic to the watch surface is a separate spec. Do NOT add a `docs/topic-writing.md`. Do NOT ship topic slash-command files. Do NOT migrate, rewrite or reformat existing topic pages.
- **Copied from spec 051 — constraints.** The goal binary surface stays at exactly twelve leaves with unchanged names, flags, argument counts, output and exit codes. The goal-side frontmatter allowlists and the goal work-on operation are not modified; `pkg/domain/goal.go`, `pkg/domain/goal_frontmatter.go`, `pkg/domain/goal_phase.go` and `pkg/ops/goal_workon.go` carry an empty diff against the baseline. Existing task, goal, theme, objective and vision commands and their tests pass unchanged. The twelve leaf names are fixed; no alias, no additional leaf, and no goal-slash-command name appears in the topic binary surface. The layered recipe in `docs/development-patterns.md` § "Adding a New Command" (Domain → Storage → Ops → CLI) governs the shape of this work.
- **Frozen names.** CLI: `createTopicCommands`, `createTopicCompleteCommand`, `createTopicDeferCommand`, `createWorkOnTopicCommand`; group `Use: "topic"`. Ops: `TopicCompleteOperation`, `TopicDeferOperation`, `TopicWorkOnOperation`, `NewTopicCompleteOperation`, `NewTopicDeferOperation`, `NewTopicWorkOnOperation`, `NewTopicGetOperation`, `NewTopicSetOperation`, `NewTopicClearOperation`, `NewTopicShowOperation`, `NewTopicListAddOperation`, `NewTopicListRemoveOperation`, `topicTagsListOperation`, `topicSetOperation`, `topicClearOperation`, `applyTopicAssigneeMatrix`, `persistTopicSessionID`. Mock output paths `mocks/topic-complete-operation.go`, `mocks/topic-defer-operation.go`, `mocks/topic-workon-operation.go` with fake names `TopicCompleteOperation`, `TopicDeferOperation`, `TopicWorkOnOperation`. Integration helper `createTempVaultWithTopicPages`. All are grep targets in the acceptance criteria.
- **Depends on prompts 1 and 2.** `domain.Topic`, `domain.TopicStatusInProgress`, `domain.TopicStatusCompleted`, `storage.TopicStorage`, `storage.NewTopicStorage` and `storage.Config.TopicsDir` must already exist. If any is missing, stop and report `"status":"failed"` naming the missing prompt; do NOT define a topic entity, a topic status type or a topic storage here to make it compile.
- **The twelve leaves are the whole surface.** No `validate`, no `watch`, no `update`, no `backfill-identifiers`, no alias, no hidden or deprecated command. If `topic --help` lists a thirteenth leaf, the acceptance criteria fail.
- **No phase is ever written.** No code path added by this prompt sets, defaults, backfills or normalises a `phase` key on a topic page. The only mention of `phase` outside tests is prompt 1's read branch and the `knownTopicScalarFields` entry that classifies it as a scalar (so `topic add <page> phase x` is refused as "not a list field" rather than silently appended).
- **No goal close-out machinery.** No `aborted_reason`, no `gate_successor`, no `--reason`, no `--gate-successor`, no `--force`, no open-task gate, no `completed` date, no `blockedBySetRefusal`, no `writeGoalCloseOutFieldsIfCloseOut` on any topic path.
- **No `TopicStatus` enum and no phase enum.** The two values come from prompt 1's plain string constants. Do NOT add a validating setter, an `AvailableTopicStatuses` collection, or a `Validate` method in the ops layer either.
- **Open question 1 — flags on `topic complete`.** Desired Behavior 4 says each leaf mirrors its goal counterpart's "flags", while Acceptance Criterion 7 defines `complete` purely as a status transition and the spec's Non-goals forbid reusing the goal close-out machinery. This prompt resolves that in favour of the Acceptance Criteria and the Non-goals: no flags. If the human auditor reads "flags" as a hard requirement, the change is to add `--force` (gating nothing today) plus `--reason` / `--gate-successor` (writing `aborted_reason` / `gate_successor` through `SetField`) — record this in `## Improvements` rather than implementing it speculatively.
- **Open question 2 — the session command for `topic work-on`.** The spec says the topic slash-command ladder is out of scope, so there is no topic-specific command to invoke. This prompt mirrors the goal and uses `vault.GetWorkOnGoalCommand()` (default `/vault-cli:work-on-goal`). The alternative is `vault.GetWorkOnCommand()` (default `/vault-cli:work-on-task`). Record this in `## Improvements`; do NOT add a third config key.
- **Open question 3 — the list-field allowlist for `topic add` / `topic remove`.** Acceptance Criterion 6 does not name the field. This prompt mirrors the theme/objective/vision precedent and allows `tags` only. The alternative is to add `blocked_by` alongside it, which would require the `blockedByAppendRefusal` / `BlockedByIsScalar` / `BLOCKED_BY_SCALAR` lint contract. Record this in `## Improvements`; do NOT implement it speculatively.
- **Do NOT modify `pkg/ops/lint.go`, `pkg/ops/list.go`, `pkg/ops/search.go`, `pkg/ops/show.go`, `pkg/ops/watch.go` or `pkg/ops/resolve.go`.** Every topic leaf is built from the existing generic builders.
- **No new scenario file.** `ls scenarios/*.md | wc -l` must still print `5`. The spec's `## Scenario coverage` section records the four-condition justification for that decision.
- **Tests.** Ginkgo v2 + Gomega, external test packages (`ops_test`, `integration_test`), counterfeiter mocks for the storage and session interfaces. No stdlib `t.Run` table tests. Every named `It` must contain a real assertion — a spec that is only named does not satisfy the acceptance criteria. Every Go file keeps its BSD license header.
- **`dupl` and `funlen`.** `createTopicCommands` carries `//nolint:dupl // Command groups are structurally similar but manage distinct entity types`, mirroring `createGoalCommands`. If `make lint` reports `dupl` on `createTopicDeferCommand` or `createWorkOnTopicCommand`, add the same annotation with the same reason; do NOT restructure the builders to dodge the linter. Keep every new function under `funlen`'s 80 lines / 50 statements and `nestif`'s depth 4.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`: the daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass. Acceptance Criterion 12's `git status` half and Acceptance Criterion 13's `git diff` half are operator-side on the spec's verification ladder.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
- Do NOT run `go mod vendor`. No new dependency; `go.mod` and `go.sum` are untouched.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite, `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code.

**The ops and integration test runs.** Capture each run's output, check its exit status separately from the name greps (a failing run still prints the names), and never pipe a test command. The ops run's own content is asserted at source level in the next block, not from its log: Ginkgo prints an error message only on failure, so a log grep for a refusal string can never match a green run.

```
go test ./pkg/ops/... -v -ginkgo.v -count=1 > /tmp/topic-ops.log 2>&1; test "$?" = "0"
```

```
go test ./integration/... -v -ginkgo.v -count=1 > /tmp/topic-integration.log 2>&1; test "$?" = "0"
grep -F -q -- 'topic --help lists exactly the twelve leaves' /tmp/topic-integration.log
grep -F -q -- 'topic --help leaf set equals the goal --help leaf set' /tmp/topic-integration.log
grep -F -q -- 'topic --help contains none of the eleven goal slash-command names' /tmp/topic-integration.log
grep -F -q -- 'topic show emits phase at .fields.phase, equal to what goal show emits' /tmp/topic-integration.log
grep -F -q -- 'topic show omits the phase key entirely when the page carries none' /tmp/topic-integration.log
grep -F -q -- 'topic get prints an empty line and exits 0 for an absent phase' /tmp/topic-integration.log
grep -F -q -- 'topic get prints the on-disk phase value and exits 0 when the page carries one' /tmp/topic-integration.log
grep -F -q -- 'topic set, get and clear round-trip a frontmatter key on disk' /tmp/topic-integration.log
grep -F -q -- 'topic add and topic remove round-trip a list field on disk' /tmp/topic-integration.log
grep -F -q -- 'topic complete moves the status to completed and refuses a second complete' /tmp/topic-integration.log
grep -F -q -- 'topic defer writes defer_date for a relative and an absolute date' /tmp/topic-integration.log
grep -F -q -- 'topic defer refuses a past date and leaves the page byte-identical' /tmp/topic-integration.log
grep -F -q -- 'topic lint reports a seeded duplicate key and passes a clean page' /tmp/topic-integration.log
grep -F -q -- 'topic search dispatches to the semantic search operation scoped to the topics directory' /tmp/topic-integration.log
grep -F -q -- 'topic list returns exactly the pages on disk in the configured directory' /tmp/topic-integration.log
grep -F -q -- 'topic show and topic set refuse a traversal name and touch nothing outside the topics directory' /tmp/topic-integration.log
grep -F -q -- 'topic work-on moves the page into its in-progress state and reports a session outcome' /tmp/topic-integration.log
```

`go test ./integration/...` builds the binary with `gexec.Build` and runs it as a subprocess — it is the real-binary proof of the leaf set, the set equality against the goal family, the JSON phase key path, the round-trips and the refusals, and it is the repeatable form of the spec's hand-reproduction, whose built-binary rung is operator-side. If `gexec.Build` fails with a VCS status error, `GOFLAGS=-buildvcs=false` is missing from the environment; export it rather than touching `.git`.

**The new ops files exist and carry the frozen names:**

```
test -f pkg/ops/topic_complete.go
test -f pkg/ops/topic_defer.go
test -f pkg/ops/topic_workon.go
test -f pkg/ops/topic_complete_test.go
test -f pkg/ops/topic_defer_test.go
test -f pkg/ops/topic_workon_test.go
test "$(grep -c 'mocks/topic-complete-operation.go --fake-name TopicCompleteOperation' pkg/ops/topic_complete.go)" = "1"
test "$(grep -c 'mocks/topic-defer-operation.go --fake-name TopicDeferOperation' pkg/ops/topic_defer.go)" = "1"
test "$(grep -c 'mocks/topic-workon-operation.go --fake-name TopicWorkOnOperation' pkg/ops/topic_workon.go)" = "1"
test "$(grep -c 'func NewTopicCompleteOperation(topicStorage storage.TopicStorage) TopicCompleteOperation' pkg/ops/topic_complete.go)" = "1"
test "$(grep -c 'func NewTopicDeferOperation(' pkg/ops/topic_defer.go)" = "1"
test "$(grep -c 'func NewTopicWorkOnOperation(' pkg/ops/topic_workon.go)" = "1"
test "$(grep -c 'func applyTopicAssigneeMatrix(' pkg/ops/topic_workon.go)" = "1"
test "$(grep -c 'func persistTopicSessionID(' pkg/ops/topic_workon.go)" = "1"
test "$(grep -c 'topic %q is already completed' pkg/ops/topic_complete.go)" = "1"
test "$(grep -c 'cannot defer to past date: %s' pkg/ops/topic_defer.go)" -ge 1
test "$(grep -c 'type TopicWorkOnOperation interface' pkg/ops/topic_workon.go)" = "1"
```

The last three are the mandated strings asserted at source level, which is the only place a green run can carry them: the already-completed refusal in § 2, the past-date refusal in § 3 item 3, and the work-on interface in § 4a. The past-date literal is `-ge 1` rather than `= 1` because § 3 item 3 mandates it in two places — the `MutationResult.Error` and the wrapped `errors.Errorf` — exactly as `goal_defer.go` carries it twice; do NOT collapse it to one occurrence to satisfy a count.

**The generic topic ops and the show case are wired:**

```
test "$(grep -c 'func NewTopicGetOperation(' pkg/ops/frontmatter_entity.go)" = "1"
test "$(grep -c 'func NewTopicSetOperation(' pkg/ops/frontmatter_entity.go)" = "1"
test "$(grep -c 'func NewTopicClearOperation(' pkg/ops/frontmatter_entity.go)" = "1"
test "$(grep -c 'func NewTopicShowOperation(' pkg/ops/frontmatter_entity.go)" = "1"
test "$(grep -c 'func NewTopicListAddOperation(' pkg/ops/frontmatter_entity.go)" = "1"
test "$(grep -c 'func NewTopicListRemoveOperation(' pkg/ops/frontmatter_entity.go)" = "1"
test "$(grep -c 'case \*domain.Topic:' pkg/ops/frontmatter_entity.go)" = "1"
test "$(grep -c 'entityType: "topic"' pkg/ops/frontmatter_entity.go)" = "2"
```

The last line is exactly `2`: `entityGetOperation` and `entityShowOperation` are the only two shared structs that carry an `entityType` field, and the topic constructors for `get` and `show` are the only two that set it. `topicSetOperation`, `topicClearOperation` and `topicTagsListOperation` are concrete structs with no `entityType` field, and `NewTopicSetOperation` / `NewTopicClearOperation` / `NewTopicListAddOperation` / `NewTopicListRemoveOperation` therefore add no literal. A count above `2` means a topic literal leaked into a goal, task, theme, objective or vision constructor.

**The twelve leaves and only the twelve:**

```
test "$(sed -n '/func createTopicCommands(/,/^}/p' pkg/cli/cli.go | grep -c 'cmd.AddCommand(')" = "12"
test "$(grep -c 'rootCmd.AddCommand(createTopicCommands(' pkg/cli/cli.go)" = "1"
test "$(grep -c 'func createTopicCompleteCommand(' pkg/cli/cli.go)" = "1"
test "$(grep -c 'func createTopicDeferCommand(' pkg/cli/cli.go)" = "1"
test "$(grep -c 'func createWorkOnTopicCommand(' pkg/cli/cli.go)" = "1"
test "$(sed -n '/func createTopicCommands(/,/^}/p' pkg/cli/cli.go | grep -c '"validate"')" = "0"
test "$(sed -n '/func createTopicCommands(/,/^}/p' pkg/cli/cli.go | grep -c 'WatchCommand')" = "0"
test "$(grep -c 'func createTaskCommands(' pkg/cli/cli.go)" = "1"
test "$(grep -c 'func createGoalCommands(' pkg/cli/cli.go)" = "1"
```

The count-of-12 assertion is the mechanical form of Acceptance Criterion 1; the `validate` and `WatchCommand` zeros are the Non-goals guard. The last two assert the sibling groups were not replaced or renamed.

**The topic group's leaf names are the frozen twelve, in the group source:**

```
sed -n '/func createTopicCommands(/,/^}/p' pkg/cli/cli.go > /tmp/topic-group.txt
test "$(wc -l < /tmp/topic-group.txt | tr -d ' ')" -ge 20
test "$(for leaf in list lint search get set clear show add remove complete defer work-on; do grep -F -q "\"$leaf\"" /tmp/topic-group.txt || echo "$leaf"; done | wc -l | tr -d ' ')" = "0"
```

The `sed` range writes the group function's body to a temp file (no process substitution, so this runs under `sh` as well as `bash`), the line count proves the range actually matched rather than silently producing an empty file, and the loop prints a leaf name only when that leaf is absent — so the assertion is `0` when all twelve are present. This is the source-level complement to the integration test's help-output set comparison; both are required, because the help output could be right while the source carries an extra leaf behind a condition.

**The goal family is untouched and the watch lists are untouched:**

```
test "$(grep -c 'func createGoalCompleteCommand(' pkg/cli/cli.go)" = "1"
test "$(grep -c 'func createGoalDeferCommand(' pkg/cli/cli.go)" = "1"
test "$(grep -c 'func createWorkOnGoalCommand(' pkg/cli/cli.go)" = "1"
test "$(grep -c 'validKinds := \[\]string{"task", "goal", "theme", "objective"}' pkg/cli/cli.go)" = "1"
test "$(grep -c 'func buildWatchTargets(' pkg/cli/cli.go)" = "1"
test "$(sed -n '/func buildWatchTargets(/,/^}/p' pkg/cli/cli.go | grep -c 'topic')" = "0"
test "$(grep -c 'topic' pkg/ops/watch.go)" = "0"
test "$(grep -c 'func NewGoalWorkOnOperation(' pkg/ops/goal_workon.go)" = "1"
test "$(grep -c 'func NewGoalCompleteOperation(' pkg/ops/goal_complete.go)" = "1"
test "$(grep -c 'func NewGoalDeferOperation(' pkg/ops/goal_defer.go)" = "1"
test "$(grep -c 'topic' pkg/ops/goal_workon.go pkg/ops/goal_complete.go pkg/ops/goal_defer.go | grep -vc ':0$')" = "0"
```

**No phase is written, no close-out machinery, no status enum:**

```
test "$(grep -cE 'Set\(.phase.|SetField\(ctx, "phase"|SetPhase' pkg/ops/topic_complete.go pkg/ops/topic_defer.go pkg/ops/topic_workon.go pkg/ops/frontmatter_entity.go | grep -vc ':0$')" = "0"
test "$(grep -cE 'aborted_reason|gate_successor|writeGoalCloseOutFieldsIfCloseOut|blockedBySetRefusal|blockedByAppendRefusal' pkg/ops/topic_complete.go pkg/ops/topic_defer.go pkg/ops/topic_workon.go | grep -vc ':0$')" = "0"
test "$(grep -cE 'type TopicStatus|AvailableTopicStatuses|func \(.*\) Validate' pkg/ops/topic_complete.go pkg/ops/topic_defer.go pkg/ops/topic_workon.go pkg/ops/frontmatter_entity.go | grep -vc ':0$')" = "0"
test "$(grep -c 'TopicStatusCompleted' pkg/ops/topic_complete.go)" -ge 1
test "$(grep -c 'TopicStatusInProgress' pkg/ops/topic_workon.go)" -ge 1
```

**The integration harness carries the twelve registration entries, the helper and no new scenario:**

```
test "$(grep -c 'Entry("topic ' integration/cli_test.go)" = "12"
test "$(grep -c 'func createTempVaultWithTopicPages(' integration/cli_test.go)" = "1"
test "$(grep -c 'claude-not-installed-for-tests' integration/cli_test.go)" = "1"
test "$(ls scenarios/*.md | wc -l | tr -d ' ')" = "5"
```

**Formatting and the full suite:**

```
test -z "$(gofmt -e -l pkg/ops/topic_complete.go pkg/ops/topic_defer.go pkg/ops/topic_workon.go pkg/ops/topic_complete_test.go pkg/ops/topic_defer_test.go pkg/ops/topic_workon_test.go pkg/ops/frontmatter_entity.go pkg/cli/cli.go integration/cli_test.go)"
go test ./pkg/... -count=1 > /tmp/topic-all-pkg.log 2>&1; test "$?" = "0"
```

Finally, walk spec 051's Acceptance Criteria 1, 2, 3, 6, 7, 8, 9 and 10 against the change and state in your completion report which requirement and which spec satisfies each one, which evidence covers each row of the spec's Failure Modes table listed in `<requirements>` § 7, and — for Acceptance Criterion 9 — that only the dispatch half is container-executable while the semantic-result half is operator-side.
</verification>
