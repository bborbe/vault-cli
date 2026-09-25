// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"bytes"
	"context"
	"encoding/json"
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
			Status:           "in_progress",
			Phase:            "human_review",
			VaultName:        "personal",
			TasksDir:         "25 Tasks",
		}
		publisher = ops.NewEscalationPublisher("broker-1:9092", "master", mockFactory)
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

	It("publishes exactly one agent-escalation command with no target", func() {
		publisher.PublishEscalation(ctx, escalation)

		Eventually(func() int {
			return mockSender.SendPublishNotificationCommandCallCount()
		}).Should(Equal(1))
		_, command := mockSender.SendPublishNotificationCommandArgsForCall(0)
		Expect(command.Type).To(Equal(notifcore.AgentEscalationNotificationType))
		Expect(command.Target).To(BeNil())
		Expect(command.Message).To(Equal(notifcore.NotificationMessage(
			"escalation: alice cleared its assignee — status in_progress, phase human_review\n" +
				"obsidian://open?vault=personal&file=25+Tasks%2FPark+the+Escalation+Task",
		)))
		Expect(command.Validate(ctx)).To(Succeed())
		Expect(command.Metadata).To(Equal(map[string]string{
			"taskIdentifier":   escalation.TaskIdentifier,
			"taskName":         escalation.TaskName,
			"previousAssignee": escalation.PreviousAssignee,
		}))
	})

	It("passes the configured brokers and topic prefix to the sender factory", func() {
		publisher.PublishEscalation(ctx, escalation)

		Eventually(func() int {
			return mockFactory.CreateCallCount()
		}).Should(Equal(1))
		_, brokers, topicPrefix := mockFactory.CreateArgsForCall(0)
		Expect(brokers).To(Equal("broker-1:9092"))
		Expect(topicPrefix).To(Equal("master"))
	})

	It("publishes nothing and attempts no connection when no broker is configured", func() {
		publisher = ops.NewEscalationPublisher("", "", mockFactory)

		publisher.PublishEscalation(ctx, escalation)

		Consistently(func() int {
			return mockFactory.CreateCallCount()
		}, "200ms", "50ms").Should(Equal(0))
		Expect(mockSender.SendPublishNotificationCommandCallCount()).To(Equal(0))
	})

	It("returns within the publish bound when the sender blocks past it", func() {
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
	})

	It("logs the frozen failure line when the publish fails", func() {
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
	})

	It("publishes onto the deployment's prefixed command topic", func() {
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
		value, err := message.Value.Encode()
		Expect(err).NotTo(HaveOccurred())
		// The body survives the production serializer. The envelope is decoded
		// rather than substring-matched because encoding/json HTML-escapes the
		// link's ampersand inside the JSON string, which is a property of the
		// encoder and not of the body — the peer's envelope escapes it the same
		// way, so the published bytes stay identical.
		var envelope struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		Expect(json.Unmarshal(value, &envelope)).To(Succeed())
		Expect(envelope.Data.Message).To(Equal(
			"escalation: alice cleared its assignee — status in_progress, phase human_review\n" +
				"obsidian://open?vault=personal&file=25+Tasks%2FPark+the+Escalation+Task",
		))
	})

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
})
