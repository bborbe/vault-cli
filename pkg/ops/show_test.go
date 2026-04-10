// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("ShowOperation", func() {
	var (
		ctx             context.Context
		err             error
		detail          ops.TaskDetail
		showOp          ops.ShowOperation
		mockTaskStorage *mocks.TaskStorage
		vaultPath       string
		vaultName       string
		taskName        string
		task            *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		showOp = ops.NewShowOperation(mockTaskStorage)
		vaultPath = "/path/to/vault"
		vaultName = "my-vault"
		taskName = "my-task"

		deferDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
		plannedDate := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
		dueDate := time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)
		task = domain.NewTask(
			map[string]any{
				"status":            "in_progress",
				"phase":             "in_progress",
				"assignee":          "alice",
				"priority":          2,
				"page_type":         "task",
				"recurring":         "weekly",
				"claude_session_id": "session-abc",
				"goals":             []any{"goal-1", "goal-2"},
			},
			domain.FileMetadata{Name: taskName, FilePath: "/tmp/nonexistent-test-file.md"},
			domain.Content("---\nstatus: in_progress\n---\nDo the thing with care.\n"),
		)
		task.SetDeferDate(
			func() *domain.DateOrDateTime { d := domain.DateOrDateTime(deferDate); return &d }(),
		)
		task.SetPlannedDate(
			func() *domain.DateOrDateTime { d := domain.DateOrDateTime(plannedDate); return &d }(),
		)
		task.SetDueDate(
			func() *domain.DateOrDateTime { d := domain.DateOrDateTime(dueDate); return &d }(),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
	})

	JustBeforeEach(func() {
		detail, err = showOp.Execute(ctx, vaultPath, vaultName, taskName)
	})

	Context("with all fields populated", func() {
		It("succeeds without error", func() {
			Expect(err).To(BeNil())
		})

		It("returns task detail with correct name", func() {
			Expect(detail.Name).To(Equal(taskName))
		})
	})

	Context("with optional fields omitted", func() {
		BeforeEach(func() {
			task.SetPhase(nil)
			task.SetAssignee("")
			_ = task.SetPriority(context.Background(), 0)
			task.SetPageType("")
			task.SetRecurring("")
			task.SetClaudeSessionID("")
			task.SetGoals(nil)
			task.SetDeferDate(nil)
			task.SetPlannedDate(nil)
			task.SetDueDate(nil)
		})

		It("succeeds without error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("task not found", func() {
		BeforeEach(func() {
			mockTaskStorage.FindTaskByNameReturns(nil, errors.New("task not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find task")))
		})
	})
})

var _ = Describe("ShowOperation completed_date", func() {
	var (
		ctx             context.Context
		showOp          ops.ShowOperation
		mockTaskStorage *mocks.TaskStorage
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		showOp = ops.NewShowOperation(mockTaskStorage)
	})

	Context("with completed_date set", func() {
		BeforeEach(func() {
			task := domain.NewTask(
				map[string]any{"status": "completed", "completed_date": "2026-03-03T12:00:00Z"},
				domain.FileMetadata{Name: "done-task", FilePath: "/tmp/nonexistent-show-test.md"},
				domain.Content("---\nstatus: completed\n---\nDone.\n"),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
		})

		It("includes completed_date in result", func() {
			detail, err := showOp.Execute(ctx, "/vault", "my-vault", "done-task")
			Expect(err).To(BeNil())
			Expect(detail.CompletedDate).To(Equal("2026-03-03T12:00:00Z"))
		})
	})

	Context("without completed_date set", func() {
		BeforeEach(func() {
			task := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "todo-task", FilePath: "/tmp/nonexistent-show-test2.md"},
				domain.Content("---\nstatus: todo\n---\nNot done yet.\n"),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
		})

		It("omits completed_date from result", func() {
			detail, err := showOp.Execute(ctx, "/vault", "my-vault", "todo-task")
			Expect(err).To(BeNil())
			Expect(detail.CompletedDate).To(BeEmpty())
		})
	})
})
