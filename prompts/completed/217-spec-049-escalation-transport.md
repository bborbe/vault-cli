---
status: completed
spec: [049-publish-escalation-on-assignee-clear]
summary: 'Added the opt-in, bounded escalation transport: a top-level notification{brokers,topic_prefix} config surface in pkg/config and pkg/ops/escalation.go with EscalationPublisher/NewEscalationPublisher, NotificationSenderFactory/NewKafkaNotificationSenderFactory and the agent-escalation command sender, mirroring bborbe/agent-task-controller, with 6 publisher specs (including the frozen master-core-notification-v1-request topic boundary spec) and 2 config specs.'
execution_id: vault-cli-exec-217-spec-049-escalation-transport
dark-factory-version: v0.193.0
created: "2026-09-17T13:20:00Z"
queued: "2026-09-17T15:50:49Z"
started: "2026-09-17T17:07:02Z"
completed: "2026-09-17T17:14:35Z"
---

# Escalation transport: optional broker config, Kafka publish sender, bounded publish

<summary>
- vault-cli gains a way to reach the shared notification core over Kafka, using the same transport the agent-side controller already publishes this notification with.
- The connection is opt-in. An operator who configures no brokers gets exactly today's behaviour: no publish, no connection attempt, no new failure mode.
- A publish can never hang a command. One attempt, bounded to five seconds; the command returns when the bound expires even if the broker never answers.
- A failed publish is logged in the peer's own wording and then swallowed — the command's exit code and standard output stay unchanged.
- The escalation carries the task's identifier, the task's name, and the assignee that let go of it, and sets no channel target — the routing decision stays in the shared core.
- No chat credential, chat id, or per-channel secret enters this repository.
- The command lands on the deployment's prefixed command topic, and a test pins that topic name against the schema the consumer actually reads.
- This prompt adds the transport and the configuration surface only. Nothing yet publishes an escalation — the two write paths are wired in the next prompt.
</summary>

<objective>
Give vault-cli a bounded, opt-in publisher that emits an `agent-escalation` notification through the shared notification core, mirroring the transport the working peer `bborbe/agent-task-controller` uses today — so that a later prompt can call it from the assignee-clear write paths without inventing its own transport. This is spec 049's prompt 1 of 3: it covers Desired Behaviors 3 and 4 and Acceptance Criteria 4 and 5.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then these files:

- `pkg/ops/frontmatter.go` — the two operations the next prompt changes. Read `frontmatterSetOperation` and `frontmatterClearOperation`. READ ONLY in this prompt: this prompt adds a new file beside them and does not touch them.
- `pkg/ops/ensure_task_identifiers.go` and `pkg/ops/update.go` — the ops-layer logging convention: `log/slog`, never `fmt.Print*`, never stdout. This repository's ops layer writes no stdout at all.
- `pkg/config/config.go` — the file you extend. Read the top-level `Config` struct (`CurrentUser`, `DefaultVault`, `Vaults`) and `configLoader.Load`. `Load` returns a `*Config` and is called by every command through `config.Loader`.
- `pkg/config/config_test.go` — the `Describe("Loader")` → `Describe("Load")` tree; the `Context("valid config file")` block shows the file's temp-config idiom (`os.WriteFile(configPath, …, 0600)` then `config.NewLoader(configPath)`).
- `mocks/mocks.go` and `mocks/frontmatter-set-operation.go` — the counterfeiter output convention: `mocks/mocks.go` is a bare `package mocks` file, every mock is generated from a `//counterfeiter:generate` directive, and `make generate` wipes and regenerates the whole directory.
- `Makefile` — `generate` (`rm -rf mocks avro` then `go generate -mod=mod ./...`), `ensure` (`go mod tidy -e`), `precommit` (`ensure format generate test check addlicense`), and the `VULNCHECK_IGNORE` / `vulncheck` / `trivy` targets.
- `docs/dod.md` — this repository's `validationPrompt`. Note two of its rules that shape this change: factory functions are pure composition with no conditionals and no I/O, and `pkg/ops/` never writes to stdout.
- `specs/in-progress/049-publish-escalation-on-assignee-clear.md` — the spec. Read its Constraints, Assumptions, Failure Modes and Non-goals sections; every requirement below comes from them.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-cqrs.md` — the cqrs command / schema / topic model this transport plugs into.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, …)` / `errors.Errorf(ctx, …)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err` for a new error, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — Interface → Constructor → Struct → Method, the shape every type below follows.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions and counterfeiter mock usage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — the linter limits this file must respect (funlen 80, gocognit 20, nestif 4, golines 100) and the license-header requirement.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.

## The peer you are mirroring, quoted verbatim

`bborbe/agent-task-controller` publishes this exact notification today. Its construction, from that repo's `main.go`:

```go
	// Publishes agent escalations into the shared notification core. No permission
	// gates this path — the core's IAM covers domain operations and channel sends,
	// not notification publishing — so the initiator name is for traceability only.
	// No Target is ever set: the deployed routing table owns the channel decision.
	notificationSender := notifcmd.NewNotificationPublishCommandSender(
		base.NewCommandCreator(base.RequestIDChannel(ctx)),
		cdb.NewCommandObjectSender(
			syncProducer,
			a.TopicPrefix,
			log.DefaultSamplerFactory,
		),
		cqrsiam.Initiator("agent-task-controller"),
	)
```

where `syncProducer` is `libkafka.NewSyncProducer(ctx, libkafka.ParseBrokersFromString(a.KafkaBrokers))`, `a.TopicPrefix` is the deployment's `TOPIC_PREFIX` (prod is `master`), and the imports are `base "github.com/bborbe/cqrs/base"`, `cdb "github.com/bborbe/cqrs/cdb"`, `cqrsiam "github.com/bborbe/cqrs/iam"`, `libkafka "github.com/bborbe/kafka"`, `log "github.com/bborbe/log"`, `notifcmd "github.com/bborbe/notification/command/notification"`.

