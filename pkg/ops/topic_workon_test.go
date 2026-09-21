// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

	"github.com/bborbe/errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("TopicWorkOnOperation", func() {
	var (
		ctx              context.Context
		err              error
		result           ops.MutationResult
		topicWorkOnOp    ops.TopicWorkOnOperation
		mockTopicStorage *mocks.TopicStorage
		mockStarter      *mocks.ClaudeSessionStarter
		mockResumer      *mocks.ClaudeResumer
		vaultPath        string
		topicName        string
		assignee         string
		topic            *domain.Topic
		isInteractive    bool
		testVault        config.Vault
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTopicStorage = &mocks.TopicStorage{}
		mockStarter = &mocks.ClaudeSessionStarter{}
		mockResumer = &mocks.ClaudeResumer{}
		topicWorkOnOp = ops.NewTopicWorkOnOperation(
			mockTopicStorage,
			func() string { return pinnedSessionID },
			mockStarter,
			mockResumer,
		)
		vaultPath = "/path/to/vault"
		topicName = "my-topic"
		assignee = "user@example.com"
		isInteractive = false
		testVault = config.Vault{
			Path:              vaultPath,
			Name:              "test-vault",
			WorkOnGoalCommand: "/vault-cli:work-on-goal",
		}

		topic = domain.NewTopic(
			map[string]any{"status": "backlog"},
			domain.FileMetadata{Name: topicName, FilePath: "/path/to/vault/Topics/my-topic.md"},
			domain.Content(""),
		)
		mockTopicStorage.FindTopicByNameReturns(topic, nil)
		mockTopicStorage.WriteTopicReturns(nil)
		mockStarter.StartSessionReturns(nil)
		mockResumer.ResumeSessionReturns(nil)
	})

	JustBeforeEach(func() {
		result, err = topicWorkOnOp.Execute(
			ctx,
			vaultPath,
			topicName,
			assignee,
			"test-vault",
			isInteractive,
			vaultPath,
			&testVault,
		)
	})

	Context("success", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("re-reads the topic from the vault path after the session finishes", func() {
			// Twice: once to load the topic, once to re-read it after the child exits
			// so the session id lands on the freshest on-disk state.
			Expect(mockTopicStorage.FindTopicByNameCallCount()).To(Equal(2))
			_, reReadVaultPath, reReadTopicName := mockTopicStorage.FindTopicByNameArgsForCall(1)
			Expect(reReadVaultPath).To(Equal(vaultPath))
			Expect(reReadTopicName).To(Equal(topicName))
		})

		It("marks the topic in_progress", func() {
			Expect(mockTopicStorage.WriteTopicCallCount()).To(BeNumerically(">=", 1))
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("status")).To(Equal(domain.TopicStatusInProgress))
		})

		It("sets the assignee", func() {
			Expect(mockTopicStorage.WriteTopicCallCount()).To(BeNumerically(">=", 1))
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("assignee")).To(Equal(assignee))
		})

		It("starts a claude session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
		})

		It("passes the topic name to the session starter", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, _, _, name, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(name).To(Equal(topicName))
		})

		It("passes isInteractive=false to the starter on the non-interactive branch", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, _, _, _, isInteractiveArg := mockStarter.StartSessionArgsForCall(0)
			Expect(isInteractiveArg).To(BeFalse())
		})

		It("does not advance or write a phase", func() {
			Expect(mockTopicStorage.WriteTopicCallCount()).To(BeNumerically(">=", 1))
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("phase")).To(Equal(""))
		})
	})

	Context("when assignee already equals current user", func() {
		BeforeEach(func() {
			topic = domain.NewTopic(
				map[string]any{"status": "backlog", "assignee": assignee},
				domain.FileMetadata{Name: topicName, FilePath: "/path/to/vault/Topics/my-topic.md"},
				domain.Content(""),
			)
			mockTopicStorage.FindTopicByNameReturns(topic, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("preserves the existing assignee", func() {
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("assignee")).To(Equal(assignee))
		})

		It("emits no assignee warning", func() {
			Expect(result.Warnings).NotTo(ContainElement(ContainSubstring("assignee not updated")))
		})
	})

	Context("when assignee is set to a different user", func() {
		const otherUser = "alice@example.com"

		BeforeEach(func() {
			topic = domain.NewTopic(
				map[string]any{"status": "backlog", "assignee": otherUser},
				domain.FileMetadata{Name: topicName, FilePath: "/path/to/vault/Topics/my-topic.md"},
				domain.Content(""),
			)
			mockTopicStorage.FindTopicByNameReturns(topic, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("preserves the other user's assignment", func() {
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("assignee")).To(Equal(otherUser))
		})

		It("emits an assignee-not-updated warning naming both users", func() {
			Expect(result.Warnings).To(ContainElement(ContainSubstring("assignee not updated")))
			Expect(result.Warnings).To(ContainElement(ContainSubstring(otherUser)))
			Expect(result.Warnings).To(ContainElement(ContainSubstring(assignee)))
		})

		It("still marks the topic in_progress", func() {
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("status")).To(Equal(domain.TopicStatusInProgress))
		})
	})

	Context("custom work on command", func() {
		BeforeEach(func() {
			testVault.WorkOnGoalCommand = "/custom-cmd"
		})

		It("uses the configured work on command in the prompt", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, prompt, _, _, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(prompt).To(MatchRegexp(`^/custom-cmd "`))
		})

		It("appends --non-interactive and names the topic file path", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, prompt, _, _, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(prompt).To(MatchRegexp(` --non-interactive$`))
			Expect(prompt).To(MatchRegexp(`/path/to/vault/Topics/my-topic\.md`))
		})
	})

	Context("when starter is nil and the topic has no cached session ID", func() {
		BeforeEach(func() {
			topicWorkOnOp = ops.NewTopicWorkOnOperation(
				mockTopicStorage,
				func() string { return pinnedSessionID },
				nil,
				nil,
			)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("skips session start", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})

		It("emits a warning about the missing starter", func() {
			Expect(
				result.Warnings,
			).To(ContainElement(ContainSubstring("claude session: claude session starter unavailable")))
		})

		It("returns an empty session ID", func() {
			Expect(result.SessionID).To(Equal(""))
		})

		It("still marks the topic in_progress", func() {
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("status")).To(Equal(domain.TopicStatusInProgress))
		})
	})

	Context("when the topic already has a session ID", func() {
		BeforeEach(func() {
			topic = domain.NewTopic(
				map[string]any{"status": "backlog", "claude_session_id": "existing-session"},
				domain.FileMetadata{Name: topicName, FilePath: "/path/to/vault/Topics/my-topic.md"},
				domain.Content(""),
			)
			mockTopicStorage.FindTopicByNameReturns(topic, nil)
		})

		It("does not start a new session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("returns the cached session ID", func() {
			Expect(result.SessionID).To(Equal("existing-session"))
		})
	})

	Context("when session start fails (hard failure)", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns(ErrTest)
		})

		It("returns a wrapped error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
		})

		It("returns Success=false", func() {
			Expect(result.Success).To(BeFalse())
		})

		It("still marks the topic in_progress", func() {
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("status")).To(Equal(domain.TopicStatusInProgress))
		})
	})

	Context("when claude returns zero turns", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns(
				errors.New(ctx, "claude returned 0 turns: Unknown command: /x"),
			)
		})

		It("returns an error carrying the child's reason", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
			Expect(err.Error()).To(ContainSubstring("claude returned 0 turns: Unknown command: /x"))
			Expect(result.Success).To(BeFalse())
		})
	})

	Context("interactive mode", func() {
		BeforeEach(func() {
			isInteractive = true
		})

		It("calls ResumeSession with the empty resumed-turn argument", func() {
			Expect(mockResumer.ResumeSessionCallCount()).To(Equal(1))
			_, sessionID, cwd, prompt := mockResumer.ResumeSessionArgsForCall(0)
			Expect(sessionID).To(Equal(pinnedSessionID))
			Expect(cwd).To(Equal(vaultPath))
			Expect(prompt).To(BeEmpty())
		})

		It("passes isInteractive=true to the starter", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, _, _, _, isInteractiveArg := mockStarter.StartSessionArgsForCall(0)
			Expect(isInteractiveArg).To(BeTrue())
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("topic not found", func() {
		BeforeEach(func() {
			mockTopicStorage.FindTopicByNameReturns(nil, ErrTest)
		})

		It("returns an error wrapped with find topic", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("find topic"))
		})

		It("does not write the topic", func() {
			Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(0))
		})

		It("does not start a session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})
	})

	Context("write error", func() {
		BeforeEach(func() {
			mockTopicStorage.WriteTopicReturns(ErrTest)
		})

		It("returns an error wrapped with write topic", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("write topic"))
		})

		It("does not start a session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})
	})

	Context("when persisting the session id fails", func() {
		BeforeEach(func() {
			mockTopicStorage.WriteTopicStub = func(_ context.Context, t *domain.Topic) error {
				if t.GetField("claude_session_id") != "" {
					return ErrTest
				}
				return nil
			}
		})

		It("reports no session id rather than advertising one that did not land", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("save session id to topic"))
			Expect(result.SessionID).To(Equal(""))
		})
	})

	Context("when the topic page carries a non-canonical prior status", func() {
		BeforeEach(func() {
			topic = domain.NewTopic(
				map[string]any{"status": "whatever"},
				domain.FileMetadata{Name: topicName, FilePath: "/path/to/vault/Topics/my-topic.md"},
				domain.Content(""),
			)
			mockTopicStorage.FindTopicByNameReturns(topic, nil)
		})

		It("still moves the topic into in_progress", func() {
			Expect(err).To(BeNil())
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("status")).To(Equal(domain.TopicStatusInProgress))
		})
	})
})
