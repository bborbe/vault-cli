// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"strings"
	"time"

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
			func() string { return pinnedSessionID },
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
		mockStarter.StartSessionReturns(nil)
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
			// Twice: once to load the task, once to re-read it after the child exits
			// so the session id is persisted only once the turn is done.
			Expect(mockTaskStorage.FindTaskByNameCallCount()).To(Equal(2))
			actualCtx, actualVaultPath, actualTaskName := mockTaskStorage.FindTaskByNameArgsForCall(
				0,
			)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualTaskName).To(Equal(taskName))
		})

		It("re-reads the task from the vault path after the session finishes", func() {
			Expect(mockTaskStorage.FindTaskByNameCallCount()).To(Equal(2))
			// The second FindTaskByName is persistSessionAndMetrics' post-spawn
			// re-read: the session id is written to disk only after the child exits.
			_, reReadVaultPath, reReadTaskName := mockTaskStorage.FindTaskByNameArgsForCall(1)
			Expect(reReadVaultPath).To(Equal(vaultPath))
			Expect(reReadTaskName).To(Equal(taskName))
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
			_, _, _, _, name, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(name).To(Equal(taskName))
		})

		It("passes isInteractive=false to the starter on the non-interactive branch", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, _, _, _, isInteractiveArg := mockStarter.StartSessionArgsForCall(0)
			Expect(isInteractiveArg).To(BeFalse())
		})

		It("Fresh run records one entry", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(2))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(1)
			Expect(writtenTask.MetricsSessions()).To(HaveLen(1))
			Expect(writtenTask.MetricsSessions()[0].SessionID).To(Equal(pinnedSessionID))
			Expect(writtenTask.MetricsSessions()[0].StartedAt).To(Equal(
				libtime.DateOrDateTime(libtimetest.ParseDateTime("2026-03-03T12:00:00Z").Time()),
			))
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
			_, _, prompt, _, _, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(prompt).To(MatchRegexp(`^/custom-cmd "`))
		})

		It("appends --non-interactive to the bootstrap prompt", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, prompt, _, _, _ := mockStarter.StartSessionArgsForCall(0)
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
				func() string { return pinnedSessionID },
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

		It("No-anchor run records nothing", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.MetricsSessions()).To(BeNil())
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
				func() string { return pinnedSessionID },
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

	Context("when task already has a session ID and prior metrics", func() {
		var preExistingStartedAt libtime.DateOrDateTime

		BeforeEach(func() {
			preExistingStartedAt = libtime.DateOrDateTime(
				libtimetest.ParseDateTime("2026-02-01T08:00:00Z").Time(),
			)
			task.SetClaudeSessionID("cached-session-456")
			task.AppendMetricsSession(domain.MetricsSession{
				SessionID: "first-session",
				StartedAt: preExistingStartedAt,
			})
		})

		It("Cached run appends and preserves", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(2))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(1)
			Expect(writtenTask.MetricsSessions()).To(HaveLen(2))
			// Entry 0 is the pre-existing entry, byte-identical and untouched.
			Expect(writtenTask.MetricsSessions()[0].SessionID).To(Equal("first-session"))
			Expect(writtenTask.MetricsSessions()[0].StartedAt).To(Equal(preExistingStartedAt))
			// Entry 1 is this run's cached-session record.
			Expect(writtenTask.MetricsSessions()[1].SessionID).To(Equal("cached-session-456"))
			// The cached session id is preserved, not overwritten.
			Expect(writtenTask.ClaudeSessionID()).To(Equal("cached-session-456"))
		})
	})

	Context("when session start fails (hard failure)", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns(ErrTest)
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
			_, sessionID, cwd, _ := mockResumer.ResumeSessionArgsForCall(0)
			Expect(sessionID).To(Equal(pinnedSessionID))
			Expect(cwd).To(Equal(vaultPath))
		})

		It("passes a continuation prompt that re-invokes the work-on command", func() {
			Expect(mockResumer.ResumeSessionCallCount()).To(Equal(1))
			_, _, _, continuation := mockResumer.ResumeSessionArgsForCall(0)
			Expect(
				continuation,
			).To(Equal(`/vault-cli:work-on-task "/path/to/vault/tasks/my-task.md"`))
		})

		It("does not pass --non-interactive in the continuation prompt", func() {
			_, _, _, continuation := mockResumer.ResumeSessionArgsForCall(0)
			Expect(strings.Contains(continuation, "--non-interactive")).To(BeFalse())
		})

		Context("with a custom work on command", func() {
			BeforeEach(func() {
				testVault.WorkOnCommand = "/custom-cmd"
			})

			It("uses the configured command in the continuation prompt", func() {
				_, _, _, continuation := mockResumer.ResumeSessionArgsForCall(0)
				Expect(continuation).To(Equal(`/custom-cmd "/path/to/vault/tasks/my-task.md"`))
			})
		})

		It("leaves the turn-1 bootstrap prompt non-interactive", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, bootstrap, _, _, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(bootstrap).To(Equal(
				`/vault-cli:work-on-task "/path/to/vault/tasks/my-task.md" --non-interactive`,
			))
		})

		It("passes isInteractive=true to the starter", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, _, _, _, isInteractiveArg := mockStarter.StartSessionArgsForCall(0)
			Expect(isInteractiveArg).To(BeTrue())
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

		Context("updateDailyNote when the note holds only a mention", func() {
			BeforeEach(func() {
				taskName = "Turn on hell - 2026W32-sat"
				task = domain.NewTask(
					map[string]any{"status": "todo"},
					domain.FileMetadata{
						Name:     taskName,
						FilePath: "/path/to/vault/tasks/Turn on hell - 2026W32-sat.md",
					},
					domain.Content(""),
				)
				mockTaskStorage.FindTaskByNameReturns(task, nil)

				dailyContent := "## Must\n" +
					"- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].\n"
				mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			})

			It("adds the task's own entry", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(content).To(ContainSubstring("- [/] [[Turn on hell - 2026W32-sat]]"))
			})

			It("leaves the mention line byte-identical", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(
					content,
				).To(ContainSubstring("- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]]."))
			})

			It("adds exactly one entry", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(strings.Count(content, "[[Turn on hell - 2026W32-sat]]")).To(Equal(2))
			})
		})

		Context("updateDailyNote with a mention line above a pending own entry", func() {
			BeforeEach(func() {
				taskName = "Turn on hell - 2026W32-sat"
				task = domain.NewTask(
					map[string]any{"status": "todo"},
					domain.FileMetadata{
						Name:     taskName,
						FilePath: "/path/to/vault/tasks/Turn on hell - 2026W32-sat.md",
					},
					domain.Content(""),
				)
				mockTaskStorage.FindTaskByNameReturns(task, nil)

				dailyContent := "## Must\n" +
					"- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].\n" +
					"- [ ] [[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today\n"
				mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			})

			It("promotes the own entry to in-progress", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(
					content,
				).To(ContainSubstring("- [/] [[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today"))
			})

			It("leaves the mention line byte-identical", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(
					content,
				).To(ContainSubstring("- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]]."))
			})

			It("does not append a second entry", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(strings.Count(content, "[[Turn on hell - 2026W32-sat]]")).To(Equal(2))
			})
		})

		Context(
			"updateDailyNote with a mention line above an already in-progress own entry",
			func() {
				BeforeEach(func() {
					taskName = "Turn on hell - 2026W32-sat"
					task = domain.NewTask(
						map[string]any{"status": "todo"},
						domain.FileMetadata{
							Name:     taskName,
							FilePath: "/path/to/vault/tasks/Turn on hell - 2026W32-sat.md",
						},
						domain.Content(""),
					)
					mockTaskStorage.FindTaskByNameReturns(task, nil)

					dailyContent := "## Must\n" +
						"- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].\n" +
						"- [/] [[Turn on hell - 2026W32-sat]]\n"
					mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
					mockDailyNoteStorage.WriteDailyNoteReturns(nil)
				})

				It("leaves the note untouched", func() {
					Expect(err).To(BeNil())
					Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(0))
				})
			},
		)

		Context("updateDailyNote with a decorated pending own entry", func() {
			BeforeEach(func() {
				taskName = "Feed Worms"
				task = domain.NewTask(
					map[string]any{"status": "todo"},
					domain.FileMetadata{
						Name:     taskName,
						FilePath: "/path/to/vault/tasks/Feed Worms.md",
					},
					domain.Content(""),
				)
				mockTaskStorage.FindTaskByNameReturns(task, nil)

				dailyContent := "## Must\n- [ ] 🐟 [[Feed Worms]]\n"
				mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
				mockDailyNoteStorage.WriteDailyNoteReturns(nil)
			})

			It("promotes the decorated pending entry in place", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(content).To(ContainSubstring("- [/] 🐟 [[Feed Worms]]"))
			})

			It("appends no duplicate entry", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(strings.Count(content, "[[Feed Worms]]")).To(Equal(1))
			})

			It("writes no undecorated duplicate", func() {
				Expect(mockDailyNoteStorage.WriteDailyNoteCallCount()).To(Equal(1))
				_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
				Expect(content).NotTo(ContainSubstring("- [/] [[Feed Worms]]"))
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

	Context("when the persist re-read after the session finishes fails", func() {
		BeforeEach(func() {
			mockTaskStorage.FindTaskByNameReturnsOnCall(0, task, nil)
			// Under the new ordering the failing call is persistSessionAndMetrics'
			// re-read, which runs only after StartSession reports a clean turn.
			mockTaskStorage.FindTaskByNameReturnsOnCall(1, nil, ErrTest)
		})

		It("returns a wrapped error and Success=false", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("re-read task after claude session"))
			Expect(result.Success).To(BeFalse())
		})

		It("does not report a session id because the persist never completed", func() {
			Expect(result.SessionID).To(Equal(""))
		})

		It("does not write a second time with the stale in-memory task", func() {
			// Execute's write only — the post-spawn persist failed before writing.
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		})

		It("does not append a metrics entry when the post-spawn re-read fails", func() {
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.MetricsSessions()).To(BeNil())
		})
	})

	Context("when persisting the session id after the child exits", func() {
		var (
			writeTaskAt, childExitAt time.Time
			writtenSessionID         string
			spawnedSessionID         string
			blockWaiter              chan struct{}
			lockDirLocker            ops.SessionLocker
		)

		BeforeEach(func() {
			blockWaiter = make(chan struct{})
			DeferCleanup(func() { close(blockWaiter) })

			lockDir, err := os.MkdirTemp("", "vault-workon-lock-*")
			Expect(err).To(BeNil())
			DeferCleanup(func() { _ = os.RemoveAll(lockDir) })
			lockDirLocker = ops.NewSessionLockerWithDir(lockDir)

			mockTaskStorage.WriteTaskStub = func(_ context.Context, t *domain.Task) error {
				if t.ClaudeSessionID() != "" {
					writtenSessionID = t.ClaudeSessionID()
					writeTaskAt = time.Now()
				}
				return nil
			}
			realStarter := ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(args []string, _ string, stdout *os.File) (<-chan error, error) {
					for i, a := range args {
						if a == "--session-id" && i+1 < len(args) {
							spawnedSessionID = args[i+1]
						}
					}
					_, err := stdout.WriteString(
						`{"session_id":"` + pinnedSessionID + `","num_turns":3,"is_error":false,"result":"done"}`,
					)
					Expect(err).To(BeNil())
					done := make(chan error, 1)
					childExitAt = time.Now()
					done <- nil
					return done, nil
				},
				// Blocking waiter: only the child-exit channel is ever ready, so the
				// select in StartSession cannot nondeterministically pick the
				// timeout branch instead of the success branch.
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
					<-blockWaiter
					return nil
				}),
				lockDirLocker,
			)
			currentDateTime := libtime.NewCurrentDateTime()
			currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
			// The mocked suite drives Execute with isInteractive=false, so the
			// non-interactive branch is the one under test.
			workOnOp = ops.NewWorkOnOperation(
				mockTaskStorage, mockDailyNoteStorage, currentDateTime,
				func() string { return pinnedSessionID },
				realStarter, mockResumer,
			)
		})

		It("writes the session id to storage only after the child exits", func() {
			Expect(err).To(BeNil())
			Expect(writeTaskAt).NotTo(BeZero())
			Expect(childExitAt).NotTo(BeZero())
			Expect(writeTaskAt.After(childExitAt)).To(BeTrue())
			// AC5's "id equals the value in task frontmatter" — capture the id written to
			// storage and the id handed to detachRun and assert they are the same value.
			// Both derive from the pinned generator today, so this holds implicitly; assert
			// it explicitly so a future refactor that mints a second id cannot pass silently.
			Expect(writtenSessionID).To(Equal(spawnedSessionID))
		})
	})

	Context("when the spawn fails", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns(ErrTest)
		})

		It("returns the wrapped spawn error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
		})

		It("returns Success=false", func() {
			Expect(result.Success).To(BeFalse())
		})

		It("never persists a session id: WriteTask reflects only Execute's initial write", func() {
			// No compensating clear exists in the new design — nothing was written
			// for this id, so there is nothing to undo.
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, onlyWritten := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(onlyWritten.ClaudeSessionID()).To(Equal(""))
		})
	})

	Context("when the session lock is already held", func() {
		var (
			lockDirLocker ops.SessionLocker
			spawned       int
		)

		BeforeEach(func() {
			lockDir, err := os.MkdirTemp("", "vault-workon-busy-lock-*")
			Expect(err).To(BeNil())
			DeferCleanup(func() { _ = os.RemoveAll(lockDir) })
			lockDirLocker = ops.NewSessionLockerWithDir(lockDir)
			spawned = 0

			// The waiter must block so the select in StartSession can never pick the
			// timeout branch; on the busy path it is never reached anyway.
			block := make(chan struct{})
			DeferCleanup(func() { close(block) })
			realStarter := ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(_ []string, _ string, _ *os.File) (<-chan error, error) {
					spawned++
					done := make(chan error, 1)
					done <- nil
					return done, nil
				},
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
					<-block
					return nil
				}),
				lockDirLocker,
			)
			currentDateTime := libtime.NewCurrentDateTime()
			currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
			workOnOp = ops.NewWorkOnOperation(
				mockTaskStorage, mockDailyNoteStorage, currentDateTime,
				func() string { return pinnedSessionID },
				realStarter, mockResumer,
			)

			// Hold the lock for pinnedSessionID so the work-on's own acquire refuses.
			held, aerr := lockDirLocker.Acquire(ctx, pinnedSessionID)
			Expect(aerr).To(BeNil())
			DeferCleanup(func() { _ = held.Release() })
		})

		It("fails hard with ErrSessionBusy and never spawns a second writer", func() {
			Expect(errors.Is(err, ops.ErrSessionBusy)).To(BeTrue())
			Expect(result.Success).To(BeFalse())
			Expect(result.Error).To(ContainSubstring(pinnedSessionID))
			Expect(spawned).To(Equal(0))
		})
	})
})