Its publish and its failure line, from that repo's `pkg/result/result_writer.go`:

```go
	command := notifcmd.NotificationPublishCommand{
		Type:    notifcore.AgentEscalationNotificationType,
		Message: notifcore.NotificationMessage(message),
		Metadata: map[string]string{
			"taskIdentifier":   e.taskIdentifier,
			"taskName":         e.taskName,
			"previousAssignee": e.previousAssignee,
		},
	}
	if err := r.notificationSender.SendPublishNotificationCommand(ctx, command); err != nil {
		glog.Warningf(
			"publish agent-escalation notification for task %s (%s) escalated by %s failed: %v",
			e.taskIdentifier,
			e.taskName,
			e.previousAssignee,
			err,
		)
		return
	}
```

with `notifcore "github.com/bborbe/notification"`.

The signatures you call, read from the modules in the Go module cache (do not guess them):

```go
// github.com/bborbe/kafka
func ParseBrokersFromString(value string) Brokers
func NewSyncProducer(ctx context.Context, brokers Brokers, opts ...SaramaConfigOptions) (SyncProducer, error)
// SyncProducer interface: SendMessage(ctx, msg) (int32, int64, error); SendMessages(ctx, msgs) error; Close() error

// github.com/bborbe/cqrs/base
func NewCommandCreator(requestIDChan <-chan RequestID) CommandCreator
func RequestIDChannel(ctx context.Context) <-chan RequestID
type TopicPrefix string

// github.com/bborbe/cqrs/cdb
func NewCommandObjectSender(syncProducer libkafka.SyncProducer, prefix base.TopicPrefix, logSamplerFactory log.SamplerFactory) CommandObjectSender

// github.com/bborbe/cqrs/iam
type Initiator string // Validate only requires non-empty

// github.com/bborbe/notification/command/notification
type NotificationPublishCommandSender interface {
	SendPublishNotificationCommand(ctx context.Context, command NotificationPublishCommand) error
}
func NewNotificationPublishCommandSender(commandCreator base.CommandCreator, commandObjectSender cdb.CommandObjectSender, initiator cqrsiam.Initiator) NotificationPublishCommandSender
type NotificationPublishCommand struct {
	Type     core.NotificationType   `json:"type"`
	Target   *core.NotificationTarget `json:"target,omitempty"`
	Message  core.NotificationMessage `json:"message"`
	Metadata map[string]string        `json:"metadata,omitempty"`
}

// github.com/bborbe/notification
var AgentEscalationNotificationType NotificationType = "agent-escalation"
var NotificationV1SchemaID = cdb.SchemaID{Group: "core", Kind: "notification", Version: "v1"}

// github.com/bborbe/log
var DefaultSamplerFactory SamplerFactory
```

Two environment facts that shape this prompt:

1. **Make no git calls, anywhere — every check in `<verification>` is git-free.** The daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass. Use `grep`, `test`, and `go test` instead.
2. **`.dark-factory.yaml` sets `GOFLAGS=-buildvcs=false`.** Keep it — `go build` otherwise tries to read the masked `.git`.
</context>

<requirements>

## 0. Scope — two production files, two test files, one generated mock pair

You create or change exactly these files:

- `pkg/ops/escalation.go` — NEW: the escalation types, the sender factory, the bounded publisher.
- `pkg/config/config.go` — the optional notification configuration surface.
- `pkg/config/config_test.go` — one YAML-load spec proving the configuration is wired.
- `pkg/ops/escalation_test.go` — NEW: the publisher's specs.
- `mocks/notification-publish-command-sender.go` and `mocks/notification-sender-factory.go` — generated by `make generate` from the directives you add.
- `go.mod` / `go.sum` — the new module dependencies.
- `Makefile` (`VULNCHECK_IGNORE`) and/or `.trivyignore` — ONLY if a newly added module introduces a vulnerability finding with no available fix; see section 7.

Nothing else. `pkg/ops/frontmatter.go` is untouched, no operation is wired to the publisher yet, `pkg/cli/` is untouched, and no documentation file changes in this prompt.

## 1. `pkg/config/config.go` — the opt-in configuration surface

Add this type immediately after the `Config` struct's closing brace, and add the `Notification` field to `Config` as the last field:

```go
// Notification carries the optional escalation-publish settings. An empty
// Brokers value means vault-cli publishes nothing and opens no broker
// connection, which is what keeps the tool usable as a standalone local binary.
type Notification struct {
	Brokers     string `yaml:"brokers,omitempty"      json:"brokers,omitempty"`
	TopicPrefix string `yaml:"topic_prefix,omitempty" json:"topic_prefix,omitempty"`
}
```

```go
// Config represents the vault-cli configuration.
type Config struct {
	CurrentUser  string           `yaml:"current_user"`
	DefaultVault string           `yaml:"default_vault"`
	Vaults       map[string]Vault `yaml:"vaults"`
	Notification Notification     `yaml:"notification,omitempty"`
}
```

Non-negotiable properties:

- The YAML keys are exactly `notification`, `brokers`, and `topic_prefix`, all at the TOP level of the config file — not inside a vault. Brokers are a property of the deployment, not of a vault.
- `Brokers` is a plain `string` holding the operator's comma-separated broker list. Do NOT parse it here, do NOT split it, do NOT validate it, do NOT add a slice type.
- Do NOT add a default broker, a default prefix, an environment-variable fallback, a per-vault override, a validation method, or an accessor method. `Load` needs no change: the YAML decoder fills the field, and `expandVaultPaths` does not touch top-level fields.
- Do NOT add a `TopicsDir`-style accessor for these two keys, and do NOT touch `Vault`, `expandVaultPaths`, `GetVault`, `GetAllVaults`, `getDefaultConfig`, the `Loader` interface or its counterfeiter directive.
- `config list` must keep printing vaults only — do not add the notification settings to any command's output.

