// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

// createTempVault creates a temporary vault with tasks, goals, and config file
func createTempVault(
	tasks map[string]string,
) (vaultPath string, configPath string, cleanup func()) {
	return createTempVaultWithGoals(tasks, nil)
}

// createTempVaultWithGoals creates a temporary vault with tasks, goals, and config file
func createTempVaultWithGoals(
	tasks map[string]string,
	goals map[string]string,
) (vaultPath string, configPath string, cleanup func()) {
	var err error
	vaultPath, err = os.MkdirTemp("", "vault-*")
	Expect(err).NotTo(HaveOccurred())

	tasksDir := filepath.Join(vaultPath, "Tasks")
	err = os.MkdirAll(tasksDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	for name, content := range tasks {
		taskPath := filepath.Join(tasksDir, name+".md")
		err = os.WriteFile(taskPath, []byte(content), 0600)
		Expect(err).NotTo(HaveOccurred())
	}

	if goals != nil {
		goalsDir := filepath.Join(vaultPath, "Goals")
		err = os.MkdirAll(goalsDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		for name, content := range goals {
			goalPath := filepath.Join(goalsDir, name+".md")
			err = os.WriteFile(goalPath, []byte(content), 0600)
			Expect(err).NotTo(HaveOccurred())
		}
	}

	configContent := fmt.Sprintf(`default_vault: test
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
    goals_dir: Goals
`, vaultPath)

	configFile, err := os.CreateTemp("", "vault-config-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	_, err = configFile.WriteString(configContent)
	Expect(err).NotTo(HaveOccurred())
	err = configFile.Close()
	Expect(err).NotTo(HaveOccurred())

	return vaultPath, configFile.Name(), func() {
		_ = os.RemoveAll(vaultPath)
		_ = os.Remove(configFile.Name())
	}
}

var _ = Describe("vault-cli integration tests", func() {
	Describe("vault-cli --help", func() {
		It("exits 0 and shows help text", func() {
			cmd := exec.Command(binPath, "--help")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("vault-cli"))
		})
	})

	Describe("command registration", func() {
		DescribeTable("exits 0 for --help",
			func(args ...string) {
				helpArgs := append(args, "--help")
				cmd := exec.Command(binPath, helpArgs...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
			},
			// Task subcommands
			Entry("task list", "task", "list"),
			Entry("task show", "task", "show"),
			Entry("task complete", "task", "complete"),
			Entry("task defer", "task", "defer"),
			Entry("task update", "task", "update"),
			Entry("task work-on", "task", "work-on"),
			Entry("task get", "task", "get"),
			Entry("task set", "task", "set"),
			Entry("task clear", "task", "clear"),
			Entry("task lint", "task", "lint"),
			Entry("task validate", "task", "validate"),
			Entry("task search", "task", "search"),
			Entry("task watch", "task", "watch"),
			Entry("task add", "task", "add"),
			Entry("task remove", "task", "remove"),
			// Goal subcommands
			Entry("goal list", "goal", "list"),
			Entry("goal lint", "goal", "lint"),
			Entry("goal search", "goal", "search"),
			Entry("goal show", "goal", "show"),
			Entry("goal get", "goal", "get"),
			Entry("goal set", "goal", "set"),
			Entry("goal clear", "goal", "clear"),
			Entry("goal complete", "goal", "complete"),
			Entry("goal add", "goal", "add"),
			Entry("goal remove", "goal", "remove"),
			// Theme subcommands
			Entry("theme list", "theme", "list"),
			Entry("theme lint", "theme", "lint"),
			Entry("theme search", "theme", "search"),
			Entry("theme show", "theme", "show"),
			Entry("theme get", "theme", "get"),
			Entry("theme set", "theme", "set"),
			Entry("theme clear", "theme", "clear"),
			Entry("theme add", "theme", "add"),
			Entry("theme remove", "theme", "remove"),
			// Objective subcommands
			Entry("objective list", "objective", "list"),
			Entry("objective lint", "objective", "lint"),
			Entry("objective search", "objective", "search"),
			Entry("objective show", "objective", "show"),
			Entry("objective get", "objective", "get"),
			Entry("objective set", "objective", "set"),
			Entry("objective clear", "objective", "clear"),
			Entry("objective complete", "objective", "complete"),
			Entry("objective add", "objective", "add"),
			Entry("objective remove", "objective", "remove"),
			// Vision subcommands
			Entry("vision list", "vision", "list"),
			Entry("vision lint", "vision", "lint"),
			Entry("vision search", "vision", "search"),
			Entry("vision show", "vision", "show"),
			Entry("vision get", "vision", "get"),
			Entry("vision set", "vision", "set"),
			Entry("vision clear", "vision", "clear"),
			Entry("vision add", "vision", "add"),
			Entry("vision remove", "vision", "remove"),
			// Decision subcommands
			Entry("decision list", "decision", "list"),
			Entry("decision ack", "decision", "ack"),
			// Root-level commands
			Entry("search", "search"),
			Entry("resolve", "resolve"),
			Entry("watch", "watch"),
			// Config subcommands
			Entry("config list", "config", "list"),
			Entry("config current-user", "config", "current-user"),
		)
	})

	Describe("frontmatter round-trip", func() {
		var vaultPath, configPath string
		var cleanup func()

		AfterEach(func() {
			cleanup()
		})

		It("preserves known fields through set operations", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"roundtrip-task": `---
status: todo
priority: 2
task_identifier: 10101010-1010-4101-a010-101010101010
---
# Roundtrip Task
Body content here.
`,
			})

			// Set status to in_progress
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set",
				"roundtrip-task",
				"status", "in_progress",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			// Verify status changed and other fields preserved
			taskPath := filepath.Join(vaultPath, "Tasks", "roundtrip-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: in_progress"))
			Expect(string(content)).To(ContainSubstring("priority: 2"))
			Expect(
				string(content),
			).To(ContainSubstring("task_identifier: 10101010-1010-4101-a010-101010101010"))
			Expect(string(content)).To(ContainSubstring("Body content here."))
		})

		It("preserves unknown frontmatter fields through set operations", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"unknown-fields-task": `---
status: todo
priority: 1
custom_field: my-custom-value
another_field: 42
task_identifier: 20202020-2020-4202-a020-202020202020
---
# Task with unknown fields
`,
			})

			// Set a known field
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set",
				"unknown-fields-task",
				"status", "in_progress",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			// Verify unknown fields are preserved
			taskPath := filepath.Join(
				vaultPath,
				"Tasks",
				"unknown-fields-task.md",
			)
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("custom_field: my-custom-value"))
			Expect(string(content)).To(ContainSubstring("another_field: 42"))
		})

		It("preserves markdown content through set operations", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"content-task": `---
status: todo
priority: 1
task_identifier: 30303030-3030-4303-a030-303030303030
---
# Content Task

This has **bold** and _italic_ text.

- bullet 1
- bullet 2

` + "```go\nfmt.Println(\"hello\")\n```\n",
			})

			// Set status
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set",
				"content-task",
				"status", "in_progress",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			// Verify markdown content preserved
			taskPath := filepath.Join(vaultPath, "Tasks", "content-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("**bold**"))
			Expect(string(content)).To(ContainSubstring("- bullet 1"))
			Expect(string(content)).To(ContainSubstring("fmt.Println"))
		})
	})

	Describe("task get/set", func() {
		var vaultPath, configPath string
		var cleanup func()

		AfterEach(func() {
			cleanup()
		})

		It("gets a known field value", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"get-task": `---
status: todo
priority: 3
task_identifier: 40404040-4040-4404-a040-404040404040
---
# Get Task
`,
			})

			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "get",
				"get-task",
				"status",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("next"))
		})

		It("sets a known field with valid value", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"set-task": `---
status: todo
priority: 1
task_identifier: 50505050-5050-4505-a050-505050505050
---
# Set Task
`,
			})

			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set",
				"set-task",
				"priority", "5",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			taskPath := filepath.Join(vaultPath, "Tasks", "set-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("priority: 5"))
		})
	})

	Describe("status normalization", func() {
		var configPath string
		var cleanup func()

		AfterEach(func() {
			cleanup()
		})

		It("normalizes legacy status 'todo' to 'next' on list", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"legacy-task": `---
status: todo
priority: 1
---
# Legacy Task
`,
			})

			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "list",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			// Task with status "todo" should appear (normalized to next)
			Expect(session.Out).To(gbytes.Say("legacy-task"))
		})
	})

	Describe("vault-cli list", func() {
		var configPath string
		var cleanup func()

		BeforeEach(func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"todo-task": `---
status: todo
priority: 2
---
# Todo Task
This is a todo task.
`,
				"done-task": `---
status: done
priority: 1
---
# Done Task
This is a done task.
`,
			})
		})

		AfterEach(func() {
			cleanup()
		})

		It("shows only non-completed tasks by default", func() {
			cmd := exec.Command(binPath, "--config", configPath, "--vault", "test", "task", "list")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("todo-task"))
			Expect(session.Out).NotTo(gbytes.Say("done-task"))
		})

		It("shows all tasks with --all flag", func() {
			cmd := exec.Command(
				binPath,
				"--config",
				configPath,
				"--vault",
				"test",
				"task",
				"list",
				"--all",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("todo-task"))
			Expect(session.Out).To(gbytes.Say("done-task"))
		})
	})

	Describe("vault-cli lint", func() {
		var configPath string
		var cleanup func()

		Context("with clean vault", func() {
			BeforeEach(func() {
				_, configPath, cleanup = createTempVault(map[string]string{
					"valid-task": `---
status: todo
priority: 2
task_identifier: 60606060-6060-4606-a060-606060606060
---
# Valid Task
This task has valid frontmatter.
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 0 and reports no issues", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"lint",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say("No lint issues found"))
			})
		})

		Context("with invalid status", func() {
			BeforeEach(func() {
				_, configPath, cleanup = createTempVault(map[string]string{
					"invalid-status-task": `---
status: garbage
priority: 2
task_identifier: 70707070-7070-4707-a070-707070707070
---
# Task with invalid status
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 1 and reports INVALID_STATUS", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"lint",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Out).To(gbytes.Say("INVALID_STATUS"))
			})
		})

		Context("with invalid priority", func() {
			BeforeEach(func() {
				_, configPath, cleanup = createTempVault(map[string]string{
					"high-priority-task": `---
status: todo
priority: high
---
# Task with string priority
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 1 and reports INVALID_PRIORITY", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"lint",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Out).To(gbytes.Say("INVALID_PRIORITY"))
			})
		})
	})

	Describe("vault-cli lint --fix", func() {
		var vaultPath, configPath string
		var cleanup func()

		Context("with legacy status: todo (silently accepted alias)", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"legacy-todo-task": `---
status: todo
priority: 2
task_identifier: 80808080-8080-4808-a080-808080808080
---
# Task with legacy todo status
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 0, reports no issues, and leaves file unchanged", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"lint",
					"--fix",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say("No lint issues found"))
				Expect(session.Out).NotTo(gbytes.Say("FIXED"))

				// Verify file was NOT rewritten — alias preserved on disk
				taskPath := filepath.Join(vaultPath, "Tasks", "legacy-todo-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("status: todo"))
				Expect(string(content)).NotTo(ContainSubstring("status: next"))
			})
		})

		Context("with priority: high", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"high-priority-task": `---
status: todo
priority: high
task_identifier: 90909090-9090-4909-a090-909090909090
---
# Task with string priority
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 0 and updates file to priority: 1", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"lint",
					"--fix",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				// Verify file was updated
				taskPath := filepath.Join(vaultPath, "Tasks", "high-priority-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("priority: 1"))
				Expect(string(content)).NotTo(ContainSubstring("priority: high"))
			})
		})
	})

	Describe("vault-cli complete", func() {
		var vaultPath, configPath string
		var cleanup func()

		Context("when task exists", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"my-task": `---
status: todo
priority: 2
---
# My Task
This is my task.
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 0 and updates status to completed", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"complete",
					"my-task",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				// Verify file was updated
				taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("status: completed"))
			})
		})

		Context("when task does not exist", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 1", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"complete",
					"non-existent-task",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
			})
		})
	})

	Describe("vault-cli task show with YAML date literal", func() {
		var vaultPath, configPath string
		var cleanup func()

		BeforeEach(func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"aqua": `---
status: todo
priority: 2
defer_date: 2026-04-13
---
# Aqua
`,
			})
		})

		AfterEach(func() {
			cleanup()
		})

		It("outputs defer_date in JSON when YAML has a native date literal", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "show", "aqua",
				"--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say(`"defer_date":\s*"2026-04-13"`))
			Expect(string(session.Out.Contents())).NotTo(ContainSubstring("00:00:00 +0000 UTC"))
		})

		// vaultPath is assigned in BeforeEach to avoid unused variable lint error
		_ = &vaultPath
	})

	Describe("vault-cli task JSON schema", func() {
		var configPath string
		var cleanup func()

		BeforeEach(func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"schema-task": `---
status: in_progress
priority: 2
assignee: bborbe
recurring: weekly
phase: todo
defer_date: 2026-04-13
planned_date: "2026-04-15"
due_date: 2026-04-20T10:30:00Z
completed_date: "2026-03-09T12:30:00Z"
last_completed: 2026-03-08
task_identifier: 043d9cac-d56b-4a36-921e-b0e35819fb66
goals:
  - "[[Example Goal]]"
tags:
  - alpha
---
body
`,
			})
		})

		AfterEach(func() {
			cleanup()
		})

		It("includes all date fields with correct values in task show --output json", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "show", "schema-task",
				"--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			var parsed map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &parsed)).To(Succeed())
			Expect(parsed).To(HaveKeyWithValue("defer_date", "2026-04-13"))
			Expect(parsed).To(HaveKeyWithValue("planned_date", "2026-04-15"))
			Expect(parsed).To(HaveKeyWithValue("due_date", "2026-04-20T10:30:00Z"))
			Expect(parsed).To(HaveKeyWithValue("completed_date", "2026-03-09T12:30:00Z"))
			Expect(string(session.Out.Contents())).NotTo(ContainSubstring("00:00:00 +0000 UTC"))
		})

		It("includes all date fields with correct values in task list --output json", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "list",
				"--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			var items []map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &items)).To(Succeed())
			Expect(items).To(HaveLen(1))
			item := items[0]
			Expect(item).To(HaveKeyWithValue("defer_date", "2026-04-13"))
			Expect(item).To(HaveKeyWithValue("planned_date", "2026-04-15"))
			Expect(item).To(HaveKeyWithValue("due_date", "2026-04-20T10:30:00Z"))
			Expect(item).To(HaveKeyWithValue("completed_date", "2026-03-09T12:30:00Z"))
			Expect(string(session.Out.Contents())).NotTo(ContainSubstring("00:00:00 +0000 UTC"))
		})
	})

	Describe("vault-cli defer", func() {
		var vaultPath, configPath string
		var cleanup func()

		Context("when task exists", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"my-task": `---
status: todo
priority: 2
---
# My Task
This is my task.
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 0 and adds defer_date", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"defer",
					"my-task",
					"+7d",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				// Verify file was updated with defer_date
				taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("defer_date:"))
			})
		})

		It("defaults to +1d when no date argument provided", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"my-task": `---
status: todo
priority: 2
---
# My Task
This is my task.
`,
			})
			defer cleanup()
			cmd := exec.Command(
				binPath,
				"--config",
				configPath,
				"--vault",
				"test",
				"task",
				"defer",
				"my-task",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			// Verify file was updated with defer_date
			taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("defer_date:"))
		})

		Context("with invalid date format", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"my-task": `---
status: todo
priority: 2
---
# My Task
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 1", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"defer",
					"my-task",
					"invalid-date",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
			})
		})

		})

	Describe("vault-cli resolve", func() {
		var configPath string
		var cleanup func()

		Context("with task and goal present", func() {
			BeforeEach(func() {
				_, configPath, cleanup = createTempVaultWithGoals(
					map[string]string{
						"my-task": `---
status: todo
priority: 2
---
# My Task
`,
					},
					map[string]string{
						"my-goal": `---
status: todo
page_type: goal
---
# My Goal
`,
					},
				)
			})

			AfterEach(func() {
				cleanup()
			})

			It("returns task match as JSON", func() {
				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"resolve", "my-task",
					"--output", "json",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				var result map[string]any
				Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
				Expect(result).To(HaveKeyWithValue("type", "task"))
				Expect(result).To(HaveKeyWithValue("name", "my-task"))
				Expect(result).To(HaveKeyWithValue("found", true))
			})

			It("returns goal match as JSON", func() {
				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"resolve", "my-goal",
					"--output", "json",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				var result map[string]any
				Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
				Expect(result).To(HaveKeyWithValue("type", "goal"))
				Expect(result).To(HaveKeyWithValue("name", "my-goal"))
				Expect(result).To(HaveKeyWithValue("found", true))
			})

			It("returns not found for unknown name", func() {
				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"resolve", "nonexistent",
					"--output", "json",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				var result map[string]any
				Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
				Expect(result).To(HaveKeyWithValue("type", ""))
				Expect(result).To(HaveKeyWithValue("name", "nonexistent"))
				Expect(result).To(HaveKeyWithValue("found", false))
				// Verify empty-string type and false are serialized (not omitempty)
				raw := string(session.Out.Contents())
				Expect(raw).To(ContainSubstring(`"type": ""`))
				Expect(raw).To(ContainSubstring(`"found": false`))
			})

			It("task-first priority when name matches both", func() {
				_, configPath2, cleanup2 := createTempVaultWithGoals(
					map[string]string{
						"collision": `---
status: todo
priority: 1
---
# Collision Task
`,
					},
					map[string]string{
						"collision": `---
status: todo
page_type: goal
---
# Collision Goal
`,
					},
				)
				defer cleanup2()

				cmd := exec.Command(
					binPath,
					"--config", configPath2,
					"--vault", "test",
					"resolve", "collision",
					"--output", "json",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				var result map[string]any
				Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
				Expect(result).To(HaveKeyWithValue("type", "task"))
				Expect(result).To(HaveKeyWithValue("found", true))
			})
		})
	})

})
