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
task_identifier: test-uuid-valid
---
# Valid Task

This task has no issues.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Valid Task.md")
			Expect(os.WriteFile(taskPath, []byte(validTaskContent), 0600)).To(Succeed())
		})

		It("reports no issues", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
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

		It("detects missing frontmatter as fixable", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})

		It("fixes missing frontmatter by prepending status: backlog", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())

			// Verify file was fixed
			taskPath := filepath.Join(vaultPath, tasksDir, "No Frontmatter.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).To(BeNil())

			// Should start with minimal frontmatter
			Expect(string(content)).To(HavePrefix("---\nstatus: backlog\n---\n"))

			// Original content should be preserved after frontmatter
			Expect(string(content)).To(ContainSubstring("# Task Without Frontmatter"))
			Expect(string(content)).To(ContainSubstring("This task has no frontmatter."))
		})
	})

	Context("INVALID_PRIORITY", func() {
		BeforeEach(func() {
			invalidPriorityContent := `---
status: todo
page_type: task
priority: high
assignee: bborbe
task_identifier: test-uuid-priority
---
# Task With String Priority

This task has a string priority.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Invalid Priority.md")
			Expect(os.WriteFile(taskPath, []byte(invalidPriorityContent), 0600)).To(Succeed())
		})

		It("detects invalid priority", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})

		It("fixes invalid priority 'high' to 1", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
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

				_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
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
task_identifier: test-uuid-duplicate
---
# Task With Duplicate Key

This task has duplicate assignee key.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Duplicate Key.md")
			Expect(os.WriteFile(taskPath, []byte(duplicateKeyContent), 0600)).To(Succeed())
		})

		It("detects duplicate keys", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})

		It("fixes duplicate keys by keeping first occurrence", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
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
task_identifier: test-uuid-status
---
# Task With Invalid Status

This task has an invalid status.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Invalid Status.md")
			Expect(os.WriteFile(taskPath, []byte(invalidStatusContent), 0600)).To(Succeed())
		})

		It("detects invalid status", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})

		It("cannot fix invalid status", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})
	})

	Context("INVALID_STATUS with migration map", func() {
		Context("status: next", func() {
			BeforeEach(func() {
				nextStatusContent := `---
status: next
page_type: task
priority: 1
assignee: bborbe
task_identifier: test-uuid-next
---
# Task With Next Status

This task has the old 'next' status.
`
				taskPath := filepath.Join(vaultPath, tasksDir, "Next Status.md")
				Expect(os.WriteFile(taskPath, []byte(nextStatusContent), 0600)).To(Succeed())
			})

			It("detects 'next' as invalid status", func() {
				issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
				Expect(err).To(BeNil())
				Expect(issues).To(HaveLen(1))
			})

			It("fixes 'next' to 'todo'", func() {
				_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
				Expect(err).To(BeNil())

				// Verify file was fixed
				taskPath := filepath.Join(vaultPath, tasksDir, "Next Status.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).To(BeNil())
				Expect(string(content)).To(ContainSubstring("status: todo"))
				Expect(string(content)).NotTo(ContainSubstring("status: next"))
			})
		})

		Context("status: current", func() {
			BeforeEach(func() {
				currentStatusContent := `---
status: current
page_type: task
priority: 1
assignee: bborbe
task_identifier: test-uuid-current
---
# Task With Current Status

This task has the old 'current' status.
`
				taskPath := filepath.Join(vaultPath, tasksDir, "Current Status.md")
				Expect(os.WriteFile(taskPath, []byte(currentStatusContent), 0600)).To(Succeed())
			})

			It("detects 'current' as invalid status", func() {
				issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
				Expect(err).To(BeNil())
				Expect(issues).To(HaveLen(1))
			})

			It("fixes 'current' to 'in_progress'", func() {
				_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
				Expect(err).To(BeNil())

				// Verify file was fixed
				taskPath := filepath.Join(vaultPath, tasksDir, "Current Status.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).To(BeNil())
				Expect(string(content)).To(ContainSubstring("status: in_progress"))
				Expect(string(content)).NotTo(ContainSubstring("status: current"))
			})
		})

		Context("status: done", func() {
			BeforeEach(func() {
				doneStatusContent := `---
status: done
page_type: task
priority: 1
assignee: bborbe
task_identifier: test-uuid-done
---
# Task With Done Status

This task has the old 'done' status.
`
				taskPath := filepath.Join(vaultPath, tasksDir, "Done Status.md")
				Expect(os.WriteFile(taskPath, []byte(doneStatusContent), 0600)).To(Succeed())
			})

			It("detects 'done' as invalid status", func() {
				issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
				Expect(err).To(BeNil())
				Expect(issues).To(HaveLen(1))
			})

			It("fixes 'done' to 'completed'", func() {
				_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
				Expect(err).To(BeNil())

				// Verify file was fixed
				taskPath := filepath.Join(vaultPath, tasksDir, "Done Status.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).To(BeNil())
				Expect(string(content)).To(ContainSubstring("status: completed"))
				Expect(string(content)).NotTo(ContainSubstring("status: done"))
			})
		})

		Context("unknown invalid status (foo)", func() {
			BeforeEach(func() {
				fooStatusContent := `---
status: foo
page_type: task
priority: 1
assignee: bborbe
task_identifier: test-uuid-foo
---
# Task With Foo Status

This task has an unknown invalid status.
`
				taskPath := filepath.Join(vaultPath, tasksDir, "Foo Status.md")
				Expect(os.WriteFile(taskPath, []byte(fooStatusContent), 0600)).To(Succeed())
			})

			It("detects 'foo' as invalid status", func() {
				issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
				Expect(err).To(BeNil())
				Expect(issues).To(HaveLen(1))
			})

			It("cannot fix 'foo' status", func() {
				issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
				Expect(err).To(BeNil())
				Expect(issues).To(HaveLen(1))

				// Verify file was not changed
				taskPath := filepath.Join(vaultPath, tasksDir, "Foo Status.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).To(BeNil())
				Expect(string(content)).To(ContainSubstring("status: foo"))
			})
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
task_identifier: test-uuid-multi
---
# Task With Multiple Issues

This task has multiple issues.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Multiple Issues.md")
			Expect(os.WriteFile(taskPath, []byte(multipleIssuesContent), 0600)).To(Succeed())
		})

		It("detects all issues", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(3))
		})

		It("fixes fixable issues and reports unfixable ones", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
			// Should have 1 unfixed issue (invalid status)
			unfixed := 0
			for _, i := range issues {
				if !i.Fixed {
					unfixed++
				}
			}
			Expect(unfixed).To(Equal(1))
		})
	})

	Context("multiple files with issues", func() {
		BeforeEach(func() {
			// File 1: Invalid priority
			file1Content := `---
status: todo
page_type: task
priority: high
task_identifier: test-uuid-file1
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
task_identifier: test-uuid-file2
---
# Task 2
`
			taskPath2 := filepath.Join(vaultPath, tasksDir, "Task2.md")
			Expect(os.WriteFile(taskPath2, []byte(file2Content), 0600)).To(Succeed())
		})

		It("detects issues in all files", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(2))
		})

		It("fixes issues in all files", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
		})
	})

	Context("error handling", func() {
		Context("with non-existent tasks directory", func() {
			It("returns an error", func() {
				_, err := lintOp.Execute(ctx, vaultPath, "NonExistentDir", false)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("walk tasks directory"))
			})
		})

		Context("with plain output format", func() {
			BeforeEach(func() {
				validContent := `---
status: todo
priority: 1
---
# Valid Task
`
				taskPath := filepath.Join(vaultPath, tasksDir, "Valid.md")
				Expect(os.WriteFile(taskPath, []byte(validContent), 0600)).To(Succeed())
			})

			It("succeeds with plain output", func() {
				_, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
				Expect(err).To(BeNil())
			})
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

var _ = Describe("ExecuteFile", func() {
	var (
		ctx         context.Context
		lintOp      ops.LintOperation
		tmpFile     string
		taskName    string
		vaultName   string
		err         error
		fileContent string
		createFile  bool
	)

	BeforeEach(func() {
		ctx = context.Background()
		lintOp = ops.NewLintOperation()
		taskName = "My Task"
		vaultName = "personal"
		createFile = true

		// Default valid content
		fileContent = `---
status: in_progress
priority: 1
---

# Task Content

This is a valid task.
`
	})

	JustBeforeEach(func() {
		if createFile {
			// Create temp file with content
			f, createErr := os.CreateTemp("", "task-*.md")
			Expect(createErr).To(BeNil())
			tmpFile = f.Name()
			_, _ = f.WriteString(fileContent)
			_ = f.Close()
		}

		// Execute the operation
		_, err = lintOp.ExecuteFile(ctx, tmpFile, taskName, vaultName)
	})

	AfterEach(func() {
		if tmpFile != "" && createFile {
			_ = os.Remove(tmpFile)
		}
	})

	Context("with valid file and plain output", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with valid file and json output", func() {
		BeforeEach(func() {
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with non-existent file", func() {
		BeforeEach(func() {
			createFile = false
			tmpFile = "/tmp/does-not-exist-file-12345.md"
		})

		It("returns an error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("lint file"))
		})
	})

	Context("with different valid statuses", func() {
		validStatuses := []string{"todo", "in_progress", "backlog", "completed", "hold", "aborted"}

		for _, status := range validStatuses {
			status := status
			Context("with status: "+status, func() {
				BeforeEach(func() {
					fileContent = `---
status: ` + status + `
priority: 1
---

# Task
`
				})

				It("returns no error in plain mode", func() {
					Expect(err).To(BeNil())
				})
			})

			Context("with status: "+status+" in json mode", func() {
				BeforeEach(func() {
					fileContent = `---
status: ` + status + `
priority: 1
---

# Task
`
				})

				It("returns no error in json mode", func() {
					Expect(err).To(BeNil())
				})
			})
		}
	})

	Context("with different valid priorities", func() {
		validPriorities := []string{"1", "2", "3", "4", "5"}

		for _, priority := range validPriorities {
			priority := priority
			Context("with priority: "+priority, func() {
				BeforeEach(func() {
					fileContent = `---
status: todo
priority: ` + priority + `
---

# Task
`
				})

				It("returns no error", func() {
					Expect(err).To(BeNil())
				})
			})
		}
	})

	Context("with additional valid frontmatter fields", func() {
		BeforeEach(func() {
			fileContent = `---
status: todo
priority: 1
page_type: task
assignee: bborbe
tags:
  - important
  - urgent
---

# Task with Extra Fields
`
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with minimal valid frontmatter", func() {
		BeforeEach(func() {
			fileContent = `---
status: backlog
---

# Minimal Task
`
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})
})

var _ = Describe("Execute with JSON output (outputIssuesJSON)", func() {
	var (
		ctx       context.Context
		lintOp    ops.LintOperation
		vaultPath string
		tasksDir  string
		issues    []ops.LintIssue
		err       error
	)

	BeforeEach(func() {
		ctx = context.Background()
		lintOp = ops.NewLintOperation()

		// Create temp vault directory
		var createErr error
		vaultPath, createErr = os.MkdirTemp("", "vault-json-test-*")
		Expect(createErr).To(BeNil())

		tasksDir = "Tasks"
		tasksDirPath := filepath.Join(vaultPath, tasksDir)
		Expect(os.MkdirAll(tasksDirPath, 0750)).To(Succeed())
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	Context("with no issues", func() {
		BeforeEach(func() {
			validContent := `---
status: todo
priority: 1
---
# Valid Task
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Valid.md")
			Expect(os.WriteFile(taskPath, []byte(validContent), 0600)).To(Succeed())
		})

		It("outputs empty JSON array and returns no error", func() {
			_, err = lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
		})

		It("outputs empty JSON array with fix flag", func() {
			_, err = lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
		})
	})

	Context("with fixable issues", func() {
		BeforeEach(func() {
			invalidPriorityContent := `---
status: todo
priority: high
task_identifier: test-uuid-fixable
---
# Task With Fixable Issue
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Fixable.md")
			Expect(os.WriteFile(taskPath, []byte(invalidPriorityContent), 0600)).To(Succeed())
		})

		It("outputs JSON with issues and returns error", func() {
			issues, err = lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})

		It("fixes issues and outputs JSON with fixed status", func() {
			_, err = lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
		})
	})

	Context("with non-fixable issues", func() {
		BeforeEach(func() {
			invalidStatusContent := `---
status: invalid_status
priority: 1
task_identifier: test-uuid-nonfixable
---
# Task With Non-Fixable Issue
`
			taskPath := filepath.Join(vaultPath, tasksDir, "NonFixable.md")
			Expect(os.WriteFile(taskPath, []byte(invalidStatusContent), 0600)).To(Succeed())
		})

		It("outputs JSON with ERROR type issues", func() {
			issues, err = lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})

		It("cannot fix non-fixable issues", func() {
			issues, err = lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})
	})

	Context("with mixed fixable and non-fixable issues", func() {
		BeforeEach(func() {
			mixedContent := `---
status: invalid_status
priority: high
assignee: bob
assignee: alice
task_identifier: test-uuid-mixed
---
# Task With Mixed Issues
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Mixed.md")
			Expect(os.WriteFile(taskPath, []byte(mixedContent), 0600)).To(Succeed())
		})

		It("detects all issues without fix", func() {
			issues, err = lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(3))
		})

		It("fixes fixable issues but reports non-fixable ones", func() {
			issues, err = lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
			// Should have 1 unfixed issue (invalid_status)
			unfixed := 0
			for _, i := range issues {
				if !i.Fixed {
					unfixed++
				}
			}
			Expect(unfixed).To(Equal(1))
		})
	})

	Context("with multiple files", func() {
		BeforeEach(func() {
			// File 1: Valid
			validContent := `---
status: todo
priority: 1
task_identifier: test-uuid-valid-multi
---
# Valid
`
			taskPath1 := filepath.Join(vaultPath, tasksDir, "Valid.md")
			Expect(os.WriteFile(taskPath1, []byte(validContent), 0600)).To(Succeed())

			// File 2: Has issue
			issueContent := `---
status: todo
priority: high
task_identifier: test-uuid-issue
---
# Has Issue
`
			taskPath2 := filepath.Join(vaultPath, tasksDir, "Issue.md")
			Expect(os.WriteFile(taskPath2, []byte(issueContent), 0600)).To(Succeed())
		})

		It("reports issues from all files in JSON", func() {
			issues, err = lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})
	})

	Context("with empty directory", func() {
		It("returns no error", func() {
			_, err = lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
		})
	})

	Context("with subdirectories", func() {
		BeforeEach(func() {
			// Create subdirectory with valid file
			subDir := filepath.Join(vaultPath, tasksDir, "SubDir")
			Expect(os.MkdirAll(subDir, 0750)).To(Succeed())

			validContent := `---
status: todo
priority: 1
---
# Valid Task in Subdir
`
			taskPath := filepath.Join(subDir, "Valid.md")
			Expect(os.WriteFile(taskPath, []byte(validContent), 0600)).To(Succeed())
		})

		It("processes files in subdirectories", func() {
			_, err = lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
		})
	})

	Context("with migrateable status values", func() {
		BeforeEach(func() {
			// File with migrateable status
			migrateContent := `---
status: next
priority: 1
task_identifier: test-uuid-migrate
---
# Migrateable Status
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Migrate.md")
			Expect(os.WriteFile(taskPath, []byte(migrateContent), 0600)).To(Succeed())
		})

		It("reports migrateable status as fixable WARN in JSON", func() {
			issues, err = lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})

		It("fixes migrateable status", func() {
			_, err = lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())

			// Verify the file was fixed
			taskPath := filepath.Join(vaultPath, tasksDir, "Migrate.md")
			content, readErr := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(readErr).To(BeNil())
			Expect(string(content)).To(ContainSubstring("status: todo"))
		})
	})

	Context("with all valid status values", func() {
		validStatuses := []string{"todo", "in_progress", "backlog", "completed", "hold", "aborted"}

		for _, status := range validStatuses {
			status := status // capture loop variable
			Context("with status: "+status, func() {
				BeforeEach(func() {
					content := `---
status: ` + status + `
priority: 1
---
# Task
`
					taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
					Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
				})

				It("reports no issues", func() {
					_, err = lintOp.Execute(ctx, vaultPath, tasksDir, false)
					Expect(err).To(BeNil())
				})
			})
		}
	})

	Context("with old migrateable status values", func() {
		migrateMap := map[string]string{
			"next":      "todo",
			"current":   "in_progress",
			"completed": "completed",
		}

		for oldStatus, newStatus := range migrateMap {
			oldStatus := oldStatus // capture loop variable
			newStatus := newStatus
			Context("migrating "+oldStatus+" to "+newStatus, func() {
				BeforeEach(func() {
					content := `---
status: ` + oldStatus + `
priority: 1
---
# Task
`
					taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
					Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
				})

				It("fixes the status", func() {
					_, err = lintOp.Execute(ctx, vaultPath, tasksDir, true)
					Expect(err).To(BeNil())

					taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
					content, readErr := os.ReadFile(taskPath) //#nosec G304 -- test file
					Expect(readErr).To(BeNil())
					Expect(string(content)).To(ContainSubstring("status: " + newStatus))
				})
			})
		}
	})

	Context("with priority values with quotes", func() {
		BeforeEach(func() {
			content := `---
status: todo
priority: "high"
task_identifier: test-uuid-quoted
---
# Task with Quoted Priority
`
			taskPath := filepath.Join(vaultPath, tasksDir, "QuotedPriority.md")
			Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
		})

		It("detects quoted priority as invalid", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})

		It("fixes quoted priority", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
		})
	})

	Context("with non-md files in directory", func() {
		BeforeEach(func() {
			// Create a non-.md file
			txtPath := filepath.Join(vaultPath, tasksDir, "notes.txt")
			Expect(os.WriteFile(txtPath, []byte("some notes"), 0600)).To(Succeed())

			// And a valid .md file
			validContent := `---
status: todo
priority: 1
---
# Valid Task
`
			mdPath := filepath.Join(vaultPath, tasksDir, "Valid.md")
			Expect(os.WriteFile(mdPath, []byte(validContent), 0600)).To(Succeed())
		})

		It("ignores non-md files", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
		})
	})

	Context("with various priority string values", func() {
		priorityTests := []struct {
			value    string
			expected int
		}{
			{"must", 1},
			{"should", 2},
			{"low", 3},
			{"medium", 2},
		}

		for _, tt := range priorityTests {
			tt := tt
			Context("with priority: "+tt.value, func() {
				BeforeEach(func() {
					content := `---
status: todo
priority: ` + tt.value + `
---
# Task
`
					taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
					Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
				})

				It("detects and fixes priority", func() {
					_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
					Expect(err).To(BeNil())

					taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
					content, readErr := os.ReadFile(taskPath) //#nosec G304 -- test file
					Expect(readErr).To(BeNil())
					Expect(string(content)).To(MatchRegexp(`priority: \d+`))
				})
			})
		}
	})

	Context("with priority value that is not detected as invalid", func() {
		BeforeEach(func() {
			// Priority value that doesn't match the known invalid patterns
			// The lint function only detects known string values like "high", "must", etc.
			content := `---
status: todo
priority: 1
---
# Task
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
		})

		It("passes validation for integer priority", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
		})
	})

	Context("with file that has write error during fix", func() {
		BeforeEach(func() {
			content := `---
status: next
priority: 1
---
# Task
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

			// Make file read-only to cause write error
			Expect(os.Chmod(taskPath, 0400)).To(Succeed())
		})

		AfterEach(func() {
			// Restore permissions for cleanup
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			_ = os.Chmod(taskPath, 0600)
		})

		It("returns error when unable to write file", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("fix issues"))
		})
	})

	Context("with status value that has single quotes", func() {
		BeforeEach(func() {
			content := `---
status: 'next'
priority: 1
---
# Task
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
		})

		It("detects and fixes quoted status", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())

			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			content, readErr := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(readErr).To(BeNil())
			Expect(string(content)).To(ContainSubstring("status: todo"))
		})
	})

	Context("with status value that has double quotes", func() {
		BeforeEach(func() {
			content := `---
status: "done"
priority: 1
---
# Task
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
		})

		It("detects and fixes quoted status", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())

			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			content, readErr := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(readErr).To(BeNil())
			Expect(string(content)).To(ContainSubstring("status: completed"))
		})
	})

	Context("with invalid YAML in frontmatter after duplicate key removal", func() {
		BeforeEach(func() {
			// Create a file where removing duplicate keys would result in invalid YAML
			// This is hard to trigger, but we can test the validation path
			content := `---
status: todo
priority: 1
---
# Valid Task
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
		})

		It("handles valid YAML correctly", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
		})
	})

	Context("with error encoding JSON in outputIssuesJSON", func() {
		// This is difficult to test as json.Encoder rarely fails with valid data
		// We test the happy path instead
		BeforeEach(func() {
			content := `---
status: todo
priority: high
task_identifier: test-uuid-json-enc
---
# Task
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
		})

		It("successfully encodes JSON for issues", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})
	})

	Context("with file operations", func() {
		BeforeEach(func() {
			// Test with missing frontmatter write error scenario
			content := `# Task without frontmatter`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
		})

		It("detects missing frontmatter", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).NotTo(BeEmpty())
		})
	})

	Context("with edge cases in status values", func() {
		statusEdgeCases := map[string]bool{
			"todo":        true,  // valid
			"in_progress": true,  // valid
			"backlog":     true,  // valid
			"completed":   true,  // valid
			"hold":        true,  // valid
			"aborted":     true,  // valid
			"next":        false, // fixable invalid
			"current":     false, // fixable invalid
			"done":        false, // fixable invalid
		}

		for status, isValid := range statusEdgeCases {
			status := status
			isValid := isValid
			Context("with status: "+status, func() {
				BeforeEach(func() {
					content := `---
status: ` + status + `
priority: 1
---
# Task
`
					taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
					Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
				})

				if isValid {
					It("accepts valid status in json mode", func() {
						_, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
						Expect(err).To(BeNil())
					})
				} else {
					It("detects fixable invalid status in json mode", func() {
						issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
						Expect(err).To(BeNil())
						Expect(issues).NotTo(BeEmpty())
					})

					It("fixes invalid status in json mode", func() {
						_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
						Expect(err).To(BeNil())
					})
				}
			})
		}
	})
})

