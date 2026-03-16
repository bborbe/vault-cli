// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("WorkOnOperation", func() {
	var (
		ctx           context.Context
		err           error
		workOnOp      ops.WorkOnOperation
		mockStorage   *mocks.Storage
		mockStarter   *mocks.ClaudeSessionStarter
		mockResumer   *mocks.ClaudeResumer
		vaultPath     string
		taskName      string
		assignee      string
		task          *domain.Task
		isInteractive bool
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockStorage = &mocks.Storage{}
		mockStarter = &mocks.ClaudeSessionStarter{}
		mockResumer = &mocks.ClaudeResumer{}
		currentDateTime := libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
		workOnOp = ops.NewWorkOnOperation(
			mockStorage,
			mockStorage,
			currentDateTime,
			mockStarter,
			mockResumer,
		)
		vaultPath = "/path/to/vault"
		taskName = "my-task"
		assignee = "user@example.com"
		isInteractive = false

		task = &domain.Task{
			Name:     taskName,
			Status:   domain.TaskStatusTodo,
			FilePath: "/path/to/vault/tasks/my-task.md",
		}
		mockStorage.FindTaskByNameReturns(task, nil)
		mockStorage.WriteTaskReturns(nil)
		mockStarter.StartSessionReturns("session-123", nil)
		mockResumer.ResumeSessionReturns(nil)
	})

	JustBeforeEach(func() {
		err = workOnOp.Execute(
			ctx,
			vaultPath,
			taskName,
			assignee,
			"test-vault",
			"plain",
			isInteractive,
		)
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
			Expect(mockStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status).To(Equal(domain.TaskStatusInProgress))
		})

		It("sets assignee correctly", func() {
			Expect(mockStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
			_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Assignee).To(Equal(assignee))
		})

		It("starts a claude session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
		})
	})

	Context("when starter is nil", func() {
		BeforeEach(func() {
			currentDateTime := libtime.NewCurrentDateTime()
			currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
			workOnOp = ops.NewWorkOnOperation(mockStorage, mockStorage, currentDateTime, nil, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("skips session start", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})
	})

	Context("when task already has a session ID", func() {
		BeforeEach(func() {
			task.ClaudeSessionID = "existing-session"
		})

		It("does not start a new session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("when session start fails", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns("", ErrTest)
		})

		It("still returns no error (session failure is a warning)", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("interactive mode", func() {
		BeforeEach(func() {
			isInteractive = true
		})

		It("calls ResumeSession", func() {
			Expect(mockResumer.ResumeSessionCallCount()).To(Equal(1))
			sessionID, cwd := mockResumer.ResumeSessionArgsForCall(0)
			Expect(sessionID).To(Equal("session-123"))
			Expect(cwd).To(Equal(vaultPath))
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
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

	Context("daily note updates", func() {
		Context("when daily note exists with pending task", func() {
			BeforeEach(func() {
				dailyContent := "## Must\n- [ ] [[my-task]]\n- [ ] other task"
				mockStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockStorage.WriteDailyNoteReturns(nil)
			})

			It("updates checkbox to in-progress", func() {
				Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockStorage.WriteDailyNoteArgsForCall(0)
				Expect(content).To(ContainSubstring("- [/] [[my-task]]"))
				Expect(content).NotTo(ContainSubstring("- [ ] [[my-task]]"))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when daily note exists with in-progress task", func() {
			BeforeEach(func() {
				dailyContent := "## Must\n- [/] [[my-task]]\n"
				mockStorage.ReadDailyNoteReturns(dailyContent, nil)
			})

			It("does not modify the daily note", func() {
				Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(0))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when daily note exists with completed task", func() {
			BeforeEach(func() {
				dailyContent := "## Must\n- [x] [[my-task]]\n"
				mockStorage.ReadDailyNoteReturns(dailyContent, nil)
			})

			It("does not modify the daily note", func() {
				Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(0))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when daily note exists without task", func() {
			BeforeEach(func() {
				dailyContent := "## Must\n- [ ] other task\n"
				mockStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockStorage.WriteDailyNoteReturns(nil)
			})

			It("appends task to Must section", func() {
				Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockStorage.WriteDailyNoteArgsForCall(0)
				Expect(content).To(ContainSubstring("## Must\n- [/] [[my-task]]"))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when daily note exists without Must section", func() {
			BeforeEach(func() {
				dailyContent := "Some content\n"
				mockStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockStorage.WriteDailyNoteReturns(nil)
			})

			It("appends task to end of file", func() {
				Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockStorage.WriteDailyNoteArgsForCall(0)
				Expect(content).To(ContainSubstring("- [/] [[my-task]]"))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when daily note does not exist", func() {
			BeforeEach(func() {
				mockStorage.ReadDailyNoteReturns("", nil)
			})

			It("does not write daily note", func() {
				Expect(mockStorage.WriteDailyNoteCallCount()).To(Equal(0))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("still marks task as in_progress", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status).To(Equal(domain.TaskStatusInProgress))
			})
		})

		Context("when daily note read fails", func() {
			BeforeEach(func() {
				mockStorage.ReadDailyNoteReturns("", ErrTest)
			})

			It("still succeeds", func() {
				Expect(err).To(BeNil())
			})

			It("still marks task as in_progress", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status).To(Equal(domain.TaskStatusInProgress))
			})
		})

		Context("when daily note write fails", func() {
			BeforeEach(func() {
				dailyContent := "## Must\n- [ ] [[my-task]]\n"
				mockStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockStorage.WriteDailyNoteReturns(ErrTest)
			})

			It("still succeeds", func() {
				Expect(err).To(BeNil())
			})

			It("still marks task as in_progress", func() {
				Expect(mockStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
				_, writtenTask := mockStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status).To(Equal(domain.TaskStatusInProgress))
			})
		})
	})
})
