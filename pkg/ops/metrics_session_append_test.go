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

var _ = Describe("AppendMetricsSessionOperation", func() {
	const (
		sessionA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		sessionB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)

	var (
		ctx             context.Context
		err             error
		mockTaskStorage *mocks.TaskStorage
		appendOp        ops.AppendMetricsSessionOperation
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		mockTaskStorage.FindTaskByNameReturns(
			domain.NewTask(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: "Alpha"},
				domain.Content(""),
			),
			nil,
		)
		mockTaskStorage.WriteTaskReturns(nil)

		pinned := libtime.NewCurrentDateTime()
		pinned.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
		appendOp = ops.NewAppendMetricsSessionOperation(mockTaskStorage, pinned)
	})

	// seedTask replaces the task the storage returns, so a spec can control the
	// metrics_sessions list the append accumulates onto.
	seedTask := func(fields map[string]any) {
		mockTaskStorage.FindTaskByNameReturns(
			domain.NewTask(fields, domain.FileMetadata{Name: "Alpha"}, domain.Content("")),
			nil,
		)
	}

	It("appends one entry with the supplied session id and the invocation time", func() {
		err = appendOp.Execute(ctx, "/vault", "Alpha", sessionA)
		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))

		_, written := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(written.MetricsSessions()).To(HaveLen(1))
		Expect(written.MetricsSessions()[0].SessionID).To(Equal(sessionA))
		Expect(written.MetricsSessions()[0].StartedAt).To(Equal(
			libtime.DateOrDateTime(libtimetest.ParseDateTime("2026-03-03T12:00:00Z").Time()),
		))
	})

	It("appends to the existing entries and preserves them in order", func() {
		preExisting := libtime.DateOrDateTime(
			libtimetest.ParseDateTime("2026-01-01T08:00:00Z").Time(),
		)
		seedTask(map[string]any{
			"status": "in_progress",
			"metrics_sessions": []domain.MetricsSession{
				{SessionID: sessionA, StartedAt: preExisting},
			},
		})

		err = appendOp.Execute(ctx, "/vault", "Alpha", sessionB)
		Expect(err).To(BeNil())

		_, written := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(written.MetricsSessions()).To(HaveLen(2))
		Expect(written.MetricsSessions()[0].SessionID).To(Equal(sessionA))
		Expect(written.MetricsSessions()[0].StartedAt).To(Equal(preExisting))
		Expect(written.MetricsSessions()[1].SessionID).To(Equal(sessionB))
	})

	It("appends a duplicate session id rather than suppressing it", func() {
		seedTask(map[string]any{
			"status": "in_progress",
			"metrics_sessions": []domain.MetricsSession{
				{
					SessionID: sessionA,
					StartedAt: libtime.DateOrDateTime(
						libtimetest.ParseDateTime("2026-01-01T08:00:00Z").Time(),
					),
				},
			},
		})

		err = appendOp.Execute(ctx, "/vault", "Alpha", sessionA)
		Expect(err).To(BeNil())

		_, written := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(written.MetricsSessions()).To(HaveLen(2))
		Expect(written.MetricsSessions()[0].SessionID).To(Equal(sessionA))
		Expect(written.MetricsSessions()[1].SessionID).To(Equal(sessionA))
	})

	It("does not write claude_session_id", func() {
		err = appendOp.Execute(ctx, "/vault", "Alpha", sessionA)
		Expect(err).To(BeNil())

		_, written := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(written.ClaudeSessionID()).To(Equal(""))
	})

	assertRefused := func(sessionID string) {
		err = appendOp.Execute(ctx, "/vault", "Alpha", sessionID)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("expected a well-formed UUID"))
		Expect(mockTaskStorage.FindTaskByNameCallCount()).To(Equal(0))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
	}

	It("refuses an empty session id before reading the task", func() {
		assertRefused("")
	})

	It("refuses a session id that is not a UUID", func() {
		assertRefused("not-a-uuid")
	})

	It("refuses a session id carrying a path separator", func() {
		assertRefused("../escape")
	})

	It("wraps a find failure", func() {
		mockTaskStorage.FindTaskByNameReturns(nil, errors.New(ctx, "boom"))
		err = appendOp.Execute(ctx, "/vault", "Alpha", sessionA)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("find task"))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
	})

	It("wraps a write failure", func() {
		mockTaskStorage.WriteTaskReturns(errors.New(ctx, "boom"))
		err = appendOp.Execute(ctx, "/vault", "Alpha", sessionA)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("write task"))
	})
})