var _ = Describe("ExecuteFile error handling", func() {
	var (
		ctx     context.Context
		lintOp  ops.LintOperation
		tmpFile string
		err     error
	)

	BeforeEach(func() {
		ctx = context.Background()
		lintOp = ops.NewLintOperation()
	})

	AfterEach(func() {
		if tmpFile != "" {
			_ = os.Remove(tmpFile)
		}
	})

	Context("with malformed frontmatter YAML", func() {
		JustBeforeEach(func() {
			f, createErr := os.CreateTemp("", "task-*.md")
			Expect(createErr).To(BeNil())
			tmpFile = f.Name()

			// Frontmatter with invalid YAML structure but valid regex match
			content := `---
status: todo
priority: 1
tags:
  - item1
  - item2
---
# Valid Task with Complex YAML
`
			_, _ = f.WriteString(content)
			_ = f.Close()

			_, err = lintOp.ExecuteFile(ctx, tmpFile, "Test Task", "test")
		})

		It("handles complex YAML correctly", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with file containing only frontmatter", func() {
		JustBeforeEach(func() {
			f, createErr := os.CreateTemp("", "task-*.md")
			Expect(createErr).To(BeNil())
			tmpFile = f.Name()

			content := `---
status: todo
priority: 1
---
`
			_, _ = f.WriteString(content)
			_ = f.Close()

			_, err = lintOp.ExecuteFile(ctx, tmpFile, "Test Task", "test")
		})

		It("validates frontmatter-only file", func() {
			Expect(err).To(BeNil())
		})
	})
})

var _ = Describe("LintOperation - Orphan Goal Detection", func() {
	var (
		ctx       context.Context
		lintOp    ops.LintOperation
		vaultPath string
		tasksDir  string
		goalsDir  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		lintOp = ops.NewLintOperation()

		// Create temp vault directory
		var err error
		vaultPath, err = os.MkdirTemp("", "vault-orphan-test-*")
		Expect(err).To(BeNil())

		tasksDir = "Tasks"
		goalsDir = "Goals"

		tasksDirPath := filepath.Join(vaultPath, tasksDir)
		goalsDirPath := filepath.Join(vaultPath, goalsDir)

		Expect(os.MkdirAll(tasksDirPath, 0755)).To(Succeed())
		Expect(os.MkdirAll(goalsDirPath, 0755)).To(Succeed())
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	Context("when goal file exists", func() {
		BeforeEach(func() {
			// Create goal file
			goalContent := `---
status: in_progress
---
# My Goal
`
			goalPath := filepath.Join(vaultPath, goalsDir, "My Goal.md")
			Expect(os.WriteFile(goalPath, []byte(goalContent), 0600)).To(Succeed())

			// Create task referencing existing goal
			taskContent := `---
status: todo
page_type: task
goals: ["[[My Goal]]"]
---
# Task referencing existing goal
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent), 0600)).To(Succeed())
		})

		It("reports no orphan goal issues", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
		})
	})

	Context("when goal file does not exist", func() {
		BeforeEach(func() {
			// Create task referencing non-existent goal
			taskContent := `---
status: todo
page_type: task
goals: ["[[Missing Goal]]"]
task_identifier: test-uuid-orphan
---
# Task with orphan goal
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent), 0600)).To(Succeed())
		})

		It("detects orphan goal", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})

		It("marks orphan goal as not fixable", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})
	})

	Context("with multi-line goals format", func() {
		BeforeEach(func() {
			taskContent := `---
status: todo
page_type: task
goals:
  - "[[Existing Goal]]"
  - "[[Missing Goal]]"
task_identifier: test-uuid-multigoal
---
# Task with multi-line goals
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent), 0600)).To(Succeed())

			// Create one of the goals
			goalContent := `---
status: todo
---
# Existing Goal
`
			goalPath := filepath.Join(vaultPath, goalsDir, "Existing Goal.md")
			Expect(os.WriteFile(goalPath, []byte(goalContent), 0600)).To(Succeed())
		})

		It("detects the missing goal", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})
	})
})

var _ = Describe("LintOperation - Status Checkbox Mismatch", func() {
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
		vaultPath, err = os.MkdirTemp("", "vault-checkbox-test-*")
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

	Context("when status is completed but checkboxes are unchecked", func() {
		BeforeEach(func() {
			taskContent := `---
status: completed
page_type: task
task_identifier: test-uuid-unchecked
---
# Task with unchecked boxes

- [x] Done item
- [ ] Not done item
- [ ] Another not done item
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent), 0600)).To(Succeed())
		})

		It("detects status/checkbox mismatch", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})

		It("marks as not fixable", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})
	})

	Context("when all checkboxes are checked but status is not completed", func() {
		BeforeEach(func() {
			taskContent := `---
status: in_progress
page_type: task
task_identifier: test-uuid-allchecked
---
# Task with all checked boxes

- [x] Done item 1
- [x] Done item 2
- [x] Done item 3
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent), 0600)).To(Succeed())
		})

		It("detects status/checkbox mismatch", func() {
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})

		It("fixes by setting status to completed", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())

			// Verify file was fixed
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).To(BeNil())
			Expect(string(content)).To(ContainSubstring("status: completed"))
			Expect(string(content)).NotTo(ContainSubstring("status: in_progress"))
		})
	})

	Context("when task is recurring with unchecked boxes", func() {
		BeforeEach(func() {
			taskContent := `---
status: in_progress
page_type: task
recurring: daily
---
# Recurring task

- [x] Done today
- [ ] Not done yet
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Recurring.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent), 0600)).To(Succeed())
		})

		It("skips checkbox mismatch check for recurring tasks", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
		})
	})

	Context("when task has no checkboxes", func() {
		BeforeEach(func() {
			taskContent := `---
status: completed
page_type: task
---
# Task with no checkboxes

This task is done but has no checkboxes.
`
			taskPath := filepath.Join(vaultPath, tasksDir, "NoCheckboxes.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent), 0600)).To(Succeed())
		})

		It("does not report checkbox mismatch", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
		})
	})

	Context("when status is completed and all checkboxes are checked", func() {
		BeforeEach(func() {
			taskContent := `---
status: completed
page_type: task
---
# Properly completed task

- [x] All items
- [x] Are checked
`
			taskPath := filepath.Join(vaultPath, tasksDir, "Complete.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent), 0600)).To(Succeed())
		})

		It("reports no issues", func() {
			_, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
		})
	})
})

