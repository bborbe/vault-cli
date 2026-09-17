---
status: completed
spec: [049-publish-escalation-on-assignee-clear]
summary: Made the CLI's agent-escalation body byte-identical to bborbe/agent-task-controller's by adding a peer-copied vaultDeeplink helper and the peer's format string, threading vault name/tasks dir through both frontmatter operation constructors from the CLI, and replacing the weak NotTo(BeEmpty()) assertion with exact-body regression locks plus an escaping/empty-field table
execution_id: vault-cli-exec-220-spec-049-escalation-body-parity
dark-factory-version: v0.193.0
created: "2026-09-17T19:45:00Z"
queued: "2026-09-17T19:50:32Z"
started: "2026-09-17T19:51:33Z"
completed: "2026-09-17T19:57:22Z"
---

# Escalation body parity: render the agent-side notification body byte-for-byte

<summary>
- A park performed through the vault CLI already notifies the operator, but the message text is not the one the agent side sends for the same event.
- This change makes the CLI's notification body identical, character for character, to the agent controller's body for the same task state.
- The message now names the task's status and phase and carries a clickable Obsidian link straight into the parked task file.
- The redundant "on task &lt;name&gt; (&lt;id&gt;)" clause is dropped, because the agent side never put the task name or id in the body — they travel in the metadata, which is already identical and stays untouched.
- The escaping of the link is the same escaping the agent side applies, so a task name containing spaces, ampersands or hashes produces the same link on both sides.
- A task with no phase renders an empty phase on both sides — deliberately, because that is what the agent side does.
- Nothing else changes: the publish stays opt-in, bounded, and non-fatal; the command's exit code, stdout and JSON output are byte-identical.
- The rendered body is now pinned by an exact-equality test rather than a "not empty" check, and the real publish path is exercised so the new text provably survives validation and serialization.
</summary>

<objective>
Make the vault CLI's `agent-escalation` notification body byte-identical to the body `bborbe/agent-task-controller` emits for the same assignee-clear transition, so a park performed by hand and a park performed by the agent produce one indistinguishable message instead of two dialects of the same event. This closes the parity half of spec 049's Goal — "carrying the same payload fields the controller-side path emits" — and its Constraint "Mirror the working peer's transport rather than inventing one". The metadata keys are already correct and verified live; the body is the only part that diverges.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then these files:

