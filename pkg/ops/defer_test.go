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

			It("sets status to deferred", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status).To(Equal(domain.TaskStatusDeferred))
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

		Context("with ISO date 2025-12-31", func() {
			BeforeEach(func() {
				dateStr = "2025-12-31"
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets defer_date to specified date", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.DeferDate).NotTo(BeNil())
				expected := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
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
})
