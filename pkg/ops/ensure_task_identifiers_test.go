// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("EnsureAllTaskIdentifiersOperation", func() {
	var (
		ctx         context.Context
		op          ops.EnsureAllTaskIdentifiersOperation
		mockStorage *mocks.TaskStorage
		vaultPath   string
		result      ops.BackfillResult
		err         error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockStorage = &mocks.TaskStorage{}
		op = ops.NewEnsureAllTaskIdentifiersOperation(mockStorage)
		vaultPath = "/vault"
	})

	JustBeforeEach(func() {
		result, err = op.Execute(ctx, vaultPath)
	})

	Context("when ListTasks fails", func() {
		BeforeEach(func() {
			mockStorage.ListTasksReturns(nil, errors.New("disk error"))
		})

		It("returns an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("list tasks"))
		})

		It("returns an empty result", func() {
			Expect(result.ModifiedFiles).To(BeEmpty())
		})
	})

	Context("when all tasks already have task_identifier", func() {
		BeforeEach(func() {
			mockStorage.ListTasksReturns([]*domain.Task{
				domain.NewTask(
					map[string]any{"task_identifier": "uuid-a"},
					domain.FileMetadata{Name: "Task A", FilePath: "/vault/Tasks/Task A.md"},
					domain.Content(""),
				),
				domain.NewTask(
					map[string]any{"task_identifier": "uuid-b"},
					domain.FileMetadata{Name: "Task B", FilePath: "/vault/Tasks/Task B.md"},
					domain.Content(""),
				),
			}, nil)
		})

		It("does not call WriteTask", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		})

		It("returns empty ModifiedFiles", func() {
			Expect(result.ModifiedFiles).To(BeEmpty())
		})

		It("returns no error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when some tasks are missing task_identifier", func() {
		BeforeEach(func() {
			mockStorage.ListTasksReturns([]*domain.Task{
				domain.NewTask(
					map[string]any{"task_identifier": "uuid-existing"},
					domain.FileMetadata{Name: "Task A", FilePath: "/vault/Tasks/Task A.md"},
					domain.Content(""),
				),
				domain.NewTask(
					map[string]any{},
					domain.FileMetadata{Name: "Task B", FilePath: "/vault/Tasks/Task B.md"},
					domain.Content(""),
				),
				domain.NewTask(
					map[string]any{},
					domain.FileMetadata{Name: "Task C", FilePath: "/vault/Tasks/Task C.md"},
					domain.Content(""),
				),
			}, nil)
			mockStorage.WriteTaskReturns(nil)
		})

		It("calls WriteTask only for tasks without identifier", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(2))
		})

		It("returns modified file paths", func() {
			Expect(result.ModifiedFiles).To(ConsistOf(
				"/vault/Tasks/Task B.md",
				"/vault/Tasks/Task C.md",
			))
		})

		It("returns no error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("has zero skipped files", func() {
			Expect(result.SkippedFiles).To(Equal(0))
		})
	})

	Context("when WriteTask fails for one task", func() {
		BeforeEach(func() {
			mockStorage.ListTasksReturns([]*domain.Task{
				domain.NewTask(
					map[string]any{},
					domain.FileMetadata{Name: "Task A", FilePath: "/vault/Tasks/Task A.md"},
					domain.Content(""),
				),
				domain.NewTask(
					map[string]any{},
					domain.FileMetadata{Name: "Task B", FilePath: "/vault/Tasks/Task B.md"},
					domain.Content(""),
				),
			}, nil)
			// First call fails, second succeeds
			mockStorage.WriteTaskReturnsOnCall(0, errors.New("permission denied"))
			mockStorage.WriteTaskReturnsOnCall(1, nil)
		})

		It("skips the failing task and continues", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("records the successful write in ModifiedFiles", func() {
			Expect(result.ModifiedFiles).To(ConsistOf("/vault/Tasks/Task B.md"))
		})

		It("increments SkippedFiles for the failed write", func() {
			Expect(result.SkippedFiles).To(Equal(1))
		})
	})

	Context("when vault has no tasks", func() {
		BeforeEach(func() {
			mockStorage.ListTasksReturns([]*domain.Task{}, nil)
		})

		It("returns no error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns empty result", func() {
			Expect(result.ModifiedFiles).To(BeEmpty())
			Expect(result.SkippedFiles).To(Equal(0))
		})
	})
})
