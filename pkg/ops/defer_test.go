// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("DeferOperation", func() {
	var (
		ctx         context.Context
		err         error
		deferOp     ops.DeferOperation
		mockStorage *mocks.Storage
		vaultPath   string
		taskName    string
		dateStr     string
		task        *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockStorage = &mocks.Storage{}
		deferOp = ops.NewDeferOperation(mockStorage)
		vaultPath = "/path/to/vault"
		taskName = "my-task"
		dateStr = "+7d"

		// Default: return a task
		task = &domain.Task{
			Name:   taskName,
			Status: domain.TaskStatusTodo,
		}
		mockStorage.FindTaskByNameReturns(task, nil)
		mockStorage.WriteTaskReturns(nil)
	})

	JustBeforeEach(func() {
		err = deferOp.Execute(ctx, vaultPath, taskName, dateStr, "test-vault", "plain")
	})

	Context("success", func() {
		Context("with relative date +7d", func() {
			BeforeEach(func() {
				dateStr = "+7d"
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("does not change task status", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status).To(Equal(domain.TaskStatusTodo))
			})

			It("sets defer_date to 7 days from now", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate).NotTo(BeNil())
				expected := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)
				actual := writtenTask.DeferDate.Truncate(24 * time.Hour)
				Expect(actual).To(Equal(expected))
			})
		})

		Context("with relative date +1d", func() {
			BeforeEach(func() {
				dateStr = "+1d"
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets defer_date to 1 day from now", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate).NotTo(BeNil())
				expected := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
				actual := writtenTask.DeferDate.Truncate(24 * time.Hour)
				Expect(actual).To(Equal(expected))
			})
		})

		Context("with weekday name monday", func() {
			BeforeEach(func() {
				dateStr = "monday"
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets defer_date to next Monday", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate).NotTo(BeNil())
				Expect(writtenTask.DeferDate.Weekday()).To(Equal(time.Monday))
				Expect(writtenTask.DeferDate.After(time.Now())).To(BeTrue())
			})
		})

		Context("with ISO date 2026-12-31", func() {
			BeforeEach(func() {
				dateStr = "2026-12-31"
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets defer_date to specified date", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate).NotTo(BeNil())
				expected := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
				actual := writtenTask.DeferDate.Truncate(24 * time.Hour)
				Expect(actual).To(Equal(expected))
			})
		})

		It("calls FindTaskByName", func() {
			Expect(mockStorage.FindTaskByNameCallCount()).To(Equal(1))
			actualCtx, actualVaultPath, actualTaskName := mockStorage.FindTaskByNameArgsForCall(0)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualTaskName).To(Equal(taskName))
		})

		It("calls WriteTask", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
		})
	})

	Context("invalid date format", func() {
		BeforeEach(func() {
			dateStr = "invalid-date"
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})

		It("does not call WriteTask", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
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

	Context("removeFromDailyNote with in-progress checkbox", func() {
		BeforeEach(func() {
			todayContent := `# 2026-03-03

## Tasks
- [/] [[my-task]]
- [ ] Other task
`
			targetContent := "# 2026-12-31\n"
			mockStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("removes in-progress checkbox from today's daily note", func() {
			Expect(err).To(BeNil())
			Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// First call should be to update today's note
			_, _, date, updatedContent := mockStorage.WriteDailyNoteArgsForCall(0)
			Expect(date).To(ContainSubstring("2026-03-03"))
			Expect(updatedContent).NotTo(ContainSubstring("my-task"))
			Expect(updatedContent).To(ContainSubstring("Other task"))
		})
	})

	Context("addToDailyNote with Should section", func() {
		BeforeEach(func() {
			todayContent := `# 2026-03-03

## Tasks
- [ ] [[my-task]]
`
			targetContent := `# 2026-12-31

## Must
- [ ] [[urgent-task]]

## Should
- [ ] [[existing-task]]

## Could
- [ ] [[nice-to-have]]
`
			mockStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("inserts task into Should section", func() {
			Expect(err).To(BeNil())
			Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// Second call should be to update target note
			_, _, date, updatedContent := mockStorage.WriteDailyNoteArgsForCall(1)
			Expect(date).To(Equal("2026-12-31"))

			// Task should be in Should section
			lines := strings.Split(updatedContent, "\n")
			shouldIdx := -1
			couldIdx := -1
			taskIdx := -1

			for i, line := range lines {
				if strings.Contains(line, "## Should") {
					shouldIdx = i
				}
				if strings.Contains(line, "## Could") {
					couldIdx = i
				}
				if strings.Contains(line, "- [ ] [[my-task]]") {
					taskIdx = i
				}
			}

			Expect(shouldIdx).To(BeNumerically(">", -1))
			Expect(couldIdx).To(BeNumerically(">", -1))
			Expect(taskIdx).To(BeNumerically(">", -1))
			// Task should be after Should heading and before Could heading
			Expect(taskIdx).To(BeNumerically(">", shouldIdx))
			Expect(taskIdx).To(BeNumerically("<", couldIdx))
		})
	})

	Context("addToDailyNote with Must but no Should section", func() {
		BeforeEach(func() {
			todayContent := `# 2026-03-03

## Tasks
- [ ] [[my-task]]
`
			targetContent := `# 2026-12-31

## Must
- [ ] [[urgent-task]]
- [ ] [[another-urgent]]

## Could
- [ ] [[nice-to-have]]
`
			mockStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("inserts task after Must section", func() {
			Expect(err).To(BeNil())
			Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// Second call should be to update target note
			_, _, date, updatedContent := mockStorage.WriteDailyNoteArgsForCall(1)
			Expect(date).To(Equal("2026-12-31"))

			// Task should be after Must section
			lines := strings.Split(updatedContent, "\n")
			mustIdx := -1
			couldIdx := -1
			taskIdx := -1

			for i, line := range lines {
				if strings.Contains(line, "## Must") {
					mustIdx = i
				}
				if strings.Contains(line, "## Could") {
					couldIdx = i
				}
				if strings.Contains(line, "- [ ] [[my-task]]") {
					taskIdx = i
				}
			}

			Expect(mustIdx).To(BeNumerically(">", -1))
			Expect(couldIdx).To(BeNumerically(">", -1))
			Expect(taskIdx).To(BeNumerically(">", -1))
			// Task should be after Must heading and before Could heading
			Expect(taskIdx).To(BeNumerically(">", mustIdx))
			Expect(taskIdx).To(BeNumerically("<", couldIdx))
		})
	})

	Context("addToDailyNote with no sections", func() {
		BeforeEach(func() {
			todayContent := `# 2026-03-03

## Tasks
- [ ] [[my-task]]
`
			targetContent := `# 2026-12-31

Some random content here.
No section headings.
`
			mockStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("appends task to end of file", func() {
			Expect(err).To(BeNil())
			Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// Second call should be to update target note
			_, _, date, updatedContent := mockStorage.WriteDailyNoteArgsForCall(1)
			Expect(date).To(Equal("2026-12-31"))
			Expect(updatedContent).To(ContainSubstring("- [ ] [[my-task]]"))

			// Task should be at the end
			lines := strings.Split(updatedContent, "\n")
			found := false
			for i := len(lines) - 3; i < len(lines); i++ {
				if i >= 0 && strings.Contains(lines[i], "- [ ] [[my-task]]") {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Context("addToDailyNote to empty daily note", func() {
		BeforeEach(func() {
			todayContent := `# 2026-03-03

## Tasks
- [ ] [[my-task]]
`
			targetContent := ""
			mockStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("creates note with Should section", func() {
			Expect(err).To(BeNil())
			Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// Second call should be to create target note
			_, _, date, updatedContent := mockStorage.WriteDailyNoteArgsForCall(1)
			Expect(date).To(Equal("2026-12-31"))
			Expect(updatedContent).To(ContainSubstring("## Should"))
			Expect(updatedContent).To(ContainSubstring("- [ ] [[my-task]]"))

			// Should section should come before the task
			shouldIdx := strings.Index(updatedContent, "## Should")
			taskIdx := strings.Index(updatedContent, "- [ ] [[my-task]]")
			Expect(shouldIdx).To(BeNumerically("<", taskIdx))
		})
	})

	Context("addToDailyNote when task already exists", func() {
		BeforeEach(func() {
			todayContent := `# 2026-03-03

## Tasks
- [ ] [[other-task]]
`
			targetContent := `# 2026-12-31

## Should
- [ ] [[my-task]]
- [ ] [[another-task]]
`
			mockStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("does not add duplicate task", func() {
			Expect(err).To(BeNil())
			// Only 1 write call for today's note (task already exists in target, so no write there)
			Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(1))

			// First (and only) call should be to update today's note
			_, _, date, _ := mockStorage.WriteDailyNoteArgsForCall(0)
			Expect(date).To(ContainSubstring("2026-03-03"))
		})
	})

	Context("addToDailyNote with ### Should (three hashes)", func() {
		BeforeEach(func() {
			todayContent := `# 2026-03-03

## Tasks
- [ ] [[my-task]]
`
			targetContent := `# 2026-12-31

### Should
- [ ] [[existing-task]]

### Could
- [ ] [[nice-to-have]]
`
			mockStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("inserts task into Should section", func() {
			Expect(err).To(BeNil())
			Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// Second call should be to update target note
			_, _, date, updatedContent := mockStorage.WriteDailyNoteArgsForCall(1)
			Expect(date).To(Equal("2026-12-31"))

			// Task should be in Should section
			lines := strings.Split(updatedContent, "\n")
			shouldIdx := -1
			couldIdx := -1
			taskIdx := -1

			for i, line := range lines {
				if strings.Contains(line, "### Should") {
					shouldIdx = i
				}
				if strings.Contains(line, "### Could") {
					couldIdx = i
				}
				if strings.Contains(line, "- [ ] [[my-task]]") {
					taskIdx = i
				}
			}

			Expect(shouldIdx).To(BeNumerically(">", -1))
			Expect(couldIdx).To(BeNumerically(">", -1))
			Expect(taskIdx).To(BeNumerically(">", -1))
			// Task should be after Should heading and before Could heading
			Expect(taskIdx).To(BeNumerically(">", shouldIdx))
			Expect(taskIdx).To(BeNumerically("<", couldIdx))
		})
	})

	Context("planned_date handling", func() {
		Context("when planned_date is before target date", func() {
			BeforeEach(func() {
				plannedDate := time.Now().AddDate(0, 0, 3) // 3 days from now (before target of +7d)
				task.PlannedDate = &plannedDate
				dateStr = "+7d"
			})

			It("clears planned_date", func() {
				Expect(err).To(BeNil())
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.PlannedDate).To(BeNil())
			})
		})

		Context("when planned_date is after target date", func() {
			BeforeEach(func() {
				plannedDate := time.Now().
					AddDate(0, 0, 14)
					// 14 days from now (after target of +7d)
				task.PlannedDate = &plannedDate
				dateStr = "+7d"
			})

			It("preserves planned_date", func() {
				Expect(err).To(BeNil())
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.PlannedDate).NotTo(BeNil())
				expected := time.Now().AddDate(0, 0, 14).Truncate(24 * time.Hour)
				actual := writtenTask.PlannedDate.Truncate(24 * time.Hour)
				Expect(actual).To(Equal(expected))
			})
		})

		Context("when planned_date is nil", func() {
			BeforeEach(func() {
				task.PlannedDate = nil
				dateStr = "+7d"
			})

			It("works without error", func() {
				Expect(err).To(BeNil())
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.PlannedDate).To(BeNil())
			})
		})
	})

	Context("past date validation", func() {
		Context("when deferring to yesterday", func() {
			BeforeEach(func() {
				yesterday := time.Now().AddDate(0, 0, -1)
				dateStr = yesterday.Format("2006-01-02")
			})

			It("returns error", func() {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("cannot defer to past"))
			})

			It("does not write task", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
			})
		})

		Context("when deferring to today", func() {
			BeforeEach(func() {
				today := time.Now()
				dateStr = today.Format("2006-01-02")
			})

			It("succeeds without error", func() {
				Expect(err).To(BeNil())
			})

			It("writes task", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			})
		})
	})
})
