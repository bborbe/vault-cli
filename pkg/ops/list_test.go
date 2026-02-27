// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

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
	var mockStorage *mocks.Storage
	var vaultPath string
	var statusFilter []domain.TaskStatus
	var showAll bool
	var tasks []*domain.Task

	BeforeEach(func() {
		ctx = context.Background()
		mockStorage = &mocks.Storage{}
		listOp = ops.NewListOperation(mockStorage)
		vaultPath = "/path/to/vault"
		statusFilter = nil
		showAll = false

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
				Status: domain.TaskStatusDone,
			},
			{
				Name:   "Task D",
				Status: domain.TaskStatusDeferred,
			},
		}
		mockStorage.ListTasksReturns(tasks, nil)
	})

	JustBeforeEach(func() {
		err = listOp.Execute(ctx, vaultPath, statusFilter, showAll)
	})

	Context("success", func() {
		Context("with default filter (no flags)", func() {
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("calls ListTasks", func() {
				Expect(mockStorage.ListTasksCallCount()).To(Equal(1))
				actualCtx, actualVaultPath := mockStorage.ListTasksArgsForCall(0)
				Expect(actualCtx).To(Equal(ctx))
				Expect(actualVaultPath).To(Equal(vaultPath))
			})
		})

		Context("with --status filter", func() {
			BeforeEach(func() {
				statusFilter = []domain.TaskStatus{domain.TaskStatusInProgress}
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("calls ListTasks", func() {
				Expect(mockStorage.ListTasksCallCount()).To(Equal(1))
			})
		})

		Context("with --all flag", func() {
			BeforeEach(func() {
				showAll = true
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("calls ListTasks", func() {
				Expect(mockStorage.ListTasksCallCount()).To(Equal(1))
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
								Status: domain.TaskStatus("done"),
							},
							{
								Name:   "Task Deferred",
								Status: domain.TaskStatus("deferred"),
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
						mockStorage.ListTasksReturns(tasks, nil)
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
			mockStorage.ListTasksReturns(nil, ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})
	})
})
