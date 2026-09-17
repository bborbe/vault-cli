---
status: approved
spec: [049-publish-escalation-on-assignee-clear]
created: "2026-09-17T13:25:00Z"
queued: "2026-09-17T15:50:49Z"
---

# Assignee clear publishes one escalation from both documented clear paths

<summary>
- Clearing a task's assignee through the CLI now tells the agent side that the task was parked by hand.
- Both documented routes do it: setting the assignee to empty, and clearing the assignee key.
- Exactly one notification per clear, and the value reported is the assignee the task held before the clear.
- Nothing else notifies. An already-empty assignee, a different frontmatter key, a replacement assignee, and a failed write all stay silent.
- The file is written first; the notification only goes out once the write has succeeded.
- A deployment with no broker configured keeps behaving exactly as it does today, with no connection attempt.
- A broken or unreachable broker never fails the command, never changes its output, and never delays it past the bound.
- Every path is pinned by a test: both routes, each silent case, the bound, and the real binary against an unreachable broker.
</summary>

<objective>
Make a hand-performed park observable: when either documented clear path empties a task's assignee, publish exactly one `agent-escalation` notification through the transport the previous prompt built, so the human-to-agent half of the handoff stops being silent. This is spec 049's prompt 2 of 3: it covers Desired Behaviors 1, 2 and 4 and Acceptance Criteria 1, 2, 3, 5 and 8. Acceptance Criterion 6 is the spec's Post-Deploy (Rung-2) check and stays operator-side; Desired Behavior 6 and Acceptance Criterion 7 belong to prompt 3.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then these files:

- `pkg/ops/frontmatter.go` — the file you change. Read `frontmatterSetOperation` (its `Execute`, the close-out helper call, the phase-regression guard, `SetField`, `WriteTask`) and `frontmatterClearOperation` (its `Execute`, `ClearField`, `WriteTask`). The two differ in write semantics: `SetField("assignee", "")` leaves the key present and empty, while `ClearField("assignee")` deletes it — which is why the previous value must be captured before the mutation on both.
- `pkg/ops/escalation.go` — the transport the previous prompt added. READ ONLY. You call `EscalationPublisher.PublishEscalation(ctx, ops.Escalation{TaskIdentifier, TaskName, PreviousAssignee})` and `NewEscalationPublisher(brokers, topicPrefix, factory)`; you do not change either.
- `pkg/ops/frontmatter_test.go` — the two existing `Describe` blocks you extend (`Describe("FrontmatterSetOperation")` and `Describe("FrontmatterClearOperation")`) and the counterfeiter mock style (`mocks.TaskStorage`, `WriteTaskArgsForCall(0)`).
- `pkg/ops/wikilink_roundtrip_test.go` — the third call site of `NewFrontmatterSetOperation`, inside a `DescribeTable` entry for the `task` kind.
- `pkg/ops/ops_suite_test.go` — the suite entry point (`TestSuite`) and the `//go:generate` counterfeiter line.
- `pkg/cli/cli.go` — the CLI wiring. Read `getVaults` (top of file), `createTaskSetCommand`, and `createTaskClearCommand`. Both commands resolve vaults with `getVaults`, then loop through a `ops.NewVaultDispatcher().FirstSuccess` closure that builds `storage.NewTaskStorage(storage.NewConfigFromVault(vault))` and the frontmatter operation. `configLoader` is a `*config.Loader`; `(*configLoader).Load(ctx)` returns the `*config.Config` whose `Notification.Brokers` and `Notification.TopicPrefix` you pass through.
- `pkg/cli/output.go` — `PrintJSON` and the plain-output convention. The two commands print `✅ Set %s=%s on: %s` and `✅ Cleared %s on: %s`; those lines must not change.
- `integration/cli_test.go` — the harness: `createTempVault(tasks map[string]string)` and `createTempVaultWithGoals`, the `gexec.Start` + `Eventually(session).Should(gexec.Exit(0))` style, `gbytes`, and the outer `Describe("vault-cli integration tests", …)`.
- `integration/integration_suite_test.go` — `binPath` is built once in `BeforeSuite` with `gexec.Build("github.com/bborbe/vault-cli")`, so the specs run the real binary as a subprocess.
- `pkg/domain/task_frontmatter.go` — `Assignee()`, `TaskIdentifier()`, and `GetField`. `task.Name` is the filename without `.md` (it lives on the embedded `domain.FileMetadata`).
- `docs/dod.md` — this repository's `validationPrompt`: `pkg/ops/` never writes to stdout, exported symbols have doc comments, and errors come from `github.com/bborbe/errors`.
- `specs/in-progress/049-publish-escalation-on-assignee-clear.md` — the spec. Read Desired Behavior, Acceptance Criteria 1-3 and 5-6, Constraints, Failure Modes and Non-goals.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, counterfeiter mocks, `Consistently` vs `Eventually`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, …)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — Interface → Constructor → Struct → Method.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — the linter limits (`funlen` 80, `gocognit` 20, `nestif` 4, `dupl`, `golines` 100) and the license header.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.

## The pieces you change, quoted verbatim

The set operation's tail, from `pkg/ops/frontmatter.go`:

```go
// NewFrontmatterSetOperation creates a new frontmatter set operation.
func NewFrontmatterSetOperation(taskStorage storage.TaskStorage) FrontmatterSetOperation {
	return &frontmatterSetOperation{
		taskStorage: taskStorage,
	}
}

type frontmatterSetOperation struct {
	taskStorage storage.TaskStorage
}
```