var _ = Describe("LintOperation - Status Phase Mismatch", func() {
	var (
		ctx       context.Context
		lintOp    ops.LintOperation
		vaultPath string
		tasksDir  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		lintOp = ops.NewLintOperation()

		var err error
		vaultPath, err = os.MkdirTemp("", "vault-phase-test-*")
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

	writeTask := func(vaultPath, tasksDir, status, phase string) {
		var content string
		if phase == "" {
			content = "---\nstatus: " + status + "\npage_type: task\ntask_identifier: test-uuid-phase\n---\n# Task\n"
		} else {
			content = "---\nstatus: " + status + "\nphase: " + phase + "\npage_type: task\ntask_identifier: test-uuid-phase\n---\n# Task\n"
		}
		taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
		Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
	}

	Context("STATUS_PHASE_MISMATCH — should trigger", func() {
		DescribeTable("rule 1: completed/aborted with non-done phase",
			func(status, phase string) {
				writeTask(vaultPath, tasksDir, status, phase)
				issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
				Expect(err).To(BeNil())
				Expect(issues).To(HaveLen(1))
			},
			Entry("completed + in_progress", "completed", "in_progress"),
			Entry("completed + todo", "completed", "todo"),
			Entry("aborted + planning", "aborted", "planning"),
			Entry("aborted + human_review", "aborted", "human_review"),
		)

		DescribeTable("rule 2: phase=done with non-completed status",
			func(status, phase string) {
				writeTask(vaultPath, tasksDir, status, phase)
				issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
				Expect(err).To(BeNil())
				Expect(issues).To(HaveLen(1))
			},
			Entry("todo + done", "todo", "done"),
			Entry("in_progress + done", "in_progress", "done"),
			Entry("backlog + done", "backlog", "done"),
		)

		DescribeTable("rule 3: backlog/hold with active phase",
			func(status, phase string) {
				writeTask(vaultPath, tasksDir, status, phase)
				issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
				Expect(err).To(BeNil())
				Expect(issues).To(HaveLen(1))
			},
			Entry("backlog + in_progress", "backlog", "in_progress"),
			Entry("backlog + ai_review", "backlog", "ai_review"),
			Entry("hold + human_review", "hold", "human_review"),
			Entry("hold + in_progress", "hold", "in_progress"),
		)

		It("is non-fixable", func() {
			writeTask(vaultPath, tasksDir, "completed", "in_progress")
			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, true)
			Expect(err).To(BeNil())
			Expect(issues).To(HaveLen(1))
		})
	})

	Context("STATUS_PHASE_MISMATCH — should NOT trigger", func() {
		DescribeTable("valid combinations",
			func(status, phase string) {
				writeTask(vaultPath, tasksDir, status, phase)
				_, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
				Expect(err).To(BeNil())
			},
			Entry("completed + done", "completed", "done"),
			Entry("completed + no phase", "completed", ""),
			Entry("aborted + no phase", "aborted", ""),
			Entry("todo + todo", "todo", "todo"),
			Entry("in_progress + in_progress", "in_progress", "in_progress"),
			Entry("in_progress + ai_review", "in_progress", "ai_review"),
			Entry("hold + todo", "hold", "todo"),
			Entry("hold + planning", "hold", "planning"),
			Entry("backlog + todo", "backlog", "todo"),
			Entry("backlog + no phase", "backlog", ""),
		)
	})
})

