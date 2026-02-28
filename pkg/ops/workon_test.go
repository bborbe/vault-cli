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

var _ = Describe("WorkOnOperation", func() {
	var (
		ctx         context.Context
		err         error
		workOnOp    ops.WorkOnOperation
		mockStorage *mocks.Storage
		vaultPath   string
		taskName    string
		assignee    string
		task        *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockStorage = &mocks.Storage{}
		workOnOp = ops.NewWorkOnOperation(mockStorage)
		vaultPath = "/path/to/vault"
		taskName = "my-task"
		assignee = "user@example.com"

		// Default: return a task
		task = &domain.Task{
			Name:   taskName,
			Status: domain.TaskStatusTodo,
		}
		mockStorage.FindTaskByNameReturns(task, nil)
		mockStorage.WriteTaskReturns(nil)
	})

	JustBeforeEach(func() {
		err = workOnOp.Execute(ctx, vaultPath, taskName, assignee)
	})

	Context("success", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("calls FindTaskByName", func() {
			Expect(mockStorage.FindTaskByNameCallCount()).To(Equal(1))
			actualCtx, actualVaultPath, actualTaskName := mockStorage.FindTaskByNameArgsForCall(0)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualTaskName).To(Equal(taskName))
		})

		It("marks task as in_progress", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status).To(Equal(domain.TaskStatusInProgress))
		})

		It("sets assignee correctly", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Assignee).To(Equal(assignee))
		})

		It("calls WriteTask with updated task", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			actualCtx, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(actualCtx).To(Equal(ctx))
			Expect(writtenTask.Name).To(Equal(taskName))
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
