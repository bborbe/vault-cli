// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("goal and topic session rebind", func() {
	const claimantID = "XXXX-interactive"

	var (
		ctx       context.Context
		startedAt libtime.DateOrDateTime
	)

	BeforeEach(func() {
		ctx = context.Background()
		startedAt = libtime.DateOrDateTime(libtimetest.ParseDateTime("2026-03-03T12:00:00Z").Time())
	})

	// assertRebindEntry asserts one metrics_sessions entry for the binding session at
	// the turn-start time, in order after any seeded entries.
	assertRebindEntry := func(sessions []domain.MetricsSession) {
		Expect(sessions).To(HaveLen(1))
		Expect(sessions[0].SessionID).To(Equal(pinnedSessionID))
		Expect(sessions[0].StartedAt.Time().Equal(startedAt.Time())).To(BeTrue())
	}

	Describe("persistGoalSessionID", func() {
		var (
			mockGoalStorage *mocks.GoalStorage
			page            *domain.Goal
		)

		BeforeEach(func() {
			mockGoalStorage = &mocks.GoalStorage{}
			page = domain.NewGoal(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: "Rebind Goal", FilePath: "/vault/Goals/Rebind Goal.md"},
				domain.Content(""),
			)
			mockGoalStorage.FindGoalByNameReturns(page, nil)
			mockGoalStorage.WriteGoalReturns(nil)
		})

		persist := func() (string, error) {
			return ops.PersistGoalSessionID(
				ctx, "/vault", "Rebind Goal", pinnedSessionID, startedAt, mockGoalStorage,
			)
		}

		It("keeps a different claimant and records the binding session at turn start", func() {
			page.SetClaudeSessionID(claimantID)

			id, err := persist()

			Expect(err).To(BeNil())
			Expect(id).To(Equal(pinnedSessionID))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, written := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(written.ClaudeSessionID()).To(Equal(claimantID))
			assertRebindEntry(written.MetricsSessions())
		})

		It("stamps an empty field and appends nothing", func() {
			id, err := persist()

			Expect(err).To(BeNil())
			Expect(id).To(Equal(pinnedSessionID))
			_, written := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(written.ClaudeSessionID()).To(Equal(pinnedSessionID))
			Expect(written.MetricsSessions()).To(BeNil())
			Expect(written.Get("metrics_sessions")).To(BeNil())
		})

		It("appends nothing when the field already holds the binding id", func() {
			page.SetClaudeSessionID(pinnedSessionID)

			id, err := persist()

			Expect(err).To(BeNil())
			Expect(id).To(Equal(pinnedSessionID))
			_, written := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(written.ClaudeSessionID()).To(Equal(pinnedSessionID))
			Expect(written.MetricsSessions()).To(BeNil())
		})

		It("keeps existing entries and appends the binding session last", func() {
			page.SetClaudeSessionID(claimantID)
			page.Set("metrics_sessions", []any{
				map[string]any{
					"session_id": "seeded-session",
					"started_at": "2026-01-01T08:00:00Z",
				},
			})

			_, err := persist()

			Expect(err).To(BeNil())
			_, written := mockGoalStorage.WriteGoalArgsForCall(0)
			sessions := written.MetricsSessions()
			Expect(sessions).To(HaveLen(2))
			Expect(sessions[0].SessionID).To(Equal("seeded-session"))
			Expect(
				sessions[0].StartedAt.Time().Equal(
					libtimetest.ParseDateTime("2026-01-01T08:00:00Z").Time(),
				),
			).To(BeTrue())
			Expect(sessions[1].SessionID).To(Equal(pinnedSessionID))
			Expect(sessions[1].StartedAt.Time().Equal(startedAt.Time())).To(BeTrue())
		})

		It("returns an empty id and writes nothing when the re-read fails", func() {
			mockGoalStorage.FindGoalByNameReturns(nil, ErrTest)

			id, err := persist()

			Expect(id).To(Equal(""))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("re-read goal after claude session"))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})

		It("returns an empty id when the write fails", func() {
			mockGoalStorage.WriteGoalReturns(ErrTest)

			id, err := persist()

			Expect(id).To(Equal(""))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("save session id to goal"))
		})
	})

	Describe("persistTopicSessionID", func() {
		var (
			mockTopicStorage *mocks.TopicStorage
			page             *domain.Topic
		)

		BeforeEach(func() {
			mockTopicStorage = &mocks.TopicStorage{}
			page = domain.NewTopic(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: "Rebind Topic", FilePath: "/vault/Topics/Rebind Topic.md"},
				domain.Content(""),
			)
			mockTopicStorage.FindTopicByNameReturns(page, nil)
			mockTopicStorage.WriteTopicReturns(nil)
		})

		persist := func() (string, error) {
			return ops.PersistTopicSessionID(
				ctx, "/vault", "Rebind Topic", pinnedSessionID, startedAt, mockTopicStorage,
			)
		}

		It("keeps a different claimant and records the binding session at turn start", func() {
			Expect(page.SetField(ctx, "claude_session_id", claimantID)).To(Succeed())

			id, err := persist()

			Expect(err).To(BeNil())
			Expect(id).To(Equal(pinnedSessionID))
			Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("claude_session_id")).To(Equal(claimantID))
			assertRebindEntry(written.MetricsSessions())
		})

		It("stamps an empty field and appends nothing", func() {
			id, err := persist()

			Expect(err).To(BeNil())
			Expect(id).To(Equal(pinnedSessionID))
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("claude_session_id")).To(Equal(pinnedSessionID))
			Expect(written.MetricsSessions()).To(BeNil())
			Expect(written.Get("metrics_sessions")).To(BeNil())
		})

		It("appends nothing when the field already holds the binding id", func() {
			Expect(page.SetField(ctx, "claude_session_id", pinnedSessionID)).To(Succeed())

			id, err := persist()

			Expect(err).To(BeNil())
			Expect(id).To(Equal(pinnedSessionID))
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("claude_session_id")).To(Equal(pinnedSessionID))
			Expect(written.MetricsSessions()).To(BeNil())
		})

		It("keeps existing entries and appends the binding session last", func() {
			Expect(page.SetField(ctx, "claude_session_id", claimantID)).To(Succeed())
			page.Set("metrics_sessions", []any{
				map[string]any{
					"session_id": "seeded-session",
					"started_at": "2026-01-01T08:00:00Z",
				},
			})

			_, err := persist()

			Expect(err).To(BeNil())
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			sessions := written.MetricsSessions()
			Expect(sessions).To(HaveLen(2))
			Expect(sessions[0].SessionID).To(Equal("seeded-session"))
			Expect(
				sessions[0].StartedAt.Time().Equal(
					libtimetest.ParseDateTime("2026-01-01T08:00:00Z").Time(),
				),
			).To(BeTrue())
			Expect(sessions[1].SessionID).To(Equal(pinnedSessionID))
			Expect(sessions[1].StartedAt.Time().Equal(startedAt.Time())).To(BeTrue())
		})

		It("returns an empty id and writes nothing when the re-read fails", func() {
			mockTopicStorage.FindTopicByNameReturns(nil, ErrTest)

			id, err := persist()

			Expect(id).To(Equal(""))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("re-read topic after claude session"))
			Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(0))
		})

		It("returns an empty id when the write fails", func() {
			mockTopicStorage.WriteTopicReturns(ErrTest)

			id, err := persist()

			Expect(id).To(Equal(""))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("save session id to topic"))
		})
	})

	Describe("Execute captures the turn start before the turn", func() {
		var (
			clock   libtime.CurrentDateTime
			turnEnd libtime.DateTime
		)

		BeforeEach(func() {
			clock = libtime.NewCurrentDateTime()
			clock.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
			turnEnd = libtimetest.ParseDateTime("2026-03-03T12:30:00Z")
		})

		Describe("goal", func() {
			var (
				mockGoalStorage *mocks.GoalStorage
				mockStarter     *mocks.ClaudeSessionStarter
				mockResumer     *mocks.ClaudeResumer
				page            *domain.Goal
				vault           *config.Vault
				op              ops.GoalWorkOnOperation
			)

			BeforeEach(func() {
				mockGoalStorage = &mocks.GoalStorage{}
				mockStarter = &mocks.ClaudeSessionStarter{}
				mockResumer = &mocks.ClaudeResumer{}
				page = domain.NewGoal(
					map[string]any{"status": "next"},
					domain.FileMetadata{Name: "Rebind Goal", FilePath: "/vault/Goals/Rebind Goal.md"},
					domain.Content(""),
				)
				mockGoalStorage.FindGoalByNameReturns(page, nil)
				mockGoalStorage.WriteGoalReturns(nil)
				mockResumer.ResumeSessionReturns(nil)
				vault = &config.Vault{
					Path:              "/vault",
					Name:              "test-vault",
					WorkOnGoalCommand: "/vault-cli:work-on-goal",
				}
				op = ops.NewGoalWorkOnOperation(
					mockGoalStorage,
					clock,
					func() string { return pinnedSessionID },
					mockStarter,
					mockResumer,
				)
			})

			simulateMidTurnClaim := func() {
				mockStarter.StartSessionStub = func(
					context.Context, string, string, string, string, bool,
				) error {
					page.SetClaudeSessionID(claimantID)
					clock.SetNow(turnEnd)
					return nil
				}
			}

			It("records the turn start, not the persist time, and keeps the claimant (headless)", func() {
				simulateMidTurnClaim()

				result, err := op.Execute(
					ctx, "/vault", "Rebind Goal", "user@example.com", "test-vault",
					false, "/vault", vault,
				)

				Expect(err).To(BeNil())
				Expect(result.SessionID).To(Equal(pinnedSessionID))
				Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(2))
				_, written := mockGoalStorage.WriteGoalArgsForCall(
					mockGoalStorage.WriteGoalCallCount() - 1,
				)
				Expect(written.ClaudeSessionID()).To(Equal(claimantID))
				assertRebindEntry(written.MetricsSessions())
			})

			It("resumes the binding session, not the claimant, on the TTY branch", func() {
				simulateMidTurnClaim()

				result, err := op.Execute(
					ctx, "/vault", "Rebind Goal", "user@example.com", "test-vault",
					true, "/vault", vault,
				)

				Expect(err).To(BeNil())
				Expect(result.SessionID).To(Equal(pinnedSessionID))
				Expect(mockResumer.ResumeSessionCallCount()).To(Equal(1))
				_, resumedID, _, _ := mockResumer.ResumeSessionArgsForCall(0)
				Expect(resumedID).To(Equal(pinnedSessionID))
				_, written := mockGoalStorage.WriteGoalArgsForCall(
					mockGoalStorage.WriteGoalCallCount() - 1,
				)
				Expect(written.ClaudeSessionID()).To(Equal(claimantID))
				assertRebindEntry(written.MetricsSessions())
			})
		})

		Describe("topic", func() {
			var (
				mockTopicStorage *mocks.TopicStorage
				mockStarter      *mocks.ClaudeSessionStarter
				mockResumer      *mocks.ClaudeResumer
				page             *domain.Topic
				vault            *config.Vault
				op               ops.TopicWorkOnOperation
			)

			BeforeEach(func() {
				mockTopicStorage = &mocks.TopicStorage{}
				mockStarter = &mocks.ClaudeSessionStarter{}
				mockResumer = &mocks.ClaudeResumer{}
				page = domain.NewTopic(
					map[string]any{"status": "backlog"},
					domain.FileMetadata{Name: "Rebind Topic", FilePath: "/vault/Topics/Rebind Topic.md"},
					domain.Content(""),
				)
				mockTopicStorage.FindTopicByNameReturns(page, nil)
				mockTopicStorage.WriteTopicReturns(nil)
				mockResumer.ResumeSessionReturns(nil)
				vault = &config.Vault{
					Path:              "/vault",
					Name:              "test-vault",
					WorkOnGoalCommand: "/vault-cli:work-on-goal",
				}
				op = ops.NewTopicWorkOnOperation(
					mockTopicStorage,
					clock,
					func() string { return pinnedSessionID },
					mockStarter,
					mockResumer,
				)
			})

			simulateMidTurnClaim := func() {
				mockStarter.StartSessionStub = func(
					context.Context, string, string, string, string, bool,
				) error {
					_ = page.SetField(ctx, "claude_session_id", claimantID)
					clock.SetNow(turnEnd)
					return nil
				}
			}

			It("records the turn start, not the persist time, and keeps the claimant (headless)", func() {
				simulateMidTurnClaim()

				result, err := op.Execute(
					ctx, "/vault", "Rebind Topic", "user@example.com", "test-vault",
					false, "/vault", vault,
				)

				Expect(err).To(BeNil())
				Expect(result.SessionID).To(Equal(pinnedSessionID))
				Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(2))
				_, written := mockTopicStorage.WriteTopicArgsForCall(
					mockTopicStorage.WriteTopicCallCount() - 1,
				)
				Expect(written.GetField("claude_session_id")).To(Equal(claimantID))
				assertRebindEntry(written.MetricsSessions())
			})

			It("resumes the binding session, not the claimant, on the TTY branch", func() {
				simulateMidTurnClaim()

				result, err := op.Execute(
					ctx, "/vault", "Rebind Topic", "user@example.com", "test-vault",
					true, "/vault", vault,
				)

				Expect(err).To(BeNil())
				Expect(result.SessionID).To(Equal(pinnedSessionID))
				Expect(mockResumer.ResumeSessionCallCount()).To(Equal(1))
				_, resumedID, _, _ := mockResumer.ResumeSessionArgsForCall(0)
				Expect(resumedID).To(Equal(pinnedSessionID))
				_, written := mockTopicStorage.WriteTopicArgsForCall(
					mockTopicStorage.WriteTopicCallCount() - 1,
				)
				Expect(written.GetField("claude_session_id")).To(Equal(claimantID))
				assertRebindEntry(written.MetricsSessions())
			})
		})
	})

	Describe("real storage round-trip", func() {
		var (
			vaultPath  string
			goalStore  storage.GoalStorage
			topicStore storage.TopicStorage
		)

		BeforeEach(func() {
			var err error
			vaultPath, err = os.MkdirTemp("", "vault-rebind-*")
			Expect(err).To(BeNil())
			DeferCleanup(func() { _ = os.RemoveAll(vaultPath) })
			Expect(os.MkdirAll(filepath.Join(vaultPath, "23 Goals"), 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(vaultPath, "23 Topics"), 0755)).To(Succeed())
			goalStore = storage.NewGoalStorage(&storage.Config{GoalsDir: "23 Goals"})
			topicStore = storage.NewTopicStorage(&storage.Config{TopicsDir: "23 Topics"})
		})

		It("records the binder in the goal file and keeps the claimant as owner", func() {
			goalPath := filepath.Join(vaultPath, "23 Goals", "Rebind Goal.md")
			Expect(os.WriteFile(goalPath, []byte(`---
status: in_progress
claude_session_id: `+claimantID+`
---
body
`), 0600)).To(Succeed())

			id, err := ops.PersistGoalSessionID(
				ctx, vaultPath, "Rebind Goal", pinnedSessionID, startedAt, goalStore,
			)

			Expect(err).To(BeNil())
			Expect(id).To(Equal(pinnedSessionID))
			raw, err := os.ReadFile(goalPath)
			Expect(err).To(BeNil())
			content := string(raw)
			Expect(strings.Count(content, "- session_id: "+pinnedSessionID)).To(Equal(1))
			Expect(strings.Count(content, "- session_id: "+claimantID)).To(Equal(0))
			Expect(content).To(ContainSubstring("claude_session_id: " + claimantID))

			written, err := goalStore.FindGoalByName(ctx, vaultPath, "Rebind Goal")
			Expect(err).To(BeNil())
			assertRebindEntry(written.MetricsSessions())
		})

		It("records the binder in the topic file and keeps the claimant as owner", func() {
			topicPath := filepath.Join(vaultPath, "23 Topics", "Rebind Topic.md")
			Expect(os.WriteFile(topicPath, []byte(`---
status: in_progress
claude_session_id: `+claimantID+`
---
body
`), 0600)).To(Succeed())

			id, err := ops.PersistTopicSessionID(
				ctx, vaultPath, "Rebind Topic", pinnedSessionID, startedAt, topicStore,
			)

			Expect(err).To(BeNil())
			Expect(id).To(Equal(pinnedSessionID))
			raw, err := os.ReadFile(topicPath)
			Expect(err).To(BeNil())
			content := string(raw)
			Expect(strings.Count(content, "- session_id: "+pinnedSessionID)).To(Equal(1))
			Expect(strings.Count(content, "- session_id: "+claimantID)).To(Equal(0))
			Expect(content).To(ContainSubstring("claude_session_id: " + claimantID))

			written, err := topicStore.FindTopicByName(ctx, vaultPath, "Rebind Topic")
			Expect(err).To(BeNil())
			assertRebindEntry(written.MetricsSessions())
		})

		It("stamps an uncontended goal without adding a metrics_sessions key", func() {
			goalPath := filepath.Join(vaultPath, "23 Goals", "Fresh Goal.md")
			Expect(os.WriteFile(goalPath, []byte(`---
status: in_progress
---
body
`), 0600)).To(Succeed())

			_, err := ops.PersistGoalSessionID(
				ctx, vaultPath, "Fresh Goal", pinnedSessionID, startedAt, goalStore,
			)

			Expect(err).To(BeNil())
			raw, err := os.ReadFile(goalPath)
			Expect(err).To(BeNil())
			content := string(raw)
			Expect(content).To(ContainSubstring("claude_session_id: " + pinnedSessionID))
			Expect(content).NotTo(ContainSubstring("metrics_sessions:"))
		})

		It("stamps an uncontended topic without adding a metrics_sessions key", func() {
			topicPath := filepath.Join(vaultPath, "23 Topics", "Fresh Topic.md")
			Expect(os.WriteFile(topicPath, []byte(`---
status: in_progress
---
body
`), 0600)).To(Succeed())

			_, err := ops.PersistTopicSessionID(
				ctx, vaultPath, "Fresh Topic", pinnedSessionID, startedAt, topicStore,
			)

			Expect(err).To(BeNil())
			raw, err := os.ReadFile(topicPath)
			Expect(err).To(BeNil())
			content := string(raw)
			Expect(content).To(ContainSubstring("claude_session_id: " + pinnedSessionID))
			Expect(content).NotTo(ContainSubstring("metrics_sessions:"))
		})
	})
})