## 2. `pkg/ops/escalation.go` — the transport

Create the file with the standard BSD license header (`// Copyright (c) 2026 Benjamin Borbe All rights reserved.` plus the license line) and this content. Every name below is frozen; later prompts and the spec's verification greps depend on them.

```go
package ops

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/cqrs/cdb"
	cqrsiam "github.com/bborbe/cqrs/iam"
	"github.com/bborbe/errors"
	libkafka "github.com/bborbe/kafka"
	liblog "github.com/bborbe/log"
	notifcore "github.com/bborbe/notification"
	notifcmd "github.com/bborbe/notification/command/notification"
)

// EscalationPublishTimeout bounds a single publish attempt. A vault-cli
// invocation must never hang on an unreachable broker, so the publish is raced
// against this deadline and the command returns when it expires.
const EscalationPublishTimeout = 5 * time.Second

// escalationPublishFailureFormat is the frozen failure line, worded exactly as
// the working peer (bborbe/agent-task-controller) words it. The first
// placeholder is the task identifier and the parenthesised second is the task
// name.
const escalationPublishFailureFormat = "publish agent-escalation notification for task %s (%s) escalated by %s failed: %v"

// escalationInitiator names this repository in the command's initiator field.
// The field is traceability only: no permission gates notification publishing.
const escalationInitiator = "vault-cli"

// Escalation describes one assignee-clear transition to announce.
type Escalation struct {
	// TaskIdentifier is the task's task_identifier frontmatter value.
	TaskIdentifier string
	// TaskName is the task's filename without the .md extension.
	TaskName string
	// PreviousAssignee is the assignee value the task held before the clear.
	PreviousAssignee string
}

//counterfeiter:generate -o ../../mocks/notification-publish-command-sender.go --fake-name NotificationPublishCommandSender . NotificationPublishCommandSender

// NotificationPublishCommandSender is the subset of the shared notification
// core's command sender this repository uses. It mirrors the interface
// github.com/bborbe/notification/command/notification declares, so this layer
// depends on an interface it owns rather than on the client type directly.
type NotificationPublishCommandSender interface {
	SendPublishNotificationCommand(ctx context.Context, command notifcmd.NotificationPublishCommand) error
}

//counterfeiter:generate -o ../../mocks/notification-sender-factory.go --fake-name NotificationSenderFactory . NotificationSenderFactory

// NotificationSenderFactory builds the command sender for a broker
// configuration. Calling Create is what opens the broker connection, so a test
// that counts Create calls observes connection attempts directly.
type NotificationSenderFactory interface {
	Create(ctx context.Context, brokers, topicPrefix string) (NotificationPublishCommandSender, error)
}

// NewKafkaNotificationSenderFactory returns the production factory. It mirrors
// the working peer bborbe/agent-task-controller: a sync producer from
// github.com/bborbe/kafka, the shared core's publish sender on top of it, and
// the deployment's topic prefix, which the consumer derives from its branch.
func NewKafkaNotificationSenderFactory() NotificationSenderFactory {
	return &kafkaNotificationSenderFactory{}
}

type kafkaNotificationSenderFactory struct{}

// Create builds a sync producer and the shared core's publish sender. The
// producer is closed once the command has been sent: a vault-cli invocation
// publishes at most once and then exits.
func (f *kafkaNotificationSenderFactory) Create(
	ctx context.Context,
	brokers, topicPrefix string,
) (NotificationPublishCommandSender, error) {
	syncProducer, err := libkafka.NewSyncProducer(ctx, libkafka.ParseBrokersFromString(brokers))
	if err != nil {
		return nil, errors.Wrap(ctx, err, "create kafka sync producer")
	}
	return &closingNotificationPublishCommandSender{
		sender: notifcmd.NewNotificationPublishCommandSender(
			base.NewCommandCreator(base.RequestIDChannel(ctx)),
			cdb.NewCommandObjectSender(
				syncProducer,
				base.TopicPrefix(topicPrefix),
				liblog.DefaultSamplerFactory,
			),
			cqrsiam.Initiator(escalationInitiator),
		),
		closer: syncProducer,
	}, nil
}

// closingNotificationPublishCommandSender closes the producer once the command
// has been sent. The sync producer returns after the broker acknowledgement, so
// closing afterwards cannot drop the message.
type closingNotificationPublishCommandSender struct {
	sender NotificationPublishCommandSender
	closer io.Closer
}

func (c *closingNotificationPublishCommandSender) SendPublishNotificationCommand(
	ctx context.Context,
	command notifcmd.NotificationPublishCommand,
) error {
	defer func() {
		_ = c.closer.Close()
	}()
	return c.sender.SendPublishNotificationCommand(ctx, command)
}

// EscalationPublisher announces an assignee-clear transition. It returns no
// error on purpose: the notification is a side effect, and a failed publish
// must never fail the write that triggered it.
type EscalationPublisher interface {
	PublishEscalation(ctx context.Context, escalation Escalation)
}

// NewEscalationPublisher creates the publisher the frontmatter write paths use.
// brokers is the operator's comma-separated broker list and topicPrefix the
// deployment's topic prefix; an empty brokers value yields a publisher that
// publishes nothing and opens no connection.
func NewEscalationPublisher(
	brokers, topicPrefix string,
	factory NotificationSenderFactory,
) EscalationPublisher {
	return &escalationPublisher{
		brokers:     brokers,
		topicPrefix: topicPrefix,
		factory:     factory,
	}
}

type escalationPublisher struct {
	brokers     string
	topicPrefix string
	factory     NotificationSenderFactory
}

// PublishEscalation emits one agent-escalation notification and returns without
// waiting longer than EscalationPublishTimeout. A failure, a timeout, or an
// unreachable broker is logged and swallowed: the caller's write already
// succeeded and its exit code and stdout must stay unchanged.
func (p *escalationPublisher) PublishEscalation(ctx context.Context, escalation Escalation) {
	if p.brokers == "" {
		// Opt-in: with no broker configured the CLI publishes nothing and opens
		// no connection, so a standalone vault-cli behaves exactly as before.
		return
	}
	done := make(chan error, 1)
	go func() {
		done <- p.publish(ctx, escalation)
	}()
	select {
	case err := <-done:
		if err != nil {
			logEscalationPublishFailure(escalation, err)
		}
	case <-time.After(EscalationPublishTimeout):
		logEscalationPublishFailure(
			escalation,
			errors.Errorf(ctx, "publish timed out after %s", EscalationPublishTimeout),
		)
	}
}

// publish builds the sender and sends exactly one command. No Target is ever
// set: the core's deployed routing table owns the channel decision, which is why
// this repository carries no chat credential.
func (p *escalationPublisher) publish(ctx context.Context, escalation Escalation) error {
	sender, err := p.factory.Create(ctx, p.brokers, p.topicPrefix)
	if err != nil {
		return errors.Wrap(ctx, err, "create notification sender")
	}
	return sender.SendPublishNotificationCommand(ctx, notifcmd.NotificationPublishCommand{
		Type:    notifcore.AgentEscalationNotificationType,
		Message: notifcore.NotificationMessage(escalationMessage(escalation)),
		Metadata: map[string]string{
			"taskIdentifier":   escalation.TaskIdentifier,
			"taskName":         escalation.TaskName,
			"previousAssignee": escalation.PreviousAssignee,
		},
	})
}

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

// logEscalationPublishFailure writes the frozen failure line through this
// package's logger. The ops layer never writes to stdout: the CLI layer owns
// output, and a failed publish must not change what a caller sees.
func logEscalationPublishFailure(escalation Escalation, err error) {
	slog.Warn(
		"escalation publish failed",
		"warning", fmt.Sprintf(
			escalationPublishFailureFormat,
			escalation.TaskIdentifier,
			escalation.TaskName,
			escalation.PreviousAssignee,
			err,
		),
		"taskIdentifier", escalation.TaskIdentifier,
		"taskName", escalation.TaskName,
		"previousAssignee", escalation.PreviousAssignee,
		"error", err,
	)
}
```

