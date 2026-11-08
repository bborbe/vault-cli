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
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("goal/topic set session take-over", func() {
	const (
		sessionA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		sessionB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
		pinnedAt = "2026-03-03T12:00:00Z"
	)

	var (
		ctx       context.Context
		pinned    libtime.CurrentDateTime
		vaultPath string
	)

	BeforeEach(func() {
		ctx = context.Background()
		pinned = libtime.NewCurrentDateTime()
		pinned.SetNow(libtimetest.ParseDateTime(pinnedAt))
		vaultPath = "/vault"
	})

	expectedStartedAt := func() libtime.DateOrDateTime {
		return libtime.DateOrDateTime(libtimetest.ParseDateTime(pinnedAt).Time())
	}

	Describe("goal set", func() {
		var (
			mockGoalStorage *mocks.GoalStorage
			setOp           ops.EntitySetOperation
			goal            *domain.Goal
			bind            func(value string) error
		)

		seedGoal := func(fields map[string]any) {
			goal = domain.NewGoal(fields, domain.FileMetadata{Name: "Alpha"}, domain.Content(""))
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		}

		BeforeEach(func() {
			mockGoalStorage = &mocks.GoalStorage{}
			mockGoalStorage.WriteGoalReturns(nil)
			setOp = ops.NewGoalSetOperation(mockGoalStorage, pinned)
			seedGoal(map[string]any{"status": "active"})
			bind = func(value string) error {
				return setOp.Execute(ctx, vaultPath, "Alpha", "claude_session_id", value, "", "")
			}
		})

		It("records the displaced owner on a take-over", func() {
			seedGoal(map[string]any{
				"status":            "active",
				"claude_session_id": sessionA,
			})

			Expect(bind(sessionB)).To(BeNil())

			Expect(goal.ClaudeSessionID()).To(Equal(sessionB))

			sessions := goal.MetricsSessions()
			Expect(sessions).To(HaveLen(1))
			Expect(sessions[0].SessionID).To(Equal(sessionA))
			Expect(sessions[0].StartedAt).To(Equal(expectedStartedAt()))

			// One write: the outgoing id and the new owner land together.
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
		})

		It("records the displaced owner when the new owner is a non-uuid display name", func() {
			seedGoal(map[string]any{
				"status":            "active",
				"claude_session_id": sessionA,
			})

			Expect(bind("incoming display name owner")).To(BeNil())

			Expect(goal.ClaudeSessionID()).To(Equal("incoming display name owner"))

			sessions := goal.MetricsSessions()
			Expect(sessions).To(HaveLen(1))
			Expect(sessions[0].SessionID).To(Equal(sessionA))
			Expect(sessions[0].StartedAt).To(Equal(expectedStartedAt()))
		})

		It("preserves existing entries in order and appends the displaced owner last", func() {
			older := libtimetest.ParseDateTime("2026-01-01T08:00:00Z")
			seedGoal(map[string]any{
				"status":            "active",
				"claude_session_id": sessionA,
				"metrics_sessions": []any{
					map[string]any{
						"session_id": "OLDER",
						"started_at": older.Time(),
					},
				},
			})

			Expect(bind(sessionB)).To(BeNil())

			sessions := goal.MetricsSessions()
			Expect(sessions).To(HaveLen(2))
			Expect(sessions[0].SessionID).To(Equal("OLDER"))
			Expect(sessions[0].StartedAt).To(Equal(libtime.DateOrDateTime(older.Time())))
			Expect(sessions[1].SessionID).To(Equal(sessionA))
		})

		It("records nothing on a first bind", func() {
			Expect(bind(sessionA)).To(BeNil())

			Expect(goal.ClaudeSessionID()).To(Equal(sessionA))
			Expect(goal.MetricsSessions()).To(BeEmpty())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
		})

		It("records nothing when re-setting the same id", func() {
			Expect(bind(sessionA)).To(BeNil())
			Expect(bind(sessionA)).To(BeNil())

			Expect(goal.ClaudeSessionID()).To(Equal(sessionA))
			Expect(goal.MetricsSessions()).To(BeEmpty())
		})

		It("records nothing when a display name is rewritten to its uuid", func() {
			seedGoal(map[string]any{
				"status":            "active",
				"claude_session_id": "vault-cli goal set fix",
			})

			Expect(bind(sessionA)).To(BeNil())

			Expect(goal.ClaudeSessionID()).To(Equal(sessionA))
			Expect(goal.MetricsSessions()).To(BeEmpty())
		})

		It("records nothing when another field is set on an owned goal", func() {
			seedGoal(map[string]any{
				"status":            "active",
				"claude_session_id": sessionA,
			})

			Expect(
				setOp.Execute(ctx, vaultPath, "Alpha", "status", "active", "", ""),
			).To(BeNil())

			Expect(goal.ClaudeSessionID()).To(Equal(sessionA))
			Expect(goal.MetricsSessions()).To(BeEmpty())
		})

		It("records nothing when the stamp is blanked", func() {
			seedGoal(map[string]any{
				"status":            "active",
				"claude_session_id": sessionA,
			})

			Expect(bind("")).To(BeNil())

			Expect(goal.ClaudeSessionID()).To(BeEmpty())
			Expect(goal.MetricsSessions()).To(BeEmpty())
		})
	})

	Describe("topic set", func() {
		var (
			mockTopicStorage *mocks.TopicStorage
			setOp            ops.EntitySetOperation
			topic            *domain.Topic
			bind             func(value string) error
		)

		seedTopic := func(fields map[string]any) {
			topic = domain.NewTopic(fields, domain.FileMetadata{Name: "Alpha"}, domain.Content(""))
			mockTopicStorage.FindTopicByNameReturns(topic, nil)
		}

		BeforeEach(func() {
			mockTopicStorage = &mocks.TopicStorage{}
			mockTopicStorage.WriteTopicReturns(nil)
			setOp = ops.NewTopicSetOperation(mockTopicStorage, pinned)
			seedTopic(map[string]any{"status": "active"})
			bind = func(value string) error {
				return setOp.Execute(ctx, vaultPath, "Alpha", "claude_session_id", value, "", "")
			}
		})

		It("records the displaced owner on a take-over", func() {
			seedTopic(map[string]any{
				"status":            "active",
				"claude_session_id": sessionA,
			})

			Expect(bind(sessionB)).To(BeNil())

			Expect(topic.GetField("claude_session_id")).To(Equal(sessionB))

			sessions := topic.MetricsSessions()
			Expect(sessions).To(HaveLen(1))
			Expect(sessions[0].SessionID).To(Equal(sessionA))
			Expect(sessions[0].StartedAt).To(Equal(expectedStartedAt()))

			Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
		})

		It("records the displaced owner when the new owner is a non-uuid display name", func() {
			seedTopic(map[string]any{
				"status":            "active",
				"claude_session_id": sessionA,
			})

			Expect(bind("incoming display name owner")).To(BeNil())

			Expect(topic.GetField("claude_session_id")).To(Equal("incoming display name owner"))

			sessions := topic.MetricsSessions()
			Expect(sessions).To(HaveLen(1))
			Expect(sessions[0].SessionID).To(Equal(sessionA))
			Expect(sessions[0].StartedAt).To(Equal(expectedStartedAt()))
		})

		It("preserves existing entries in order and appends the displaced owner last", func() {
			older := libtimetest.ParseDateTime("2026-01-01T08:00:00Z")
			seedTopic(map[string]any{
				"status":            "active",
				"claude_session_id": sessionA,
				"metrics_sessions": []any{
					map[string]any{
						"session_id": "OLDER",
						"started_at": older.Time(),
					},
				},
			})

			Expect(bind(sessionB)).To(BeNil())

			sessions := topic.MetricsSessions()
			Expect(sessions).To(HaveLen(2))
			Expect(sessions[0].SessionID).To(Equal("OLDER"))
			Expect(sessions[0].StartedAt).To(Equal(libtime.DateOrDateTime(older.Time())))
			Expect(sessions[1].SessionID).To(Equal(sessionA))
		})

		It("records nothing on a first bind", func() {
			Expect(bind(sessionA)).To(BeNil())

			Expect(topic.GetField("claude_session_id")).To(Equal(sessionA))
			Expect(topic.MetricsSessions()).To(BeEmpty())
			Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
		})

		It("records nothing when re-setting the same id", func() {
			Expect(bind(sessionA)).To(BeNil())
			Expect(bind(sessionA)).To(BeNil())

			Expect(topic.GetField("claude_session_id")).To(Equal(sessionA))
			Expect(topic.MetricsSessions()).To(BeEmpty())
		})

		It("records nothing when a display name is rewritten to its uuid", func() {
			seedTopic(map[string]any{
				"status":            "active",
				"claude_session_id": "vault-cli topic set fix",
			})

			Expect(bind(sessionA)).To(BeNil())

			Expect(topic.GetField("claude_session_id")).To(Equal(sessionA))
			Expect(topic.MetricsSessions()).To(BeEmpty())
		})

		It("records nothing when another field is set on an owned topic", func() {
			seedTopic(map[string]any{
				"status":            "active",
				"claude_session_id": sessionA,
			})

			Expect(
				setOp.Execute(ctx, vaultPath, "Alpha", "status", "active", "", ""),
			).To(BeNil())

			Expect(topic.GetField("claude_session_id")).To(Equal(sessionA))
			Expect(topic.MetricsSessions()).To(BeEmpty())
		})

		It("records nothing when the stamp is blanked", func() {
			seedTopic(map[string]any{
				"status":            "active",
				"claude_session_id": sessionA,
			})

			Expect(bind("")).To(BeNil())

			Expect(topic.GetField("claude_session_id")).To(BeEmpty())
			Expect(topic.MetricsSessions()).To(BeEmpty())
		})
	})

	Describe("goal set take-over through the YAML boundary", func() {
		const goalFixture = `---
claude_session_id: aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa
status: active
---
body line
`

		var goalsDir string

		BeforeEach(func() {
			var err error
			vaultPath, err = os.MkdirTemp("", "vault-session-takeover-*")
			Expect(err).To(BeNil())
			goalsDir = filepath.Join(vaultPath, "23 Goals")
			Expect(os.MkdirAll(goalsDir, 0755)).To(Succeed())
			Expect(
				os.WriteFile(filepath.Join(goalsDir, "Alpha.md"), []byte(goalFixture), 0600),
			).To(Succeed())
		})

		AfterEach(func() {
			if vaultPath != "" {
				_ = os.RemoveAll(vaultPath)
			}
		})

		It("persists the displaced owner and the new owner in one write", func() {
			cfg := &storage.Config{GoalsDir: "23 Goals"}
			Expect(
				ops.NewGoalSetOperation(storage.NewGoalStorage(cfg), pinned).
					Execute(ctx, vaultPath, "Alpha", "claude_session_id", sessionB, "", ""),
			).To(Succeed())

			raw, err := os.ReadFile(filepath.Join(goalsDir, "Alpha.md"))
			Expect(err).To(BeNil())
			written := string(raw)

			// The `- ` list-item prefix is load-bearing: a bare `session_id: A`
			// also matches the `claude_session_id: A` line.
			Expect(strings.Count(written, "- session_id: "+sessionA)).To(Equal(1))
			Expect(written).To(ContainSubstring("started_at:"))
			Expect(written).To(ContainSubstring("claude_session_id: " + sessionB))
		})
	})
})
