// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("WorkOnOperation", func() {
	var (
		ctx                  context.Context
		err                  error
		result               ops.MutationResult
		workOnOp             ops.WorkOnOperation
		mockTaskStorage      *mocks.TaskStorage
		mockDailyNoteStorage *mocks.DailyNoteStorage
		mockStarter          *mocks.ClaudeSessionStarter
		mockResumer          *mocks.ClaudeResumer
		vaultPath            string
		taskName             string
		assignee             string
		task                 *domain.Task
		isInteractive        bool
		testVault            config.Vault
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		mockDailyNoteStorage = &mocks.DailyNoteStorage{}
		mockStarter = &mocks.ClaudeSessionStarter{}
		mockResumer = &mocks.ClaudeResumer{}
		currentDateTime := libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
		workOnOp = ops.NewWorkOnOperation(
			mockTaskStorage,
			mockDailyNoteStorage,
			currentDateTime,
			mockStarter,
			mockResumer,
		)
		vaultPath = "/path/to/vault"
		taskName = "my-task"
		assignee = "user@example.com"
		isInteractive = false
		testVault = config.Vault{
			Path:          vaultPath,
			Name:          "test-vault",
			WorkOnCommand: "/vault-cli:work-on-task",
		}

		task = domain.NewTask(
			map[string]any{"status": "todo"},
			domain.FileMetadata{Name: taskName, FilePath: "/path/to/vault/tasks/my-task.md"},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockTaskStorage.WriteTaskReturns(nil)
		mockStarter.StartSessionReturns("session-123", nil)
		mockResumer.ResumeSessionReturns(nil)
	})

	JustBeforeEach(func() {
		result, err = workOnOp.Execute(
			ctx,
			vaultPath,
			taskName,
			assignee,
			"test-vault",
			isInteractive,
			vaultPath,
			&testVault,
		)
	})

	Context("success", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("calls FindTaskByName", func() {
			Expect(mockTaskStorage.FindTaskByNameCallCount()).To(Equal(1))
			actualCtx, actualVaultPath, actualTaskName := mockTaskStorage.FindTaskByNameArgsForCall(
				0,
			)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualTaskName).To(Equal(taskName))
		})

		It("marks task as in_progress", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status()).To(Equal(domain.TaskStatusInProgress))
		})

		It("sets assignee correctly", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Assignee()).To(Equal(assignee))
		})

		It("starts a claude session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
		})

		It("passes task name to session starter", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, _, name := mockStarter.StartSessionArgsForCall(0)
			Expect(name).To(Equal(taskName))
		})
	})

	Context("when assignee already equals current user", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "todo", "assignee": assignee},
				domain.FileMetadata{Name: taskName, FilePath: "/path/to/vault/tasks/my-task.md"},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("preserves the existing assignee", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Assignee()).To(Equal(assignee))
		})

		It("emits no assignee warning", func() {
			Expect(result.Warnings).NotTo(ContainElement(ContainSubstring("assignee not updated")))
		})
	})

	Context("when assignee is set to a different user", func() {
		const otherUser = "alice@example.com"

		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "todo", "assignee": otherUser},
				domain.FileMetadata{Name: taskName, FilePath: "/path/to/vault/tasks/my-task.md"},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("preserves the other user's assignment", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Assignee()).To(Equal(otherUser))
		})

		It("emits an assignee-not-updated warning naming both users", func() {
			Expect(result.Warnings).To(ContainElement(ContainSubstring("assignee not updated")))
			Expect(result.Warnings).To(ContainElement(ContainSubstring(otherUser)))
			Expect(result.Warnings).To(ContainElement(ContainSubstring(assignee)))
		})

		It("still marks the task in_progress (status is independent of assignee)", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status()).To(Equal(domain.TaskStatusInProgress))
		})
	})

	Context("custom work on command", func() {
		BeforeEach(func() {
			testVault.WorkOnCommand = "/custom-cmd"
		})

		It("uses the configured work on command in the prompt", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, prompt, _, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(prompt).To(MatchRegexp(`^/custom-cmd "`))
		})

		It("appends --non-interactive to the bootstrap prompt", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, prompt, _, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(prompt).To(MatchRegexp(` --non-interactive$`))
			Expect(prompt).To(MatchRegexp(`/path/to/vault/tasks/my-task\.md`))
		})
	})

	Context("when starter is nil and task has no cached session ID", func() {
		BeforeEach(func() {
			currentDateTime := libtime.NewCurrentDateTime()
			currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
			workOnOp = ops.NewWorkOnOperation(
				mockTaskStorage,
				mockDailyNoteStorage,
				currentDateTime,
				nil,
				nil,
			)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("skips session start", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})

		It("emits warning about missing starter", func() {
			Expect(
				result.Warnings,
			).To(ContainElement(ContainSubstring("claude session: claude session starter unavailable")))
		})

		It("returns empty session ID", func() {
			Expect(result.SessionID).To(Equal(""))
		})
	})

	Context("when starter is nil but task has cached session ID", func() {
		BeforeEach(func() {
			task.SetClaudeSessionID("cached-session-456")
			currentDateTime := libtime.NewCurrentDateTime()
			currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
			workOnOp = ops.NewWorkOnOperation(
				mockTaskStorage,
				mockDailyNoteStorage,
				currentDateTime,
				nil,
				nil,
			)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("skips session start", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})

		It("returns cached session ID", func() {
			Expect(result.SessionID).To(Equal("cached-session-456"))
		})

		It("emits no warnings", func() {
			Expect(result.Warnings).To(BeEmpty())
		})
	})

	Context("when task already has a session ID", func() {
		BeforeEach(func() {
			task.SetClaudeSessionID("existing-session")
		})

		It("does not start a new session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("when session start fails (hard failure)", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns("", ErrTest)
		})

		It("returns wrapped error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
		})

		It("returns Success=false", func() {
			Expect(result.Success).To(BeFalse())
		})
	})

	Context("when claude returns zero turns", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns(
				"",
				errors.New(ctx, "claude returned 0 turns: Unknown command: /x"),
			)
		})

		It("returns non-nil error wrapped with start work-on session and Success=false", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
			Expect(err.Error()).To(ContainSubstring("claude returned 0 turns: Unknown command: /x"))
			Expect(result.Success).To(BeFalse())
		})

		It("still marks task as in_progress", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status()).To(Equal(domain.TaskStatusInProgress))
		})
	})

	Context("interactive mode", func() {
		BeforeEach(func() {
			isInteractive = true
		})

		It("calls ResumeSession", func() {
			Expect(mockResumer.ResumeSessionCallCount()).To(Equal(1))
			_, sessionID, cwd := mockResumer.ResumeSessionArgsForCall(0)
			Expect(sessionID).To(Equal("session-123"))
			Expect(cwd).To(Equal(vaultPath))
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("task not found", func() {
		BeforeEach(func() {
			mockTaskStorage.FindTaskByNameReturns(nil, ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})

		It("does not call WriteTask", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("write error", func() {
		BeforeEach(func() {
			mockTaskStorage.WriteTaskReturns(ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})
	})

	Context("daily note updates", func() {
		Context("when daily note exists with pending task", func() {
			BeforeEach(func() {
				dailyContent := "## Must\n- [ ] [[my-task]]\n- [ ] other task"
				mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			})

			It("updates checkbox to in-progress", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(content).To(ContainSubstring("- [/] [[my-task]]"))
				Expect(content).NotTo(ContainSubstring("- [ ] [[my-task]]"))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when daily note exists with asterisk-prefixed pending task", func() {
			BeforeEach(func() {
				dailyContent := "## Must\n* [ ] [[my-task]]\n* [ ] other task"
				mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			})

			It("updates checkbox to in-progress and preserves asterisk marker", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(content).To(ContainSubstring("* [/] [[my-task]]"))
				Expect(content).NotTo(ContainSubstring("* [ ] [[my-task]]"))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when daily note exists with in-progress task", func() {
			BeforeEach(func() {
				dailyContent := "## Must\n- [/] [[my-task]]\n"
				mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
			})

			It("does not modify the daily note", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(0))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when daily note exists with completed task", func() {
			BeforeEach(func() {
				dailyContent := "## Must\n- [x] [[my-task]]\n"
				mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
			})

			It("does not modify the daily note", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(0))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when daily note exists without task", func() {
			BeforeEach(func() {
				dailyContent := "## Must\n- [ ] other task\n"
				mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			})

			It("appends task to Must section", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(content).To(ContainSubstring("## Must\n- [/] [[my-task]]"))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when daily note exists without Must section", func() {
			BeforeEach(func() {
				dailyContent := "Some content\n"
				mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			})

			It("appends task to end of file", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(content).To(ContainSubstring("- [/] [[my-task]]"))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when daily note does not exist", func() {
			BeforeEach(func() {
				mockDailyNoteStorage.ReadDailyNoteReturns("", nil)
			})

			It("does not write daily note", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(0))
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("still marks task as in_progress", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status()).To(Equal(domain.TaskStatusInProgress))
			})
		})

		Context("when daily note read fails", func() {
			BeforeEach(func() {
				mockDailyNoteStorage.ReadDailyNoteReturns("", ErrTest)
			})

			It("still succeeds", func() {
				Expect(err).To(BeNil())
			})

			It("still marks task as in_progress", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status()).To(Equal(domain.TaskStatusInProgress))
			})
		})

		Context("when daily note write fails", func() {
			BeforeEach(func() {
				dailyContent := "## Must\n- [ ] [[my-task]]\n"
				mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockDailyNoteStorage.WriteDailyNoteReturns(ErrTest)
			})

			It("still succeeds", func() {
				Expect(err).To(BeNil())
			})

			It("still marks task as in_progress", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status()).To(Equal(domain.TaskStatusInProgress))
			})
		})
	})

	Context("phase advancement", func() {
		Context("when phase is missing (nil)", func() {
			It("sets phase to planning", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Phase()).NotTo(BeNil())
				Expect(*writtenTask.Phase()).To(Equal(domain.TaskPhasePlanning))
			})
		})

		Context("when phase is empty string", func() {
			BeforeEach(func() {
				task = domain.NewTask(
					map[string]any{"status": "todo", "phase": ""},
					domain.FileMetadata{
						Name:     taskName,
						FilePath: "/path/to/vault/tasks/my-task.md",
					},
					domain.Content(""),
				)
				mockTaskStorage.FindTaskByNameReturns(task, nil)
			})

			It("sets phase to planning", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Phase()).NotTo(BeNil())
				Expect(*writtenTask.Phase()).To(Equal(domain.TaskPhasePlanning))
			})
		})

		Context("when phase is todo", func() {
			BeforeEach(func() {
				task = domain.NewTask(
					map[string]any{"status": "todo", "phase": "todo"},
					domain.FileMetadata{
						Name:     taskName,
						FilePath: "/path/to/vault/tasks/my-task.md",
					},
					domain.Content(""),
				)
				mockTaskStorage.FindTaskByNameReturns(task, nil)
			})

			It("sets phase to planning", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Phase()).NotTo(BeNil())
				Expect(*writtenTask.Phase()).To(Equal(domain.TaskPhasePlanning))
			})
		})

		Context("when phase is in_progress (resume case)", func() {
			BeforeEach(func() {
				task = domain.NewTask(
					map[string]any{"status": "in_progress", "phase": "in_progress"},
					domain.FileMetadata{
						Name:     taskName,
						FilePath: "/path/to/vault/tasks/my-task.md",
					},
					domain.Content(""),
				)
				mockTaskStorage.FindTaskByNameReturns(task, nil)
			})

			It("leaves phase unchanged", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Phase()).NotTo(BeNil())
				Expect(*writtenTask.Phase()).To(Equal(domain.TaskPhaseInProgress))
			})
		})
	})
})