Properties that are not negotiable:

- **The opt-in gate lives in `PublishEscalation`, not in the constructor.** `NewEscalationPublisher` stays pure composition — no conditionals, no I/O — because `docs/dod.md` requires it. The `if p.brokers == ""` guard is what makes an unconfigured CLI perform no publish and open no connection.
- **The bound is the `select`.** Do NOT rely on the sender honouring `ctx`: `libkafka`'s sync producer calls Sarama's `SendMessage`, which ignores `ctx` entirely. The goroutine plus `time.After` is what guarantees the caller returns within `EscalationPublishTimeout` even against a sender that never returns. The channel is buffered (`make(chan error, 1)`) so an abandoned goroutine can still send and exit.
- **Exactly one publish attempt.** No retry, no loop, no backoff, no second attempt after a timeout.
- **No Target field is ever set.** Do NOT add a target parameter, a channel name, a chat id, or a target constant.
- **`PublishEscalation` returns nothing.** Do NOT give it an `error` return, do NOT propagate a publish failure to the caller, and do NOT make the constructor return an error.
- **Metadata keys are exactly** `taskIdentifier`, `taskName`, `previousAssignee` — matching the peer, byte for byte.
- **The notification type is `notifcore.AgentEscalationNotificationType`.** Do NOT declare a local notification-type constant and do NOT write the literal `"agent-escalation"` yourself.
- **The prefix is passed through, never dropped.** `p.topicPrefix` goes into `base.TopicPrefix(topicPrefix)` unchanged; an empty prefix is legal (the topic is then unprefixed) but the deployment's prod prefix is `master`, so the value must not be normalised, defaulted, or trimmed.
- **No stdout.** `slog` only. Do NOT import `fmt` for anything but `Sprintf` into the logger and the message, and do NOT use `fmt.Print*`, `os.Stdout`, or `println` anywhere in this file.

## 3. Add the module dependencies in the right order

The imports in section 2 come first, then the dependency resolution:

1. Write `pkg/ops/escalation.go` and `pkg/ops/escalation_test.go` (section 4) with their imports in place.
2. Run `make ensure` (it runs `go mod tidy -e`). Running `go mod tidy` BEFORE the importing files exist would silently leave `go.mod` unchanged, because nothing imports the new modules yet.
3. Expect these modules to appear as direct requirements: `github.com/bborbe/kafka`, `github.com/bborbe/notification`, `github.com/bborbe/cqrs`, and `github.com/bborbe/log`. The peer `bborbe/agent-task-controller` uses `kafka v1.25.16`, `notification v0.6.1`, `cqrs v0.6.11`; if a newer version resolves cleanly, that is fine.
4. Do NOT add an `exclude` or `replace` directive to `go.mod` — `docs/dod.md` requires that `go install github.com/bborbe/vault-cli@latest` keeps working.
5. Do NOT run `go mod vendor`, do NOT write `-mod=vendor` anywhere, and do NOT commit a `vendor/` directory.

## 4. `pkg/ops/escalation_test.go` — the publisher's specs

Create the file in `package ops_test` with the BSD license header and these imports:

```go
import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/cqrs/cdb"
	cqrsiam "github.com/bborbe/cqrs/iam"
	kafkamocks "github.com/bborbe/kafka/mocks"
	liblog "github.com/bborbe/log"
	notifcore "github.com/bborbe/notification"
	notifcmd "github.com/bborbe/notification/command/notification"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/ops"
)
```

`kafkamocks` is aliased because the package is named `mocks` and would otherwise collide with this repository's own `mocks` package.

Scaffolding:

