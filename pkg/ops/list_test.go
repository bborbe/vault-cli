// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("ListOperation", func() {
	var ctx context.Context
	var err error
	var listOp ops.ListOperation
	var mockPageStorage *mocks.PageStorage
	var vaultPath string
	var pagesDir string
	var statusFilters []string
	var showAll bool
	var assigneeFilter string
	var goalFilter string
	var tasks []*domain.Task

	BeforeEach(func() {
		ctx = context.Background()
		mockPageStorage = &mocks.PageStorage{}
		listOp = ops.NewListOperation(mockPageStorage)
		vaultPath = "/path/to/vault"
		pagesDir = "Tasks"
		statusFilters = nil
		showAll = false
		assigneeFilter = ""
		goalFilter = ""

		// Default: return some test tasks
		tasks = []*domain.Task{
			domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "Task A"},
				domain.Content(""),
			),
			domain.NewTask(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: "Task B"},
				domain.Content(""),
			),
			domain.NewTask(
				map[string]any{"status": "completed"},
				domain.FileMetadata{Name: "Task C"},
				domain.Content(""),
			),
			domain.NewTask(
				map[string]any{"status": "hold"},
				domain.FileMetadata{Name: "Task D"},
				domain.Content(""),
			),
		}
		mockPageStorage.ListPagesReturns(tasks, nil)
	})

	JustBeforeEach(func() {
		_, err = listOp.Execute(
			ctx,
			vaultPath,
			"test-vault",
			pagesDir,
			statusFilters,
			showAll,
			assigneeFilter,
			goalFilter,
		)
	})

	Context("success", func() {
		Context("with default filter (no flags)", func() {
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("calls ListPages", func() {
				Expect(mockPageStorage.ListPagesCallCount()).To(Equal(1))
				actualCtx, actualVaultPath, actualPagesDir := mockPageStorage.ListPagesArgsForCall(
					0,
				)
				Expect(actualCtx).To(Equal(ctx))
				Expect(actualVaultPath).To(Equal(vaultPath))
				Expect(actualPagesDir).To(Equal(pagesDir))
			})
		})

		Context("with --status filter", func() {
			BeforeEach(func() {
				statusFilters = []string{"in_progress"}
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("calls ListPages", func() {
				Expect(mockPageStorage.ListPagesCallCount()).To(Equal(1))
			})
		})

		Context("with --status filter (case-insensitive)", func() {
			BeforeEach(func() {
				statusFilters = []string{"In_Progress"}
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("calls ListPages", func() {
				Expect(mockPageStorage.ListPagesCallCount()).To(Equal(1))
			})
		})

		Context("with --all flag", func() {
			BeforeEach(func() {
				showAll = true
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("calls ListPages", func() {
				Expect(mockPageStorage.ListPagesCallCount()).To(Equal(1))
			})

			Context(
				"with tasks of all statuses including backlog, completed, hold, aborted",
				func() {
					BeforeEach(func() {
						tasks = []*domain.Task{
							domain.NewTask(
								map[string]any{"status": "todo"},
								domain.FileMetadata{Name: "Task Todo"},
								domain.Content(""),
							),
							domain.NewTask(
								map[string]any{"status": "in_progress"},
								domain.FileMetadata{Name: "Task InProgress"},
								domain.Content(""),
							),
							domain.NewTask(
								map[string]any{"status": "completed"},
								domain.FileMetadata{Name: "Task Done"},
								domain.Content(""),
							),
							domain.NewTask(
								map[string]any{"status": "hold"},
								domain.FileMetadata{Name: "Task Deferred"},
								domain.Content(""),
							),
							domain.NewTask(
								map[string]any{"status": "backlog"},
								domain.FileMetadata{Name: "Task Backlog"},
								domain.Content(""),
							),
							domain.NewTask(
								map[string]any{"status": "completed"},
								domain.FileMetadata{Name: "Task Completed"},
								domain.Content(""),
							),
							domain.NewTask(
								map[string]any{"status": "hold"},
								domain.FileMetadata{Name: "Task Hold"},
								domain.Content(""),
							),
							domain.NewTask(
								map[string]any{"status": "aborted"},
								domain.FileMetadata{Name: "Task Aborted"},
								domain.Content(""),
							),
						}
						mockPageStorage.ListPagesReturns(tasks, nil)
					})

					It("returns no error and processes all task statuses", func() {
						Expect(err).To(BeNil())
					})
				},
			)
		})
	})

	Context("storage error", func() {
		BeforeEach(func() {
			mockPageStorage.ListPagesReturns(nil, ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})
	})

	Context("with --goal filter", func() {
		BeforeEach(func() {
			goalFilter = "Return to Live Trading"
			taskWithGoal := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "Task With Goal"},
				domain.Content(""),
			)
			taskWithGoal.SetGoals([]string{"Return to Live Trading"})
			taskWithOtherGoal := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "Task Without Goal"},
				domain.Content(""),
			)
			taskWithOtherGoal.SetGoals([]string{"Other Goal"})
			tasks = []*domain.Task{
				taskWithGoal,
				taskWithOtherGoal,
				domain.NewTask(
					map[string]any{"status": "todo"},
					domain.FileMetadata{Name: "Task No Goals"},
					domain.Content(""),
				),
			}
			mockPageStorage.ListPagesReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with --goal filter and no matching tasks", func() {
		BeforeEach(func() {
			goalFilter = "Nonexistent Goal"
			taskA := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "Task A"},
				domain.Content(""),
			)
			taskA.SetGoals([]string{"Some Goal"})
			tasks = []*domain.Task{taskA}
			mockPageStorage.ListPagesReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with --assignee filter (case-insensitive)", func() {
		BeforeEach(func() {
			assigneeFilter = "localclaw"
			taskMatch := domain.NewTask(
				map[string]any{"status": "todo", "assignee": "LocalClaw"},
				domain.FileMetadata{Name: "Task With Matching Assignee"},
				domain.Content(""),
			)
			taskDiff := domain.NewTask(
				map[string]any{"status": "todo", "assignee": "alice"},
				domain.FileMetadata{Name: "Task With Different Assignee"},
				domain.Content(""),
			)
			taskNoAssignee := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "Task Without Assignee"},
				domain.Content(""),
			)
			tasks = []*domain.Task{taskMatch, taskDiff, taskNoAssignee}
			mockPageStorage.ListPagesReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with --assignee filter matching lowercase stored value", func() {
		BeforeEach(func() {
			assigneeFilter = "LocalClaw"
			taskLower := domain.NewTask(
				map[string]any{"status": "todo", "assignee": "localclaw"},
				domain.FileMetadata{Name: "Task With Lowercase Assignee"},
				domain.Content(""),
			)
			tasks = []*domain.Task{taskLower}
			mockPageStorage.ListPagesReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with --assignee filter not matching different name", func() {
		BeforeEach(func() {
			assigneeFilter = "bob"
			taskAlice := domain.NewTask(
				map[string]any{"status": "todo", "assignee": "alice"},
				domain.FileMetadata{Name: "Task With Alice"},
				domain.Content(""),
			)
			tasks = []*domain.Task{taskAlice}
			mockPageStorage.ListPagesReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with --goal and --status filters combined", func() {
		BeforeEach(func() {
			goalFilter = "My Goal"
			statusFilters = []string{"in_progress"}
			matchBoth := domain.NewTask(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: "Matching Both"},
				domain.Content(""),
			)
			matchBoth.SetGoals([]string{"My Goal"})
			goalMatchOnly := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "Goal Match Status Mismatch"},
				domain.Content(""),
			)
			goalMatchOnly.SetGoals([]string{"My Goal"})
			statusMatchOnly := domain.NewTask(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: "Status Match Goal Mismatch"},
				domain.Content(""),
			)
			statusMatchOnly.SetGoals([]string{"Other Goal"})
			tasks = []*domain.Task{matchBoth, goalMatchOnly, statusMatchOnly}
			mockPageStorage.ListPagesReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with --goal filter case sensitivity", func() {
		BeforeEach(func() {
			goalFilter = "my goal"
			taskDiffCase := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "Task With Different Case"},
				domain.Content(""),
			)
			taskDiffCase.SetGoals([]string{"My Goal"})
			tasks = []*domain.Task{taskDiffCase}
			mockPageStorage.ListPagesReturns(tasks, nil)
		})

		It("returns no error (case-sensitive means no match)", func() {
			Expect(err).To(BeNil())
		})
	})
})

var _ = Describe("ListOperation JSON output", func() {
	var ctx context.Context
	var listOp ops.ListOperation
	var mockPageStorage *mocks.PageStorage
	var items []ops.TaskListItem
	var execErr error

	BeforeEach(func() {
		ctx = context.Background()
		mockPageStorage = &mocks.PageStorage{}
		listOp = ops.NewListOperation(mockPageStorage)
	})

	Context("with all enriched fields populated", func() {
		BeforeEach(func() {
			deferDate := domain.DateOrDateTime(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC))
			plannedDate := domain.DateOrDateTime(time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
			dueDate := domain.DateOrDateTime(time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC))
			enrichedTask := domain.NewTask(
				map[string]any{
					"status":            "in_progress",
					"assignee":          "alice",
					"priority":          1,
					"page_type":         "feature",
					"recurring":         "weekly",
					"claude_session_id": "sess-abc123",
					"phase":             "in_progress",
				},
				domain.FileMetadata{Name: "Enriched Task"},
				domain.Content(""),
			)
			enrichedTask.SetDeferDate(&deferDate)
			enrichedTask.SetPlannedDate(&plannedDate)
			enrichedTask.SetDueDate(&dueDate)
			tasks := []*domain.Task{enrichedTask}
			mockPageStorage.ListPagesReturns(tasks, nil)
			items, execErr = listOp.Execute(ctx, "/vault", "my-vault", "Tasks", nil, true, "", "")
		})

		It("succeeds", func() {
			Expect(execErr).To(BeNil())
			Expect(items).To(HaveLen(1))
		})

		It("includes category from page_type", func() {
			Expect(execErr).To(BeNil())
			Expect(items[0].Category).To(Equal("feature"))
		})

		It("includes recurring field", func() {
			Expect(execErr).To(BeNil())
			Expect(items[0].Recurring).To(Equal("weekly"))
		})

		It("includes defer_date formatted as YYYY-MM-DD", func() {
			Expect(execErr).To(BeNil())
			Expect(items[0].DeferDate).To(Equal("2026-03-15"))
		})

		It("includes planned_date formatted as YYYY-MM-DD", func() {
			Expect(execErr).To(BeNil())
			Expect(items[0].PlannedDate).To(Equal("2026-03-20"))
		})

		It("includes due_date formatted as YYYY-MM-DD", func() {
			Expect(execErr).To(BeNil())
			Expect(items[0].DueDate).To(Equal("2026-03-25"))
		})

		It("includes claude_session_id", func() {
			Expect(execErr).To(BeNil())
			Expect(items[0].ClaudeSessionID).To(Equal("sess-abc123"))
		})

		It("includes phase", func() {
			Expect(execErr).To(BeNil())
			Expect(items[0].Phase).To(Equal("in_progress"))
		})
	})

	Context("with optional fields empty", func() {
		BeforeEach(func() {
			tasks := []*domain.Task{
				domain.NewTask(
					map[string]any{"status": "todo"},
					domain.FileMetadata{Name: "Minimal Task"},
					domain.Content(""),
				),
			}
			mockPageStorage.ListPagesReturns(tasks, nil)
			items, execErr = listOp.Execute(ctx, "/vault", "my-vault", "Tasks", nil, true, "", "")
		})

		It("omits empty optional fields", func() {
			Expect(execErr).To(BeNil())
			Expect(items).To(HaveLen(1))
			Expect(items[0].Category).To(BeEmpty())
			Expect(items[0].Recurring).To(BeEmpty())
			Expect(items[0].DeferDate).To(BeEmpty())
			Expect(items[0].PlannedDate).To(BeEmpty())
			Expect(items[0].DueDate).To(BeEmpty())
			Expect(items[0].ClaudeSessionID).To(BeEmpty())
			Expect(items[0].Phase).To(BeEmpty())
		})

		It("still includes required fields", func() {
			Expect(execErr).To(BeNil())
			Expect(items).To(HaveLen(1))
			Expect(items[0].Name).To(Equal("Minimal Task"))
			Expect(items[0].Status).To(Equal("todo"))
			Expect(items[0].Vault).To(Equal("my-vault"))
		})
	})

	Context("with --goal filter", func() {
		BeforeEach(func() {
			matchingTask := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "Matching Task"},
				domain.Content(""),
			)
			matchingTask.SetGoals([]string{"Target Goal", "Other Goal"})
			nonMatchingTask := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "Non-matching Task"},
				domain.Content(""),
			)
			nonMatchingTask.SetGoals([]string{"Different Goal"})
			tasks := []*domain.Task{matchingTask, nonMatchingTask}
			mockPageStorage.ListPagesReturns(tasks, nil)
			items, execErr = listOp.Execute(
				ctx,
				"/vault",
				"my-vault",
				"Tasks",
				nil,
				true,
				"",
				"Target Goal",
			)
		})

		It("returns only tasks matching the goal", func() {
			Expect(execErr).To(BeNil())
			Expect(items).To(HaveLen(1))
			Expect(items[0].Name).To(Equal("Matching Task"))
		})
	})

	Context("with datetime date fields (non-zero time component)", func() {
		BeforeEach(func() {
			loc := time.FixedZone("CET", 3600)
			deferDate := domain.DateOrDateTime(time.Date(2026, 3, 18, 16, 0, 0, 0, loc))
			plannedDate := domain.DateOrDateTime(time.Date(2026, 3, 20, 9, 30, 0, 0, time.UTC))
			dueDate := domain.DateOrDateTime(time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC))
			datetimeTask := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "Datetime Task"},
				domain.Content(""),
			)
			datetimeTask.SetDeferDate(&deferDate)
			datetimeTask.SetPlannedDate(&plannedDate)
			datetimeTask.SetDueDate(&dueDate)
			tasks := []*domain.Task{datetimeTask}
			mockPageStorage.ListPagesReturns(tasks, nil)
			items, execErr = listOp.Execute(ctx, "/vault", "my-vault", "Tasks", nil, true, "", "")
		})

		It("formats defer_date with time component as RFC3339", func() {
			Expect(execErr).To(BeNil())
			Expect(items[0].DeferDate).To(Equal("2026-03-18T16:00:00+01:00"))
		})

		It("formats planned_date with time component as RFC3339", func() {
			Expect(execErr).To(BeNil())
			Expect(items[0].PlannedDate).To(Equal("2026-03-20T09:30:00Z"))
		})

		It("formats due_date at midnight UTC as YYYY-MM-DD", func() {
			Expect(execErr).To(BeNil())
			Expect(items[0].DueDate).To(Equal("2026-03-25"))
		})
	})

	Context("with modified_date populated", func() {
		BeforeEach(func() {
			modTime := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
			tasks := []*domain.Task{
				domain.NewTask(
					map[string]any{"status": "todo"},
					domain.FileMetadata{Name: "Modified Task", ModifiedDate: &modTime},
					domain.Content(""),
				),
			}
			mockPageStorage.ListPagesReturns(tasks, nil)
			items, execErr = listOp.Execute(ctx, "/vault", "my-vault", "Tasks", nil, true, "", "")
		})

		It("includes non-empty modified_date in result", func() {
			Expect(execErr).To(BeNil())
			Expect(items).To(HaveLen(1))
			Expect(items[0].ModifiedDate).To(Equal("2026-03-18T12:00:00Z"))
		})
	})

	Context("with completed_date populated", func() {
		BeforeEach(func() {
			completedTask := domain.NewTask(
				map[string]any{"status": "completed", "completed_date": "2026-03-03T12:00:00Z"},
				domain.FileMetadata{Name: "Completed Task"},
				domain.Content(""),
			)
			tasks := []*domain.Task{completedTask}
			mockPageStorage.ListPagesReturns(tasks, nil)
			items, execErr = listOp.Execute(ctx, "/vault", "my-vault", "Tasks", nil, true, "", "")
		})

		It("includes completed_date in result", func() {
			Expect(execErr).To(BeNil())
			Expect(items).To(HaveLen(1))
			Expect(items[0].CompletedDate).To(Equal("2026-03-03T12:00:00Z"))
		})
	})

	Context("with nil date fields", func() {
		BeforeEach(func() {
			tasks := []*domain.Task{
				domain.NewTask(
					map[string]any{"status": "todo"},
					domain.FileMetadata{Name: "No Dates Task"},
					domain.Content(""),
				),
			}
			mockPageStorage.ListPagesReturns(tasks, nil)
			items, execErr = listOp.Execute(ctx, "/vault", "my-vault", "Tasks", nil, true, "", "")
		})

		It("omits nil date fields from result", func() {
			Expect(execErr).To(BeNil())
			Expect(items[0].DeferDate).To(BeEmpty())
			Expect(items[0].PlannedDate).To(BeEmpty())
			Expect(items[0].DueDate).To(BeEmpty())
		})
	})

	Context("with multiple --status filters", func() {
		BeforeEach(func() {
			tasks := []*domain.Task{
				domain.NewTask(
					map[string]any{"status": "in_progress"},
					domain.FileMetadata{Name: "IP Task"},
					domain.Content(""),
				),
				domain.NewTask(
					map[string]any{"status": "completed"},
					domain.FileMetadata{Name: "Done Task"},
					domain.Content(""),
				),
				domain.NewTask(
					map[string]any{"status": "todo"},
					domain.FileMetadata{Name: "Todo Task"},
					domain.Content(""),
				),
				domain.NewTask(
					map[string]any{"status": "hold"},
					domain.FileMetadata{Name: "Hold Task"},
					domain.Content(""),
				),
			}
			mockPageStorage.ListPagesReturns(tasks, nil)
			items, execErr = listOp.Execute(
				ctx,
				"/vault",
				"my-vault",
				"Tasks",
				[]string{"in_progress", "completed"},
				false,
				"",
				"",
			)
		})

		It("includes tasks matching requested statuses", func() {
			Expect(execErr).To(BeNil())
			Expect(items).To(HaveLen(2))
		})

		It("excludes tasks not matching requested statuses", func() {
			Expect(execErr).To(BeNil())
			for _, item := range items {
				Expect(item.Status).NotTo(Equal("todo"))
				Expect(item.Status).NotTo(Equal("hold"))
			}
		})
	})
})