```go
	if err := task.SetField(ctx, key, value); err != nil {
		if key == "status" && strings.Contains(err.Error(), "missing close-out field(s)") {
			err = errors.Errorf(ctx,
				"%s\nTry: vault-cli task set \"%s\" status %s --reason \"<text>\" --gate-successor \"<successor|none>\"",
				err.Error(), taskName, value)
		}
		return errors.Wrap(ctx, err, "set field")
	}

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	return nil
}
```

The clear operation, complete, from `pkg/ops/frontmatter.go`:

```go
// NewFrontmatterClearOperation creates a new frontmatter clear operation.
func NewFrontmatterClearOperation(taskStorage storage.TaskStorage) FrontmatterClearOperation {
	return &frontmatterClearOperation{
		taskStorage: taskStorage,
	}
}

type frontmatterClearOperation struct {
	taskStorage storage.TaskStorage
}

// Execute clears (removes) the value of a frontmatter field on a task.
func (o *frontmatterClearOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key string,
) error {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}

	task.ClearField(key)

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	return nil
}
```

The three call sites you must update:

```go
// pkg/cli/cli.go — inside createTaskSetCommand's RunE closure
				setOp := ops.NewFrontmatterSetOperation(taskStore)
```

```go
// pkg/cli/cli.go — inside createTaskClearCommand's RunE closure
				clearOp := ops.NewFrontmatterClearOperation(taskStore)
```

```go
// pkg/ops/frontmatter_test.go — Describe("FrontmatterSetOperation")
		setOp = ops.NewFrontmatterSetOperation(mockTaskStorage)
```

```go
// pkg/ops/frontmatter_test.go — Describe("FrontmatterClearOperation")
		clearOp = ops.NewFrontmatterClearOperation(mockTaskStorage)
```

```go
// pkg/ops/wikilink_roundtrip_test.go — the "task" DescribeTable entry
			return ops.NewFrontmatterSetOperation(storage.NewTaskStorage(cfg)).
				Execute(ctx, vaultPath, name, key, value, "", "", false)
```

Two environment facts that shape this prompt:

1. **Make no git calls, anywhere — every check in `<verification>` is git-free.** The daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass.
2. **`.dark-factory.yaml` sets `GOFLAGS=-buildvcs=false`.** Keep it — `gexec.Build` in the integration suite runs `go build`, which otherwise fails on the masked `.git`.
</context>

<requirements>

## 0. Scope — one ops file, one CLI file, three test files

- `pkg/ops/frontmatter.go` — the transition rule, the previous-assignee capture, and the publish call on both paths.
- `pkg/cli/cli.go` — one new helper and the wiring of both commands.
- `pkg/ops/frontmatter_test.go` — update the two existing constructor call sites, add the transition specs.
- `pkg/ops/wikilink_roundtrip_test.go` — update the one constructor call site.
- `integration/cli_test.go` — one helper and two subprocess specs.

Nothing else. `pkg/ops/escalation.go` and `pkg/config/config.go` are read-only; no new flag, subcommand, config key, or output line; no documentation file changes (prompt 3 owns those); no version bump.

## 1. `pkg/ops/frontmatter.go` — the shared transition rule

Add this function at the end of the file, after `checkPhaseRegression`:

```go
// publishAssigneeClearEscalation emits one agent-escalation notification when a
// frontmatter write cleared a non-empty assignee. It is the single transition
// rule both clear paths share.
//
// previousAssignee MUST be read before the mutation: `task set <task> assignee ""`
// leaves the key present and empty while `task clear <task> assignee` deletes it,
// so a read taken after the mutation yields "" on both paths and the notification
// would lose the one value it exists to carry. value is always "" on the clear
// path, which has no value argument.
func publishAssigneeClearEscalation(
	ctx context.Context,
	publisher EscalationPublisher,
	task *domain.Task,
	key, value, previousAssignee string,
) {
	if key != "assignee" || value != "" || previousAssignee == "" {
		return
	}
	publisher.PublishEscalation(ctx, Escalation{
		TaskIdentifier:   task.TaskIdentifier(),
		TaskName:         task.Name,
		PreviousAssignee: previousAssignee,
	})
}
```

Non-negotiable properties:

- All three conditions are required. `key != "assignee"` keeps an unrelated `task set` from emitting arbitrary notifications; `value != ""` keeps a non-empty-to-non-empty change silent; `previousAssignee == ""` keeps the empty-to-empty transition silent.
- The publisher is called AFTER the caller's successful write, never before. A failed write must emit nothing.
- `PublishEscalation` is called directly on the interface. Do NOT nil-check the publisher, do NOT wrap the call in a recover, and do NOT add a retry.

## 2. `pkg/ops/frontmatter.go` — the set path

Change the constructor and struct:

```go
// NewFrontmatterSetOperation creates a new frontmatter set operation.
func NewFrontmatterSetOperation(
	taskStorage storage.TaskStorage,
	publisher EscalationPublisher,
) FrontmatterSetOperation {
	return &frontmatterSetOperation{
		taskStorage: taskStorage,
		publisher:   publisher,
	}
}

type frontmatterSetOperation struct {
	taskStorage storage.TaskStorage
	publisher   EscalationPublisher
}
```

In `Execute`, immediately after the `FindTaskByName` error check and BEFORE `writeTaskCloseOutFieldsIfCloseOut`, capture the previous assignee:

```go
	// Read the assignee before the mutation. `task set <task> assignee ""` leaves
	// the key present and empty, so a read taken after the write can no longer
	// tell a cleared assignee from one that was never set.
	previousAssignee := task.Assignee()
```

Then replace the operation's tail so the publish happens after the successful write:

