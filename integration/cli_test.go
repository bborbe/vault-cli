// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

// createTempVault creates a temporary vault with tasks and config file
func createTempVault(
	tasks map[string]string,
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

	configContent := fmt.Sprintf(`default_vault: test
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
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
task_identifier: test-uuid-roundtrip
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
			Expect(string(content)).To(ContainSubstring("task_identifier: test-uuid-roundtrip"))
			Expect(string(content)).To(ContainSubstring("Body content here."))
		})

		It("preserves unknown frontmatter fields through set operations", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"unknown-fields-task": `---
status: todo
priority: 1
custom_field: my-custom-value
another_field: 42
task_identifier: test-uuid-unknown
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
task_identifier: test-uuid-content
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
task_identifier: test-uuid-get
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
			Expect(session.Out).To(gbytes.Say("todo"))
		})

		It("sets a known field with valid value", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"set-task": `---
status: todo
priority: 1
task_identifier: test-uuid-set
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

		It("normalizes legacy status 'next' to 'todo' on list", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"legacy-task": `---
status: next
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
			// Task with status "next" should appear (normalized to todo)
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
task_identifier: test-uuid-valid
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
					"next-status-task": `---
status: next
priority: 2
---
# Task with next status
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

		Context("with status: next", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"next-status-task": `---
status: next
priority: 2
task_identifier: test-uuid-next
---
# Task with next status
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 0, shows FIXED, and updates file to status: todo", func() {
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
				Expect(session.Out).To(gbytes.Say("FIXED"))

				// Verify file was updated
				taskPath := filepath.Join(vaultPath, "Tasks", "next-status-task.md")
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
task_identifier: test-uuid-high
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

})
