// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("task set refusal for metrics_sessions", func() {
	var (
		ctx             context.Context
		err             error
		setOp           ops.FrontmatterSetOperation
		mockTaskStorage *mocks.TaskStorage
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		setOp = ops.NewFrontmatterSetOperation(
			mockTaskStorage,
			ops.NewEscalationPublisher("", "", &mocks.NotificationSenderFactory{}),
			"personal",
			"25 Tasks",
		)
		mockTaskStorage.FindTaskByNameReturns(
			domain.NewTask(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: "Alpha"},
				domain.Content(""),
			),
			nil,
		)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	It("task set refuses metrics_sessions and writes nothing", func() {
		err = setOp.Execute(
			ctx, "/vault", "Alpha", "metrics_sessions", `{"session_id":"x"}`, "", "", false,
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("metrics_sessions"))
		Expect(err.Error()).To(ContainSubstring("append-metrics-session"))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
	})

	It("task set refusal is not bypassed by force", func() {
		err = setOp.Execute(
			ctx, "/vault", "Alpha", "metrics_sessions", `{"session_id":"x"}`, "", "", true,
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("metrics_sessions"))
		Expect(err.Error()).To(ContainSubstring("append-metrics-session"))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
	})
})

var _ = Describe("task list refusal for metrics_sessions", func() {
	var (
		ctx             context.Context
		err             error
		addOp           ops.EntityListAddOperation
		removeOp        ops.EntityListRemoveOperation
		mockTaskStorage *mocks.TaskStorage
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		addOp = ops.NewTaskListAddOperation(mockTaskStorage)
		removeOp = ops.NewTaskListRemoveOperation(mockTaskStorage)
		mockTaskStorage.FindTaskByNameReturns(
			domain.NewTask(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: "Alpha"},
				domain.Content(""),
			),
			nil,
		)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	It("task add refuses metrics_sessions and writes nothing", func() {
		err = addOp.Execute(ctx, "/vault", "Alpha", "metrics_sessions", "x")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("metrics_sessions"))
		Expect(err.Error()).To(ContainSubstring("append-metrics-session"))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
	})

	It("task remove refuses metrics_sessions and writes nothing", func() {
		err = removeOp.Execute(ctx, "/vault", "Alpha", "metrics_sessions", "x")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("metrics_sessions"))
		Expect(err.Error()).To(ContainSubstring("append-metrics-session"))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
	})

	It("leaves the tags and goals list mutations untouched", func() {
		// `add` appends its argument as one list element — the comma-split
		// coercion lives on the `set` path, not here. The point of this spec is
		// that the metrics_sessions refusal did not swallow the neighbouring
		// list fields.
		err = addOp.Execute(ctx, "/vault", "Alpha", "tags", "a")
		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		_, written := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(written.Tags()).To(Equal([]string{"a"}))

		err = addOp.Execute(ctx, "/vault", "Alpha", "goals", "g1")
		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(2))
		_, written = mockTaskStorage.WriteTaskArgsForCall(1)
		Expect(written.Goals()).To(Equal([]string{"g1"}))
	})
})