```go
	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	publishAssigneeClearEscalation(ctx, o.publisher, task, key, value, previousAssignee)

	return nil
}
```

Nothing else in `Execute` changes: the close-out helper, the phase-regression guard, the `SetField` error translation and its `Try:` hint, and the error wording all stay byte-identical.

## 3. `pkg/ops/frontmatter.go` — the clear path

Change the constructor and struct the same way:

```go
// NewFrontmatterClearOperation creates a new frontmatter clear operation.
func NewFrontmatterClearOperation(
	taskStorage storage.TaskStorage,
	publisher EscalationPublisher,
) FrontmatterClearOperation {
	return &frontmatterClearOperation{
		taskStorage: taskStorage,
		publisher:   publisher,
	}
}

type frontmatterClearOperation struct {
	taskStorage storage.TaskStorage
	publisher   EscalationPublisher
}
```

In `Execute`, capture the previous assignee after the `FindTaskByName` error check and BEFORE `task.ClearField(key)`, then publish after the successful write:

```go
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}

	// Read the assignee before the deletion. ClearField removes the key outright,
	// so a read taken after the write always yields "".
	previousAssignee := task.Assignee()

	task.ClearField(key)

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	publishAssigneeClearEscalation(ctx, o.publisher, task, key, "", previousAssignee)

	return nil
}
```

The interface signatures `FrontmatterSetOperation` and `FrontmatterClearOperation` do NOT change — only their constructors gain a parameter. Do NOT regenerate or hand-edit the mocks for those two interfaces.

## 4. `pkg/cli/cli.go` — build the publisher once per command

Add this helper immediately after `getVaults`:

```go
// escalationPublisher builds the optional escalation publisher from the
// operator's config. A config that names no brokers yields a publisher that
// publishes nothing and opens no connection, so a standalone vault-cli behaves
// exactly as it did before this feature existed.
func escalationPublisher(
	ctx context.Context,
	configLoader *config.Loader,
) (ops.EscalationPublisher, error) {
	cfg, err := (*configLoader).Load(ctx)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "load config")
	}
	return ops.NewEscalationPublisher(
		cfg.Notification.Brokers,
		cfg.Notification.TopicPrefix,
		ops.NewKafkaNotificationSenderFactory(),
	), nil
}
```

Then in BOTH `createTaskSetCommand` and `createTaskClearCommand`, inside `RunE` and immediately after the existing `getVaults` error check, add:

```go
			publisher, err := escalationPublisher(ctx, configLoader)
			if err != nil {
				return err
			}
```

and pass it to the operation inside the dispatcher closure:

```go
				setOp := ops.NewFrontmatterSetOperation(taskStore, publisher)
```

```go
				clearOp := ops.NewFrontmatterClearOperation(taskStore, publisher)
```

Non-negotiable properties:

- The publisher is built ONCE per command invocation, before the dispatcher loop — it does not depend on which vault resolved.
- No new flag, no new subcommand, no new output line, and no change to the plain-text or JSON output of either command. The two `fmt.Printf` success lines and the two JSON result maps stay byte-identical.
- `getVaults` stays byte-identical and keeps its 29 call sites (`grep -c 'getVaults(ctx, configLoader, vaultName)' pkg/cli/cli.go` returns 29).
- If `funlen`, `gocyclo`, `gocognit` or `nestif` fires on either command function, the fix is the extraction you already did — do NOT add a `//nolint` directive for a limit that was not previously suppressed, and do NOT restructure the commands.

## 5. `pkg/ops/frontmatter_test.go` — update the two existing call sites, add the transition specs

### 5a. The existing blocks keep their current behaviour

In `Describe("FrontmatterSetOperation")`'s `BeforeEach`, replace the constructor call with a publisher that has no brokers configured, so every existing spec in that block behaves exactly as it does today (no publish, no connection attempt):

```go
		mockFactory := &mocks.NotificationSenderFactory{}
		publisher := ops.NewEscalationPublisher("", "", mockFactory)
		setOp = ops.NewFrontmatterSetOperation(mockTaskStorage, publisher)
```

Do the same in `Describe("FrontmatterClearOperation")`'s `BeforeEach`:

```go
		mockFactory := &mocks.NotificationSenderFactory{}
		publisher := ops.NewEscalationPublisher("", "", mockFactory)
		clearOp = ops.NewFrontmatterClearOperation(mockTaskStorage, publisher)
```

Do NOT change any other line of those two blocks — every existing `It`, fixture and assertion stays as it is. In particular, the existing `Context("setting assignee field")` (which sets `bob` on a task whose assignee is empty) and `Context("clearing assignee field")` (which clears `alice`) must keep passing unchanged, and neither may be rewritten to assert a publish.

### 5b. `pkg/ops/wikilink_roundtrip_test.go`

Add `"github.com/bborbe/vault-cli/mocks"` to the file's import block — the file does not import `mocks` today, and the no-broker publisher below needs `mocks.NotificationSenderFactory`. Without this the file will not compile.

Replace the `task` entry's constructor call with a no-broker publisher, leaving the rest of the entry as it is:

```go
			return ops.NewFrontmatterSetOperation(
				storage.NewTaskStorage(cfg),
				ops.NewEscalationPublisher("", "", &mocks.NotificationSenderFactory{}),
			).Execute(ctx, vaultPath, name, key, value, "", "", false)
```

### 5c. The new `Describe` block — the transition rule, both paths