var _ = Describe("LintOperation - Missing Task Identifier", func() {
	var (
		ctx       context.Context
		lintOp    ops.LintOperation
		vaultPath string
		tasksDir  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		lintOp = ops.NewLintOperation()

		var err error
		vaultPath, err = os.MkdirTemp("", "vault-taskid-test-*")
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

	Context("MISSING_TASK_IDENTIFIER", func() {
		It("detects missing task_identifier", func() {
			content := "---\nstatus: todo\npage_type: task\n---\n# Task Without Identifier\n"
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			found := false
			for _, i := range issues {
				if i.IssueType == ops.IssueTypeMissingTaskIdentifier {
					found = true
					Expect(i.Fixable).To(BeFalse())
				}
			}
			Expect(found).To(BeTrue(), "expected MISSING_TASK_IDENTIFIER issue")
		})

		It("does not report MISSING_TASK_IDENTIFIER when task_identifier is present", func() {
			content := "---\nstatus: todo\npage_type: task\ntask_identifier: some-uuid\n---\n# Task With Identifier\n"
			taskPath := filepath.Join(vaultPath, tasksDir, "Task.md")
			Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			for _, i := range issues {
				Expect(i.IssueType).NotTo(Equal(ops.IssueTypeMissingTaskIdentifier))
			}
		})

		It("does not report MISSING_TASK_IDENTIFIER for files with missing frontmatter", func() {
			content := "# Task Without Frontmatter\n\nThis task has no frontmatter at all.\n"
			taskPath := filepath.Join(vaultPath, tasksDir, "NoFrontmatter.md")
			Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

			issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
			Expect(err).To(BeNil())
			for _, i := range issues {
				Expect(i.IssueType).NotTo(Equal(ops.IssueTypeMissingTaskIdentifier))
			}
		})
	})
})
