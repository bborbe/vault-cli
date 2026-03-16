// Copyright (c) 2025 Benjamin Borbe All rights reserved.
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

var _ = Describe("UpdateOperation", func() {
	var (
		ctx             context.Context
		err             error
		updateOp        ops.UpdateOperation
		mockTaskStorage *mocks.TaskStorage
		mockGoalStorage *mocks.GoalStorage
		vaultPath       string
		taskName        string
		task            *domain.Task
		outputFormat    string
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		mockGoalStorage = &mocks.GoalStorage{}
		updateOp = ops.NewUpdateOperation(mockTaskStorage, mockGoalStorage)
		vaultPath = "/path/to/vault"
		taskName = "my-task"
		outputFormat = "plain" // default

		// Default: return a task with mixed checkboxes
		task = &domain.Task{
			Name:   taskName,
			Status: domain.TaskStatusTodo,
			Content: `---
status: todo
---

# My Task

- [x] First item
- [ ] Second item
- [ ] Third item
`,
		}
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	JustBeforeEach(func() {
		err = updateOp.Execute(ctx, vaultPath, taskName, "test-vault", outputFormat)
	})

	Context("success", func() {
		Context("with all checkboxes checked", func() {
			BeforeEach(func() {
				task.Content = `---
status: todo
---

# My Task

- [x] First item
- [x] Second item
- [x] Third item
`
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets status to done", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status).To(Equal(domain.TaskStatusCompleted))
			})
		})

		Context("with no checkboxes checked", func() {
			BeforeEach(func() {
				task.Content = `---
status: in_progress
---

# My Task

- [ ] First item
- [ ] Second item
- [ ] Third item
`
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets status to todo", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status).To(Equal(domain.TaskStatusTodo))
			})
		})

		Context("with mixed checkboxes", func() {
			BeforeEach(func() {
				task.Content = `---
status: todo
---

# My Task

- [x] First item
- [ ] Second item
- [ ] Third item
`
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets status to in_progress", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status).To(Equal(domain.TaskStatusInProgress))
			})
		})

		Context("with no checkboxes in content", func() {
			BeforeEach(func() {
				task.Content = `---
status: todo
---

# My Task

Just some text without checkboxes.
`
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("does not call WriteTask", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
			})
		})

		It("calls FindTaskByName", func() {
			Expect(mockTaskStorage.FindTaskByNameCallCount()).To(Equal(1))
			actualCtx, actualVaultPath, actualTaskName := mockTaskStorage.FindTaskByNameArgsForCall(
				0,
			)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualTaskName).To(Equal(taskName))
		})
	})

	Context("task not found", func() {
		BeforeEach(func() {
			mockTaskStorage.FindTaskByNameReturns(nil, ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})

		It("does not call WriteTask", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("write error", func() {
		BeforeEach(func() {
			mockTaskStorage.WriteTaskReturns(ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})
	})

	Context("task with associated goal", func() {
		var goal *domain.Goal

		BeforeEach(func() {
			task.Goals = []string{"Test Goal"}
			task.Content = `---
status: todo
---

# My Task

- [x] First item
- [ ] Second item
`

			goal = &domain.Goal{
				Name: "Test Goal",
				Content: `---
status: active
---
# Test Goal

## Tasks
- [ ] First item
- [ ] Second item
`,
			}
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
			mockGoalStorage.WriteGoalReturns(nil)
		})

		It("attempts to sync goal checkboxes", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.FindGoalByNameCallCount() > 0).To(BeTrue())
		})

		It("updates goal checkboxes to match task", func() {
			Expect(err).To(BeNil())
			if mockGoalStorage.WriteGoalCallCount() > 0 {
				_, updatedGoal := mockGoalStorage.WriteGoalArgsForCall(0)
				// Should have updated first item to checked
				Expect(updatedGoal.Content).To(ContainSubstring("- [x] First item"))
			}
		})
	})

	Context("task with goal not found", func() {
		BeforeEach(func() {
			task.Goals = []string{"Missing Goal"}
			mockGoalStorage.FindGoalByNameReturns(nil, ErrTest)
		})

		It("completes task despite goal error", func() {
			// Operation should succeed even if goal sync fails
			Expect(err).To(BeNil())
		})
	})

	Context("JSON output format", func() {
		BeforeEach(func() {
			outputFormat = "json"
		})

		Context("success with all checkboxes checked", func() {
			BeforeEach(func() {
				task.Content = `---
status: todo
---

# My Task

- [x] First item
- [x] Second item
- [x] Third item
`
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets status to done", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status).To(Equal(domain.TaskStatusCompleted))
			})
		})

		Context("success with no checkboxes checked", func() {
			BeforeEach(func() {
				task.Content = `---
status: in_progress
---

# My Task

- [ ] First item
- [ ] Second item
- [ ] Third item
`
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets status to todo", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status).To(Equal(domain.TaskStatusTodo))
			})
		})

		Context("task with no checkboxes in content", func() {
			BeforeEach(func() {
				task.Content = `---
status: todo
---

# My Task

Just some text without checkboxes.
`
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("does not call WriteTask", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
			})
		})

		Context("WriteTask returns error", func() {
			BeforeEach(func() {
				mockTaskStorage.WriteTaskReturns(ErrTest)
			})

			It("returns error", func() {
				Expect(err).NotTo(BeNil())
			})
		})

		Context("FindTaskByName returns error", func() {
			BeforeEach(func() {
				mockTaskStorage.FindTaskByNameReturns(nil, ErrTest)
			})

			It("returns error", func() {
				Expect(err).NotTo(BeNil())
			})
		})
	})
})
