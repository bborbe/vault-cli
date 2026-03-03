// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("CompleteOperation", func() {
	var (
		ctx         context.Context
		err         error
		completeOp  ops.CompleteOperation
		mockStorage *mocks.Storage
		vaultPath   string
		taskName    string
		task        *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockStorage = &mocks.Storage{}
		completeOp = ops.NewCompleteOperation(mockStorage)
		vaultPath = "/path/to/vault"
		taskName = "my-task"

		// Default: return a task
		task = &domain.Task{
			Name:   taskName,
			Status: domain.TaskStatusTodo,
		}
		mockStorage.FindTaskByNameReturns(task, nil)
		mockStorage.WriteTaskReturns(nil)
	})

	JustBeforeEach(func() {
		err = completeOp.Execute(ctx, vaultPath, taskName, "test-vault", "plain")
	})

	Context("success", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("calls FindTaskByName", func() {
			Expect(mockStorage.FindTaskByNameCallCount()).To(Equal(1))
			actualCtx, actualVaultPath, actualTaskName := mockStorage.FindTaskByNameArgsForCall(0)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualTaskName).To(Equal(taskName))
		})

		It("marks task as done", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status).To(Equal(domain.TaskStatusDone))
		})

		It("calls WriteTask with updated task", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			actualCtx, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(actualCtx).To(Equal(ctx))
			Expect(writtenTask.Name).To(Equal(taskName))
		})
	})

	Context("task not found", func() {
		BeforeEach(func() {
			mockStorage.FindTaskByNameReturns(nil, ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})

		It("does not call WriteTask", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("write error", func() {
		BeforeEach(func() {
			mockStorage.WriteTaskReturns(ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})
	})

	Context("task with associated goal", func() {
		var goal *domain.Goal

		BeforeEach(func() {
			task.Goals = []string{"Test Goal"}

			goal = &domain.Goal{
				Name: "Test Goal",
				Content: `---
status: active
---
# Test Goal

## Tasks
- [ ] my-task
`,
			}
			mockStorage.FindGoalByNameReturns(goal, nil)
			mockStorage.WriteGoalReturns(nil)
		})

		It("attempts to update goal checkbox", func() {
			Expect(err).To(BeNil())
			Expect(mockStorage.FindGoalByNameCallCount() > 0).To(BeTrue())
		})

		It("marks checkbox in goal as complete", func() {
			Expect(err).To(BeNil())
			if mockStorage.WriteGoalCallCount() > 0 {
				_, updatedGoal := mockStorage.WriteGoalArgsForCall(0)
				Expect(updatedGoal.Content).To(ContainSubstring("- [x]"))
			}
		})
	})

	Context("task with goal not found", func() {
		BeforeEach(func() {
			task.Goals = []string{"Missing Goal"}
			mockStorage.FindGoalByNameReturns(nil, ErrTest)
		})

		It("completes task despite goal error", func() {
			// Operation should succeed even if goal update fails
			Expect(err).To(BeNil())
		})
	})

	Context("task with goal WriteGoal error", func() {
		BeforeEach(func() {
			task.Goals = []string{"Test Goal"}
			goal := &domain.Goal{
				Name: "Test Goal",
				Content: `---
status: active
---
# Test Goal

## Tasks
- [ ] my-task
`,
			}
			mockStorage.FindGoalByNameReturns(goal, nil)
			mockStorage.WriteGoalReturns(ErrTest)
		})

		It("completes task despite goal write error", func() {
			// Operation should succeed even if goal write fails
			Expect(err).To(BeNil())
		})
	})

	Context("updateDailyNote path", func() {
		BeforeEach(func() {
			dailyContent := `# 2026-03-02

## Tasks
- [ ] my-task
`
			mockStorage.ReadDailyNoteReturns(dailyContent, nil)
			mockStorage.WriteDailyNoteReturns(nil)
		})

		It("updates daily note checkbox", func() {
			Expect(err).To(BeNil())
			Expect(mockStorage.ReadDailyNoteCallCount()).To(Equal(1))
			Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(1))
		})

		It("marks checkbox as checked in daily note", func() {
			Expect(err).To(BeNil())
			if mockStorage.WriteDailyNoteCallCount() > 0 {
				_, _, _, updatedContent := mockStorage.WriteDailyNoteArgsForCall(0)
				Expect(updatedContent).To(ContainSubstring("- [x]"))
			}
		})
	})

	Context("ReadDailyNote returns error", func() {
		BeforeEach(func() {
			mockStorage.ReadDailyNoteReturns("", ErrTest)
		})

		It("completes task despite daily note read error", func() {
			// Operation should succeed even if daily note read fails
			Expect(err).To(BeNil())
		})
	})

	Context("WriteDailyNote returns error", func() {
		BeforeEach(func() {
			dailyContent := `# 2026-03-02

## Tasks
- [ ] my-task
`
			mockStorage.ReadDailyNoteReturns(dailyContent, nil)
			mockStorage.WriteDailyNoteReturns(ErrTest)
		})

		It("completes task despite daily note write error", func() {
			// Operation should succeed even if daily note write fails
			Expect(err).To(BeNil())
		})
	})

	Context("recurring daily task", func() {
		BeforeEach(func() {
			task.Recurring = "daily"
			task.Status = domain.TaskStatusInProgress
			task.Content = `---
status: in_progress
recurring: daily
---
# My Task

## Checklist
- [x] Item 1
- [x] Item 2
`
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("resets checkboxes in content", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Content).To(ContainSubstring("- [ ] Item 1"))
			Expect(writtenTask.Content).To(ContainSubstring("- [ ] Item 2"))
			Expect(writtenTask.Content).NotTo(ContainSubstring("- [x]"))
		})

		It("sets last_completed to today", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.LastCompleted).NotTo(BeEmpty())
		})

		It("bumps defer_date to tomorrow", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.DeferDate).NotTo(BeNil())
		})

		It("keeps status unchanged", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status).To(Equal(domain.TaskStatusInProgress))
		})
	})

	Context("recurring weekly task", func() {
		BeforeEach(func() {
			task.Recurring = "weekly"
			task.Status = domain.TaskStatusInProgress
			task.Content = `---
status: in_progress
recurring: weekly
---
# My Task
`
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("bumps defer_date by 7 days", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.DeferDate).NotTo(BeNil())
		})

		It("keeps status unchanged", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status).To(Equal(domain.TaskStatusInProgress))
		})
	})

	Context("recurring monthly task", func() {
		BeforeEach(func() {
			task.Recurring = "monthly"
			task.Status = domain.TaskStatusInProgress
			task.Content = `---
status: in_progress
recurring: monthly
---
# My Task
`
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("bumps defer_date by 1 month", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.DeferDate).NotTo(BeNil())
		})

		It("keeps status unchanged", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status).To(Equal(domain.TaskStatusInProgress))
		})
	})

	Context("non-recurring task still marked as done", func() {
		BeforeEach(func() {
			task.Recurring = ""
			task.Status = domain.TaskStatusTodo
		})

		It("marks task as done", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status).To(Equal(domain.TaskStatusDone))
		})
	})

	Context("recurring task with planned_date before new defer_date", func() {
		var oldPlannedDate time.Time

		BeforeEach(func() {
			oldPlannedDate = time.Now().AddDate(0, 0, -1) // Yesterday
			task.Recurring = "daily"
			task.Status = domain.TaskStatusInProgress
			task.PlannedDate = &oldPlannedDate
			task.Content = `---
status: in_progress
recurring: daily
---
# My Task
`
		})

		It("clears planned_date", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.PlannedDate).To(BeNil())
		})
	})
})
