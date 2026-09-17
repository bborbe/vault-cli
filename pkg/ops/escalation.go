// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