- `pkg/ops/escalation.go` — the file whose message construction you change. Read `Escalation`, `escalationMessage`, `escalationPublisher.publish`, and the existing `escalationPublishFailureFormat` constant (a frozen literal that mirrors the peer's failure line — the same mirroring doctrine this prompt applies to the body).
- `pkg/ops/frontmatter.go` — where the transition rule lives. Read `publishAssigneeClearEscalation`, `frontmatterSetOperation` and `frontmatterClearOperation` (both their constructors and their `Execute` tails).
- `pkg/cli/cli.go` — the CLI wiring. Read `escalationPublisher` (near the top) and the `RunE` closures of `createTaskSetCommand` and `createTaskClearCommand`. Both resolve vaults with `getVaults`, then run an `ops.NewVaultDispatcher().FirstSuccess` closure whose parameter is `vault *config.Vault` — that closure already builds `storage.NewTaskStorage(storage.NewConfigFromVault(vault))` and already passes `vault.Path` into `Execute`.
- `pkg/config/config.go` — read the `Vault` struct and `GetTasksDir`. Note that `configLoader.Load` lowercases `vault.Name`; that lowercased slug is the value the peer uses too (its `VAULT_NAME` must match `^[a-z][a-z0-9-]*$`, per `agent-task-controller`'s `routing.ValidateVaultName`).
- `pkg/ops/escalation_test.go` — the publisher suite you extend. Six existing specs; the fixture and the assertion you strengthen are both here.
- `pkg/ops/frontmatter_test.go` — three constructor call sites to update (`Describe("FrontmatterSetOperation")`, `Describe("FrontmatterClearOperation")`, `Describe("Frontmatter assignee-clear escalation")`). The third block is where the wiring is proven. It already imports `notifcore "github.com/bborbe/notification"` and `notifcmd "github.com/bborbe/notification/command/notification"`.
- `pkg/ops/wikilink_roundtrip_test.go` — the seventh constructor call site, inside the `task` `DescribeTable` entry.
- `docs/dod.md` — this repository's `validationPrompt`. Note: `pkg/ops/` never writes to stdout, exported symbols have doc comments, errors come from `github.com/bborbe/errors`, and a user-visible change carries a `## Unreleased` CHANGELOG entry.
- `specs/in-progress/049-publish-escalation-on-assignee-clear.md` — the spec. Read its Goal, Constraints, Non-goals and Failure Modes.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, `DescribeTable` / `Entry`, counterfeiter mocks, `Eventually` vs `Consistently`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, …)` / `errors.Errorf(ctx, …)` from `github.com/bborbe/errors`; never `fmt.Errorf`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — Interface → Constructor → Struct → Method.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — the linter limits (`funlen` 80, `gocognit` 20, `nestif` 4, `dupl`, `golines` 100) and the license header.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.

## The peer's body construction, quoted verbatim

`bborbe/agent-task-controller` emits this notification today. From that repo's `pkg/result/result_writer.go`, `vaultDeeplink` (the file's lines 108-115):

```go
// vaultDeeplink renders an Obsidian URI for the task file, so the escalation
// message is one click from the notification into the parked task.
func vaultDeeplink(vaultName, relPath string) string {
	return fmt.Sprintf(
		"obsidian://open?vault=%s&file=%s",
		url.QueryEscape(vaultName),
		url.QueryEscape(strings.TrimSuffix(relPath, ".md")),
	)
}
```

and the body itself, inside that file's `publishEscalation` (lines 146-153):

```go
	relPath := filepath.Join(r.taskDir, e.taskName+".md")
	message := fmt.Sprintf(
		"escalation: %s cleared its assignee — status %s, phase %s\n%s",
		e.previousAssignee,
		e.status,
		e.phase,
		vaultDeeplink(r.vaultName, relPath),
	)
```

The `—` is U+2014 (em dash). The peer's `escalation` struct carries `taskIdentifier`, `taskName`, `previousAssignee`, `status` and `phase`; `taskDir` and `vaultName` come from the writer's own fields. Its `status` and `phase` are the merged (post-clear) values, and its phase is the empty string when the merged frontmatter has no phase — see the file's lines 509-512:

```go
			escalatedPhase := ""
			if p := merged.Phase(); p != nil {
				escalatedPhase = string(*p)
			}
```

The peer never puts the task name or the task identifier in the body. They travel in `Metadata` only — which is exactly what vault-cli already does, and which this prompt does not change.

## What vault-cli emits today, and why it diverges

From `pkg/ops/escalation.go`:

```go
// escalationMessage renders the notification body. The routing table owns the
// channel, so the body carries what the operator needs to recognise the task.
func escalationMessage(escalation Escalation) string {
	return fmt.Sprintf(
		"escalation: %s cleared its assignee on task %s (%s)",
		escalation.PreviousAssignee,
		escalation.TaskName,
		escalation.TaskIdentifier,
	)
}
```

Verified live on 2026-09-17 by reading the published command off the CQRS topic, the current body is:

```
escalation: bborbe cleared its assignee on task Interactive Session Escalation Does Not Notify Operator (1a0bf5c6-6f3c-4f6b-9b0f-3c1e5a0dd7e2)
```

The transport, the type, the nil target, the metadata keys, the bound and the failure line all already mirror the peer. Only the body was invented — spec 049 specified the metadata keys and left the body unspecified, and prompt 217's open-question 6 recorded that it froze a format of its own. This prompt replaces that format with the peer's.

Two environment facts that shape this prompt:

1. **Make no git calls, anywhere — every check in `<verification>` is git-free.** The daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass.
2. **`.dark-factory.yaml` sets `GOFLAGS=-buildvcs=false`.** Keep it — the integration suite's `gexec.Build` runs `go build`, which otherwise fails on a masked `.git`.
</context>

<requirements>

## 0. Scope — two source files, four test files, one changelog bullet

- `pkg/ops/escalation.go` — the four new `Escalation` fields, `vaultDeeplink`, and the rewritten `escalationMessage`.
- `pkg/ops/frontmatter.go` — the two constructors, the two structs, the transition helper and its two call sites.
- `pkg/cli/cli.go` — the two constructor call sites.
- `pkg/ops/escalation_test.go` — the fixture, the exact-body regression lock, and the body table.
- `pkg/ops/frontmatter_test.go` — four constructor call sites plus the wiring assertion.
- `pkg/ops/wikilink_roundtrip_test.go` — one constructor call site.
- `CHANGELOG.md` — one `## Unreleased` bullet.

Nothing else. `pkg/config/config.go`, `pkg/storage/`, `pkg/domain/`, the `EscalationPublisher` interface, `NewEscalationPublisher`, `EscalationPublishTimeout`, `escalationPublishFailureFormat`, `escalationInitiator` and the whole sender/factory chain are read-only. No new flag, subcommand, config key, or output line. No `Metadata` key change. No README change. No version bump.

## 1. `pkg/ops/escalation.go` — `Escalation` gains the fields the body needs

Replace the struct so it carries everything `escalationMessage` renders. Keep the three existing fields, their names, and their order; append the four new ones:

```go
// Escalation describes one assignee-clear transition to announce.
type Escalation struct {
	// TaskIdentifier is the task's task_identifier frontmatter value.
	TaskIdentifier string
	// TaskName is the task's filename without the .md extension.
	TaskName string
	// PreviousAssignee is the assignee value the task held before the clear.
	PreviousAssignee string
	// Status is the task's status after the clear, as the normalizing Status()
	// accessor reads it. Empty when the frontmatter carries no recognizable
	// status — the peer renders the same empty string in that case.
	Status string
	// Phase is the task's phase after the clear, or the empty string when the
	// frontmatter carries no phase. The peer renders an absent phase the same
	// way, so this is deliberately not defaulted to anything.
	Phase string
	// VaultName is the vault's configured name — the lowercased slug the peer's
	// VAULT_NAME carries — and is what the Obsidian link opens.
	VaultName string
	// TasksDir is the vault's configured tasks directory, joined with the task
	// name to form the vault-relative path the link points at.
	TasksDir string
}
```

Add the imports `net/url`, `path/filepath` and `strings` to the file's stdlib group. `fmt` is already there. No other import changes.

## 2. `pkg/ops/escalation.go` — `vaultDeeplink`, mirroring the peer

Add this function immediately above `escalationMessage`:

```go
// vaultDeeplink renders an Obsidian URI for the task file, so the escalation
// message is one click from the notification into the parked task. It is a
// deliberate byte-for-byte copy of the working peer's helper of the same name
// in bborbe/agent-task-controller's pkg/result/result_writer.go, escaping
// included: the two producers must render one link for one task, and a
// differently escaped link is a different link.
func vaultDeeplink(vaultName, relPath string) string {
	return fmt.Sprintf(
		"obsidian://open?vault=%s&file=%s",
		url.QueryEscape(vaultName),
		url.QueryEscape(strings.TrimSuffix(relPath, ".md")),
	)
}
```

Non-negotiable properties:

- Both escapes go through `url.QueryEscape`, exactly as the peer does. Do NOT hand-roll escaping, do NOT use `url.PathEscape`, do NOT skip escaping for a vault name that "looks safe". `QueryEscape` is what makes a space render as `+` and a `/` as `%2F`, and it is what the peer produces.
- The `.md` suffix is trimmed before escaping, not after.
- The helper takes the vault-relative path, not an absolute one.

## 3. `pkg/ops/escalation.go` — `escalationMessage` rewritten

Replace the function body with the peer's format, verbatim:

```go
// escalationMessage renders the notification body. It is a deliberate
// byte-for-byte copy of the working peer's body in bborbe/agent-task-controller's
// pkg/result/result_writer.go, so one park renders one message whichever side
// performed it. The task name and the task identifier are NOT in the body — the
// peer carries them in Metadata only, and so does this repository.
func escalationMessage(escalation Escalation) string {
	relPath := filepath.Join(escalation.TasksDir, escalation.TaskName+".md")
	return fmt.Sprintf(
		"escalation: %s cleared its assignee — status %s, phase %s\n%s",
		escalation.PreviousAssignee,
		escalation.Status,
		escalation.Phase,
		vaultDeeplink(escalation.VaultName, relPath),
	)
}
```

Non-negotiable properties:

- The format string is exactly `"escalation: %s cleared its assignee — status %s, phase %s\n%s"` — one em dash (U+2014), a comma after `status %s`, a newline before the link. Not a hyphen, not an en dash, not a semicolon, not `\n\n`.
- The `on task %s (%s)` clause is gone, together with its two arguments. The task name and identifier must not appear anywhere in the body.
- The link is the fourth argument, so it is the second line of the message.
- `relPath` is `filepath.Join(escalation.TasksDir, escalation.TaskName+".md")` — the same join the peer performs with its `taskDir` and `taskName`.
- `escalationMessage` stays a pure function of its argument: no I/O, no clock, no context, no config lookup.

For a fixture of `PreviousAssignee: "alice"`, `Status: "in_progress"`, `Phase: "human_review"`, `VaultName: "personal"`, `TasksDir: "25 Tasks"`, `TaskName: "Park the Escalation Task"`, the rendered body is exactly:

```
escalation: alice cleared its assignee — status in_progress, phase human_review
obsidian://open?vault=personal&file=25+Tasks%2FPark+the+Escalation+Task
```

## 4. `pkg/ops/frontmatter.go` — thread the vault identity to the transition rule

### 4a. The transition helper

`publishAssigneeClearEscalation` is the only path to the publisher, so the vault identity has to reach it. Keep the function's existing doc comment, add one sentence to it naming the two new parameters, and extend the signature and the `Escalation` construction:

```go
// publishAssigneeClearEscalation emits one agent-escalation notification when a
// frontmatter write cleared a non-empty assignee. It is the single transition
// rule both clear paths share.
//
// previousAssignee MUST be read before the mutation: `task set <task> assignee ""`
// leaves the key present and empty while `task clear <task> assignee` deletes it,
// so a read taken after the mutation yields "" on both paths and the notification
// would lose the one value it exists to carry. value is always "" on the clear
// path, which has no value argument. vaultName and tasksDir are the vault's
// configured identity, carried so the body can render the same Obsidian link the
// agent-side peer renders.
func publishAssigneeClearEscalation(
	ctx context.Context,
	publisher EscalationPublisher,
	task *domain.Task,
	key, value, previousAssignee, vaultName, tasksDir string,
) {
	if key != "assignee" || value != "" || previousAssignee == "" {
		return
	}
	// An absent phase renders as the empty string, exactly as the peer renders
	// it: the two producers must agree on the body for a task with no phase, so
	// this is not defaulted, not skipped, and not replaced by a placeholder.
	escalatedPhase := ""
	if p := task.Phase(); p != nil {
		escalatedPhase = string(*p)
	}
	publisher.PublishEscalation(ctx, Escalation{
		TaskIdentifier:   task.TaskIdentifier(),
		TaskName:         task.Name,
		PreviousAssignee: previousAssignee,
		Status:           string(task.Status()),
		Phase:            escalatedPhase,
		VaultName:        vaultName,
		TasksDir:         tasksDir,
	})
}
```

Keep the three-condition gate byte-identical: `key != "assignee" || value != "" || previousAssignee == ""`. Do not widen it, do not reorder it, do not add a condition.

`task.Status()` is the normalizing accessor and returns `""` for an absent or unrecognized status — that is the peer's behaviour too, so do NOT substitute a literal like `"in_progress"` and do NOT call `GetField("status")` instead.

### 4b. Both constructors and both structs

Append the two parameters after the existing ones — the existing order `(taskStorage, publisher)` is preserved:

```go
// NewFrontmatterSetOperation creates a new frontmatter set operation.
func NewFrontmatterSetOperation(
	taskStorage storage.TaskStorage,
	publisher EscalationPublisher,
	vaultName, tasksDir string,
) FrontmatterSetOperation {
	return &frontmatterSetOperation{
		taskStorage: taskStorage,
		publisher:   publisher,
		vaultName:   vaultName,
		tasksDir:    tasksDir,
	}
}

type frontmatterSetOperation struct {
	taskStorage storage.TaskStorage
	publisher   EscalationPublisher
	vaultName   string
	tasksDir    string
}
```

```go
// NewFrontmatterClearOperation creates a new frontmatter clear operation.
func NewFrontmatterClearOperation(
	taskStorage storage.TaskStorage,
	publisher EscalationPublisher,
	vaultName, tasksDir string,
) FrontmatterClearOperation {
	return &frontmatterClearOperation{
		taskStorage: taskStorage,
		publisher:   publisher,
		vaultName:   vaultName,
		tasksDir:    tasksDir,
	}
}

type frontmatterClearOperation struct {
	taskStorage storage.TaskStorage
	publisher   EscalationPublisher
	vaultName   string
	tasksDir    string
}
```

The `FrontmatterSetOperation` and `FrontmatterClearOperation` interfaces do NOT change — only their constructors gain a parameter. Do NOT touch `mocks/frontmatter-set-operation.go` or `mocks/frontmatter-clear-operation.go`; `make generate` regenerates the whole directory anyway.

### 4c. The two call sites

Update both `Execute` tails to pass the operation's own fields, leaving the surrounding code untouched:

```go
	publishAssigneeClearEscalation(ctx, o.publisher, task, key, value, previousAssignee, o.vaultName, o.tasksDir)
```

```go
	publishAssigneeClearEscalation(ctx, o.publisher, task, key, "", previousAssignee, o.vaultName, o.tasksDir)
```

Every other line of both `Execute` methods stays byte-identical: the `previousAssignee` capture and its comment, the close-out helper, the phase-regression guard, the `SetField` error translation and its `Try:` hint, the `WriteTask` error wording, and the position of the publish after the successful write.

## 5. `pkg/cli/cli.go` — hand the vault's identity to each operation

Both dispatcher closures already receive `vault *config.Vault`. Pass its configured name and its tasks directory:

```go
				setOp := ops.NewFrontmatterSetOperation(taskStore, publisher, vault.Name, vault.GetTasksDir())
```

```go
				clearOp := ops.NewFrontmatterClearOperation(taskStore, publisher, vault.Name, vault.GetTasksDir())
```

Non-negotiable properties:

- `vault.Name` is the right value, and it is already lowercased by `configLoader.Load`. Do NOT re-case it, do NOT title-case it for "readability", and do NOT substitute the vault's directory basename. The peer's `VAULT_NAME` is a lowercase slug (it must match `^[a-z][a-z0-9-]*$`), so the lowercased config name is what makes the two links identical.
- `vault.GetTasksDir()` is the right accessor for the tasks directory — the same one `storage.NewConfigFromVault` uses. Do NOT read `vault.TasksDir` directly: the accessor is what supplies the `Tasks` default for a vault that does not configure the key.
- Do NOT add a fallback for an empty `vault.Name`, and do NOT add a config key or a default for it.
- `escalationPublisher(ctx, configLoader)` is unchanged and still called ONCE per command invocation, before the dispatcher loop. It does not depend on which vault resolved, and it must not move inside the closure.
- No new flag, no new subcommand, no new output line. The two `fmt.Printf` success lines (`✅ Set %s=%s on: %s`, `✅ Cleared %s on: %s`) and both JSON result maps stay byte-identical.
- `getVaults` stays byte-identical and keeps its 29 call sites.

## 6. `pkg/ops/escalation_test.go` — the regression lock

### 6a. The fixture carries every field the body renders

Replace the `escalation = ops.Escalation{…}` assignment in `BeforeEach`:

```go
		escalation = ops.Escalation{
			TaskIdentifier:   "0f6a3a0e-0000-4000-8000-000000000001",
			TaskName:         "Park the Escalation Task",
			PreviousAssignee: "alice",
			Status:           "in_progress",
			Phase:            "human_review",
			VaultName:        "personal",
			TasksDir:         "25 Tasks",
		}
```

The task name carries spaces and the tasks directory carries a space on purpose: a fixture without them cannot fail on a missing escape.

### 6b. The existing spec asserts the exact body

In `It("publishes exactly one agent-escalation command with no target")`, replace `Expect(command.Message).NotTo(BeEmpty())` with an exact comparison and add the validator check:

```go
		Expect(command.Message).To(Equal(notifcore.NotificationMessage(
			"escalation: alice cleared its assignee — status in_progress, phase human_review\n" +
				"obsidian://open?vault=personal&file=25+Tasks%2FPark+the+Escalation+Task",
		)))
		Expect(command.Validate(ctx)).To(Succeed())
```

`command.Message` is a `notifcore.NotificationMessage`, not a `string`: `Equal("…")` with a bare string literal fails on type, so the expected value must be wrapped in the `notifcore.NotificationMessage` conversion exactly as above.

The `command.Validate(ctx)` assertion is the boundary check: the real publish path (`cdb.commandObjectSender.createMessage`) calls `commandObject.Validate`, which validates the notification command, so a body that the library rejects must fail here rather than at deploy time.

Leave every other assertion in that spec as it is — the type, the nil target, the metadata map, and the call count.

### 6c. A table pinning the body's escaping and its empty-field rendering

Add this `DescribeTable` inside the same `Describe("EscalationPublisher")` block, after the existing specs:

```go
	DescribeTable("renders the agent-side escalation body",
		func(e ops.Escalation, expected string) {
			publisher.PublishEscalation(ctx, e)

			command := recordedCommand()
			Expect(command.Message).To(Equal(notifcore.NotificationMessage(expected)))
			Expect(command.Validate(ctx)).To(Succeed())
		},
		Entry("a task name and tasks dir carrying spaces",
			ops.Escalation{
				TaskIdentifier:   "0f6a3a0e-0000-4000-8000-000000000001",
				TaskName:         "Park the Escalation Task",
				PreviousAssignee: "alice",
				Status:           "in_progress",
				Phase:            "human_review",
				VaultName:        "personal",
				TasksDir:         "25 Tasks",
			},
			"escalation: alice cleared its assignee — status in_progress, phase human_review\n"+
				"obsidian://open?vault=personal&file=25+Tasks%2FPark+the+Escalation+Task",
		),
		Entry("characters the link must escape",
			ops.Escalation{
				TaskIdentifier:   "0f6a3a0e-0000-4000-8000-000000000002",
				TaskName:         "Sync & Review #3",
				PreviousAssignee: "bob",
				Status:           "in_progress",
				Phase:            "execution",
				VaultName:        "personal",
				TasksDir:         "25 Tasks",
			},
			"escalation: bob cleared its assignee — status in_progress, phase execution\n"+
				"obsidian://open?vault=personal&file=25+Tasks%2FSync+%26+Review+%233",
		),
		Entry("an absent status and phase render empty, exactly as the peer renders them",
			ops.Escalation{
				TaskIdentifier:   "0f6a3a0e-0000-4000-8000-000000000003",
				TaskName:         "Park the Escalation Task",
				PreviousAssignee: "alice",
				VaultName:        "personal",
				TasksDir:         "25 Tasks",
			},
			"escalation: alice cleared its assignee — status , phase \n"+
				"obsidian://open?vault=personal&file=25+Tasks%2FPark+the+Escalation+Task",
		),
	)
```

The third entry is not a placeholder: `status , phase ` with nothing between the comma and the newline is the peer's own rendering for a task whose frontmatter carries neither field, and a producer that "helpfully" substitutes `unknown` would produce a body the peer never produces. Do NOT change that expectation and do NOT add a default.

The second entry is the escaping boundary: an unescaped `&` turns the rest of the task name into a second URI parameter, and an unescaped `#` starts a fragment — either one silently opens the wrong note. `QueryEscape` renders them as `%26` and `%23`, and the space before the name as `+`.

`recordedCommand` is **not** defined in `Describe("EscalationPublisher")` — the only one in the repository is a closure inside `Describe("Frontmatter assignee-clear escalation")` in `pkg/ops/frontmatter_test.go` (~line 1200) and is out of scope here. Define an equivalent helper in this block (in the existing `var` / `BeforeEach` scope): wait via `Eventually(func() int { return mockSender.SendPublishNotificationCommandCallCount() }).Should(Equal(1))`, then `_, command := mockSender.SendPublishNotificationCommandArgsForCall(0)`, and return it. Do not re-derive the command inline.

The six existing specs in this block keep their current behaviour and assertions, including `It("publishes onto the deployment's prefixed command topic")` — that spec drives the body through the real `notifcmd.NewNotificationPublishCommandSender` and the real `cdb` command-object serializer against a mocked Kafka producer, which is what proves the new body survives validation and JSON marshalling on the production path. Add one assertion to it, after the existing topic assertion:

```go
		value, err := message.Value.Encode()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(value)).To(ContainSubstring(
			"obsidian://open?vault=personal&file=25+Tasks%2FPark+the+Escalation+Task",
		))
```

`message.Value` is a `sarama.Encoder`; use its `Encode()` method rather than a type assertion (this repository's linter rejects unchecked type assertions). Do not add a `sarama` import for this.

## 7. `pkg/ops/frontmatter_test.go` — the wiring proof

### 7a. The two plain blocks

`Describe("FrontmatterSetOperation")` and `Describe("FrontmatterClearOperation")` build a publisher with no brokers, so nothing publishes there. Give each constructor a literal identity and change nothing else in those blocks:

```go
		setOp = ops.NewFrontmatterSetOperation(mockTaskStorage, publisher, "personal", "25 Tasks")
```

```go
		clearOp = ops.NewFrontmatterClearOperation(mockTaskStorage, publisher, "personal", "25 Tasks")
```

### 7b. The escalation block

In `Describe("Frontmatter assignee-clear escalation")`, add two fixture variables next to the existing ones and set them in `BeforeEach`:

```go
		vaultName = "personal"
		tasksDir = "25 Tasks"
```

pass them to both constructors:

```go
		setOp = ops.NewFrontmatterSetOperation(mockTaskStorage, publisher, vaultName, tasksDir)
		clearOp = ops.NewFrontmatterClearOperation(mockTaskStorage, publisher, vaultName, tasksDir)
```

and add a phase to the task fixture so the rendered body exercises it:

```go
		task = domain.NewTask(
			map[string]any{
				"status":          "in_progress",
				"phase":           "human_review",
				"assignee":        previousAssignee,
				"task_identifier": taskIdentifier,
			},
			domain.FileMetadata{Name: taskName},
			domain.Content(""),
		)
```

Nothing else in that block's `BeforeEach` changes.

### 7c. The two positive specs assert the rendered body

In `It("publishes one escalation when task set empties a non-empty assignee")` and `It("publishes one escalation when task clear removes a non-empty assignee")`, add this assertion after the existing metadata assertion in each — the expected text is the same in both, because the body does not depend on which path performed the clear:

```go
		Expect(command.Message).To(Equal(notifcore.NotificationMessage(
			"escalation: alice cleared its assignee — status in_progress, phase human_review\n" +
				"obsidian://open?vault=personal&file=25+Tasks%2Fmy-task",
		)))
```

This is the wiring lock: it fails if the operation forwards an empty vault name, an empty tasks directory, an empty status, or an empty phase into the `Escalation`, even though every metadata assertion still passes.

Leave the nine silent-case specs and the bound spec untouched.

### 7d. `pkg/ops/wikilink_roundtrip_test.go`

Give the one `task` `DescribeTable` entry's constructor a literal identity, leaving the rest of the entry as it is:

```go
			return ops.NewFrontmatterSetOperation(
				storage.NewTaskStorage(cfg),
				ops.NewEscalationPublisher("", "", &mocks.NotificationSenderFactory{}),
				"personal", "25 Tasks",
			).Execute(ctx, vaultPath, name, key, value, "", "", false)
```

No import changes — `mocks` is already imported there.

## 8. `CHANGELOG.md` — one bullet under `## Unreleased`

`docs/dod.md` requires a user-visible change to carry a `## Unreleased` entry, and this change alters what the operator reads in chat. The file **already has** a `## Unreleased` section (line 11, holding the two spec-049 bullets) — append to it. If it is somehow absent, create it in the position the DoD fixes: below the `All notable changes…` preamble block and above the newest `## vX.Y.Z` heading. **Never create a second `## Unreleased` heading** — a duplicate breaks `make precommit`. Add exactly one bullet:

```
- fix: the `agent-escalation` body the CLI publishes on an assignee clear is now byte-identical to the body `bborbe/agent-task-controller` publishes for the same transition — it names the task's status and phase and carries an Obsidian link into the parked task file, and it no longer repeats the task name and identifier that already travel in the notification metadata.
```

Do NOT create a `## vX.Y.Z` heading, do NOT move or edit any existing version section, and do NOT touch `.claude-plugin/` — the release is cut after the merge.

## 9. Self-check before finishing

- Re-read the changed hunks in `pkg/ops/escalation.go` and confirm the format string is `"escalation: %s cleared its assignee — status %s, phase %s\n%s"` character for character, that the em dash is U+2014, that the `on task` clause is gone, and that `vaultDeeplink` escapes both values with `url.QueryEscape`.
- Confirm `pkg/ops/escalation.go`'s `EscalationPublisher`, `NewEscalationPublisher`, `NewKafkaNotificationSenderFactory`, `EscalationPublishTimeout`, `escalationPublishFailureFormat` and the whole `publish`/`logEscalationPublishFailure` pair are byte-identical to before, and that the `Metadata` map's three keys and their values are unchanged.
- Confirm `pkg/config/config.go` and everything under `pkg/storage/` and `pkg/domain/` are byte-identical to before this prompt.
- Walk spec 049's Goal ("carrying the same payload fields the controller-side path emits") and its Constraint "Mirror the working peer's transport rather than inventing one", and state in the completion report which requirement and which assertion satisfies each. State explicitly that the Acceptance Criteria list itself carries no body-parity checkbox and that this prompt closes the Goal/Constraint half.
- Walk the spec's Failure Modes table and state in the completion report which evidence covers each row, and that none of the rows changes behaviour: a publish failure still never fails the write, the bound is unchanged, and the opt-in gate is unchanged.
- Walk `docs/dod.md`: no `fmt.Print*` added under `pkg/ops/`, errors use `github.com/bborbe/errors`, exported symbols have doc comments, tests use Ginkgo v2 / Gomega with counterfeiter mocks, and the `## Unreleased` entry exists in the DoD's position.
- Confirm each check in `<verification>` passes by running it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 049 — non-goals.** Do NOT change the controller-side publish path, its clear sites, or its behaviour. Do NOT re-notify on later edits of an already-parked task — one notification per clear. Do NOT backfill notifications for tasks parked before this ships. Do NOT cover a raw editor write of the assignee field. Do NOT implement Matrix delivery. Do NOT make the publish mandatory: a deployment with no broker configured must keep today's behaviour exactly, with no connection attempt and no new failure mode.
- **Copied from spec 049 — constraints.** Both documented clear paths are covered — `task set … assignee ""` and `task clear … assignee` — because an operator following the README or `docs/task-writing.md` uses the second. The two paths differ in write semantics, so the previous value must be captured BEFORE the mutation on both, and the publish must stay AFTER the successful write on both. A publish failure never fails the write: the command's exit code and stdout stay unchanged for callers that do not care about the publish. Opt-in is load-bearing. The emitted metadata keys are `taskIdentifier`, `taskName`, `previousAssignee` — unchanged by this prompt. A publish failure logs one line carrying the peer's own wording through the ops layer's logger, never stdout — this repository's ops layer writes no stdout.
- **Frozen literals.** The struct field names `Status`, `Phase`, `VaultName`, `TasksDir`; the helper names `escalationMessage`, `vaultDeeplink`; the constructor parameter order `(taskStorage, publisher, vaultName, tasksDir)`; the CLI call shape `ops.NewFrontmatterSetOperation(taskStore, publisher, vault.Name, vault.GetTasksDir())` and its clear twin; the helper signature `publishAssigneeClearEscalation(ctx, publisher, task, key, value, previousAssignee, vaultName, tasksDir)`; the format strings `"escalation: %s cleared its assignee — status %s, phase %s\n%s"` and `"obsidian://open?vault=%s&file=%s"`; the test names `renders the agent-side escalation body`, `a task name and tasks dir carrying spaces`, `characters the link must escape`, `an absent status and phase render empty, exactly as the peer renders them`.
- **Every existing test must still pass.** The six publisher specs, the nine silent-case specs, the bound spec, both plain frontmatter blocks, the wikilink round-trip table, and every integration spec keep their fixtures and assertions apart from the additions this prompt names.
- **No new flag, subcommand, config key, or output line.** The two commands' plain-text success lines and JSON result maps are byte-identical.
- **Error idiom.** `errors.Wrap(ctx, …)` / `errors.Errorf(ctx, …)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no `context.Background()` in `pkg/` (tests may use it).
- **Tests.** Ginkgo v2 + Gomega, counterfeiter mocks, external test packages (`ops_test`) — no stdlib `t.Run` table tests. Every positive assertion on the asynchronous publish goes through `Eventually`; every "nothing was published" assertion goes through `Consistently`. Never pipe a test command.
- **No version bumps.** Leave the newest `## vX.Y.Z` heading and both `.claude-plugin/` JSON files untouched. The only CHANGELOG change is the one `## Unreleased` bullet.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code.

**The body is the peer's body, and the invented clause is gone:**

```
test "$(grep -c 'escalation: %s cleared its assignee — status %s, phase %s' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'obsidian://open?vault=%s&file=%s' pkg/ops/escalation.go)" = "1"
! grep -q 'on task %s (%s)' pkg/ops/escalation.go
test "$(grep -c 'escalation.TaskIdentifier' pkg/ops/escalation.go)" = "3"
```

The `on task` absence check is the clause removal. The count of 4 is the identifier's remaining occurrences — three inside `logEscalationPublishFailure` and one in the `Metadata` map, all of which this prompt leaves alone. A fifth occurrence means the identifier crept back into the body, where the peer never puts it; do not satisfy this check by editing the logger or the metadata.

**The link is built the way the peer builds it:**

```
test "$(grep -c 'func vaultDeeplink(vaultName, relPath string) string {' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'url.QueryEscape(vaultName)' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'url.QueryEscape(strings.TrimSuffix(relPath, ".md"))' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'filepath.Join(escalation.TasksDir, escalation.TaskName+".md")' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'vaultDeeplink(escalation.VaultName, relPath)' pkg/ops/escalation.go)" = "1"
test "$(grep -cE '^[[:space:]]+"net/url"$' pkg/ops/escalation.go)" = "1"
test "$(grep -cE '^[[:space:]]+"path/filepath"$' pkg/ops/escalation.go)" = "1"
test "$(grep -cE '^[[:space:]]+"strings"$' pkg/ops/escalation.go)" = "1"
```

**`Escalation` carries the four new fields, and the transition rule fills all four:**

```
test "$(grep -cE '^[[:space:]]+Status[[:space:]]+string' pkg/ops/escalation.go)" = "1"
test "$(grep -cE '^[[:space:]]+Phase[[:space:]]+string' pkg/ops/escalation.go)" = "1"
test "$(grep -cE '^[[:space:]]+VaultName[[:space:]]+string' pkg/ops/escalation.go)" = "1"
test "$(grep -cE '^[[:space:]]+TasksDir[[:space:]]+string' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'key, value, previousAssignee, vaultName, tasksDir string,' pkg/ops/frontmatter.go)" = "1"
test "$(grep -c 'escalatedPhase = string(*p)' pkg/ops/frontmatter.go)" = "1"
test "$(grep -cE '^[[:space:]]+Status:[[:space:]]+string\(task\.Status\(\)\),' pkg/ops/frontmatter.go)" = "1"
test "$(grep -cE '^[[:space:]]+Phase:[[:space:]]+escalatedPhase,' pkg/ops/frontmatter.go)" = "1"
test "$(grep -cE '^[[:space:]]+VaultName:[[:space:]]+vaultName,' pkg/ops/frontmatter.go)" = "1"
test "$(grep -cE '^[[:space:]]+TasksDir:[[:space:]]+tasksDir,' pkg/ops/frontmatter.go)" = "1"
test "$(grep -c 'key != "assignee" || value != "" || previousAssignee == ""' pkg/ops/frontmatter.go)" = "1"
```

**Both constructors take the identity, both operations hold it, and both call sites pass it:**

```
test "$(grep -c 'vaultName, tasksDir string,' pkg/ops/frontmatter.go)" = "3"
test "$(grep -cE '^[[:space:]]+vaultName[[:space:]]+string' pkg/ops/frontmatter.go)" = "2"
test "$(grep -cE '^[[:space:]]+tasksDir[[:space:]]+string' pkg/ops/frontmatter.go)" = "2"
test "$(grep -cE '^[[:space:]]+vaultName:[[:space:]]+vaultName,' pkg/ops/frontmatter.go)" = "2"
test "$(grep -cE '^[[:space:]]+tasksDir:[[:space:]]+tasksDir,' pkg/ops/frontmatter.go)" = "2"
test "$(grep -c 'publishAssigneeClearEscalation(ctx, o.publisher, task, key, value, previousAssignee, o.vaultName, o.tasksDir)' pkg/ops/frontmatter.go)" = "1"
test "$(grep -c 'publishAssigneeClearEscalation(ctx, o.publisher, task, key, "", previousAssignee, o.vaultName, o.tasksDir)' pkg/ops/frontmatter.go)" = "1"
```

**The CLI passes the vault's configured name and tasks directory, and nothing else moved:**

```
test "$(grep -c 'ops.NewFrontmatterSetOperation(taskStore, publisher, vault.Name, vault.GetTasksDir())' pkg/cli/cli.go)" = "1"
test "$(grep -c 'ops.NewFrontmatterClearOperation(taskStore, publisher, vault.Name, vault.GetTasksDir())' pkg/cli/cli.go)" = "1"
test "$(grep -c 'func escalationPublisher(' pkg/cli/cli.go)" = "1"
test "$(grep -c 'escalationPublisher(ctx, configLoader)' pkg/cli/cli.go)" = "2"
test "$(grep -c 'getVaults(ctx, configLoader, vaultName)' pkg/cli/cli.go)" = "29"
test "$(grep -c '✅ Set %s=%s on: %s' pkg/cli/cli.go)" = "2"
test "$(grep -c '✅ Cleared %s on: %s' pkg/cli/cli.go)" = "2"
```

**The transport is untouched:**

```
test "$(grep -c 'const EscalationPublishTimeout = 5 \* time.Second' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'escalationPublishFailureFormat = "publish agent-escalation notification for task %s (%s) escalated by %s failed: %v"' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'const escalationInitiator = "vault-cli"' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'func NewEscalationPublisher(' pkg/ops/escalation.go)" = "1"
test "$(grep -cE '^[[:space:]]+"previousAssignee":[[:space:]]+escalation\.PreviousAssignee,' pkg/ops/escalation.go)" = "1"
test "$(grep -cE '^[[:space:]]+"taskIdentifier":[[:space:]]+escalation\.TaskIdentifier,' pkg/ops/escalation.go)" = "1"
test "$(grep -cE '^[[:space:]]+"taskName":[[:space:]]+escalation\.TaskName,' pkg/ops/escalation.go)" = "1"
```

**The regression lock exists in the source, in the file the spec names:**

```
test "$(grep -c 'escalation: alice cleared its assignee — status in_progress, phase human_review' pkg/ops/escalation_test.go)" = "2"
test "$(grep -c 'obsidian://open?vault=personal&file=25+Tasks%2FPark+the+Escalation+Task' pkg/ops/escalation_test.go)" -ge 2
test "$(grep -c 'obsidian://open?vault=personal&file=25+Tasks%2FSync+%26+Review+%233' pkg/ops/escalation_test.go)" = "1"
test "$(grep -c 'escalation: alice cleared its assignee — status , phase ' pkg/ops/escalation_test.go)" = "1"
test "$(grep -c 'command.Validate(ctx)' pkg/ops/escalation_test.go)" -ge 2
test "$(grep -c 'DescribeTable("renders the agent-side escalation body"' pkg/ops/escalation_test.go)" = "1"
test "$(grep -c 'message.Value.Encode()' pkg/ops/escalation_test.go)" = "1"
! grep -q 'NotTo(BeEmpty())' pkg/ops/escalation_test.go
```

The first count is 1 because the fixture body appears once; the second is `-ge 2` because the same link is pinned in the exact-body assertion and in the table's first entry. The `NotTo(BeEmpty())` absence check is the point of the prompt: the weak assertion must be gone, not merely supplemented.

**The wiring is locked, and every constructor call site was updated:**

```
test "$(grep -c 'obsidian://open?vault=personal&file=25+Tasks%2Fmy-task' pkg/ops/frontmatter_test.go)" = "2"
test "$(grep -c 'ops.NewFrontmatterSetOperation(mockTaskStorage, publisher, "personal", "25 Tasks")' pkg/ops/frontmatter_test.go)" = "1"
test "$(grep -c 'ops.NewFrontmatterClearOperation(mockTaskStorage, publisher, "personal", "25 Tasks")' pkg/ops/frontmatter_test.go)" = "1"
test "$(grep -c 'ops.NewFrontmatterSetOperation(mockTaskStorage, publisher, vaultName, tasksDir)' pkg/ops/frontmatter_test.go)" = "1"
test "$(grep -c 'ops.NewFrontmatterClearOperation(mockTaskStorage, publisher, vaultName, tasksDir)' pkg/ops/frontmatter_test.go)" = "1"
test "$(grep -c '"personal", "25 Tasks",' pkg/ops/wikilink_roundtrip_test.go)" = "1"
test "$(grep -rc 'ops.NewFrontmatterSetOperation(\|ops.NewFrontmatterClearOperation(' --include='*.go' pkg/ | awk -F: '{s+=$NF} END {print s}')" = "7"
```

The count of 7 is every call site of both constructors — two in `pkg/cli/cli.go`, four in `pkg/ops/frontmatter_test.go`, one in `pkg/ops/wikilink_roundtrip_test.go`. The constructors' own definition lines carry no `ops.` prefix and are not counted. A call site left on the old two-argument form will not compile, but this check names the file that was missed.

**The ops and integration suites run, and the spec names appear in the run's output.** Capture the output, check the exit status separately from the name greps (a failing run still prints the names), and never pipe a test command:

```
go test ./pkg/ops/... -v -count=1 > /tmp/escalation-parity-ops.log 2>&1; test "$?" = "0"
grep -F -q 'renders the agent-side escalation body' /tmp/escalation-parity-ops.log
grep -F -q 'a task name and tasks dir carrying spaces' /tmp/escalation-parity-ops.log
grep -F -q 'characters the link must escape' /tmp/escalation-parity-ops.log
grep -F -q 'an absent status and phase render empty, exactly as the peer renders them' /tmp/escalation-parity-ops.log
grep -F -q 'publishes exactly one agent-escalation command with no target' /tmp/escalation-parity-ops.log
grep -F -q 'prefixed command topic' /tmp/escalation-parity-ops.log
grep -F -q 'publishes one escalation when task set empties a non-empty assignee' /tmp/escalation-parity-ops.log
grep -F -q 'publishes one escalation when task clear removes a non-empty assignee' /tmp/escalation-parity-ops.log
grep -F -q 'publishes nothing when task set fails to write' /tmp/escalation-parity-ops.log
grep -F -q 'returns success within the publish bound when the sender blocks past it' /tmp/escalation-parity-ops.log
```

```
go test ./integration/... -v -count=1 > /tmp/escalation-parity-integration.log 2>&1; test "$?" = "0"
grep -F -q 'task set assignee empty exits 0 and reports the failure when the broker is unreachable' /tmp/escalation-parity-integration.log
grep -F -q 'task clear assignee exits 0 with unchanged output when no broker is configured' /tmp/escalation-parity-integration.log
```

If `gexec.Build` fails with a VCS status error, `GOFLAGS=-buildvcs=false` is missing from the environment — the container sets it in `.dark-factory.yaml`; export it rather than touching `.git`.

**The changelog entry exists, in the position the DoD fixes:**

```
test "$(grep -c '^## Unreleased$' CHANGELOG.md)" = "1"
test "$(grep -n '^## Unreleased$' CHANGELOG.md | cut -d: -f1)" -lt "$(grep -n '^## v' CHANGELOG.md | head -1 | cut -d: -f1)"
test "$(grep -c 'byte-identical to the body' CHANGELOG.md)" = "1"
test "$(grep -c '^## v' CHANGELOG.md)" -ge 1
```

**Formatting:**

```
test -z "$(gofmt -e -l pkg/ops/escalation.go pkg/ops/frontmatter.go pkg/cli/cli.go pkg/ops/escalation_test.go pkg/ops/frontmatter_test.go pkg/ops/wikilink_roundtrip_test.go)"
```

Finally, walk spec 049's Goal and its mirroring Constraint against the change and state in your completion report which requirement and which assertion satisfies each, which evidence covers each row of the spec's Failure Modes table, and that the Acceptance Criteria list itself carries no body-parity checkbox.
</verification>

<!--
OPEN QUESTIONS FOR THE AUDITOR — not instructions for the executing agent.

1. THE PARITY REQUIREMENT'S HOME. The caller describes body parity as "an
   explicit acceptance criterion of spec 049". It is not in that spec's
   Acceptance Criteria list. It lives in the spec's Goal ("carrying the same
   payload fields the controller-side path emits") and its Constraints ("Mirror
   the working peer's transport rather than inventing one" / "The emitted
   metadata keys are taskIdentifier, taskName, and previousAssignee, matching
   what the peer emits"). Prompt 217's own open-question 6 recorded the gap
   explicitly: "The notification message body is not specified by the spec —
   only the metadata keys are. This prompt freezes 'escalation: <previousAssignee>
   cleared its assignee on task <taskName> (<taskIdentifier>)'." The body in
   production today is that frozen literal, verified live on 2026-09-17. If the
   auditor wants a checkbox to hang this on, spec 049 needs an Acceptance
   Criterion added; this prompt closes the Goal/Constraint half without one.

2. SCOPE BEYOND "ONE FILE'S MESSAGE CONSTRUCTION PLUS ITS TEST". The body needs
   four values the ops layer does not have today: the task's status and phase
   (available on the parsed task) and the vault's name and tasks directory
   (available only in the CLI's config). Two routes were available:
   (a) this one — append `vaultName, tasksDir` to the two frontmatter operation
   constructors and pass `vault.Name, vault.GetTasksDir()` from the dispatcher
   closure, which already receives `vault *config.Vault`; and
   (b) extend `storage.Config` with `VaultName` and expose it through the
   exported `storage.TaskStorage` interface, which would leave both constructors
   and cli.go untouched.
   (b) was rejected: it widens an exported interface that this repository also
   ships as a library, and it puts a rendering concern (the Obsidian link) behind
   a storage accessor. (a) changes only call sites that already exist and keeps
   `escalationPublisher` built once per command, which prompt 218 froze as a
   non-negotiable property. If the auditor prefers (b), the requirement set
   changes materially and the prompt should be rewritten rather than patched.

3. CHANGELOG BULLET. The caller's file list is "one file's message construction
   plus its test". A `## Unreleased` bullet is added beyond that list because
   `docs/dod.md` — this repository's `validationPrompt` — requires an entry for a
   user-visible change, and the operator-visible chat text does change. Drop
   section 8 and its three verification checks if the reviewer wants a strictly
   two-source-file diff; `make precommit` still passes without it, because
   `scripts/check-changelog.sh` validates the file's structure and not the
   presence of an entry.

4. EMPTY `vault.Name`. A vault entry that configures no `name:` key yields an
   empty vault parameter in the link (`obsidian://open?vault=&file=…`). No
   fallback is added: the operator's config names every vault, the peer's
   VAULT_NAME is required and can never be empty, and a fallback would be a new
   mechanism with no named consumer. Flagged so the auditor can decide whether
   `configLoader.Load` should default `Vault.Name` from the map key instead.

5. `status , phase ` IS INTENTIONAL. The third table entry pins the peer's
   rendering for a task whose frontmatter carries neither status nor phase.
   It reads like a placeholder bug and is not one — the peer's code produces
   exactly this, and a producer that substitutes `unknown` would emit a body
   the peer never emits. If the auditor considers that rendering itself a defect,
   the fix belongs in the peer first, and this prompt must follow it, not lead.

6. THE INTEGRATION SUITE DOES NOT COVER THE BODY. Both existing integration
   specs configure an unreachable broker (`127.0.0.1:1`) or no broker at all, so
   no message is ever captured at the binary level. The body's survival of
   validation and JSON marshalling is therefore pinned at the ops layer, by the
   existing `prefixed command topic` spec extended in 6c, which drives the real
   `notifcmd` sender and the real `cdb` command-object serializer against a
   mocked Kafka producer. An end-to-end body assertion needs a live broker and
   belongs to the spec's Rung-2 check, not here.
-->
