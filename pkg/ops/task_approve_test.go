// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	stderrors "errors"
	"time"

	liberrors "github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("TaskApproveOperation", func() {
	var (
		ctx         context.Context
		mockStorage *mocks.Storage
		pinned      libtime.CurrentDateTime
		approveOp   ops.TaskApproveOperation
	)

	// seedTask replaces the task the storage returns, so a spec can control the
	// phase and the pre-existing frontmatter the approve guards inspect.
	seedTask := func(fields map[string]any) {
		mockStorage.FindTaskByNameReturns(
			domain.NewTask(fields, domain.FileMetadata{Name: "Alpha"}, domain.Content("")),
			nil,
		)
	}

	BeforeEach(func() {
		ctx = context.Background()
		mockStorage = &mocks.Storage{}
		mockStorage.WriteTaskReturns(nil)

		pinned = libtime.NewCurrentDateTime()
		pinned.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
		approveOp = ops.NewTaskApproveOperation(mockStorage, pinned)

		seedTask(map[string]any{"status": "next", "phase": "todo", "assignee": "someone"})
	})

	It("writes all three approval keys in exactly one write", func() {
		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())
		Expect(result.Name).To(Equal("Alpha"))
		Expect(result.Vault).To(Equal("vault-a"))

		Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Status()).To(Equal(domain.TaskStatusNext))
		Expect(written.Phase()).To(Equal(domain.TaskPhasePlanning.Ptr()))
		Expect(written.GetString("approved_by")).To(Equal("operator"))
		Expect(written.Get("approved_at")).NotTo(BeNil())
	})

	It("preserves an in_progress status across the approval", func() {
		seedTask(map[string]any{"status": "in_progress", "phase": "todo", "assignee": "someone"})

		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())

		Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Status()).To(Equal(domain.TaskStatusInProgress))
		Expect(written.Phase()).To(Equal(domain.TaskPhasePlanning.Ptr()))
	})

	It("records the injected instant as a bare time.Time", func() {
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		raw, ok := written.Get("approved_at").(time.Time)
		Expect(ok).To(BeTrue())
		Expect(raw.Equal(pinned.Now().Time())).To(BeTrue())
	})

	It("serializes approved_at unquoted and round-trips it as a time.Time", func() {
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		out, marshalErr := yaml.Marshal(written.RawMap())
		Expect(marshalErr).NotTo(HaveOccurred())
		Expect(string(out)).To(ContainSubstring("approved_at: 2026-03-03T12:00:00Z"))
		Expect(string(out)).NotTo(ContainSubstring(`approved_at: "2026-03-03T12:00:00Z"`))

		var roundTripped map[string]any
		Expect(yaml.Unmarshal(out, &roundTripped)).To(Succeed())
		rt, ok := roundTripped["approved_at"].(time.Time)
		Expect(ok).To(BeTrue())
		Expect(rt.Equal(pinned.Now().Time())).To(BeTrue())
	})

	It("writes the approver named on the call", func() {
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "Manager Layer", "", "")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.GetString("approved_by")).To(Equal("Manager Layer"))
	})

	It("preserves every other frontmatter key", func() {
		seedTask(map[string]any{
			"status":          "next",
			"phase":           "todo",
			"assignee":        "someone",
			"priority":        1,
			"page_type":       "task",
			"task_identifier": "22222222-2222-2222-2222-222222222222",
			"custom_key":      "keep",
		})

		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Assignee()).To(Equal("someone"))
		Expect(written.Priority()).To(Equal(domain.Priority(1)))
		Expect(written.PageType()).To(Equal("task"))
		Expect(written.TaskIdentifier()).To(Equal("22222222-2222-2222-2222-222222222222"))
		Expect(written.Get("custom_key")).To(Equal("keep"))
	})

	// The ownership rule: approval is the one moment an owner is fixed. The
	// precedence is flag, then the task's own assignee, then the configured
	// current user — and a task that resolves to no owner is refused unwritten.

	It("AC1: fills an empty assignee from the configured current user", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo"})

		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "bborbe")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())

		Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Assignee()).To(Equal("bborbe"))
	})

	It("AC2: keeps an existing assignee over the configured current user", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo", "assignee": "someone-else"})

		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "bborbe")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Assignee()).To(Equal("someone-else"))
	})

	It("AC3: the flag overrides an empty assignee", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo"})

		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "X", "bborbe")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Assignee()).To(Equal("X"))
	})

	It("AC3: the flag overrides an existing assignee", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo", "assignee": "someone-else"})

		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "X", "bborbe")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Assignee()).To(Equal("X"))
	})

	It("AC4: refuses to approve an unowned task with zero writes", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo"})

		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("assignee"))
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		Expect(mockStorage.FindTaskByNameCallCount()).To(Equal(1))
	})

	It("refuses an empty approver before reading the task", func() {
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("approved_by"))
		Expect(mockStorage.FindTaskByNameCallCount()).To(Equal(0))
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
	})

	It("refuses a zero clock with zero writes", func() {
		zero := libtime.NewCurrentDateTime()
		zero.SetNow(libtime.DateTime(time.Time{}))
		zeroOp := ops.NewTaskApproveOperation(mockStorage, zero)

		_, execErr := zeroOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("approved_at"))
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
	})

	DescribeTable("refuses a task that is not waiting in the inbox",
		func(fields map[string]any, expectedPhase string) {
			seedTask(fields)

			_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(HaveOccurred())
			Expect(execErr.Error()).To(ContainSubstring(expectedPhase))
			Expect(execErr.Error()).To(ContainSubstring("todo"))
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		},
		Entry("planning", map[string]any{"status": "next", "phase": "planning"}, "planning"),
		Entry("execution", map[string]any{"status": "next", "phase": "execution"}, "execution"),
		Entry("ai_review", map[string]any{"status": "next", "phase": "ai_review"}, "ai_review"),
		Entry("human_review", map[string]any{"status": "next", "phase": "human_review"}, "human_review"),
		Entry("done", map[string]any{"status": "next", "phase": "done"}, "done"),
		Entry("no phase key at all", map[string]any{"status": "next"}, "(none)"),
	)

	DescribeTable("refuses a todo task that already carries an approval record",
		func(fields map[string]any, expectedKeys string) {
			seedTask(fields)

			_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(HaveOccurred())
			Expect(execErr.Error()).To(ContainSubstring(expectedKeys))
			Expect(execErr.Error()).To(ContainSubstring("re-approving"))
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		},
		Entry("only approved_by",
			map[string]any{"status": "next", "phase": "todo", "approved_by": "someone"},
			"approved_by",
		),
		Entry("only approved_at",
			map[string]any{
				"status":      "next",
				"phase":       "todo",
				"approved_at": "2026-01-01T00:00:00Z",
			},
			"approved_at",
		),
		Entry("both keys",
			map[string]any{
				"status":      "next",
				"phase":       "todo",
				"approved_by": "someone",
				"approved_at": "2026-01-01T00:00:00Z",
			},
			"approved_by, approved_at",
		),
	)

	It("wraps a find failure", func() {
		mockStorage.FindTaskByNameReturns(nil, liberrors.New(ctx, "boom"))
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("find task"))
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
	})

	It("wraps a write failure", func() {
		mockStorage.WriteTaskReturns(liberrors.New(ctx, "boom"))
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("write task"))
	})

	It("preserves the not-found class", func() {
		mockStorage.FindTaskByNameReturns(nil, storage.ErrNotFound)
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(stderrors.Is(execErr, storage.ErrNotFound)).To(BeTrue())
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
	})
})
