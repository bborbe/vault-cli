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

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("NewConfigFromVault", func() {
	It("creates config from vault with custom directories", func() {
		vault := &config.Vault{
			TasksDir:      "My Tasks",
			GoalsDir:      "My Goals",
			ThemesDir:     "My Themes",
			ObjectivesDir: "My Objectives",
			VisionDir:     "My Vision",
			DailyDir:      "My Daily",
		}

		cfg := storage.NewConfigFromVault(vault)
		Expect(cfg.TasksDir).To(Equal("My Tasks"))
		Expect(cfg.GoalsDir).To(Equal("My Goals"))
		Expect(cfg.ThemesDir).To(Equal("My Themes"))
		Expect(cfg.ObjectivesDir).To(Equal("My Objectives"))
		Expect(cfg.VisionDir).To(Equal("My Vision"))
		Expect(cfg.DailyDir).To(Equal("My Daily"))
	})

	It("uses default directories when vault dirs are empty", func() {
		vault := &config.Vault{}

		cfg := storage.NewConfigFromVault(vault)
		Expect(cfg.TasksDir).To(Equal("Tasks"))
		Expect(cfg.GoalsDir).To(Equal("Goals"))
		Expect(cfg.ThemesDir).To(Equal("21 Themes"))
		Expect(cfg.ObjectivesDir).To(Equal("22 Objectives"))
		Expect(cfg.VisionDir).To(Equal("20 Vision"))
		Expect(cfg.DailyDir).To(Equal("Daily Notes"))
	})
})

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
		store = storage.NewStorage(nil) // Use default config

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
				Expect(task.Priority).To(Equal(domain.Priority(1)))
				Expect(task.Assignee).To(Equal("bborbe"))
				Expect(task.Goals).To(ContainElement("Build vault-cli Core Library"))
			})

			It("reads a task with string priority as -1 (resilient parsing)", func() {
				// Create a task with string priority value
				taskContent := `---
status: todo
page_type: task
priority: medium
assignee: bborbe
---
# Task with String Priority

This task has a string priority value that should parse as -1.
`
				taskPath := filepath.Join(tasksDir, "String Priority Task.md")
				Expect(os.WriteFile(taskPath, []byte(taskContent), 0600)).To(Succeed())

				task, err := store.ReadTask(ctx, vaultPath, "String Priority Task")
				Expect(err).To(BeNil())
				Expect(task).NotTo(BeNil())
				Expect(task.Name).To(Equal("String Priority Task"))
				Expect(task.Status).To(Equal(domain.TaskStatusTodo))
				Expect(task.Priority).To(Equal(domain.Priority(-1)))
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

		Describe("ListTasks", func() {
			It("lists all tasks in vault", func() {
				// Create another task
				taskContent2 := `---
status: in_progress
page_type: task
---
# Second Task

Another test task.
`
				taskPath2 := filepath.Join(tasksDir, "Second Task.md")
				Expect(os.WriteFile(taskPath2, []byte(taskContent2), 0600)).To(Succeed())

				tasks, err := store.ListTasks(ctx, vaultPath)
				Expect(err).To(BeNil())
				Expect(tasks).To(HaveLen(2))

				names := []string{tasks[0].Name, tasks[1].Name}
				Expect(names).To(ContainElement("Test Task"))
				Expect(names).To(ContainElement("Second Task"))
			})

			It("returns empty list when no tasks exist", func() {
				// Create a fresh temp vault
				emptyVault, err := os.MkdirTemp("", "empty-vault-*")
				Expect(err).To(BeNil())
				defer func() { _ = os.RemoveAll(emptyVault) }()

				emptyTasksDir := filepath.Join(emptyVault, "Tasks")
				Expect(os.MkdirAll(emptyTasksDir, 0755)).To(Succeed())

				tasks, err := store.ListTasks(ctx, emptyVault)
				Expect(err).To(BeNil())
				Expect(tasks).To(HaveLen(0))
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

		Describe("WriteGoal and FindGoalByName", func() {
			It("round-trips goal correctly", func() {
				newGoal := &domain.Goal{
					Name:     "New Goal",
					FilePath: filepath.Join(goalsDir, "New Goal.md"),
					Status:   domain.GoalStatusActive,
					Theme:    "Testing",
				}

				// Write goal
				Expect(store.WriteGoal(ctx, newGoal)).To(Succeed())

				// Find by name
				found, err := store.FindGoalByName(ctx, vaultPath, "New Goal")
				Expect(err).To(BeNil())
				Expect(found).NotTo(BeNil())
				Expect(found.Name).To(Equal("New Goal"))
				Expect(found.Status).To(Equal(domain.GoalStatusActive))
				Expect(found.Theme).To(Equal("Testing"))
			})

			It("finds goal by partial name", func() {
				// First write a goal
				newGoal := &domain.Goal{
					Name:     "Unique Goal Name",
					FilePath: filepath.Join(goalsDir, "Unique Goal Name.md"),
					Status:   domain.GoalStatusActive,
				}
				Expect(store.WriteGoal(ctx, newGoal)).To(Succeed())

				// Find by partial name
				found, err := store.FindGoalByName(ctx, vaultPath, "unique")
				Expect(err).To(BeNil())
				Expect(found.Name).To(Equal("Unique Goal Name"))
			})
		})
	})

	Context("Theme operations", func() {
		var themesDir string

		BeforeEach(func() {
			// Note: ReadTheme hardcodes "Themes" directory, not config.ThemesDir
			themesDir = filepath.Join(vaultPath, "Themes")
			Expect(os.MkdirAll(themesDir, 0755)).To(Succeed())
		})

		Describe("WriteTheme and ReadTheme", func() {
			It("round-trips theme correctly", func() {
				themePath := filepath.Join(themesDir, "Health & Fitness.md")
				newTheme := &domain.Theme{
					Name:     "Health & Fitness",
					FilePath: themePath,
					Status:   domain.ThemeStatusActive,
				}

				// Write theme
				Expect(store.WriteTheme(ctx, newTheme)).To(Succeed())

				// Read back
				read, err := store.ReadTheme(ctx, vaultPath, "Health & Fitness")
				Expect(err).To(BeNil())
				Expect(read).NotTo(BeNil())
				Expect(read.Name).To(Equal("Health & Fitness"))
				Expect(read.Status).To(Equal(domain.ThemeStatusActive))
			})
		})
	})

	Context("ListPages", func() {
		var customDir string

		BeforeEach(func() {
			customDir = filepath.Join(vaultPath, "Custom Pages")
			Expect(os.MkdirAll(customDir, 0755)).To(Succeed())
		})

		It("lists all pages in directory", func() {
			// Create multiple pages
			page1 := `---
status: todo
page_type: task
---
# Page 1
`
			page2 := `---
status: in_progress
page_type: task
---
# Page 2
`
			Expect(
				os.WriteFile(filepath.Join(customDir, "Page 1.md"), []byte(page1), 0600),
			).To(Succeed())
			Expect(
				os.WriteFile(filepath.Join(customDir, "Page 2.md"), []byte(page2), 0600),
			).To(Succeed())

			pages, err := store.ListPages(ctx, vaultPath, "Custom Pages")
			Expect(err).To(BeNil())
			Expect(pages).To(HaveLen(2))

			names := []string{pages[0].Name, pages[1].Name}
			Expect(names).To(ContainElement("Page 1"))
			Expect(names).To(ContainElement("Page 2"))
		})

		It("returns empty list for empty directory", func() {
			emptyDir := filepath.Join(vaultPath, "Empty")
			Expect(os.MkdirAll(emptyDir, 0755)).To(Succeed())

			pages, err := store.ListPages(ctx, vaultPath, "Empty")
			Expect(err).To(BeNil())
			Expect(pages).To(HaveLen(0))
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