Add this block to `pkg/ops/frontmatter_test.go` after the existing `Describe("FrontmatterClearOperation")` block, and add exactly two imports to the file: `notifcore "github.com/bborbe/notification"` and `notifcmd "github.com/bborbe/notification/command/notification"`. Do NOT add `"time"` — it is already imported, and a second copy is a compile error. (`context`, `errors`, `time`, `mocks`, `domain`, `ops`, Ginkgo and Gomega are already imported.)

```go
var _ = Describe("Frontmatter assignee-clear escalation", func() {
	var (
		ctx               context.Context
		err               error
		mockTaskStorage   *mocks.TaskStorage
		mockSender        *mocks.NotificationPublishCommandSender
		mockFactory       *mocks.NotificationSenderFactory
		publisher         ops.EscalationPublisher
		setOp             ops.FrontmatterSetOperation
		clearOp           ops.FrontmatterClearOperation
		vaultPath         string
		taskName          string
		previousAssignee  string
		taskIdentifier    string
		task              *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		mockSender = &mocks.NotificationPublishCommandSender{}
		mockSender.SendPublishNotificationCommandReturns(nil)
		mockFactory = &mocks.NotificationSenderFactory{}
		mockFactory.CreateReturns(mockSender, nil)
		publisher = ops.NewEscalationPublisher("broker-1:9092", "master", mockFactory)
		setOp = ops.NewFrontmatterSetOperation(mockTaskStorage, publisher)
		clearOp = ops.NewFrontmatterClearOperation(mockTaskStorage, publisher)
		vaultPath = "/path/to/vault"
		taskName = "my-task"
		previousAssignee = "alice"
		taskIdentifier = "0f6a3a0e-0000-4000-8000-000000000001"
		task = domain.NewTask(
			map[string]any{
				"status":          "in_progress",
				"assignee":        previousAssignee,
				"task_identifier": taskIdentifier,
			},
			domain.FileMetadata{Name: taskName},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	// recordedCommand waits for the asynchronous publish and returns the single
	// command it recorded. PublishEscalation performs the publish in a goroutine,
	// so every positive assertion must go through Eventually.
	recordedCommand := func() notifcmd.NotificationPublishCommand {
		Eventually(func() int {
			return mockSender.SendPublishNotificationCommandCallCount()
		}).Should(Equal(1))
		_, command := mockSender.SendPublishNotificationCommandArgsForCall(0)
		return command
	}

	// assertSilent asserts that nothing was published and no connection was
	// attempted, for the whole duration a stray asynchronous publish would need.
	assertSilent := func() {
		Consistently(func() int {
			return mockSender.SendPublishNotificationCommandCallCount()
		}, "200ms", "50ms").Should(Equal(0))
		Expect(mockFactory.CreateCallCount()).To(Equal(0))
	}
```

Then these specs, with their frozen names:

**`It("publishes one escalation when task set empties a non-empty assignee")`**

```go
		err = setOp.Execute(ctx, vaultPath, taskName, "assignee", "", "", "", false)

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(writtenTask.Assignee()).To(Equal(""))

		command := recordedCommand()
		Expect(command.Type).To(Equal(notifcore.AgentEscalationNotificationType))
		Expect(command.Target).To(BeNil())
		Expect(command.Metadata).To(Equal(map[string]string{
			"taskIdentifier":   taskIdentifier,
			"taskName":         taskName,
			"previousAssignee": previousAssignee,
		}))
```

The `previousAssignee` reference is the point of the spec: `task set assignee ""` leaves the key present and empty, so an implementation that read the assignee after `SetField` would record `""` and fail here. Asserting the fixture variable (not a literal that a post-mutation read would also produce) is what makes this spec catch that.

**`It("publishes one escalation when task clear removes a non-empty assignee")`**

```go
		err = clearOp.Execute(ctx, vaultPath, taskName, "assignee")

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(writtenTask.Assignee()).To(Equal(""))

		command := recordedCommand()
		Expect(command.Type).To(Equal(notifcore.AgentEscalationNotificationType))
		Expect(command.Target).To(BeNil())
		Expect(command.Metadata).To(Equal(map[string]string{
			"taskIdentifier":   taskIdentifier,
			"taskName":         taskName,
			"previousAssignee": previousAssignee,
		}))
```

`task clear` deletes the key outright, so a post-mutation read yields `""` here too — the same fixture-variable assertion catches it.

**`It("publishes nothing when task set writes an empty assignee over an empty assignee")`**

```go
		task.ClearField("assignee")

		err = setOp.Execute(ctx, vaultPath, taskName, "assignee", "", "", "", false)

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		assertSilent()
```

**`It("publishes nothing when task clear removes an already-absent assignee")`**

```go
		task.ClearField("assignee")

		err = clearOp.Execute(ctx, vaultPath, taskName, "assignee")

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		assertSilent()
```

**`It("publishes nothing when task set replaces one assignee with another")`**

```go
		err = setOp.Execute(ctx, vaultPath, taskName, "assignee", "bob", "", "", false)

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		assertSilent()
```

**`It("publishes nothing when a different frontmatter key is set through task set")`**

```go
		err = setOp.Execute(ctx, vaultPath, taskName, "priority", "3", "", "", false)

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		assertSilent()
```

**`It("publishes nothing when a different frontmatter key is cleared through task clear")`**

```go
		err = clearOp.Execute(ctx, vaultPath, taskName, "status")

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		assertSilent()
```

**`It("publishes nothing when task set fails to write")`**

```go
		mockTaskStorage.WriteTaskReturns(errors.New("write failed"))

		err = setOp.Execute(ctx, vaultPath, taskName, "assignee", "", "", "", false)

		Expect(err).To(MatchError(ContainSubstring("write task")))
		assertSilent()
```

