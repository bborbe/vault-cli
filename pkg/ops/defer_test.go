// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"strings"
	"time"

	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("DeferOperation", func() {
	var (
		ctx                  context.Context
		err                  error
		deferOp              ops.DeferOperation
		mockTaskStorage      *mocks.TaskStorage
		mockDailyNoteStorage *mocks.DailyNoteStorage
		vaultPath            string
		taskName             string
		dateStr              string
		task                 *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		mockDailyNoteStorage = &mocks.DailyNoteStorage{}
		currentDateTime := libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
		deferOp = ops.NewDeferOperation(mockTaskStorage, mockDailyNoteStorage, currentDateTime)
		vaultPath = "/path/to/vault"
		taskName = "my-task"
		dateStr = "+7d"

		// Default: return a task
		task = domain.NewTask(
			map[string]any{"status": "todo"},
			domain.FileMetadata{Name: taskName},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	JustBeforeEach(func() {
		_, err = deferOp.Execute(ctx, vaultPath, taskName, dateStr, "test-vault")
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
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status()).To(Equal(domain.TaskStatusTodo))
			})

			It("sets defer_date to 7 days from now", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate()).NotTo(BeNil())
				expected := libtimetest.ParseDateTime("2026-03-03T12:00:00Z").
					Time().
					AddDate(0, 0, 7).
					Truncate(24 * time.Hour)
				actual := writtenTask.DeferDate().Time()
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
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate()).NotTo(BeNil())
				expected := libtimetest.ParseDateTime("2026-03-03T12:00:00Z").
					Time().
					AddDate(0, 0, 1).
					Truncate(24 * time.Hour)
				actual := writtenTask.DeferDate().Time()
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
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate()).NotTo(BeNil())
				Expect(writtenTask.DeferDate().Time().Weekday()).To(Equal(time.Monday))
				Expect(
					writtenTask.DeferDate().Time().After(
						libtimetest.ParseDateTime("2026-03-03T12:00:00Z").Time(),
					),
				).To(BeTrue())
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
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate()).NotTo(BeNil())
				expected := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
				actual := writtenTask.DeferDate().Time()
				Expect(actual).To(Equal(expected))
			})
		})

		Context("with RFC3339 datetime 2026-12-31T16:00:00+01:00", func() {
			BeforeEach(func() {
				dateStr = "2026-12-31T16:00:00+01:00"
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets defer_date with time component", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate()).NotTo(BeNil())
				loc := time.FixedZone("CET", 3600)
				expected := time.Date(2026, 12, 31, 16, 0, 0, 0, loc)
				Expect(writtenTask.DeferDate().Time().Equal(expected)).To(BeTrue())
			})
		})

		Context("+1d on task with existing DeferDate with time component", func() {
			BeforeEach(func() {
				dateStr = "+1d"
				loc := time.FixedZone("CET", 3600)
				existing := time.Date(2026, 3, 4, 16, 0, 0, 0, loc)
				dd := domain.DateOrDateTime(existing)
				task.SetDeferDate(dd.Ptr())
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("preserves time and adds 1 day", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate()).NotTo(BeNil())
				loc := time.FixedZone("CET", 3600)
				expected := time.Date(2026, 3, 5, 16, 0, 0, 0, loc)
				Expect(writtenTask.DeferDate().Time().Equal(expected)).To(BeTrue())
			})
		})

		Context("+1d on task with date-only DeferDate", func() {
			BeforeEach(func() {
				dateStr = "+1d"
				existing := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
				dd := domain.DateOrDateTime(existing)
				task.SetDeferDate(dd.Ptr())
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("stays date-only, adds 1 day", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate()).NotTo(BeNil())
				// Date-only: no time preservation, just date arithmetic from now
				resultUTC := writtenTask.DeferDate().Time().UTC()
				Expect(resultUTC.Hour()).To(Equal(0))
				Expect(resultUTC.Minute()).To(Equal(0))
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

		It("calls WriteTask", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
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
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
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

	Context("removeFromDailyNote with in-progress checkbox", func() {
		BeforeEach(func() {
			todayContent := `# 2026-03-03

## Tasks
- [/] [[my-task]]
- [ ] Other task
`
			targetContent := "# 2026-12-31\n"
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("removes in-progress checkbox from today's daily note", func() {
			Expect(err).To(BeNil())
			Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// First call should be to update today's note
			_, _, date, updatedContent := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
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
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("inserts task into Should section", func() {
			Expect(err).To(BeNil())
			Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// Second call should be to update target note
			_, _, date, updatedContent := mockDailyNoteStorage.WriteDailyNoteArgsForCall(1)
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
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("inserts task after Must section", func() {
			Expect(err).To(BeNil())
			Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// Second call should be to update target note
			_, _, date, updatedContent := mockDailyNoteStorage.WriteDailyNoteArgsForCall(1)
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
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("appends task to end of file", func() {
			Expect(err).To(BeNil())
			Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// Second call should be to update target note
			_, _, date, updatedContent := mockDailyNoteStorage.WriteDailyNoteArgsForCall(1)
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
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("creates note with Should section", func() {
			Expect(err).To(BeNil())
			Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// Second call should be to create target note
			_, _, date, updatedContent := mockDailyNoteStorage.WriteDailyNoteArgsForCall(1)
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
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("does not add duplicate task", func() {
			Expect(err).To(BeNil())
			// Only 1 write call for today's note (task already exists in target, so no write there)
			Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))

			// First (and only) call should be to update today's note
			_, _, date, _ := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
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
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(0, todayContent, nil)
			mockDailyNoteStorage.ReadDailyNoteReturnsOnCall(1, targetContent, nil)
			mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			dateStr = "2026-12-31"
		})

		It("inserts task into Should section", func() {
			Expect(err).To(BeNil())
			Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(2))

			// Second call should be to update target note
			_, _, date, updatedContent := mockDailyNoteStorage.WriteDailyNoteArgsForCall(1)
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
				plannedDate := libtimetest.ParseDateTime("2026-03-03T12:00:00Z").
					Time().
					AddDate(0, 0, 3)
					// 3 days from now (before target of +7d)
				dd := domain.DateOrDateTime(libtime.ToDate(plannedDate).Time())
				task.SetPlannedDate(dd.Ptr())
				dateStr = "+7d"
			})

			It("clears planned_date", func() {
				Expect(err).To(BeNil())
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.PlannedDate()).To(BeNil())
			})
		})

		Context("when planned_date is after target date", func() {
			BeforeEach(func() {
				plannedDate := libtimetest.ParseDateTime("2026-03-03T12:00:00Z").Time().
					AddDate(0, 0, 14)
					// 14 days from now (after target of +7d)
				dd := domain.DateOrDateTime(libtime.ToDate(plannedDate).Time())
				task.SetPlannedDate(dd.Ptr())
				dateStr = "+7d"
			})

			It("preserves planned_date", func() {
				Expect(err).To(BeNil())
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.PlannedDate()).NotTo(BeNil())
				expected := libtimetest.ParseDateTime("2026-03-03T12:00:00Z").
					Time().
					AddDate(0, 0, 14).
					Truncate(24 * time.Hour)
				actual := writtenTask.PlannedDate().Time()
				Expect(actual).To(Equal(expected))
			})
		})

		Context("when planned_date is nil", func() {
			BeforeEach(func() {
				task.SetPlannedDate(nil)
				dateStr = "+7d"
			})

			It("works without error", func() {
				Expect(err).To(BeNil())
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.PlannedDate()).To(BeNil())
			})
		})
	})

	Context("past date validation", func() {
		Context("when deferring to yesterday", func() {
			BeforeEach(func() {
				yesterday := libtimetest.ParseDateTime("2026-03-03T12:00:00Z").
					Time().
					AddDate(0, 0, -1)
				dateStr = yesterday.Format("2006-01-02")
			})

			It("returns error", func() {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("cannot defer to past"))
			})

			It("does not write task", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
			})
		})

		Context("when deferring to today", func() {
			BeforeEach(func() {
				today := libtimetest.ParseDateTime("2026-03-03T12:00:00Z").Time()
				dateStr = today.Format("2006-01-02")
			})

			It("succeeds without error", func() {
				Expect(err).To(BeNil())
			})

			It("writes task", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			})
		})
	})
})
