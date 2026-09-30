// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"
	"time"

	notifcore "github.com/bborbe/notification"
	notifcmd "github.com/bborbe/notification/command/notification"
	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("FrontmatterGetOperation", func() {
	var (
		ctx             context.Context
		err             error
		result          string
		getOp           ops.FrontmatterGetOperation
		mockTaskStorage *mocks.TaskStorage
		vaultPath       string
		taskName        string
		key             string
		task            *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		getOp = ops.NewFrontmatterGetOperation(mockTaskStorage)
		vaultPath = "/path/to/vault"
		taskName = "my-task"

		// Default: return a task with some fields set
		task = domain.NewTask(
			map[string]any{
				"status":            "in_progress",
				"phase":             "in_progress",
				"claude_session_id": "session-123",
				"assignee":          "alice",
				"priority":          3,
				"defer_date":        "2024-12-31",
				"planned_date":      "2025-03-15",
				"due_date":          "2025-06-30",
				"recurring":         "weekly",
				"last_completed":    "2025-03-10",
				"page_type":         "task",
				"goals":             []any{"goal-1", "goal-2"},
				"tags":              []any{"urgent", "backend"},
			},
			domain.FileMetadata{Name: taskName},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
	})

	JustBeforeEach(func() {
		result, err = getOp.Execute(ctx, vaultPath, taskName, key)
	})

	Context("getting phase field", func() {
		BeforeEach(func() {
			key = "phase"
		})

		It("returns the phase value", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("in_progress"))
		})
	})

	Context("getting claude_session_id field", func() {
		BeforeEach(func() {
			key = "claude_session_id"
		})

		It("returns the claude_session_id value", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("session-123"))
		})
	})

	Context("getting assignee field", func() {
		BeforeEach(func() {
			key = "assignee"
		})

		It("returns the assignee value", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("alice"))
		})
	})

	Context("getting status field", func() {
		BeforeEach(func() {
			key = "status"
		})

		It("returns the status value", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("in_progress"))
		})
	})

	Context("getting priority field", func() {
		BeforeEach(func() {
			key = "priority"
		})

		It("returns the priority value", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("3"))
		})
	})

	Context("getting defer_date field", func() {
		BeforeEach(func() {
			key = "defer_date"
		})

		It("returns the defer_date value in YYYY-MM-DD format", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("2024-12-31"))
		})
	})

	Context("getting empty field", func() {
		BeforeEach(func() {
			key = "phase"
			task.SetPhase(nil)
		})

		It("returns empty string with no error", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("getting defer_date when nil", func() {
		BeforeEach(func() {
			key = "defer_date"
			task.SetDeferDate(nil)
		})

		It("returns empty string with no error", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("getting planned_date field", func() {
		BeforeEach(func() {
			key = "planned_date"
		})

		It("returns the planned_date value in YYYY-MM-DD format", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("2025-03-15"))
		})
	})

	Context("getting planned_date when nil", func() {
		BeforeEach(func() {
			key = "planned_date"
			task.SetPlannedDate(nil)
		})

		It("returns empty string with no error", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("getting due_date field", func() {
		BeforeEach(func() {
			key = "due_date"
		})

		It("returns the due_date value in YYYY-MM-DD format", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("2025-06-30"))
		})
	})

	Context("getting due_date when nil", func() {
		BeforeEach(func() {
			key = "due_date"
			task.SetDueDate(nil)
		})

		It("returns empty string with no error", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("getting recurring field", func() {
		BeforeEach(func() {
			key = "recurring"
		})

		It("returns the recurring value", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("weekly"))
		})
	})

	Context("getting last_completed field", func() {
		BeforeEach(func() {
			key = "last_completed"
		})

		It("returns the last_completed value", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("2025-03-10"))
		})
	})

	Context("getting page_type field", func() {
		BeforeEach(func() {
			key = "page_type"
		})

		It("returns the page_type value", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("task"))
		})
	})

	Context("getting goals field", func() {
		BeforeEach(func() {
			key = "goals"
		})

		It("returns the goals as comma-separated string", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("goal-1,goal-2"))
		})
	})

	Context("getting goals when empty", func() {
		BeforeEach(func() {
			key = "goals"
			task.SetGoals(nil)
		})

		It("returns empty string with no error", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("getting tags field", func() {
		BeforeEach(func() {
			key = "tags"
		})

		It("returns the tags as comma-separated string", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("urgent,backend"))
		})
	})

	Context("getting tags when empty", func() {
		BeforeEach(func() {
			key = "tags"
			task.SetTags(nil)
		})

		It("returns empty string with no error", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("unknown key", func() {
		BeforeEach(func() {
			key = "unknown_key"
		})

		It("returns no error and empty result", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("getting flag field when absent", func() {
		BeforeEach(func() {
			key = "flag"
		})

		It("returns empty string with no error", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("getting flag field when set", func() {
		BeforeEach(func() {
			key = "flag"
			_ = task.SetFlag(context.Background(), true)
		})

		It("returns true", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("true"))
		})
	})

	Context("task not found", func() {
		BeforeEach(func() {
			key = "phase"
			mockTaskStorage.FindTaskByNameReturns(nil, errors.New("task not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find task")))
		})
	})
})

var _ = Describe("FrontmatterSetOperation", func() {
	var (
		ctx             context.Context
		err             error
		setOp           ops.FrontmatterSetOperation
		mockTaskStorage *mocks.TaskStorage
		vaultPath       string
		taskName        string
		key             string
		value           string
		reason          string
		gateSuccessor   string
		actor           string
		force           bool
		task            *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		mockFactory := &mocks.NotificationSenderFactory{}
		publisher := ops.NewEscalationPublisher("", "", mockFactory)
		setOp = ops.NewFrontmatterSetOperation(
			mockTaskStorage,
			libtime.NewCurrentDateTime(),
			publisher,
			"personal",
			"25 Tasks",
		)
		vaultPath = "/path/to/vault"
		taskName = "my-task"

		// Default: return a task
		task = domain.NewTask(
			map[string]any{"aborted_reason": "test reason", "gate_successor": "none"},
			domain.FileMetadata{Name: taskName},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockTaskStorage.WriteTaskReturns(nil)
		reason = ""
		gateSuccessor = ""
		// A named actor by default: the CLI always supplies one (its --by
		// default is "unknown"), and a truthy flag write with a blank actor is
		// refused. The blank-actor refusal has its own specs below.
		actor = "operator"
		force = false
	})

	JustBeforeEach(func() {
		err = setOp.Execute(ctx, vaultPath, taskName, key, value, reason, gateSuccessor, actor, force)
	})

	Context("setting phase field", func() {
		BeforeEach(func() {
			key = "phase"
			value = "planning"
		})

		It("updates the phase field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Phase()).NotTo(BeNil())
			Expect(*writtenTask.Phase()).To(Equal(domain.TaskPhasePlanning))
		})
	})

	Context("setting invalid phase field", func() {
		BeforeEach(func() {
			key = "phase"
			value = "invalid_phase_value"
		})

		It("returns an error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("unknown task phase"))
		})

		It("does not write the task", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("rejecting phase regression execution -> todo on in_progress task", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "in_progress", "phase": "execution"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "todo"
		})

		It("returns a regression error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("refusing to set phase"))
			Expect(err.Error()).To(ContainSubstring("--force"))
		})

		It("does not write the task", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("rejecting phase regression human_review -> todo on in_progress task", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "in_progress", "phase": "human_review"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "todo"
		})

		It("returns a regression error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("refusing to set phase"))
		})
	})

	// Observed 2026-09-19: vault-ui's PATCH /api/tasks/{id}/phase shells out to
	// `vault-cli task set <id> phase <value>`, so a board drag back to the
	// planning column silently regressed six finished Personal-vault tasks in one
	// autocommit (bd76e6b4b1). The original guard only rejected a `todo` target,
	// so `planning` passed unguarded.
	Context("rejecting phase regression execution -> planning on in_progress task", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "in_progress", "phase": "execution"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "planning"
		})

		It("returns a regression error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("refusing to set phase"))
			Expect(err.Error()).To(ContainSubstring("--force"))
		})

		It("does not write the task", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("rejecting phase regression ai_review -> planning on in_progress task", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "in_progress", "phase": "ai_review"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "planning"
		})

		It("returns a regression error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("refusing to set phase"))
		})
	})

	Context("rejecting phase regression human_review -> planning on in_progress task", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "in_progress", "phase": "human_review"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "planning"
		})

		It("returns a regression error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("refusing to set phase"))
		})
	})

	Context("rejecting phase planning on a completed task", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "completed", "phase": "done"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "planning"
		})

		It("returns a regression error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("refusing to set phase"))
		})

		It("does not write the task", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	// The must-still-pass case. Broadening the guard must not break a forward
	// move out of planning. The `todo` -> `planning` entry contract is no longer a
	// legal `task set`: it is refused below and performed by `vault-cli task approve`.
	DescribeTableSubtree("refusing todo -> planning on a task in the approval inbox",
		func(status string) {
			BeforeEach(func() {
				task = domain.NewTask(
					map[string]any{"status": status, "phase": "todo"},
					domain.FileMetadata{Name: taskName},
					domain.Content(""),
				)
				mockTaskStorage.FindTaskByNameReturns(task, nil)
				key = "phase"
				value = "planning"
			})

			It("refuses with an error naming the approve command", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("vault-cli task approve"))
				Expect(err.Error()).To(ContainSubstring("todo"))
			})

			It("does not write the task", func() {
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
			})

			It("leaves the phase at todo", func() {
				Expect(task.Phase()).NotTo(BeNil())
				Expect(*task.Phase()).To(Equal(domain.TaskPhaseTodo))
			})
		},
		Entry("next", "next"),
		Entry("in_progress", "in_progress"),
	)

	Context("refusing todo -> planning even when force is set", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "next", "phase": "todo"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "planning"
			force = true
		})

		It("returns the refusal error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("vault-cli task approve"))
		})

		It("does not write the task", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("allowing execution -> planning on a next-status task", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "next", "phase": "execution"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "planning"
		})

		It("writes the phase", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Phase()).NotTo(BeNil())
			Expect(*writtenTask.Phase()).To(Equal(domain.TaskPhasePlanning))
		})
	})

	Context("allowing planning -> execution on in_progress task", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "in_progress", "phase": "planning"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "execution"
		})

		It("writes the phase", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		})
	})

	Context("allowing execution -> planning with --force on in_progress task", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "in_progress", "phase": "execution"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "planning"
			force = true
		})

		It("writes the phase", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		})
	})

	Context("allowing execution -> todo with --force on in_progress task", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "in_progress", "phase": "execution"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "todo"
			force = true
		})

		It("writes the regressed phase", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(*writtenTask.Phase()).To(Equal(domain.TaskPhaseTodo))
		})
	})

	Context("allowing planning -> todo on in_progress task", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "in_progress", "phase": "planning"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "phase"
			value = "todo"
		})

		It("writes the phase", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		})
	})

	Context("setting flag field", func() {
		BeforeEach(func() {
			key = "flag"
			value = "true"
		})

		It("updates the flag field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Flag()).To(BeTrue())
		})
	})

	Context("setting flag field to false", func() {
		BeforeEach(func() {
			key = "flag"
			value = "no"
		})

		It("writes flag false", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Flag()).To(BeFalse())
		})
	})

	Context("setting invalid flag field", func() {
		BeforeEach(func() {
			key = "flag"
			value = "banana"
		})

		It("returns an error naming the value and the accepted set", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("banana"))
			Expect(err.Error()).To(ContainSubstring("true"))
			Expect(err.Error()).To(ContainSubstring("false"))
		})

		It("does not write the task", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("setting claude_session_id field", func() {
		BeforeEach(func() {
			key = "claude_session_id"
			value = "session-456"
		})

		It("updates the claude_session_id field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.ClaudeSessionID()).To(Equal("session-456"))
		})
	})

	Context("setting assignee field", func() {
		BeforeEach(func() {
			key = "assignee"
			value = "bob"
		})

		It("updates the assignee field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Assignee()).To(Equal("bob"))
		})
	})

	Context("setting status field", func() {
		BeforeEach(func() {
			key = "status"
			value = "completed"
		})

		It("updates the status field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status()).To(Equal(domain.TaskStatusCompleted))
		})
	})

	Context("one-step close-out via status set", func() {
		BeforeEach(func() {
			task = domain.NewTask(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: taskName},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			key = "status"
			reason = "reason text"
			gateSuccessor = "none"
		})

		Context("target is a close-out status", func() {
			BeforeEach(func() {
				value = "aborted"
			})

			It("persists reason and successor with the aborted status", func() {
				Expect(err).To(BeNil())
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status()).To(Equal(domain.TaskStatusAborted))
				Expect(writtenTask.GetString("aborted_reason")).To(Equal("reason text"))
				Expect(writtenTask.GetString("gate_successor")).To(Equal("none"))
			})
		})

		Context("target is not a close-out status", func() {
			BeforeEach(func() {
				value = "in_progress"
			})

			It("writes no close-out fields", func() {
				Expect(err).To(BeNil())
				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
				Expect(writtenTask.Status()).To(Equal(domain.TaskStatusInProgress))
				Expect(writtenTask.GetString("aborted_reason")).To(Equal(""))
				Expect(writtenTask.GetString("gate_successor")).To(Equal(""))
			})
		})
	})

	Context("setting priority field", func() {
		BeforeEach(func() {
			key = "priority"
			value = "1"
		})

		It("updates the priority field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Priority()).To(Equal(domain.Priority(1)))
		})
	})

	Context("setting defer_date field", func() {
		BeforeEach(func() {
			key = "defer_date"
			value = "2025-06-15"
		})

		It("updates the defer_date field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.DeferDate()).NotTo(BeNil())
			Expect(writtenTask.DeferDate().Format("2006-01-02")).To(Equal("2025-06-15"))
		})
	})

	Context("clearing defer_date with empty string", func() {
		BeforeEach(func() {
			key = "defer_date"
			value = ""
			deferDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
			task.SetDeferDate(
				func() *libtime.DateOrDateTime { d := libtime.DateOrDateTime(deferDate); return &d }(),
			)
		})

		It("sets defer_date to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.DeferDate()).To(BeNil())
		})
	})

	Context("invalid date format", func() {
		BeforeEach(func() {
			key = "defer_date"
			value = "2025-13-45"
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("invalid date format")))
		})
	})

	Context("setting planned_date field", func() {
		BeforeEach(func() {
			key = "planned_date"
			value = "2025-06-15"
		})

		It("updates the planned_date field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.PlannedDate()).NotTo(BeNil())
			Expect(writtenTask.PlannedDate().Format("2006-01-02")).To(Equal("2025-06-15"))
		})
	})

	Context("clearing planned_date with empty string", func() {
		BeforeEach(func() {
			key = "planned_date"
			value = ""
			plannedDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
			task.SetPlannedDate(
				func() *libtime.DateOrDateTime { d := libtime.DateOrDateTime(plannedDate); return &d }(),
			)
		})

		It("sets planned_date to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.PlannedDate()).To(BeNil())
		})
	})

	Context("invalid planned_date format", func() {
		BeforeEach(func() {
			key = "planned_date"
			value = "2025-13-45"
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("invalid date format")))
		})
	})

	Context("setting due_date field", func() {
		BeforeEach(func() {
			key = "due_date"
			value = "2025-06-15"
		})

		It("updates the due_date field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.DueDate()).NotTo(BeNil())
			Expect(writtenTask.DueDate().Format("2006-01-02")).To(Equal("2025-06-15"))
		})
	})

	Context("clearing due_date with empty string", func() {
		BeforeEach(func() {
			key = "due_date"
			value = ""
			dueDate := time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)
			task.SetDueDate(
				func() *libtime.DateOrDateTime { d := libtime.DateOrDateTime(dueDate); return &d }(),
			)
		})

		It("sets due_date to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.DueDate()).To(BeNil())
		})
	})

	Context("invalid due_date format", func() {
		BeforeEach(func() {
			key = "due_date"
			value = "2025-13-45"
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("invalid date format")))
		})
	})

	Context("setting recurring field", func() {
		BeforeEach(func() {
			key = "recurring"
			value = "monthly"
		})

		It("updates the recurring field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Recurring()).To(Equal("monthly"))
		})
	})

	Context("setting last_completed field", func() {
		BeforeEach(func() {
			key = "last_completed"
			value = "2025-03-15"
		})

		It("updates the last_completed field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.LastCompleted()).To(Equal("2025-03-15"))
		})
	})

	Context("setting page_type field", func() {
		BeforeEach(func() {
			key = "page_type"
			value = "task"
		})

		It("updates the page_type field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.PageType()).To(Equal("task"))
		})
	})

	Context("setting goals field", func() {
		BeforeEach(func() {
			key = "goals"
			value = "goal-a,goal-b"
		})

		It("updates the goals field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Goals()).To(Equal([]string{"goal-a", "goal-b"}))
		})
	})

	Context("clearing goals with empty string", func() {
		BeforeEach(func() {
			key = "goals"
			value = ""
			task.SetGoals([]string{"old"})
		})

		It("sets goals to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Goals()).To(BeNil())
		})
	})

	Context("setting tags field", func() {
		BeforeEach(func() {
			key = "tags"
			value = "tag-a,tag-b"
		})

		It("updates the tags field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Tags()).To(Equal([]string{"tag-a", "tag-b"}))
		})
	})

	Context("clearing tags with empty string", func() {
		BeforeEach(func() {
			key = "tags"
			value = ""
			task.SetTags([]string{"old"})
		})

		It("sets tags to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Tags()).To(BeNil())
		})
	})

	Context("unknown key", func() {
		BeforeEach(func() {
			key = "unknown_key"
			value = "value"
		})

		It("returns no error and calls WriteTask once", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		})
	})

	Context("task not found", func() {
		BeforeEach(func() {
			key = "phase"
			value = "planning"
			mockTaskStorage.FindTaskByNameReturns(nil, errors.New("task not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find task")))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("write error", func() {
		BeforeEach(func() {
			key = "phase"
			value = "planning"
			mockTaskStorage.WriteTaskReturns(errors.New("write failed"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("write task")))
		})
	})
})

var _ = Describe("FrontmatterClearOperation", func() {
	var (
		ctx             context.Context
		err             error
		clearOp         ops.FrontmatterClearOperation
		mockTaskStorage *mocks.TaskStorage
		vaultPath       string
		taskName        string
		key             string
		task            *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		mockFactory := &mocks.NotificationSenderFactory{}
		publisher := ops.NewEscalationPublisher("", "", mockFactory)
		clearOp = ops.NewFrontmatterClearOperation(mockTaskStorage, publisher, "personal", "25 Tasks")
		vaultPath = "/path/to/vault"
		taskName = "my-task"

		// Default: return a task with fields set
		task = domain.NewTask(
			map[string]any{
				"phase":             "in_progress",
				"claude_session_id": "session-123",
				"assignee":          "alice",
				"status":            "in_progress",
				"priority":          3,
				"defer_date":        "2024-12-31",
				"planned_date":      "2025-03-15",
				"due_date":          "2025-06-30",
				"recurring":         "weekly",
				"last_completed":    "2025-03-10",
				"page_type":         "task",
				"goals":             []any{"goal-1", "goal-2"},
				"tags":              []any{"urgent", "backend"},
			},
			domain.FileMetadata{Name: taskName},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	JustBeforeEach(func() {
		err = clearOp.Execute(ctx, vaultPath, taskName, key)
	})

	Context("clearing phase field", func() {
		BeforeEach(func() {
			key = "phase"
		})

		It("clears the phase field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Phase()).To(BeNil())
		})
	})

	Context("clearing claude_session_id field", func() {
		BeforeEach(func() {
			key = "claude_session_id"
		})

		It("clears the claude_session_id field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.ClaudeSessionID()).To(Equal(""))
		})
	})

	Context("clearing assignee field", func() {
		BeforeEach(func() {
			key = "assignee"
		})

		It("clears the assignee field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Assignee()).To(Equal(""))
		})
	})

	Context("clearing status field", func() {
		BeforeEach(func() {
			key = "status"
		})

		It("clears the status field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Status()).To(Equal(domain.TaskStatus("")))
		})
	})

	Context("clearing priority field", func() {
		BeforeEach(func() {
			key = "priority"
		})

		It("clears the priority field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Priority()).To(Equal(domain.Priority(0)))
		})
	})

	Context("clearing defer_date field", func() {
		BeforeEach(func() {
			key = "defer_date"
		})

		It("clears the defer_date field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.DeferDate()).To(BeNil())
		})
	})

	Context("clearing planned_date field", func() {
		BeforeEach(func() {
			key = "planned_date"
		})

		It("clears the planned_date field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.PlannedDate()).To(BeNil())
		})
	})

	Context("clearing due_date field", func() {
		BeforeEach(func() {
			key = "due_date"
		})

		It("clears the due_date field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.DueDate()).To(BeNil())
		})
	})

	Context("clearing recurring field", func() {
		BeforeEach(func() {
			key = "recurring"
		})

		It("clears the recurring field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Recurring()).To(Equal(""))
		})
	})

	Context("clearing last_completed field", func() {
		BeforeEach(func() {
			key = "last_completed"
		})

		It("clears the last_completed field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.LastCompleted()).To(Equal(""))
		})
	})

	Context("clearing page_type field", func() {
		BeforeEach(func() {
			key = "page_type"
		})

		It("clears the page_type field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.PageType()).To(Equal(""))
		})
	})

	Context("clearing goals field", func() {
		BeforeEach(func() {
			key = "goals"
		})

		It("clears the goals field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Goals()).To(BeNil())
		})
	})

	Context("clearing tags field", func() {
		BeforeEach(func() {
			key = "tags"
		})

		It("clears the tags field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Tags()).To(BeNil())
		})
	})

	Context("unknown key", func() {
		BeforeEach(func() {
			key = "unknown_key"
		})

		It("returns no error and calls WriteTask once", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		})
	})

	Context("task not found", func() {
		BeforeEach(func() {
			key = "phase"
			mockTaskStorage.FindTaskByNameReturns(nil, errors.New("task not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find task")))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("write error", func() {
		BeforeEach(func() {
			key = "phase"
			mockTaskStorage.WriteTaskReturns(errors.New("write failed"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("write task")))
		})
	})
})

var _ = Describe("Frontmatter assignee-clear escalation", func() {
	var (
		ctx              context.Context
		err              error
		mockTaskStorage  *mocks.TaskStorage
		mockSender       *mocks.NotificationPublishCommandSender
		mockFactory      *mocks.NotificationSenderFactory
		publisher        ops.EscalationPublisher
		setOp            ops.FrontmatterSetOperation
		clearOp          ops.FrontmatterClearOperation
		vaultPath        string
		taskName         string
		previousAssignee string
		taskIdentifier   string
		vaultName        string
		tasksDir         string
		task             *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		mockSender = &mocks.NotificationPublishCommandSender{}
		mockSender.SendPublishNotificationCommandReturns(nil)
		mockFactory = &mocks.NotificationSenderFactory{}
		mockFactory.CreateReturns(mockSender, nil)
		publisher = ops.NewEscalationPublisher("broker-1:9092", "master", mockFactory)
		vaultName = "personal"
		tasksDir = "25 Tasks"
		setOp = ops.NewFrontmatterSetOperation(
			mockTaskStorage,
			libtime.NewCurrentDateTime(),
			publisher,
			vaultName,
			tasksDir,
		)
		clearOp = ops.NewFrontmatterClearOperation(mockTaskStorage, publisher, vaultName, tasksDir)
		vaultPath = "/path/to/vault"
		taskName = "my-task"
		previousAssignee = "alice"
		taskIdentifier = "0f6a3a0e-0000-4000-8000-000000000001"
		task = domain.NewTask(
			map[string]any{
				"status":          "in_progress",
				"phase":           "human_review",
				"assignee":        previousAssignee,
				"task_identifier": taskIdentifier,
			},
			domain.FileMetadata{Name: taskName},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	// recordedCommand waits for the asynchronous publish and returns the single
	// command it recorded. PublishEscalation performs the publish in a goroutine,
	// so every positive assertion must go through Eventually.
	recordedCommand := func() notifcmd.NotificationPublishCommand {
		Eventually(func() int {
			return mockSender.SendPublishNotificationCommandCallCount()
		}).Should(Equal(1))
		_, command := mockSender.SendPublishNotificationCommandArgsForCall(0)
		return command
	}

	// assertSilent asserts that nothing was published and no connection was
	// attempted, for the whole duration a stray asynchronous publish would need.
	assertSilent := func() {
		Consistently(func() int {
			return mockSender.SendPublishNotificationCommandCallCount()
		}, "200ms", "50ms").Should(Equal(0))
		Expect(mockFactory.CreateCallCount()).To(Equal(0))
	}

	It("publishes one escalation when task set empties a non-empty assignee", func() {
		err = setOp.Execute(ctx, vaultPath, taskName, "assignee", "", "", "", "", false)

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(writtenTask.Assignee()).To(Equal(""))

		command := recordedCommand()
		Expect(command.Type).To(Equal(notifcore.AgentEscalationNotificationType))
		Expect(command.Target).To(BeNil())
		Expect(command.Metadata).To(Equal(map[string]string{
			"taskIdentifier":   taskIdentifier,
			"taskName":         taskName,
			"previousAssignee": previousAssignee,
		}))
		Expect(command.Message).To(Equal(notifcore.NotificationMessage(
			"escalation: alice cleared its assignee — status in_progress, phase human_review\n" +
				"obsidian://open?vault=personal&file=25+Tasks%2Fmy-task",
		)))
	})

	It("publishes one escalation when task clear removes a non-empty assignee", func() {
		err = clearOp.Execute(ctx, vaultPath, taskName, "assignee")

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(writtenTask.Assignee()).To(Equal(""))

		command := recordedCommand()
		Expect(command.Type).To(Equal(notifcore.AgentEscalationNotificationType))
		Expect(command.Target).To(BeNil())
		Expect(command.Metadata).To(Equal(map[string]string{
			"taskIdentifier":   taskIdentifier,
			"taskName":         taskName,
			"previousAssignee": previousAssignee,
		}))
		Expect(command.Message).To(Equal(notifcore.NotificationMessage(
			"escalation: alice cleared its assignee — status in_progress, phase human_review\n" +
				"obsidian://open?vault=personal&file=25+Tasks%2Fmy-task",
		)))
	})

	It("publishes nothing when task set writes an empty assignee over an empty assignee", func() {
		task.ClearField("assignee")

		err = setOp.Execute(ctx, vaultPath, taskName, "assignee", "", "", "", "", false)

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		assertSilent()
	})

	It("publishes nothing when task clear removes an already-absent assignee", func() {
		task.ClearField("assignee")

		err = clearOp.Execute(ctx, vaultPath, taskName, "assignee")

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		assertSilent()
	})

	It("publishes nothing when task set replaces one assignee with another", func() {
		err = setOp.Execute(ctx, vaultPath, taskName, "assignee", "bob", "", "", "", false)

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		assertSilent()
	})

	It("publishes nothing when a different frontmatter key is set through task set", func() {
		err = setOp.Execute(ctx, vaultPath, taskName, "priority", "3", "", "", "", false)

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		assertSilent()
	})

	It("publishes nothing when a different frontmatter key is cleared through task clear", func() {
		err = clearOp.Execute(ctx, vaultPath, taskName, "status")

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		assertSilent()
	})

	It("publishes nothing when task set fails to write", func() {
		mockTaskStorage.WriteTaskReturns(errors.New("write failed"))

		err = setOp.Execute(ctx, vaultPath, taskName, "assignee", "", "", "", "", false)

		Expect(err).To(MatchError(ContainSubstring("write task")))
		assertSilent()
	})

	It("publishes nothing when task clear fails to write", func() {
		mockTaskStorage.WriteTaskReturns(errors.New("write failed"))

		err = clearOp.Execute(ctx, vaultPath, taskName, "assignee")

		Expect(err).To(MatchError(ContainSubstring("write task")))
		assertSilent()
	})

	It("returns success within the publish bound when the sender blocks past it", func() {
		blocked := make(chan struct{})
		DeferCleanup(func() {
			close(blocked)
		})
		mockSender.SendPublishNotificationCommandStub = func(
			context.Context, notifcmd.NotificationPublishCommand,
		) error {
			<-blocked
			return nil
		}

		start := time.Now()
		err = setOp.Execute(ctx, vaultPath, taskName, "assignee", "", "", "", "", false)
		elapsed := time.Since(start)

		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(writtenTask.Assignee()).To(Equal(""))
		Expect(elapsed).To(BeNumerically(">=", ops.EscalationPublishTimeout))
		Expect(elapsed).To(BeNumerically("<", ops.EscalationPublishTimeout+3*time.Second))
	})
})

// The flag provenance contract: the flag and the actor and instant that set it
// are composed onto one map and persisted by the operation's single WriteTask,
// so no read can observe an unattributed flag.
var _ = Describe("FrontmatterSetOperation flag provenance", func() {
	var (
		ctx             context.Context
		err             error
		setOp           ops.FrontmatterSetOperation
		clearOp         ops.FrontmatterClearOperation
		mockTaskStorage *mocks.TaskStorage
		pinned          libtime.CurrentDateTime
		vaultPath       string
		taskName        string
		task            *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		pinned = libtime.NewCurrentDateTime()
		pinned.SetNow(libtimetest.ParseDateTime("2026-09-30T08:00:00Z"))
		publisher := ops.NewEscalationPublisher("", "", &mocks.NotificationSenderFactory{})
		setOp = ops.NewFrontmatterSetOperation(
			mockTaskStorage,
			pinned,
			publisher,
			"personal",
			"25 Tasks",
		)
		clearOp = ops.NewFrontmatterClearOperation(mockTaskStorage, publisher, "personal", "25 Tasks")
		vaultPath = "/path/to/vault"
		taskName = "Alpha"
		task = domain.NewTask(
			map[string]any{"status": "next", "page_type": "task"},
			domain.FileMetadata{Name: taskName},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	// seedFlaggedRow puts a pre-existing attributed flag on the row, so a spec
	// can assert what a later write does to it.
	seedFlaggedRow := func() {
		task.Set("flag", true)
		task.Set("flag_set_by", "agent-x")
		task.Set("flag_set_at", pinned.Now().Time())
	}

	written := func() *domain.Task {
		_, w := mockTaskStorage.WriteTaskArgsForCall(0)
		return w
	}

	DescribeTable("records the actor that set the flag in exactly one write",
		func(actor string) {
			Expect(setOp.Execute(
				ctx, vaultPath, taskName, "flag", "true", "", "", actor, false,
			)).To(Succeed())

			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			Expect(written().Flag()).To(BeTrue())
			Expect(written().GetString("flag_set_by")).To(Equal(actor))
			Expect(written().Get("flag_set_at")).NotTo(BeNil())
		},
		Entry("operator", "operator"),
		Entry("agent-x", "agent-x"),
		Entry("unknown", "unknown"),
	)

	DescribeTable("writes provenance for a mixed-case truthy value",
		func(value string) {
			Expect(setOp.Execute(
				ctx, vaultPath, taskName, "flag", value, "", "", "agent-x", false,
			)).To(Succeed())

			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			Expect(written().Flag()).To(BeTrue())
			Expect(written().GetString("flag_set_by")).To(Equal("agent-x"))
		},
		Entry("uppercase TRUE", "TRUE"),
		Entry("yes", "yes"),
		Entry("padded whitespace", "  Yes  "),
	)

	It("refuses a blank actor with zero writes and an untouched row", func() {
		err = setOp.Execute(ctx, vaultPath, taskName, "flag", "true", "", "", "", false)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("flag_set_by"))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		Expect(task.Get("flag")).To(BeNil())
		Expect(task.Get("flag_set_by")).To(BeNil())
		Expect(task.Get("flag_set_at")).To(BeNil())
	})

	It("records the injected instant as a bare time.Time", func() {
		Expect(setOp.Execute(
			ctx, vaultPath, taskName, "flag", "true", "", "", "operator", false,
		)).To(Succeed())

		raw, ok := written().Get("flag_set_at").(time.Time)
		Expect(ok).To(BeTrue())
		Expect(raw.Equal(pinned.Now().Time())).To(BeTrue())
	})

	It("serializes flag_set_at unquoted and round-trips it as a time.Time", func() {
		Expect(setOp.Execute(
			ctx, vaultPath, taskName, "flag", "true", "", "", "operator", false,
		)).To(Succeed())

		out, marshalErr := yaml.Marshal(written().RawMap())
		Expect(marshalErr).NotTo(HaveOccurred())
		Expect(string(out)).To(ContainSubstring("flag_set_at: 2026-09-30T08:00:00Z"))
		Expect(string(out)).NotTo(ContainSubstring(`flag_set_at: "2026-09-30T08:00:00Z"`))

		var roundTripped map[string]any
		Expect(yaml.Unmarshal(out, &roundTripped)).To(Succeed())
		rt, ok := roundTripped["flag_set_at"].(time.Time)
		Expect(ok).To(BeTrue())
		Expect(rt.Equal(pinned.Now().Time())).To(BeTrue())
	})

	It("follows the last writer, not the first", func() {
		Expect(setOp.Execute(
			ctx, vaultPath, taskName, "flag", "true", "", "", "agent-x", false,
		)).To(Succeed())
		Expect(setOp.Execute(
			ctx, vaultPath, taskName, "flag", "true", "", "", "operator", false,
		)).To(Succeed())

		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(2))
		_, second := mockTaskStorage.WriteTaskArgsForCall(1)
		Expect(second.GetString("flag_set_by")).To(Equal("operator"))
	})

	It("clears all three keys when the flag is set with the empty value", func() {
		seedFlaggedRow()

		Expect(setOp.Execute(
			ctx, vaultPath, taskName, "flag", "", "", "", "operator", false,
		)).To(Succeed())

		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		Expect(written().Get("flag")).To(BeNil())
		Expect(written().Get("flag_set_by")).To(BeNil())
		Expect(written().Get("flag_set_at")).To(BeNil())
	})

	It("clears all three keys when the flag is cleared", func() {
		seedFlaggedRow()

		Expect(clearOp.Execute(ctx, vaultPath, taskName, "flag")).To(Succeed())

		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		Expect(written().Get("flag")).To(BeNil())
		Expect(written().Get("flag_set_by")).To(BeNil())
		Expect(written().Get("flag_set_at")).To(BeNil())
	})

	It("keeps flag: false and drops only the provenance on an explicit falsy write", func() {
		seedFlaggedRow()

		Expect(setOp.Execute(
			ctx, vaultPath, taskName, "flag", "no", "", "", "operator", false,
		)).To(Succeed())

		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		Expect(written().Get("flag")).To(Equal(false))
		Expect(written().Flag()).To(BeFalse())
		Expect(written().GetField("flag")).To(Equal("false"))
		Expect(written().Get("flag_set_by")).To(BeNil())
		Expect(written().Get("flag_set_at")).To(BeNil())
	})

	It("does not backfill provenance onto a pre-field row when another key is written", func() {
		task.Set("flag", true)

		Expect(setOp.Execute(
			ctx, vaultPath, taskName, "priority", "3", "", "", "operator", false,
		)).To(Succeed())

		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		Expect(written().Flag()).To(BeTrue())
		Expect(written().Get("flag_set_by")).To(BeNil())
		Expect(written().Get("flag_set_at")).To(BeNil())
	})

	DescribeTable("refuses a direct provenance write with zero writes",
		func(key string) {
			err = setOp.Execute(ctx, vaultPath, taskName, key, "someone", "", "", "operator", false)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(key))
			Expect(err.Error()).To(ContainSubstring("flag"))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		},
		Entry("flag_set_by", "flag_set_by"),
		Entry("flag_set_at", "flag_set_at"),
	)
})
