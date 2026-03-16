// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"
	"time"

	libtime "github.com/bborbe/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("ShowOperation", func() {
	var (
		ctx             context.Context
		err             error
		showOp          ops.ShowOperation
		mockTaskStorage *mocks.TaskStorage
		vaultPath       string
		vaultName       string
		taskName        string
		outputFormat    string
		task            *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		showOp = ops.NewShowOperation(mockTaskStorage)
		vaultPath = "/path/to/vault"
		vaultName = "my-vault"
		taskName = "my-task"
		outputFormat = "plain"

		deferDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
		plannedDate := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
		dueDate := time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)
		task = &domain.Task{
			Name:            taskName,
			Status:          domain.TaskStatusInProgress,
			Phase:           "implementation",
			Assignee:        "alice",
			Priority:        domain.Priority(2),
			PageType:        "task",
			Recurring:       "weekly",
			ClaudeSessionID: "session-abc",
			Goals:           []string{"goal-1", "goal-2"},
			DeferDate:       libtime.ToDate(deferDate).Ptr(),
			PlannedDate:     libtime.ToDate(plannedDate).Ptr(),
			DueDate:         libtime.ToDate(dueDate).Ptr(),
			Content:         "---\nstatus: in_progress\n---\nDo the thing with care.\n",
			FilePath:        "/tmp/nonexistent-test-file.md",
		}
		mockTaskStorage.FindTaskByNameReturns(task, nil)
	})

	JustBeforeEach(func() {
		err = showOp.Execute(ctx, vaultPath, vaultName, taskName, outputFormat)
	})

	Context("plain output", func() {
		BeforeEach(func() {
			outputFormat = "plain"
		})

		It("succeeds without error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("json output with all fields", func() {
		BeforeEach(func() {
			outputFormat = "json"
		})

		It("succeeds without error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("json output with optional fields omitted", func() {
		BeforeEach(func() {
			outputFormat = "json"
			task.Phase = ""
			task.Assignee = ""
			task.Priority = 0
			task.PageType = ""
			task.Recurring = ""
			task.ClaudeSessionID = ""
			task.Goals = nil
			task.DeferDate = nil
			task.PlannedDate = nil
			task.DueDate = nil
		})

		It("succeeds without error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("task not found", func() {
		BeforeEach(func() {
			mockTaskStorage.FindTaskByNameReturns(nil, errors.New("task not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find task")))
		})
	})
})
