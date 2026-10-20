// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("FrontmatterSetOperation session take-over", func() {
	const (
		sessionA = "AAAA"
		sessionB = "BBBB"
		pinnedAt = "2026-03-03T12:00:00Z"
	)

	var (
		ctx             context.Context
		mockTaskStorage *mocks.TaskStorage
		setOp           ops.FrontmatterSetOperation
		task            *domain.Task
		bind            func(value string) error
	)

	// seedTask replaces the task the storage returns, so a spec can control the
	// owner and the metrics_sessions list the write accumulates onto.
	seedTask := func(fields map[string]any) {
		task = domain.NewTask(fields, domain.FileMetadata{Name: "Alpha"}, domain.Content(""))
		mockTaskStorage.FindTaskByNameReturns(task, nil)
	}

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		mockTaskStorage.WriteTaskReturns(nil)

		pinned := libtime.NewCurrentDateTime()
		pinned.SetNow(libtimetest.ParseDateTime(pinnedAt))
		setOp = ops.NewFrontmatterSetOperation(
			mockTaskStorage,
			pinned,
			ops.NewEscalationPublisher("", "", &mocks.NotificationSenderFactory{}),
			"personal",
			"25 Tasks",
		)

		seedTask(map[string]any{"status": "in_progress"})

		bind = func(value string) error {
			return setOp.Execute(ctx, "/vault", "Alpha", "claude_session_id", value, "", "", "", false)
		}
	})

	It("records the displaced owner on a take-over", func() {
		Expect(bind(sessionA)).To(BeNil())
		Expect(bind(sessionB)).To(BeNil())

		Expect(task.ClaudeSessionID()).To(Equal(sessionB))

		sessions := task.MetricsSessions()
		Expect(sessions).To(HaveLen(1))
		Expect(sessions[0].SessionID).To(Equal(sessionA))
		Expect(sessions[0].StartedAt).To(
			Equal(libtime.DateOrDateTime(libtimetest.ParseDateTime(pinnedAt).Time())),
		)

		// One write per command: the outgoing id and the new owner land together.
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(2))
	})

	It("records nothing on a first bind", func() {
		Expect(bind(sessionB)).To(BeNil())

		Expect(task.ClaudeSessionID()).To(Equal(sessionB))
		Expect(task.MetricsSessions()).To(BeEmpty())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
	})

	It("records nothing when re-binding the same value", func() {
		Expect(bind(sessionA)).To(BeNil())
		Expect(bind(sessionA)).To(BeNil())

		Expect(task.ClaudeSessionID()).To(Equal(sessionA))
		Expect(task.MetricsSessions()).To(BeEmpty())
	})

	It("records the outgoing owner when the owner is cleared", func() {
		Expect(bind(sessionA)).To(BeNil())
		Expect(bind("")).To(BeNil())

		Expect(task.ClaudeSessionID()).To(BeEmpty())

		sessions := task.MetricsSessions()
		Expect(sessions).To(HaveLen(1))
		Expect(sessions[0].SessionID).To(Equal(sessionA))
	})

	It("preserves existing entries in order and appends the displaced owner last", func() {
		older := libtimetest.ParseDateTime("2026-01-01T08:00:00Z")
		seedTask(map[string]any{
			"status":            "in_progress",
			"claude_session_id": sessionA,
			"metrics_sessions": []any{
				map[string]any{
					"session_id": "OLDER",
					"started_at": older.Time(),
				},
			},
		})

		Expect(bind(sessionB)).To(BeNil())

		sessions := task.MetricsSessions()
		Expect(sessions).To(HaveLen(2))
		Expect(sessions[0].SessionID).To(Equal("OLDER"))
		Expect(sessions[0].StartedAt).To(Equal(libtime.DateOrDateTime(older.Time())))
		Expect(sessions[1].SessionID).To(Equal(sessionA))
	})

	It("records nothing when another field is set on an owned task", func() {
		seedTask(map[string]any{
			"status":            "in_progress",
			"claude_session_id": sessionA,
		})

		Expect(
			setOp.Execute(ctx, "/vault", "Alpha", "phase", "execution", "", "", "", false),
		).To(BeNil())

		Expect(task.ClaudeSessionID()).To(Equal(sessionA))
		Expect(task.MetricsSessions()).To(BeEmpty())
	})

	It("writes nothing when the task cannot be read", func() {
		mockTaskStorage.FindTaskByNameReturns(nil, errors.New(ctx, "boom"))

		err := bind(sessionB)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("find task"))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
	})
})
