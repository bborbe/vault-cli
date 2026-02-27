// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("Storage", func() {
	var (
		ctx       context.Context
		store     storage.Storage
		vaultPath string
		tasksDir  string
		goalsDir  string
		dailyDir  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = storage.NewStorage()

		// Create temp vault directory
		var err error
		vaultPath, err = os.MkdirTemp("", "vault-test-*")
		Expect(err).To(BeNil())

		tasksDir = filepath.Join(vaultPath, "Tasks")
		goalsDir = filepath.Join(vaultPath, "Goals")
		dailyDir = filepath.Join(vaultPath, "Daily Notes")

		Expect(os.MkdirAll(tasksDir, 0755)).To(Succeed())
		Expect(os.MkdirAll(goalsDir, 0755)).To(Succeed())
		Expect(os.MkdirAll(dailyDir, 0755)).To(Succeed())
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	Context("Task operations", func() {
		var taskContent string

		BeforeEach(func() {
			taskContent = `---
status: todo
page_type: task
goals:
  - Build vault-cli Core Library
priority: 1
assignee: bborbe
---
# Test Task

This is a test task.

- [ ] Subtask 1
- [ ] Subtask 2
`
			taskPath := filepath.Join(tasksDir, "Test Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent), 0600)).To(Succeed())
		})

		Describe("ReadTask", func() {
			It("reads a task successfully", func() {
				task, err := store.ReadTask(ctx, vaultPath, "Test Task")
				Expect(err).To(BeNil())
				Expect(task).NotTo(BeNil())
				Expect(task.Name).To(Equal("Test Task"))
				Expect(task.Status).To(Equal(domain.TaskStatusTodo))
				Expect(task.PageType).To(Equal("task"))
				Expect(task.Priority).To(Equal(1))
				Expect(task.Assignee).To(Equal("bborbe"))
				Expect(task.Goals).To(ContainElement("Build vault-cli Core Library"))
			})
		})

		Describe("WriteTask", func() {
			It("writes a task successfully", func() {
				task, err := store.ReadTask(ctx, vaultPath, "Test Task")
				Expect(err).To(BeNil())

				task.Status = domain.TaskStatusDone
				Expect(store.WriteTask(ctx, task)).To(Succeed())

				// Read back and verify
				updatedTask, err := store.ReadTask(ctx, vaultPath, "Test Task")
				Expect(err).To(BeNil())
				Expect(updatedTask.Status).To(Equal(domain.TaskStatusDone))
			})
		})

		Describe("FindTaskByName", func() {
			It("finds task by exact name", func() {
				task, err := store.FindTaskByName(ctx, vaultPath, "Test Task")
				Expect(err).To(BeNil())
				Expect(task).NotTo(BeNil())
				Expect(task.Name).To(Equal("Test Task"))
			})

			It("finds task by partial name", func() {
				task, err := store.FindTaskByName(ctx, vaultPath, "test")
				Expect(err).To(BeNil())
				Expect(task).NotTo(BeNil())
				Expect(task.Name).To(Equal("Test Task"))
			})

			It("returns error when task not found", func() {
				_, err := store.FindTaskByName(ctx, vaultPath, "Nonexistent")
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Context("Goal operations", func() {
		var goalContent string

		BeforeEach(func() {
			goalContent = `---
status: active
page_type: goal
theme: Development
priority: 1
---
# Test Goal

This is a test goal.

## Tasks

- [ ] Task 1
- [x] Task 2
- [ ] Task 3
`
			goalPath := filepath.Join(goalsDir, "Test Goal.md")
			Expect(os.WriteFile(goalPath, []byte(goalContent), 0600)).To(Succeed())
		})

		Describe("ReadGoal", func() {
			It("reads a goal successfully", func() {
				goal, err := store.ReadGoal(ctx, vaultPath, "Test Goal")
				Expect(err).To(BeNil())
				Expect(goal).NotTo(BeNil())
				Expect(goal.Name).To(Equal("Test Goal"))
				Expect(goal.Status).To(Equal(domain.GoalStatusActive))
				Expect(goal.Theme).To(Equal("Development"))
				Expect(len(goal.Tasks)).To(Equal(3))
				Expect(goal.Tasks[0].Checked).To(BeFalse())
				Expect(goal.Tasks[1].Checked).To(BeTrue())
			})
		})
	})

	Context("Daily note operations", func() {
		Describe("ReadDailyNote", func() {
			It("reads existing daily note", func() {
				dailyContent := "# 2024-01-01\n\n## Tasks\n\n- [ ] Task 1\n"
				notePath := filepath.Join(dailyDir, "2024-01-01.md")
				Expect(os.WriteFile(notePath, []byte(dailyContent), 0600)).To(Succeed())

				content, err := store.ReadDailyNote(ctx, vaultPath, "2024-01-01")
				Expect(err).To(BeNil())
				Expect(content).To(Equal(dailyContent))
			})

			It("returns empty string when daily note doesn't exist", func() {
				content, err := store.ReadDailyNote(ctx, vaultPath, "2024-01-01")
				Expect(err).To(BeNil())
				Expect(content).To(Equal(""))
			})
		})

		Describe("WriteDailyNote", func() {
			It("writes daily note successfully", func() {
				content := "# 2024-01-01\n\n## Tasks\n\n- [ ] Task 1\n"
				Expect(store.WriteDailyNote(ctx, vaultPath, "2024-01-01", content)).To(Succeed())

				// Read back and verify
				readContent, err := store.ReadDailyNote(ctx, vaultPath, "2024-01-01")
				Expect(err).To(BeNil())
				Expect(readContent).To(Equal(content))
			})
		})
	})
})