```go
var _ = Describe("EscalationPublisher", func() {
	var (
		ctx         context.Context
		mockSender  *mocks.NotificationPublishCommandSender
		mockFactory *mocks.NotificationSenderFactory
		publisher   ops.EscalationPublisher
		escalation  ops.Escalation
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockSender = &mocks.NotificationPublishCommandSender{}
		mockSender.SendPublishNotificationCommandReturns(nil)
		mockFactory = &mocks.NotificationSenderFactory{}
		mockFactory.CreateReturns(mockSender, nil)
		escalation = ops.Escalation{
			TaskIdentifier:   "0f6a3a0e-0000-4000-8000-000000000001",
			TaskName:         "Park the Escalation Task",
			PreviousAssignee: "alice",
		}
		publisher = ops.NewEscalationPublisher("broker-1:9092", "master", mockFactory)
	})
```

Every assertion on an asynchronous effect MUST go through `Eventually`, because `PublishEscalation` performs the publish in a goroutine. A bare `Expect(mockSender.SendPublishNotificationCommandCallCount()).To(Equal(1))` immediately after the call is a race and will flake.

The five specs, with their frozen names and their required assertions:

**Spec 1 — `It("publishes exactly one agent-escalation command with no target")`**

```go
		publisher.PublishEscalation(ctx, escalation)

		Eventually(func() int {
			return mockSender.SendPublishNotificationCommandCallCount()
		}).Should(Equal(1))
		_, command := mockSender.SendPublishNotificationCommandArgsForCall(0)
		Expect(command.Type).To(Equal(notifcore.AgentEscalationNotificationType))
		Expect(command.Target).To(BeNil())
		Expect(command.Message).NotTo(BeEmpty())
		Expect(command.Metadata).To(Equal(map[string]string{
			"taskIdentifier":   escalation.TaskIdentifier,
			"taskName":         escalation.TaskName,
			"previousAssignee": escalation.PreviousAssignee,
		}))
```

The metadata assertion is an exact map comparison on purpose: it fails if a fourth key is added or a key is renamed.

**Spec 2 — `It("passes the configured brokers and topic prefix to the sender factory")`**

```go
		publisher.PublishEscalation(ctx, escalation)

		Eventually(func() int {
			return mockFactory.CreateCallCount()
		}).Should(Equal(1))
		_, brokers, topicPrefix := mockFactory.CreateArgsForCall(0)
		Expect(brokers).To(Equal("broker-1:9092"))
		Expect(topicPrefix).To(Equal("master"))
```

This is the guard against the "published with the wrong topic prefix" failure mode: a dropped or normalised prefix publishes into a topic nothing consumes.

**Spec 3 — `It("publishes nothing and attempts no connection when no broker is configured")`**

```go
		publisher = ops.NewEscalationPublisher("", "", mockFactory)

		publisher.PublishEscalation(ctx, escalation)

		Consistently(func() int {
			return mockFactory.CreateCallCount()
		}, "200ms", "50ms").Should(Equal(0))
		Expect(mockSender.SendPublishNotificationCommandCallCount()).To(Equal(0))
```

This is Acceptance Criterion 4's "zero recorded commands and zero connection attempts", with `Create` as the connection attempt.

**Spec 4 — `It("returns within the publish bound when the sender blocks past it")`**

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
		publisher.PublishEscalation(ctx, escalation)
		elapsed := time.Since(start)

		Expect(elapsed).To(BeNumerically(">=", ops.EscalationPublishTimeout))
		Expect(elapsed).To(BeNumerically("<", ops.EscalationPublishTimeout+3*time.Second))
```

The stub blocks until `DeferCleanup` closes the channel, so the sender never returns on its own: the only thing that can end the call is the bound. The lower bound proves the bound was actually waited out rather than the sender being ignored; the upper bound is the scheduling slack. This is Acceptance Criterion 5's bound test.

**Spec 5 — `It("logs the frozen failure line when the publish fails")`**

```go
		var buf bytes.Buffer
		original := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))
		DeferCleanup(func() {
			slog.SetDefault(original)
		})
		mockSender.SendPublishNotificationCommandReturns(errors.New("broker refused"))

		publisher.PublishEscalation(ctx, escalation)

		Eventually(func() string {
			return buf.String()
		}).Should(ContainSubstring("escalated by alice failed:"))
		Expect(buf.String()).To(ContainSubstring(escalation.TaskIdentifier))
		Expect(buf.String()).To(ContainSubstring("publish agent-escalation notification for task"))
```

`DeferCleanup` restoring the previous default logger is required: `slog.SetDefault` is process-global and would leak into sibling specs.

**Spec 6 — `It("publishes onto the deployment's prefixed command topic")`** — the boundary spec

This one drives the REAL notification sender and the REAL cqrs command-object sender, with only the Kafka producer faked. It is the test that catches a moved schema identifier or a dropped prefix, both of which publish into a topic nothing consumes:

```go
		producer := &kafkamocks.KafkaSyncProducer{}
		producer.SendMessageReturns(0, 0, nil)
		mockFactory.CreateStub = func(
			_ context.Context, _, topicPrefix string,
		) (ops.NotificationPublishCommandSender, error) {
			return notifcmd.NewNotificationPublishCommandSender(
				base.NewCommandCreator(base.RequestIDChannel(ctx)),
				cdb.NewCommandObjectSender(
					producer,
					base.TopicPrefix(topicPrefix),
					liblog.DefaultSamplerFactory,
				),
				cqrsiam.Initiator("vault-cli-test"),
			), nil
		}

		publisher.PublishEscalation(ctx, escalation)

		Eventually(func() int {
			return producer.SendMessageCallCount()
		}).Should(Equal(1))
		_, message := producer.SendMessageArgsForCall(0)
		Expect(message.Topic).To(Equal("master-core-notification-v1-request"))
