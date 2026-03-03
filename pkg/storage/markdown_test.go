// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"

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

			It("returns error when writing to read-only directory", func() {
				// Create read-only directory
				readOnlyVault, err := os.MkdirTemp("", "vault-readonly-*")
				Expect(err).To(BeNil())
				defer func() { _ = os.RemoveAll(readOnlyVault) }()

				readOnlyTasksDir := filepath.Join(readOnlyVault, "Tasks")
				Expect(os.MkdirAll(readOnlyTasksDir, 0755)).To(Succeed())

				// Make directory read-only
				Expect(os.Chmod(readOnlyTasksDir, 0444)).To(Succeed())

				task := &domain.Task{
					Name:     "Read-Only Task",
					FilePath: filepath.Join(readOnlyTasksDir, "Read-Only Task.md"),
					Status:   domain.TaskStatusTodo,
				}

				err = store.WriteTask(ctx, task)
				Expect(err).NotTo(BeNil())
			})

			It("round-trips task with all fields preserved", func() {
				newTask := &domain.Task{
					Name:     "Complete Task",
					FilePath: filepath.Join(tasksDir, "Complete Task.md"),
					Status:   domain.TaskStatusInProgress,
					PageType: "task",
					Priority: 2,
					Assignee: "alice",
					Goals:    []string{"Goal A", "Goal B"},
					Content: `---
status: in_progress
page_type: task
goals:
  - Goal A
  - Goal B
priority: 2
assignee: alice
---
# Complete Task

Task with all fields.
`,
				}

				// Write task
				Expect(store.WriteTask(ctx, newTask)).To(Succeed())

				// Find by name
				found, err := store.FindTaskByName(ctx, vaultPath, "Complete Task")
				Expect(err).To(BeNil())
				Expect(found).NotTo(BeNil())
				Expect(found.Name).To(Equal("Complete Task"))
				Expect(found.Status).To(Equal(domain.TaskStatusInProgress))
				Expect(found.PageType).To(Equal("task"))
				Expect(found.Priority).To(Equal(domain.Priority(2)))
				Expect(found.Assignee).To(Equal("alice"))
				Expect(found.Goals).To(Equal([]string{"Goal A", "Goal B"}))
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

			It("skips non-.md files", func() {
				// Create a non-.md file
				txtPath := filepath.Join(tasksDir, "notes.txt")
				Expect(os.WriteFile(txtPath, []byte("not a markdown file"), 0600)).To(Succeed())

				tasks, err := store.ListTasks(ctx, vaultPath)
				Expect(err).To(BeNil())
				// Should only have the Test Task from BeforeEach
				Expect(tasks).To(HaveLen(1))
				Expect(tasks[0].Name).To(Equal("Test Task"))
			})

			It("skips files with invalid frontmatter", func() {
				// Create file with invalid frontmatter
				invalidContent := `---
status: todo
invalid yaml: [unclosed
---
# Invalid Task
`
				invalidPath := filepath.Join(tasksDir, "Invalid Task.md")
				Expect(os.WriteFile(invalidPath, []byte(invalidContent), 0600)).To(Succeed())

				// ListTasks should continue and return valid tasks
				tasks, err := store.ListTasks(ctx, vaultPath)
				Expect(err).To(BeNil())
				// Should only have the Test Task from BeforeEach
				Expect(tasks).To(HaveLen(1))
				Expect(tasks[0].Name).To(Equal("Test Task"))
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

			It("returns error when writing to read-only directory", func() {
				// Create read-only directory
				readOnlyVault, err := os.MkdirTemp("", "vault-readonly-*")
				Expect(err).To(BeNil())
				defer func() { _ = os.RemoveAll(readOnlyVault) }()

				readOnlyGoalsDir := filepath.Join(readOnlyVault, "Goals")
				Expect(os.MkdirAll(readOnlyGoalsDir, 0755)).To(Succeed())

				// Make directory read-only
				Expect(os.Chmod(readOnlyGoalsDir, 0444)).To(Succeed())

				goal := &domain.Goal{
					Name:     "Read-Only Goal",
					FilePath: filepath.Join(readOnlyGoalsDir, "Read-Only Goal.md"),
					Status:   domain.GoalStatusActive,
				}

				err = store.WriteGoal(ctx, goal)
				Expect(err).NotTo(BeNil())
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

			It("returns error when writing to read-only directory", func() {
				// Create read-only directory
				readOnlyVault, err := os.MkdirTemp("", "vault-readonly-*")
				Expect(err).To(BeNil())
				defer func() { _ = os.RemoveAll(readOnlyVault) }()

				readOnlyThemesDir := filepath.Join(readOnlyVault, "Themes")
				Expect(os.MkdirAll(readOnlyThemesDir, 0755)).To(Succeed())

				// Make directory read-only
				Expect(os.Chmod(readOnlyThemesDir, 0444)).To(Succeed())

				theme := &domain.Theme{
					Name:     "Read-Only Theme",
					FilePath: filepath.Join(readOnlyThemesDir, "Read-Only Theme.md"),
					Status:   domain.ThemeStatusActive,
				}

				err = store.WriteTheme(ctx, theme)
				Expect(err).NotTo(BeNil())
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

			It("returns error when writing to invalid path", func() {
				// Try to write to a path that cannot be created
				invalidVault := "/nonexistent/vault/path"
				content := "# 2024-01-01\n"

				err := store.WriteDailyNote(ctx, invalidVault, "2024-01-01", content)
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Context("parseFrontmatter", func() {
		It("handles file with no frontmatter markers", func() {
			content := `# Task without frontmatter

This is just plain markdown content.
`
			// Create a file without frontmatter
			noFrontmatterPath := filepath.Join(tasksDir, "No Frontmatter.md")
			Expect(os.WriteFile(noFrontmatterPath, []byte(content), 0600)).To(Succeed())

			// ReadTask should return an error since parseFrontmatter expects frontmatter
			_, err := store.ReadTask(ctx, vaultPath, "No Frontmatter")
			Expect(err).NotTo(BeNil())
		})

		It("handles file with malformed YAML in frontmatter", func() {
			content := `---
status: todo
invalid: [unclosed array
page_type: task
---
# Task with malformed YAML
`
			malformedPath := filepath.Join(tasksDir, "Malformed YAML.md")
			Expect(os.WriteFile(malformedPath, []byte(content), 0600)).To(Succeed())

			// ReadTask should return an error due to malformed YAML
			_, err := store.ReadTask(ctx, vaultPath, "Malformed YAML")
			Expect(err).NotTo(BeNil())
		})

		It("parses valid frontmatter correctly", func() {
			content := `---
status: done
page_type: task
priority: 3
assignee: bob
goals:
  - Goal X
  - Goal Y
---
# Valid Task

Task with valid frontmatter.
`
			validPath := filepath.Join(tasksDir, "Valid Frontmatter.md")
			Expect(os.WriteFile(validPath, []byte(content), 0600)).To(Succeed())

			task, err := store.ReadTask(ctx, vaultPath, "Valid Frontmatter")
			Expect(err).To(BeNil())
			Expect(task).NotTo(BeNil())
			Expect(task.Name).To(Equal("Valid Frontmatter"))
			Expect(task.Status).To(Equal(domain.TaskStatusDone))
			Expect(task.PageType).To(Equal("task"))
			Expect(task.Priority).To(Equal(domain.Priority(3)))
			Expect(task.Assignee).To(Equal("bob"))
			Expect(task.Goals).To(Equal([]string{"Goal X", "Goal Y"}))
		})
	})

	Context("Frontmatter serialization safety", func() {
		Describe("Task metadata field exclusion", func() {
			It("excludes Name, Content, FilePath from frontmatter on WriteTask", func() {
				// Create a task with both frontmatter fields and metadata fields
				task := &domain.Task{
					Name:     "Test Metadata Exclusion",
					FilePath: filepath.Join(tasksDir, "Test Metadata Exclusion.md"),
					Content: `---
status: todo
page_type: task
priority: 1
assignee: alice
---
# Test Metadata Exclusion

Task body content.
`,
					Status:   domain.TaskStatusTodo,
					PageType: "task",
					Priority: 1,
					Assignee: "alice",
				}

				// Write the task
				Expect(store.WriteTask(ctx, task)).To(Succeed())

				// Read raw file bytes
				rawBytes, err := os.ReadFile(task.FilePath)
				Expect(err).To(BeNil())
				rawContent := string(rawBytes)

				// Verify frontmatter does NOT contain name, content, or filepath keys
				Expect(rawContent).NotTo(ContainSubstring("name:"))
				Expect(rawContent).NotTo(ContainSubstring("content:"))
				Expect(rawContent).NotTo(ContainSubstring("filepath:"))

				// Verify frontmatter contains only expected YAML fields
				Expect(rawContent).To(ContainSubstring("status: todo"))
				Expect(rawContent).To(ContainSubstring("page_type: task"))
				Expect(rawContent).To(ContainSubstring("priority: 1"))
				Expect(rawContent).To(ContainSubstring("assignee: alice"))

				// Verify frontmatter structure is correct (starts with ---)
				Expect(rawContent).To(HavePrefix("---\n"))
			})

			It("prevents content embedding corruption on WriteTask", func() {
				// Create a task where Content contains a full markdown file with its own frontmatter
				contentWithFrontmatter := `---
status: todo
page_type: task
priority: 2
---
# Task with embedded frontmatter

This content itself has frontmatter delimiters.
`
				task := &domain.Task{
					Name:     "Embedded Frontmatter Test",
					FilePath: filepath.Join(tasksDir, "Embedded Frontmatter Test.md"),
					Content:  contentWithFrontmatter,
					Status:   domain.TaskStatusInProgress,
					PageType: "task",
					Priority: 2,
				}

				// Write the task
				Expect(store.WriteTask(ctx, task)).To(Succeed())

				// Read raw file bytes
				rawBytes, err := os.ReadFile(task.FilePath)
				Expect(err).To(BeNil())
				rawContent := string(rawBytes)

				// Count frontmatter delimiters (should be exactly 2: opening and closing)
				delimiterCount := 0
				lines := strings.Split(rawContent, "\n")
				for _, line := range lines {
					if line == "---" {
						delimiterCount++
					}
				}

				// Verify exactly one frontmatter block (2 delimiters)
				Expect(delimiterCount).To(Equal(2), "Should have exactly 2 '---' delimiters (one frontmatter block)")

				// Verify the frontmatter contains the struct's status (in_progress), not the content's status (todo)
				Expect(rawContent).To(ContainSubstring("status: in_progress"))
				Expect(rawContent).NotTo(ContainSubstring("status: todo"))
			})
		})

		Describe("Goal metadata field exclusion", func() {
			It("excludes Name, Content, FilePath, Tasks from frontmatter on WriteGoal", func() {
				// Create a goal with both frontmatter fields and metadata fields
				goal := &domain.Goal{
					Name:     "Test Goal Metadata Exclusion",
					FilePath: filepath.Join(goalsDir, "Test Goal Metadata Exclusion.md"),
					Content: `---
status: active
page_type: goal
theme: Testing
priority: 1
---
# Test Goal Metadata Exclusion

Goal body content.

- [ ] Task 1
- [x] Task 2
`,
					Status:   domain.GoalStatusActive,
					PageType: "goal",
					Theme:    "Testing",
					Priority: 1,
					Tasks: []domain.CheckboxItem{
						{Line: 7, Checked: false, Text: "Task 1"},
						{Line: 8, Checked: true, Text: "Task 2"},
					},
				}

				// Write the goal
				Expect(store.WriteGoal(ctx, goal)).To(Succeed())

				// Read raw file bytes
				rawBytes, err := os.ReadFile(goal.FilePath)
				Expect(err).To(BeNil())
				rawContent := string(rawBytes)

				// Verify frontmatter does NOT contain name, content, filepath, or tasks keys
				Expect(rawContent).NotTo(ContainSubstring("name:"))
				Expect(rawContent).NotTo(ContainSubstring("content:"))
				Expect(rawContent).NotTo(ContainSubstring("filepath:"))
				Expect(rawContent).NotTo(ContainSubstring("tasks:"))

				// Verify frontmatter contains only expected YAML fields
				Expect(rawContent).To(ContainSubstring("status: active"))
				Expect(rawContent).To(ContainSubstring("page_type: goal"))
				Expect(rawContent).To(ContainSubstring("theme: Testing"))
				Expect(rawContent).To(ContainSubstring("priority: 1"))
			})
		})

		Describe("Theme metadata field exclusion", func() {
			var themesDir string

			BeforeEach(func() {
				themesDir = filepath.Join(vaultPath, "Themes")
				Expect(os.MkdirAll(themesDir, 0755)).To(Succeed())
			})

			It("excludes Name, Content, FilePath from frontmatter on WriteTheme", func() {
				// Create a theme with both frontmatter fields and metadata fields
				theme := &domain.Theme{
					Name:     "Test Theme Metadata Exclusion",
					FilePath: filepath.Join(themesDir, "Test Theme Metadata Exclusion.md"),
					Content: `---
status: active
page_type: theme
priority: 1
assignee: bob
---
# Test Theme Metadata Exclusion

Theme body content.
`,
					Status:   domain.ThemeStatusActive,
					PageType: "theme",
					Priority: 1,
					Assignee: "bob",
				}

				// Write the theme
				Expect(store.WriteTheme(ctx, theme)).To(Succeed())

				// Read raw file bytes
				rawBytes, err := os.ReadFile(theme.FilePath)
				Expect(err).To(BeNil())
				rawContent := string(rawBytes)

				// Verify frontmatter does NOT contain name, content, or filepath keys
				Expect(rawContent).NotTo(ContainSubstring("name:"))
				Expect(rawContent).NotTo(ContainSubstring("content:"))
				Expect(rawContent).NotTo(ContainSubstring("filepath:"))

				// Verify frontmatter contains only expected YAML fields
				Expect(rawContent).To(ContainSubstring("status: active"))
				Expect(rawContent).To(ContainSubstring("page_type: theme"))
				Expect(rawContent).To(ContainSubstring("priority: 1"))
				Expect(rawContent).To(ContainSubstring("assignee: bob"))
			})
		})
	})
})