**`It("publishes nothing when task clear fails to write")`**

```go
		mockTaskStorage.WriteTaskReturns(errors.New("write failed"))

		err = clearOp.Execute(ctx, vaultPath, taskName, "assignee")

		Expect(err).To(MatchError(ContainSubstring("write task")))
		assertSilent()
```

**`It("returns success within the publish bound when the sender blocks past it")`**

```go
		blocked := make(chan struct{})
		DeferCleanup(func() {
			close(blocked)
		})
		mockSender.SendPublishNotificationCommandStub = func(
			context.Context, notifcmd.NotificationPublishCommand,
		) error {
			<-blocked
			return nil
		}

		start := time.Now()
		err = setOp.Execute(ctx, vaultPath, taskName, "assignee", "", "", "", false)
		elapsed := time.Since(start)

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(writtenTask.Assignee()).To(Equal(""))
		Expect(elapsed).To(BeNumerically(">=", ops.EscalationPublishTimeout))
		Expect(elapsed).To(BeNumerically("<", ops.EscalationPublishTimeout+3*time.Second))
```

This is Acceptance Criterion 5 through the operation: the write succeeded, the cleared assignee is in the written task, the command returned success, and it returned at the bound rather than waiting for the blocking sender.

## 6. `integration/cli_test.go` — the real binary

Add a helper next to `createTempVaultWithTopics`, following its shape exactly (`os.MkdirTemp` for the vault, `os.MkdirAll` for `Tasks`, `os.CreateTemp` for the config, a cleanup that removes both):

```go
// createTempVaultWithBrokers creates a temporary vault whose config carries a
// notification section naming the given brokers, and returns the vault path, the
// config path and a cleanup func.
func createTempVaultWithBrokers(
	brokers, topicPrefix string,
) (vaultPath string, configPath string, cleanup func()) {
```

Its config content is this YAML, rendered with `fmt.Sprintf`, interpolating the vault path into `path: %s`, the `brokers` argument into `brokers: "%s"` and the `topicPrefix` argument into `topic_prefix: "%s"`:

```yaml
default_vault: test
vaults:
  test:
    name: test
    path: <vaultPath>
    tasks_dir: Tasks
notification:
  brokers: "127.0.0.1:1"
  topic_prefix: "master"
```

Add `"time"` to the file's imports.

Then add this `Describe` block inside the outer `Describe("vault-cli integration tests", …)`, alongside the existing blocks:

```go
	Describe("vault-cli task assignee clear escalation", func() {
		It("task set assignee empty exits 0 and reports the failure when the broker is unreachable", func() {
			vaultPath, configPath, cleanup := createTempVaultWithBrokers("127.0.0.1:1", "master")
			defer cleanup()

			taskPath := filepath.Join(vaultPath, "Tasks", "Park Me.md")
			Expect(os.WriteFile(taskPath, []byte(
				"---\nstatus: in_progress\nassignee: alice\ntask_identifier: 0f6a3a0e-0000-4000-8000-000000000001\n---\n",
			), 0600)).To(Succeed())

			cmd := exec.Command(binPath, "--config", configPath, "task", "set", "Park Me", "assignee", "")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 30*time.Second).Should(gexec.Exit(0))

			Expect(string(session.Out.Contents())).To(Equal("✅ Set assignee= on: Park Me\n"))
			Expect(string(session.Err.Contents())).
				To(ContainSubstring("publish agent-escalation notification for task"))
			Expect(string(session.Err.Contents())).
				To(ContainSubstring("escalated by alice failed:"))

			content, err := os.ReadFile(taskPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).NotTo(ContainSubstring("alice"))
			Expect(string(content)).To(ContainSubstring("status: in_progress"))
		})

		It("task clear assignee exits 0 with unchanged output when no broker is configured", func() {
			vaultPath, configPath, cleanup := createTempVault(map[string]string{
				"Park Me": "---\nstatus: in_progress\nassignee: alice\n---\n",
			})
			defer cleanup()

			taskPath := filepath.Join(vaultPath, "Tasks", "Park Me.md")

			cmd := exec.Command(binPath, "--config", configPath, "task", "clear", "Park Me", "assignee")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 10*time.Second).Should(gexec.Exit(0))

			Expect(string(session.Out.Contents())).To(Equal("✅ Cleared assignee on: Park Me\n"))
			Expect(session.Err.Contents()).To(BeEmpty())

			content, err := os.ReadFile(taskPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).NotTo(ContainSubstring("assignee"))
			Expect(string(content)).To(ContainSubstring("status: in_progress"))
		})
	})
```

Rules for both specs:

- The 30-second `Eventually` timeout on the first spec is required: with brokers configured and unreachable, the command spends up to the publish bound before returning, so the default one-second matcher timeout would flake. The second spec may keep a shorter explicit timeout because an unconfigured CLI never opens a connection.
- Do NOT assert the exact YAML rendering of an emptied assignee. The first spec asserts the old value is gone and the frontmatter survived; the emitter's quoting of an empty string is not part of this change and pinning it would break on an unrelated emitter change.
- Do NOT start a broker, a Kafka container, or any network service. `127.0.0.1:1` is a reserved port nothing listens on, so the connection is refused immediately and the bound is what ends the attempt.
- Do NOT add these specs to the existing `createTempVault` / `createTempVaultWithGoals` configs — every other spec in the file depends on those staying notification-free.

## 7. Failure modes and security — what each spec carries

Map the spec's Failure Modes table onto this prompt and state the mapping in your completion report:

