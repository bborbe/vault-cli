// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("LintOperation", func() {
	var (
		ctx       context.Context
		lintOp    ops.LintOperation
		vaultPath string
		tasksDir  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		lintOp = ops.NewLintOperation()

		// Create temp vault directory
		var err error
		vaultPath, err = os.MkdirTemp("", "vault-lint-test-*")
		Expect(err).To(BeNil())

		tasksDir = "Tasks"
		tasksDirPath := filepath.Join(vaultPath, tasksDir)
		Expect(os.MkdirAll(tasksDirPath, 0755)).To(Succeed())
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	Context("when there are no issues", func() {
		BeforeEach(func() {
			validTaskContent := `---
status: todo
page_type: task
priority: 1
assignee: bborbe
---
# Valid Task

This task has no issues.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Valid Task.md")
			Expect(os.WriteFile(taskPath, []byte(validTaskContent), 0600)).To(Succeed())
		})

		It("reports no issues", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
		})
	})

	Context("MISSING_FRONTMATTER", func() {
		BeforeEach(func() {
			noFrontmatterContent := `# Task Without Frontmatter

This task has no frontmatter.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "No Frontmatter.md")
			Expect(os.WriteFile(taskPath, []byte(noFrontmatterContent), 0600)).To(Succeed())
		})

		It("detects missing frontmatter", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("found 1 lint issue"))
		})

		It("cannot fix missing frontmatter", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("found 1 lint issue"))
		})
	})

	Context("INVALID_PRIORITY", func() {
		BeforeEach(func() {
			invalidPriorityContent := `---
status: todo
page_type: task
priority: high
assignee: bborbe
---
# Task With String Priority

This task has a string priority.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Invalid Priority.md")
			Expect(os.WriteFile(taskPath, []byte(invalidPriorityContent), 0600)).To(Succeed())
		})

		It("detects invalid priority", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("found 1 lint issue"))
		})

		It("fixes invalid priority 'high' to 1", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())

			// Verify file was fixed
			taskPath := filepath.Join(vaultPath, tasksDir, "Invalid Priority.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).To(BeNil())
			Expect(string(content)).To(ContainSubstring("priority: 1"))
			Expect(string(content)).NotTo(ContainSubstring("priority: high"))
		})
	})

	Context("INVALID_PRIORITY with different string values", func() {
		DescribeTable("fixes various priority string values",
			func(priorityValue string, expectedInt int) {
				taskContent := `---
status: todo
page_type: task
priority: ` + priorityValue + `
assignee: bborbe
---
# Task
`
				taskPath := filepath.Join(vaultPath, tasksDir, "Priority Test.md")
				Expect(os.WriteFile(taskPath, []byte(taskContent), 0600)).To(Succeed())

				err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
				Expect(err).To(BeNil())

				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).To(BeNil())
				Expect(
					string(content),
				).To(ContainSubstring("priority: " + string(rune('0'+expectedInt))))
			},
			Entry("high -> 1", "high", 1),
			Entry("must -> 1", "must", 1),
			Entry("medium -> 2", "medium", 2),
			Entry("should -> 2", "should", 2),
			Entry("low -> 3", "low", 3),
		)
	})

	Context("DUPLICATE_KEY", func() {
		BeforeEach(func() {
			duplicateKeyContent := `---
status: todo
page_type: task
priority: 1
assignee: bborbe
assignee: alice
---
# Task With Duplicate Key

This task has duplicate assignee key.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Duplicate Key.md")
			Expect(os.WriteFile(taskPath, []byte(duplicateKeyContent), 0600)).To(Succeed())
		})

		It("detects duplicate keys", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("found 1 lint issue"))
		})

		It("fixes duplicate keys by keeping first occurrence", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())

			// Verify file was fixed
			taskPath := filepath.Join(vaultPath, tasksDir, "Duplicate Key.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).To(BeNil())
			// Should have only one assignee line
			Expect(string(content)).To(ContainSubstring("assignee: bborbe"))
			// Second assignee should be removed
			lines := 0
			for _, line := range []byte(string(content)) {
				if line == 'a' {
					lines++
				}
			}
			// Count occurrences more precisely
			contentStr := string(content)
			firstIdx := indexOf(contentStr, "assignee: bborbe")
			secondIdx := indexOf(contentStr[firstIdx+1:], "assignee:")
			Expect(secondIdx).To(Equal(-1), "Should not have second assignee line")
		})
	})

	Context("INVALID_STATUS", func() {
		BeforeEach(func() {
			invalidStatusContent := `---
status: invalid_status
page_type: task
priority: 1
assignee: bborbe
---
# Task With Invalid Status

This task has an invalid status.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Invalid Status.md")
			Expect(os.WriteFile(taskPath, []byte(invalidStatusContent), 0600)).To(Succeed())
		})

		It("detects invalid status", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("found 1 lint issue"))
		})

		It("cannot fix invalid status", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("found 1 lint issue"))
		})
	})

	Context("multiple issues in one file", func() {
		BeforeEach(func() {
			multipleIssuesContent := `---
status: invalid_status
page_type: task
priority: high
assignee: bborbe
assignee: alice
---
# Task With Multiple Issues

This task has multiple issues.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Multiple Issues.md")
			Expect(os.WriteFile(taskPath, []byte(multipleIssuesContent), 0600)).To(Succeed())
		})

		It("detects all issues", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("found 3 lint issue"))
		})

		It("fixes fixable issues and reports unfixable ones", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).NotTo(BeNil())
			// Should have 1 unfixed issue (invalid status)
			Expect(err.Error()).To(ContainSubstring("found 1 lint issue"))
		})
	})

	Context("multiple files with issues", func() {
		BeforeEach(func() {
			// File 1: Invalid priority
			file1Content := `---
status: todo
page_type: task
priority: high
---
# Task 1
`
			taskPath1 := filepath.Join(vaultPath, tasksDir, "Task1.md")
			Expect(os.WriteFile(taskPath1, []byte(file1Content), 0600)).To(Succeed())

			// File 2: Duplicate key
			file2Content := `---
status: todo
page_type: task
assignee: bborbe
assignee: alice
---
# Task 2
`
			taskPath2 := filepath.Join(vaultPath, tasksDir, "Task2.md")
			Expect(os.WriteFile(taskPath2, []byte(file2Content), 0600)).To(Succeed())
		})

		It("detects issues in all files", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("found 2 lint issue"))
		})

		It("fixes issues in all files", func() {
			err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
		})
	})
})

// Helper function to find substring index
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
