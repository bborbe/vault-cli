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
			{
				Name:   "Task A",
				Status: domain.TaskStatusTodo,
			},
			{
				Name:   "Task B",
				Status: domain.TaskStatusInProgress,
			},
			{
				Name:   "Task C",
				Status: domain.TaskStatusCompleted,
			},
			{
				Name:   "Task D",
				Status: domain.TaskStatusHold,
			},
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
							{
								Name:   "Task Todo",
								Status: domain.TaskStatus("todo"),
							},
							{
								Name:   "Task InProgress",
								Status: domain.TaskStatus("in_progress"),
							},
							{
								Name:   "Task Done",
								Status: domain.TaskStatus("completed"),
							},
							{
								Name:   "Task Deferred",
								Status: domain.TaskStatus("hold"),
							},
							{
								Name:   "Task Backlog",
								Status: domain.TaskStatus("backlog"),
							},
							{
								Name:   "Task Completed",
								Status: domain.TaskStatus("completed"),
							},
							{
								Name:   "Task Hold",
								Status: domain.TaskStatus("hold"),
							},
							{
								Name:   "Task Aborted",
								Status: domain.TaskStatus("aborted"),
							},
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
			tasks = []*domain.Task{
				{
					Name:   "Task With Goal",
					Status: domain.TaskStatusTodo,
					Goals:  []string{"Return to Live Trading"},
				},
				{
					Name:   "Task Without Goal",
					Status: domain.TaskStatusTodo,
					Goals:  []string{"Other Goal"},
				},
				{
					Name:   "Task No Goals",
					Status: domain.TaskStatusTodo,
				},
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
			tasks = []*domain.Task{
				{
					Name:   "Task A",
					Status: domain.TaskStatusTodo,
					Goals:  []string{"Some Goal"},
				},
			}
			mockPageStorage.ListPagesReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with --assignee filter (case-insensitive)", func() {
		BeforeEach(func() {
			assigneeFilter = "localclaw"
			tasks = []*domain.Task{
				{
					Name:     "Task With Matching Assignee",
					Status:   domain.TaskStatusTodo,
					Assignee: "LocalClaw",
				},
				{
					Name:     "Task With Different Assignee",
					Status:   domain.TaskStatusTodo,
					Assignee: "alice",
				},
				{
					Name:     "Task Without Assignee",
					Status:   domain.TaskStatusTodo,
					Assignee: "",
				},
			}
			mockPageStorage.ListPagesReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with --assignee filter matching lowercase stored value", func() {
		BeforeEach(func() {
			assigneeFilter = "LocalClaw"
			tasks = []*domain.Task{
				{
					Name:     "Task With Lowercase Assignee",
					Status:   domain.TaskStatusTodo,
					Assignee: "localclaw",
				},
			}
			mockPageStorage.ListPagesReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with --assignee filter not matching different name", func() {
		BeforeEach(func() {
			assigneeFilter = "bob"
			tasks = []*domain.Task{
				{
					Name:     "Task With Alice",
					Status:   domain.TaskStatusTodo,
					Assignee: "alice",
				},
			}
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
			tasks = []*domain.Task{
				{
					Name:   "Matching Both",
					Status: domain.TaskStatusInProgress,
					Goals:  []string{"My Goal"},
				},
				{
					Name:   "Goal Match Status Mismatch",
					Status: domain.TaskStatusTodo,
					Goals:  []string{"My Goal"},
				},
				{
					Name:   "Status Match Goal Mismatch",
					Status: domain.TaskStatusInProgress,
					Goals:  []string{"Other Goal"},
				},
			}
			mockPageStorage.ListPagesReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with --goal filter case sensitivity", func() {
		BeforeEach(func() {
			goalFilter = "my goal"
			tasks = []*domain.Task{
				{
					Name:   "Task With Different Case",
					Status: domain.TaskStatusTodo,
					Goals:  []string{"My Goal"},
				},
			}
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
			tasks := []*domain.Task{
				{
					Name:            "Enriched Task",
					Status:          domain.TaskStatusInProgress,
					Assignee:        "alice",
					Priority:        1,
					PageType:        "feature",
					Recurring:       "weekly",
					DeferDate:       &deferDate,
					PlannedDate:     &plannedDate,
					DueDate:         &dueDate,
					ClaudeSessionID: "sess-abc123",
					Phase:           domain.TaskPhaseInProgress.Ptr(),
				},
			}
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
				{
					Name:   "Minimal Task",
					Status: domain.TaskStatusTodo,
				},
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
			tasks := []*domain.Task{
				{
					Name:   "Matching Task",
					Status: domain.TaskStatusTodo,
					Goals:  []string{"Target Goal", "Other Goal"},
				},
				{
					Name:   "Non-matching Task",
					Status: domain.TaskStatusTodo,
					Goals:  []string{"Different Goal"},
				},
			}
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
			tasks := []*domain.Task{
				{
					Name:        "Datetime Task",
					Status:      domain.TaskStatusTodo,
					DeferDate:   &deferDate,
					PlannedDate: &plannedDate,
					DueDate:     &dueDate,
				},
			}
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
				{
					Name:         "Modified Task",
					Status:       domain.TaskStatusTodo,
					ModifiedDate: &modTime,
				},
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
			tasks := []*domain.Task{
				{
					Name:          "Completed Task",
					Status:        domain.TaskStatusCompleted,
					CompletedDate: "2026-03-03T12:00:00Z",
				},
			}
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
				{
					Name:   "No Dates Task",
					Status: domain.TaskStatusTodo,
				},
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
				{Name: "IP Task", Status: domain.TaskStatusInProgress},
				{Name: "Done Task", Status: domain.TaskStatusCompleted},
				{Name: "Todo Task", Status: domain.TaskStatusTodo},
				{Name: "Hold Task", Status: domain.TaskStatusHold},
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