```

- The expected topic literal `master-core-notification-v1-request` is frozen. It is the deployment's prod command topic: the prefix, joined to the schema-derived name `core-notification-v1-request`.
- A single `SendMessage` call is itself the proof that the command validated and serialized: the cqrs sender validates the command object (non-empty request id, an operation matching the cqrs operation pattern, a non-empty initiator) and JSON-marshals the payload before it ever reaches the producer. If any of that fails, no message is sent and this spec fails on the call count.
- Do NOT add a second spec asserting the unprefixed topic, and do NOT import `github.com/IBM/sarama` — the `Topic` field is a plain string and needs no sarama type.

## 5. `pkg/config/config_test.go` — the configuration is wired

Add one `Context` inside `Describe("Loader")` → `Describe("Load")`, directly after the existing `Context("topics_dir in vault config", …)` block:

```go
		Context("notification in config", func() {
			BeforeEach(func() {
				configData := `default_vault: main
vaults:
  main:
    name: main
    path: /vault/main
notification:
  brokers: "broker-1:9092,broker-2:9092"
  topic_prefix: "master"
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("loads the notification broker settings", func() {
				cfg, err := loader.Load(ctx)
				Expect(err).To(BeNil())
				Expect(cfg.Notification.Brokers).To(Equal("broker-1:9092,broker-2:9092"))
				Expect(cfg.Notification.TopicPrefix).To(Equal("master"))
			})

			It("leaves the notification settings empty when the section is absent", func() {
				configData := `default_vault: main
vaults:
  main:
    name: main
    path: /vault/main
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)

				cfg, err := loader.Load(ctx)
				Expect(err).To(BeNil())
				Expect(cfg.Notification.Brokers).To(BeEmpty())
				Expect(cfg.Notification.TopicPrefix).To(BeEmpty())
			})
		})
```

The spec names `loads the notification broker settings` and `leaves the notification settings empty when the section is absent` are frozen. Asserting the raw struct fields is mandatory — it is the only assertion that proves the YAML tags are wired; a `Brokers` value read through any accessor would be untested surface this change does not have.

## 6. Generate the mocks

After adding the two `//counterfeiter:generate` directives, run `make generate`. It wipes `mocks/`, rewrites `mocks/mocks.go`, and regenerates every mock from the directives in the tree, so `mocks/notification-publish-command-sender.go` and `mocks/notification-sender-factory.go` appear. Do NOT hand-write either file, and do NOT edit `mocks/mocks.go`.

## 7. Dependency hygiene — vulnerability and license checks

`make precommit` runs `make check`, which includes `vulncheck`, `osv-scanner` and `trivy` against the whole module graph. Two new modules pull in a large transitive set, so:

- If a check reports a finding introduced by a newly added module AND no fixed version is available for that module, add the advisory id to the existing ignore list — `VULNCHECK_IGNORE` in the `Makefile` for `govulncheck` findings, `.trivyignore` for trivy findings — following the file's existing style, with a one-line comment naming the module and stating that no fix is available.
- If a fixed version IS available, upgrade the module instead of ignoring the finding. Never silence a finding that has a fix.
- State in your completion report exactly which advisories (if any) you ignored and why. If you ignored none, say so.

## 8. Failure modes and security — what each spec carries

Map the spec's Failure Modes table onto this prompt and state the mapping in your completion report:

- **Broker unreachable** → spec 4 (the bound) and spec 5 (the frozen failure line); the publish fails within the bound and the caller is unaffected.
- **The core's schema identifier moves** → spec 6 (the topic literal). A moved schema id changes the topic and fails that spec.
- **Published with the wrong topic prefix** → spec 2 (the prefix reaches the factory unmodified) and spec 6 (the prefixed topic literal).
- **No broker configured** → spec 3 (`Consistently` zero `Create` calls, zero recorded commands). This is the load-bearing backward-compatibility row.
- **The same task is cleared twice** → not this prompt's concern; the transition rule that makes the second clear silent is prompt 2's.
- **Crash between the successful write and the publish** → not addressable here; the spec records it as irreversible for that park.
- **The core cannot route the published command** → the core's own behaviour; the CLI's part is done, and this prompt adds no routing.

Security properties to preserve, not to build:

- **No credential.** The publisher uses the broker connection the operator configured; no chat token, chat id, or per-channel secret is added, and none may be added. The verification greps the tree for chat-specific identifiers and must find none.
- **No arbitrary notification surface.** This prompt exposes exactly one notification type and one fixed metadata shape; there is no method that publishes an operator-supplied payload.
- **The gate is operator-owned.** Whether the CLI ever opens a broker connection is decided by the operator's own config file, so the standalone default is preserved rather than assumed.
- **The metadata carries agent names only.** `previousAssignee` is an assignee value; no task body content, no file content, and no path is included.

## 9. Self-check before finishing

