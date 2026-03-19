// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

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
			Phase:           domain.TaskPhaseInProgress.Ptr(),
			Assignee:        "alice",
			Priority:        domain.Priority(2),
			PageType:        "task",
			Recurring:       "weekly",
			ClaudeSessionID: "session-abc",
			Goals:           []string{"goal-1", "goal-2"},
			DeferDate:       func() *domain.DateOrDateTime { d := domain.DateOrDateTime(deferDate); return &d }(),
			PlannedDate:     func() *domain.DateOrDateTime { d := domain.DateOrDateTime(plannedDate); return &d }(),
			DueDate:         func() *domain.DateOrDateTime { d := domain.DateOrDateTime(dueDate); return &d }(),
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
			task.Phase = nil
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

var _ = Describe("ShowOperation JSON completed_date", func() {
	var (
		ctx             context.Context
		showOp          ops.ShowOperation
		mockTaskStorage *mocks.TaskStorage
		capturedOutput  []byte
	)

	captureStdout := func(fn func()) []byte {
		r, w, pipeErr := os.Pipe()
		Expect(pipeErr).To(BeNil())
		orig := os.Stdout
		os.Stdout = w
		fn()
		w.Close()
		os.Stdout = orig
		data, readErr := io.ReadAll(r)
		Expect(readErr).To(BeNil())
		return data
	}

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		showOp = ops.NewShowOperation(mockTaskStorage)
	})

	Context("with completed_date set", func() {
		BeforeEach(func() {
			task := &domain.Task{
				Name:          "done-task",
				Status:        domain.TaskStatusCompleted,
				CompletedDate: "2026-03-03T12:00:00Z",
				Content:       "---\nstatus: completed\n---\nDone.\n",
				FilePath:      "/tmp/nonexistent-show-test.md",
			}
			mockTaskStorage.FindTaskByNameReturns(task, nil)

			capturedOutput = captureStdout(func() {
				execErr := showOp.Execute(ctx, "/vault", "my-vault", "done-task", "json")
				Expect(execErr).To(BeNil())
			})
		})

		It("includes completed_date in JSON output", func() {
			var detail ops.TaskDetail
			Expect(json.Unmarshal(capturedOutput, &detail)).To(Succeed())
			Expect(detail.CompletedDate).To(Equal("2026-03-03T12:00:00Z"))
		})
	})

	Context("without completed_date set", func() {
		BeforeEach(func() {
			task := &domain.Task{
				Name:     "todo-task",
				Status:   domain.TaskStatusTodo,
				Content:  "---\nstatus: todo\n---\nNot done yet.\n",
				FilePath: "/tmp/nonexistent-show-test2.md",
			}
			mockTaskStorage.FindTaskByNameReturns(task, nil)

			capturedOutput = captureStdout(func() {
				execErr := showOp.Execute(ctx, "/vault", "my-vault", "todo-task", "json")
				Expect(execErr).To(BeNil())
			})
		})

		It("omits completed_date from JSON output", func() {
			Expect(capturedOutput).NotTo(ContainSubstring(`"completed_date"`))
		})
	})
})