- **Broker unreachable** → the first integration spec (exit 0, unchanged stdout, the frozen failure line on stderr, the file still cleared) and the bound spec in 5c.
- **No broker configured** → the second integration spec (exit 0, unchanged stdout, empty stderr) plus every `assertSilent()` case in 5c, all of which assert zero connection attempts.
- **Task write fails** → the two write-failure specs in 5c; no publish is emitted and the command still returns the existing write error.
- **The same task is cleared twice** → the empty-to-empty specs in 5c: the second clear emits nothing.
- **Two concurrent clears of the same task** → both may publish; the extra notification is harmless and the task file is correct. No test, no lock, and no dedup logic — adding one would be untested surface the spec does not ask for.
- **Crash between the successful write and the publish** → not addressable by code; the spec records it as irreversible for that park, and the operator re-sets and re-clears the assignee.
- **The core's schema identifier moves, or the prefix is wrong** → prompt 1's topic spec; this prompt passes the operator's prefix through unchanged.
- **The core cannot route the published command** → the core's own behaviour; this prompt adds no routing.

Security properties to preserve, not to build:

- **The publish is gated on one specific transition.** `publishAssigneeClearEscalation` is the only path to the publisher, and it requires `key == "assignee"`, an empty new value, and a non-empty previous value — so an unrelated `task set` cannot emit an arbitrary notification. Do not widen that gate.
- **No credential.** No chat token, chat id, or per-channel secret is added; the routing decision stays in the shared core, and no target is ever set.
- **No task body content leaves the repository.** The notification carries the task identifier, the task name, and an assignee value — nothing else. Do not add the task body, the file path, or the vault path to the payload.

## 8. Self-check before finishing