- Re-read `pkg/ops/escalation.go` and confirm: the opt-in guard is inside `PublishEscalation`, `NewEscalationPublisher` contains no conditional and no I/O, the `select` carries the bound, the metadata keys are the three frozen ones, and no `Target` is ever set.
- Confirm `pkg/ops/frontmatter.go` and everything under `pkg/cli/` are byte-identical to before this prompt.
- Walk spec 049's Acceptance Criteria 4 and 5 and state in the completion report which spec satisfies each.
- Walk `docs/dod.md`: every exported type, function, and interface has a doc comment, errors use `github.com/bborbe/errors`, nothing under `pkg/ops/` writes to stdout, factories are pure composition, tests use Ginkgo v2 / Gomega with counterfeiter mocks, and `go.mod` has no `exclude` or `replace` directive.
- Confirm each check in `<verification>` passes by running it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 049 — non-goals.** Do NOT change the controller-side publish path or its clear sites. Do NOT re-notify on later edits of an already-parked task. Do NOT backfill notifications for tasks parked before this ships. Do NOT cover a raw editor write of the assignee field. Do NOT implement Matrix delivery. Do NOT make the publish mandatory — a deployment with no broker configured must keep today's behaviour exactly, with no connection attempt and no new failure mode.
- **Copied from spec 049 — constraints.** Opt-in is load-bearing: no broker config means no publish, no connection attempt, behaviour byte-identical to today. The publish is bounded: a single attempt, a 5-second timeout, and a publish failure never fails the write and never changes the command's exit code or stdout. No chat credentials: no chat token, chat id, or per-channel secret enters this repository. Mirror the working peer's transport (`bborbe/agent-task-controller`) rather than inventing one. The topic prefix is required for the deployed topology even though the peer's flag parser marks it optional — the consumer always derives its prefix from its branch, so a producer that omits it publishes into a topic nothing consumes. The emitted metadata keys are `taskIdentifier`, `taskName`, `previousAssignee`. A publish failure logs one line carrying the peer's own wording through the ops layer's logger, never stdout.
- **Frozen literals.** `EscalationPublishTimeout` = `5 * time.Second`; the failure format string `publish agent-escalation notification for task %s (%s) escalated by %s failed: %v`; the metadata keys `taskIdentifier`, `taskName`, `previousAssignee`; the topic `master-core-notification-v1-request`; the initiator `vault-cli`; the YAML keys `notification`, `brokers`, `topic_prefix`; the config struct field `Notification`; the test names `publishes exactly one agent-escalation command with no target`, `passes the configured brokers and topic prefix to the sender factory`, `publishes nothing and attempts no connection when no broker is configured`, `returns within the publish bound when the sender blocks past it`, `logs the frozen failure line when the publish fails`, `publishes onto the deployment's prefixed command topic`, `loads the notification broker settings`, `leaves the notification settings empty when the section is absent`. All of these are grep targets.
- **Interface names.** `EscalationPublisher`, `NotificationPublishCommandSender`, `NotificationSenderFactory`, `Escalation`, and `NewKafkaNotificationSenderFactory` are the names later prompts and the spec's verification use. Do NOT rename them, do NOT collapse them into one type, and do NOT change `PublishEscalation`'s signature.
- **`pkg/ops/frontmatter.go` is read-only in this prompt.** No operation is wired to the publisher here; that is prompt 2.
- **`pkg/cli/` is read-only in this prompt.** No command changes; no new flag, subcommand, or output line.
- **No stdout from `pkg/ops/`.** Log through `log/slog`; never `fmt.Print*`, `os.Stdout`, or `println`.
- **Error idiom.** `errors.Wrap(ctx, …)` / `errors.Errorf(ctx, …)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err` for a newly constructed error; no `context.Background()` in `pkg/` (tests may use it).
- **Tests.** Ginkgo v2 + Gomega in `package ops_test` / `package config_test`, counterfeiter mocks for the two new interfaces, no stdlib `t.Run` table tests. Every Go file keeps its BSD license header (`make addlicense` adds it for new files; run it rather than hand-writing the header on the generated mocks). Existing tests must still pass unchanged.
- **No version bumps.** Leave `CHANGELOG.md`'s newest version heading and both `.claude-plugin/` JSON files untouched; the changelog bullet is prompt 3's.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite, `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make vulncheck`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code.

**The transport exists, is bounded, and is opt-in:**

```
test "$(grep -c 'EscalationPublishTimeout = 5 \* time.Second' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'escalationPublishFailureFormat = "publish agent-escalation notification for task %s (%s) escalated by %s failed: %v"' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'AgentEscalationNotificationType' pkg/ops/escalation.go)" = "1"
test "$(grep -c '"taskIdentifier":' pkg/ops/escalation.go)" = "1"
test "$(grep -c '"taskName":' pkg/ops/escalation.go)" = "1"
test "$(grep -c '"previousAssignee":' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'if p.brokers == ""' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'case <-time.After(EscalationPublishTimeout):' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'make(chan error, 1)' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'NewEscalationPublisher' pkg/ops/escalation.go)" -ge 2
test "$(grep -c 'ParseBrokersFromString' pkg/ops/escalation.go)" = "1"
test "$(grep -c 'base.TopicPrefix(topicPrefix)' pkg/ops/escalation.go)" = "1"
```

