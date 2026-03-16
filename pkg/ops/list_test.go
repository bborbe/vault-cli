// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	libtime "github.com/bborbe/time"
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
	var statusFilter string
	var showAll bool
	var assigneeFilter string
	var tasks []*domain.Task

	BeforeEach(func() {
		ctx = context.Background()
		mockPageStorage = &mocks.PageStorage{}
		listOp = ops.NewListOperation(mockPageStorage)
		vaultPath = "/path/to/vault"
		pagesDir = "Tasks"
		statusFilter = ""
		showAll = false
		assigneeFilter = ""

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
		err = listOp.Execute(
			ctx,
			vaultPath,
			"test-vault",
			pagesDir,
			statusFilter,
			showAll,
			assigneeFilter,
			"plain",
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
				statusFilter = "in_progress"
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
				statusFilter = "In_Progress"
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
})

var _ = Describe("ListOperation JSON output", func() {
	var ctx context.Context
	var listOp ops.ListOperation
	var mockPageStorage *mocks.PageStorage
	var capturedOutput []byte

	captureStdout := func(fn func()) []byte {
		r, w, err := os.Pipe()
		Expect(err).To(BeNil())
		orig := os.Stdout
		os.Stdout = w
		fn()
		w.Close()
		os.Stdout = orig
		data, err := io.ReadAll(r)
		Expect(err).To(BeNil())
		return data
	}

	BeforeEach(func() {
		ctx = context.Background()
		mockPageStorage = &mocks.PageStorage{}
		listOp = ops.NewListOperation(mockPageStorage)
	})

	Context("with all enriched fields populated", func() {
		BeforeEach(func() {
			deferDate := libtime.ToDate(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC))
			plannedDate := libtime.ToDate(time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
			dueDate := libtime.ToDate(time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC))
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
					Phase:           "implementation",
				},
			}
			mockPageStorage.ListPagesReturns(tasks, nil)

			capturedOutput = captureStdout(func() {
				err := listOp.Execute(ctx, "/vault", "my-vault", "Tasks", "", true, "", "json")
				Expect(err).To(BeNil())
			})
		})

		It("includes category from page_type", func() {
			var items []ops.TaskListItem
			Expect(json.Unmarshal(capturedOutput, &items)).To(Succeed())
			Expect(items).To(HaveLen(1))
			Expect(items[0].Category).To(Equal("feature"))
		})

		It("includes recurring field", func() {
			var items []ops.TaskListItem
			Expect(json.Unmarshal(capturedOutput, &items)).To(Succeed())
			Expect(items[0].Recurring).To(Equal("weekly"))
		})

		It("includes defer_date formatted as YYYY-MM-DD", func() {
			var items []ops.TaskListItem
			Expect(json.Unmarshal(capturedOutput, &items)).To(Succeed())
			Expect(items[0].DeferDate).To(Equal("2026-03-15"))
		})

		It("includes planned_date formatted as YYYY-MM-DD", func() {
			var items []ops.TaskListItem
			Expect(json.Unmarshal(capturedOutput, &items)).To(Succeed())
			Expect(items[0].PlannedDate).To(Equal("2026-03-20"))
		})

		It("includes due_date formatted as YYYY-MM-DD", func() {
			var items []ops.TaskListItem
			Expect(json.Unmarshal(capturedOutput, &items)).To(Succeed())
			Expect(items[0].DueDate).To(Equal("2026-03-25"))
		})

		It("includes claude_session_id", func() {
			var items []ops.TaskListItem
			Expect(json.Unmarshal(capturedOutput, &items)).To(Succeed())
			Expect(items[0].ClaudeSessionID).To(Equal("sess-abc123"))
		})

		It("includes phase", func() {
			var items []ops.TaskListItem
			Expect(json.Unmarshal(capturedOutput, &items)).To(Succeed())
			Expect(items[0].Phase).To(Equal("implementation"))
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

			capturedOutput = captureStdout(func() {
				err := listOp.Execute(ctx, "/vault", "my-vault", "Tasks", "", true, "", "json")
				Expect(err).To(BeNil())
			})
		})

		It("omits empty optional fields from JSON", func() {
			Expect(capturedOutput).NotTo(ContainSubstring(`"category"`))
			Expect(capturedOutput).NotTo(ContainSubstring(`"recurring"`))
			Expect(capturedOutput).NotTo(ContainSubstring(`"defer_date"`))
			Expect(capturedOutput).NotTo(ContainSubstring(`"planned_date"`))
			Expect(capturedOutput).NotTo(ContainSubstring(`"due_date"`))
			Expect(capturedOutput).NotTo(ContainSubstring(`"claude_session_id"`))
			Expect(capturedOutput).NotTo(ContainSubstring(`"phase"`))
		})

		It("still includes required fields", func() {
			var items []ops.TaskListItem
			Expect(json.Unmarshal(capturedOutput, &items)).To(Succeed())
			Expect(items).To(HaveLen(1))
			Expect(items[0].Name).To(Equal("Minimal Task"))
			Expect(items[0].Status).To(Equal("todo"))
			Expect(items[0].Vault).To(Equal("my-vault"))
		})
	})
})