- Re-read the changed hunks in `pkg/ops/frontmatter.go` and confirm: both constructors gain exactly one parameter, both structs gain exactly one field, `previousAssignee` is captured BEFORE the mutation on both paths, the publish call sits AFTER the successful write on both paths, and no existing error wording moved.
- Confirm `pkg/ops/escalation.go`, `pkg/config/config.go`, and the two frontmatter mocks are byte-identical to before this prompt.
- Walk spec 049's Acceptance Criteria 1, 2, 3, 5 and 8 and state in the completion report which requirement and which spec satisfies each. State explicitly that Acceptance Criterion 6 is Post-Deploy (Rung-2) and is not evidenced by this prompt. For Criterion 2, name explicitly which assertion per path would fail if the previous assignee were read after the mutation.
- Walk `docs/dod.md`: no `fmt.Print*` added under `pkg/ops/`, errors use `github.com/bborbe/errors`, tests use Ginkgo v2 / Gomega with counterfeiter mocks, and no version string moved.
- Confirm each check in `<verification>` passes by running it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 049 — non-goals.** Do NOT change the controller-side publish path, its clear sites, or its behaviour. Do NOT re-notify on later edits of an already-parked task — one notification per clear. Do NOT backfill notifications for tasks parked before this ships. Do NOT cover a raw editor write of the assignee field. Do NOT implement Matrix delivery. Do NOT make the publish mandatory: a deployment with no broker configured must keep today's behaviour exactly, with no connection attempt and no new failure mode.
- **Copied from spec 049 — constraints.** Both documented clear paths are covered — `task set … assignee ""` and `task clear … assignee` — because an operator following the README or `docs/task-writing.md` uses the second, and a spec that covered only the first would leave the documented route silent while claiming the Goal met. The two paths differ in write semantics — `set` leaves the key present and empty, `clear` deletes it — so the previous value must be captured BEFORE the mutation on both. A publish failure never fails the write: the command's exit code and stdout stay unchanged for callers that do not care about the publish. Opt-in is load-bearing. The emitted metadata keys are `taskIdentifier`, `taskName`, `previousAssignee`. A publish failure logs one line carrying the peer's own wording through the ops layer's logger, never stdout — this repository's ops layer writes no stdout.
- **Frozen literals.** The helper name `publishAssigneeClearEscalation`; the constructor parameter order `(taskStorage, publisher)`; the metadata keys `taskIdentifier`, `taskName`, `previousAssignee`; the CLI helper name `escalationPublisher`; the config field path `cfg.Notification.Brokers` / `cfg.Notification.TopicPrefix`; the test names `publishes one escalation when task set empties a non-empty assignee`, `publishes one escalation when task clear removes a non-empty assignee`, `publishes nothing when task set writes an empty assignee over an empty assignee`, `publishes nothing when task clear removes an already-absent assignee`, `publishes nothing when task set replaces one assignee with another`, `publishes nothing when a different frontmatter key is set through task set`, `publishes nothing when a different frontmatter key is cleared through task clear`, `publishes nothing when task set fails to write`, `publishes nothing when task clear fails to write`, `returns success within the publish bound when the sender blocks past it`, `task set assignee empty exits 0 and reports the failure when the broker is unreachable`, `task clear assignee exits 0 with unchanged output when no broker is configured`.
- **Every existing test must still pass unchanged.** In particular the existing `Context("setting assignee field")` and `Context("clearing assignee field")` keep their fixtures and assertions, and the `wikilink_roundtrip_test.go` table keeps its expectations.
- **No new flag, subcommand, config key, or output line.** The two commands' plain-text success lines and JSON result maps are byte-identical.
- **`pkg/ops/escalation.go` and `pkg/config/config.go` are read-only.** If you believe the transport needs a change, do not make it — say so in the completion report instead.
- **Error idiom.** `errors.Wrap(ctx, …)` / `errors.Errorf(ctx, …)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no `context.Background()` in `pkg/` (tests may use it).
- **Tests.** Ginkgo v2 + Gomega, counterfeiter mocks, external test packages (`ops_test`, `integration_test`) — no stdlib `t.Run` table tests. Every positive assertion on the asynchronous publish goes through `Eventually`; every "nothing was published" assertion goes through `Consistently`. Never pipe a test command.
- **No version bumps.** Leave `CHANGELOG.md`'s newest version heading and both `.claude-plugin/` JSON files untouched; the changelog bullet is prompt 3's.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code.

**Both clear paths reach the publish — the check spec 049's rationale actually asks for.** The notification method itself has one call site (inside the publisher, prompt 1), so the meaningful count is the shared helper's call sites, one per path:

```
test "$(grep -c 'publishAssigneeClearEscalation(ctx, o.publisher, task, key, value, previousAssignee)' pkg/ops/frontmatter.go)" = "1"
test "$(grep -c 'publishAssigneeClearEscalation(ctx, o.publisher, task, key, "", previousAssignee)' pkg/ops/frontmatter.go)" = "1"
test "$(grep -c 'publishAssigneeClearEscalation(' pkg/ops/frontmatter.go)" = "3"
test "$(grep -rc 'SendPublishNotificationCommand' --include='*.go' pkg/ops/ | awk -F: '{s+=$NF} END {print s}')" -ge 2
```

The third line is 3 because the helper's own definition line also matches; the first two pin one call site per path, written exactly as the requirement states.

**The previous assignee is captured before the mutation on both paths:**

```
test "$(grep -c 'previousAssignee := task.Assignee()' pkg/ops/frontmatter.go)" = "2"
test "$(grep -n 'previousAssignee := task.Assignee()' pkg/ops/frontmatter.go | head -1 | cut -d: -f1)" -lt "$(grep -n 'writeTaskCloseOutFieldsIfCloseOut(ctx, task, value, reason, gateSuccessor)' pkg/ops/frontmatter.go | head -1 | cut -d: -f1)"
test "$(grep -n 'previousAssignee := task.Assignee()' pkg/ops/frontmatter.go | tail -1 | cut -d: -f1)" -lt "$(grep -n 'task.ClearField(key)' pkg/ops/frontmatter.go | head -1 | cut -d: -f1)"
```

The first line proves the capture happens on BOTH paths (one occurrence per path). The second and third prove the set-path capture precedes the first mutation and the clear-path capture precedes `ClearField`.

**The transition rule is the only gate, and the publish follows the write:**

```
test "$(grep -c 'key != "assignee" || value != "" || previousAssignee == ""' pkg/ops/frontmatter.go)" = "1"
test "$(grep -n 'publishAssigneeClearEscalation(ctx, o.publisher, task, key, value, previousAssignee)' pkg/ops/frontmatter.go | cut -d: -f1)" -gt "$(grep -n 'return errors.Wrap(ctx, err, "write task")' pkg/ops/frontmatter.go | head -1 | cut -d: -f1)"
test "$(grep -n 'publishAssigneeClearEscalation(ctx, o.publisher, task, key, "", previousAssignee)' pkg/ops/frontmatter.go | cut -d: -f1)" -gt "$(grep -n 'return errors.Wrap(ctx, err, "write task")' pkg/ops/frontmatter.go | tail -1 | cut -d: -f1)"
```

**Both constructors take the publisher and both commands pass one:**

```
test "$(grep -c 'publisher EscalationPublisher,' pkg/ops/frontmatter.go)" = "2"
test "$(grep -c 'publisher:   publisher,' pkg/ops/frontmatter.go)" = "2"
test "$(grep -c 'ops.NewFrontmatterSetOperation(taskStore, publisher)' pkg/cli/cli.go)" = "1"
test "$(grep -c 'ops.NewFrontmatterClearOperation(taskStore, publisher)' pkg/cli/cli.go)" = "1"
test "$(grep -c 'ops.NewFrontmatterSetOperation(mockTaskStorage, publisher)' pkg/ops/frontmatter_test.go)" = "1"
test "$(grep -c 'ops.NewFrontmatterClearOperation(mockTaskStorage, publisher)' pkg/ops/frontmatter_test.go)" = "1"
test "$(grep -c 'func escalationPublisher(' pkg/cli/cli.go)" = "1"
test "$(grep -c 'cfg.Notification.Brokers' pkg/cli/cli.go)" = "1"
test "$(grep -c 'cfg.Notification.TopicPrefix' pkg/cli/cli.go)" = "1"
test "$(grep -c 'escalationPublisher(ctx, configLoader)' pkg/cli/cli.go)" = "2"
test "$(grep -c 'getVaults(ctx, configLoader, vaultName)' pkg/cli/cli.go)" = "29"
```

**No other write emits a command, and the commands' output is unchanged:**

```
test "$(grep -c 'publishAssigneeClearEscalation' pkg/ops/frontmatter.go)" = "4"
test "$(grep -c '✅ Set %s=%s on: %s' pkg/cli/cli.go)" = "1"
test "$(grep -c '✅ Cleared %s on: %s' pkg/cli/cli.go)" = "1"
```

**The ops specs run, and every assertion that carries an acceptance criterion is present in the source.** Capture the run's output, check its exit status separately from the name greps (a failing run still prints the names), and never pipe a test command:

```
go test ./pkg/ops/... -v -count=1 > /tmp/assignee-ops.log 2>&1; test "$?" = "0"
grep -F -q 'publishes one escalation when task set empties a non-empty assignee' /tmp/assignee-ops.log
grep -F -q 'publishes one escalation when task clear removes a non-empty assignee' /tmp/assignee-ops.log
grep -F -q 'publishes nothing when task set writes an empty assignee over an empty assignee' /tmp/assignee-ops.log
grep -F -q 'publishes nothing when task clear removes an already-absent assignee' /tmp/assignee-ops.log
grep -F -q 'publishes nothing when task set replaces one assignee with another' /tmp/assignee-ops.log
grep -F -q 'publishes nothing when a different frontmatter key is set through task set' /tmp/assignee-ops.log
grep -F -q 'publishes nothing when a different frontmatter key is cleared through task clear' /tmp/assignee-ops.log
grep -F -q 'publishes nothing when task set fails to write' /tmp/assignee-ops.log
grep -F -q 'publishes nothing when task clear fails to write' /tmp/assignee-ops.log
grep -F -q 'returns success within the publish bound when the sender blocks past it' /tmp/assignee-ops.log
grep -F -q 'setting assignee field' /tmp/assignee-ops.log
grep -F -q 'clearing assignee field' /tmp/assignee-ops.log
```

```
test "$(grep -c '"previousAssignee": previousAssignee,' pkg/ops/frontmatter_test.go)" = "2"
test "$(grep -c 'assertSilent()' pkg/ops/frontmatter_test.go)" = "7"
test "$(grep -c 'Consistently' pkg/ops/frontmatter_test.go)" -ge 1
test "$(grep -c 'ops.EscalationPublishTimeout' pkg/ops/frontmatter_test.go)" -ge 2
test "$(grep -c 'AgentEscalationNotificationType' pkg/ops/frontmatter_test.go)" = "2"
```

The `"previousAssignee": previousAssignee,` count of 2 is the guard for Acceptance Criterion 2: both paths must assert against the fixture variable, so a literal that a post-mutation read would also produce cannot satisfy them. The `assertSilent()` count of 7 is the guard for Acceptance Criterion 3: one call per silent case, so deleting a case fails this check.

**The integration specs run against the real binary:**

```
go test ./integration/... -v -count=1 > /tmp/assignee-integration.log 2>&1; test "$?" = "0"
grep -F -q 'task set assignee empty exits 0 and reports the failure when the broker is unreachable' /tmp/assignee-integration.log
grep -F -q 'task clear assignee exits 0 with unchanged output when no broker is configured' /tmp/assignee-integration.log
```

```
test "$(grep -c 'func createTempVaultWithBrokers(' integration/cli_test.go)" = "1"
test "$(grep -c 'escalated by alice failed:' integration/cli_test.go)" = "1"
test "$(grep -c 'gexec.Start' integration/cli_test.go)" -ge 2
test "$(grep -c '"time"' integration/cli_test.go)" = "1"
```

If `gexec.Build` fails with a VCS status error, `GOFLAGS=-buildvcs=false` is missing from the environment — the container sets it in `.dark-factory.yaml`; export it rather than touching `.git`.

**Formatting:**

```
test -z "$(gofmt -e -l pkg/ops/frontmatter.go pkg/ops/frontmatter_test.go pkg/ops/wikilink_roundtrip_test.go pkg/cli/cli.go integration/cli_test.go)"
```

Finally, walk spec 049's Acceptance Criteria 1, 2, 3, 5 and 8 against the change and state in your completion report which requirement and which spec satisfies each one, plus which evidence covers each row of the spec's Failure Modes table (section 7 above is the mapping). State explicitly that Acceptance Criterion 6 is Post-Deploy (Rung-2) and is not evidenced by this prompt.
</verification>

<!--
OPEN QUESTIONS FOR THE AUDITOR — not instructions for the executing agent.

1. Acceptance Criterion 5's wording ("returns success within 5 seconds") against a
   bound of exactly 5 seconds: the operation returns AT the bound, not strictly
   before it. Both the publisher spec (prompt 1, spec 4) and this prompt's bound
   spec therefore assert elapsed >= 5s AND elapsed < 8s. If "within 5 seconds" was
   meant strictly, either the bound or the criterion has to move.

2. Spec 049's Verification greps `grep -rc 'SendPublishNotificationCommand' pkg/ops/`
   and requires >= 2 with the rationale "both clear paths reach the publish; a
   single call site means one path is unwired". The notification method has exactly
   one call site (inside the publisher, added by prompt 1) plus the interface
   declaration, so that grep returns 2 — while the two clear paths reach it through
   the shared helper `publishAssigneeClearEscalation`. The verification above adds
   the per-path helper grep, which is the check the rationale asks for. If the
   intent was two direct call sites of the notification method, the design would
   have to duplicate the command construction and the bound in both paths.

3. The spec does not say whether a failed CONFIG LOAD should fail the command.
   This prompt fails it (`escalationPublisher` returns the wrapped error), matching
   how `getVaults` already treats a config read failure. If the intent was
   "a publish concern never fails a write", the helper should log and return a
   no-broker publisher instead.

4. The two integration specs use `127.0.0.1:1` as the unreachable broker. Port 1 is
   privileged and unbound, so the connection is refused immediately and the
   publish fails well inside the bound. If a container image ever listens on port
   1, swap the literal for another reserved port — the spec text names no address.

5. Spec 049 says the CHANGELOG bullet must name the assignee-clear publish; that
   bullet is prompt 3's, together with the README amendment and the
   `docs/task-writing.md` reconciliation. Prompt 2 deliberately changes no
   documentation file, so the tree is briefly in the state the spec's Constraints
   describe as the tension to reconcile.
-->

<!-- DARK-FACTORY-REPORT -->