**The ops layer still writes no stdout, and no chat-specific code entered the repository** (spec 049's own container-executable checks):

```
test "$(grep -c 'fmt.Print\|os.Stdout\|println(' pkg/ops/escalation.go)" = "0"
test "$(grep -rniE 'telegram|chat_id|chat_token' --include='*.go' pkg/ | grep -v _test | wc -l | tr -d ' ')" = "0"
```

**`SendPublishNotificationCommand` appears at least twice inside `pkg/ops/`** — the interface declaration and the single call site in the publisher:

```
test "$(grep -rc 'SendPublishNotificationCommand' --include='*.go' pkg/ops/ | awk -F: '{s+=$NF} END {print s}')" -ge 2
```

**The config surface is wired and defaults to off:**

```
test "$(grep -c 'Notification Notification' pkg/config/config.go)" = "1"
test "$(grep -c 'type Notification struct' pkg/config/config.go)" = "1"
test "$(grep -c 'yaml:"brokers,omitempty"' pkg/config/config.go)" = "1"
test "$(grep -c 'yaml:"topic_prefix,omitempty"' pkg/config/config.go)" = "1"
test "$(grep -c 'yaml:"notification,omitempty"' pkg/config/config.go)" = "1"
test "$(grep -c 'func (c \*configLoader) getDefaultConfig' pkg/config/config.go)" = "1"
grep -F -q 'Expect(cfg.Notification.Brokers).To(Equal("broker-1:9092,broker-2:9092"))' pkg/config/config_test.go
grep -F -q 'Expect(cfg.Notification.Brokers).To(BeEmpty())' pkg/config/config_test.go
```

**The mocks were generated, not hand-written:**

```
test -f mocks/notification-publish-command-sender.go
test -f mocks/notification-sender-factory.go
grep -F -q 'Code generated by counterfeiter' mocks/notification-publish-command-sender.go
grep -F -q 'Code generated by counterfeiter' mocks/notification-sender-factory.go
```

**The publisher's specs run, and every assertion that carries an acceptance criterion is present in the source.** Capture the run's output, check its exit status separately from the name greps (a failing run still prints the names), and never pipe a test command:

```
go test ./pkg/ops/... -v -count=1 -args -ginkgo.v > /tmp/escalation-ops.log 2>&1; test "$?" = "0"
grep -F -q 'publishes exactly one agent-escalation command with no target' /tmp/escalation-ops.log
grep -F -q 'passes the configured brokers and topic prefix to the sender factory' /tmp/escalation-ops.log
grep -F -q 'publishes nothing and attempts no connection when no broker is configured' /tmp/escalation-ops.log
grep -F -q 'returns within the publish bound when the sender blocks past it' /tmp/escalation-ops.log
grep -F -q 'logs the frozen failure line when the publish fails' /tmp/escalation-ops.log
grep -F -q 'publishes onto the deployment's prefixed command topic' /tmp/escalation-ops.log
```

```
test "$(grep -c 'master-core-notification-v1-request' pkg/ops/escalation_test.go)" = "1"
test "$(grep -c 'Consistently' pkg/ops/escalation_test.go)" -ge 1
test "$(grep -c 'ops.EscalationPublishTimeout' pkg/ops/escalation_test.go)" -ge 2
test "$(grep -c 'Eventually' pkg/ops/escalation_test.go)" -ge 4
test "$(grep -c 'kafkamocks.KafkaSyncProducer' pkg/ops/escalation_test.go)" -ge 1
test "$(grep -c 'IBM/sarama' pkg/ops/escalation_test.go)" = "0"
```

**The config spec runs:**

```
go test ./pkg/config/... -v -count=1 -args -ginkgo.v > /tmp/escalation-config.log 2>&1; test "$?" = "0"
grep -F -q 'loads the notification broker settings' /tmp/escalation-config.log
grep -F -q 'leaves the notification settings empty when the section is absent' /tmp/escalation-config.log
```

**The dependency graph is what the spec expects, and nothing was vendored:**

```
test "$(grep -c 'github.com/bborbe/kafka v' go.mod)" = "1"
test "$(grep -c 'github.com/bborbe/notification v' go.mod)" = "1"
test "$(grep -c 'github.com/bborbe/cqrs v' go.mod)" = "1"
test "$(grep -c '^replace ' go.mod)" = "0"
test "$(grep -c '^exclude ' go.mod)" = "0"
test ! -d vendor
go build ./...
```

**Formatting:**

```
test -z "$(gofmt -e -l pkg/ops/escalation.go pkg/ops/escalation_test.go pkg/config/config.go pkg/config/config_test.go)"
```

Finally, walk spec 049's Acceptance Criteria 4 and 5 against the change and state in your completion report which requirement and which spec satisfies each one, plus which evidence covers each row of the spec's Failure Modes table (section 8 above is the mapping).
</verification>

<!--
OPEN QUESTIONS FOR THE AUDITOR — not instructions for the executing agent.

1. Failure-line placeholder order. The spec writes the frozen string as
   "for task <name> (<id>) escalated by <assignee> failed: <err>", but the peer's
   own code passes (taskIdentifier, taskName) into that format string, so the peer
   renders "for task <id> (<name>)". This prompt mirrors the peer's argument order
   ("the peer's own wording", per the spec's Constraints) rather than the spec's
   placeholder labels. If the labels were deliberate, the argument order in
   section 2 must be swapped.

2. Acceptance Criterion 5 says the operation "returns success within 5 seconds"
   while the bound is exactly 5 seconds, so a blocking sender makes the call
   return AT the bound, not strictly before it. Spec 4 therefore asserts
   elapsed >= 5s AND elapsed < 8s. If "within 5 seconds" was meant strictly,
   either the bound or the criterion has to move.

3. Spec 049's Verification greps `grep -rc 'SendPublishNotificationCommand' pkg/ops/`
   and requires >= 2, with the rationale "both clear paths reach the publish; a
   single call site means one path is unwired". This design has ONE call site of
   that method (inside the publisher) plus the interface declaration, so the count
   is 2 while the two clear paths reach it through one shared helper. Prompt 2
   adds a second grep counting the helper's two call sites, which is the check the
   rationale actually asks for. If the intent was two direct call sites of the
   notification method, the design must change (and would duplicate the command
   construction and the bound in both paths).

4. The config surface location is not fixed by the spec ("an operator-controlled
   config field"). This prompt puts it in the existing config file as a top-level
   `notification: {brokers, topic_prefix}` section. If the spec's author intended
   environment variables or CLI flags instead, prompt 1's section 1 is the place
   to change.

5. The spec names two new module dependencies (kafka, notification). The peer's
   transport also requires `github.com/bborbe/cqrs` (base/cdb/iam) and
   `github.com/bborbe/log` (the sampler factory), which become direct
   requirements too; `github.com/bborbe/kafka/mocks` is a test-only import of an
   already-required module. The spec's "two new module dependencies" line reads as
   a minimum, not an exhaustive list.

6. The notification message body is not specified by the spec — only the metadata
   keys are. This prompt freezes
   "escalation: <previousAssignee> cleared its assignee on task <taskName> (<taskIdentifier>)".

7. No scenario prompt is emitted for this spec. Applying the four-condition test:
   the transition, no-config, and bound behaviours are all reachable by unit and
   integration tests; the only unreachable behaviour is the real broker to
   notification-core round trip, and the spec deliberately assigns that to its own
   Post-Deploy (Rung-2) operator rung with a deploy_check, not to a committed
   scenario file. The spec names no scenario file in its Acceptance Criteria.
-->

<!-- DARK-FACTORY-REPORT -->
