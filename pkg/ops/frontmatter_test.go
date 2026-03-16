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
		deferDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		plannedDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
		task = &domain.Task{
			Name:            taskName,
			Phase:           "implementation",
			ClaudeSessionID: "session-123",
			Assignee:        "alice",
			Status:          domain.TaskStatusInProgress,
			Priority:        domain.Priority(3),
			DeferDate:       libtime.ToDate(deferDate).Ptr(),
			PlannedDate:     libtime.ToDate(plannedDate).Ptr(),
			Recurring:       "weekly",
			LastCompleted:   "2025-03-10",
			PageType:        "task",
			Goals:           []string{"goal-1", "goal-2"},
			Tags:            []string{"urgent", "backend"},
		}
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
			Expect(result).To(Equal("implementation"))
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
			task.Phase = ""
		})

		It("returns empty string with no error", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("getting defer_date when nil", func() {
		BeforeEach(func() {
			key = "defer_date"
			task.DeferDate = nil
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
			task.PlannedDate = nil
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
			task.Goals = nil
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
			task.Tags = nil
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

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("unknown field: unknown_key")))
			Expect(result).To(Equal(""))
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
		task            *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		setOp = ops.NewFrontmatterSetOperation(mockTaskStorage)
		vaultPath = "/path/to/vault"
		taskName = "my-task"

		// Default: return a task
		task = &domain.Task{
			Name: taskName,
		}
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	JustBeforeEach(func() {
		err = setOp.Execute(ctx, vaultPath, taskName, key, value)
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
			Expect(writtenTask.Phase).To(Equal("planning"))
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
			Expect(writtenTask.ClaudeSessionID).To(Equal("session-456"))
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
			Expect(writtenTask.Assignee).To(Equal("bob"))
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
			Expect(writtenTask.Status).To(Equal(domain.TaskStatusCompleted))
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
			Expect(writtenTask.Priority).To(Equal(domain.Priority(1)))
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
			Expect(writtenTask.DeferDate).NotTo(BeNil())
			Expect(writtenTask.DeferDate.Format("2006-01-02")).To(Equal("2025-06-15"))
		})
	})

	Context("clearing defer_date with empty string", func() {
		BeforeEach(func() {
			key = "defer_date"
			value = ""
			deferDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
			task.DeferDate = libtime.ToDate(deferDate).Ptr()
		})

		It("sets defer_date to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.DeferDate).To(BeNil())
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
			Expect(writtenTask.PlannedDate).NotTo(BeNil())
			Expect(writtenTask.PlannedDate.Format("2006-01-02")).To(Equal("2025-06-15"))
		})
	})

	Context("clearing planned_date with empty string", func() {
		BeforeEach(func() {
			key = "planned_date"
			value = ""
			plannedDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
			task.PlannedDate = libtime.ToDate(plannedDate).Ptr()
		})

		It("sets planned_date to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.PlannedDate).To(BeNil())
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

	Context("setting recurring field", func() {
		BeforeEach(func() {
			key = "recurring"
			value = "monthly"
		})

		It("updates the recurring field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Recurring).To(Equal("monthly"))
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
			Expect(writtenTask.LastCompleted).To(Equal("2025-03-15"))
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
			Expect(writtenTask.PageType).To(Equal("task"))
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
			Expect(writtenTask.Goals).To(Equal([]string{"goal-a", "goal-b"}))
		})
	})

	Context("clearing goals with empty string", func() {
		BeforeEach(func() {
			key = "goals"
			value = ""
			task.Goals = []string{"old"}
		})

		It("sets goals to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Goals).To(BeNil())
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
			Expect(writtenTask.Tags).To(Equal([]string{"tag-a", "tag-b"}))
		})
	})

	Context("clearing tags with empty string", func() {
		BeforeEach(func() {
			key = "tags"
			value = ""
			task.Tags = []string{"old"}
		})

		It("sets tags to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Tags).To(BeNil())
		})
	})

	Context("unknown key", func() {
		BeforeEach(func() {
			key = "unknown_key"
			value = "value"
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("unknown field: unknown_key")))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
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
		clearOp = ops.NewFrontmatterClearOperation(mockTaskStorage)
		vaultPath = "/path/to/vault"
		taskName = "my-task"

		// Default: return a task with fields set
		deferDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		plannedDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
		task = &domain.Task{
			Name:            taskName,
			Phase:           "implementation",
			ClaudeSessionID: "session-123",
			Assignee:        "alice",
			Status:          domain.TaskStatusInProgress,
			Priority:        domain.Priority(3),
			DeferDate:       libtime.ToDate(deferDate).Ptr(),
			PlannedDate:     libtime.ToDate(plannedDate).Ptr(),
			Recurring:       "weekly",
			LastCompleted:   "2025-03-10",
			PageType:        "task",
			Goals:           []string{"goal-1", "goal-2"},
			Tags:            []string{"urgent", "backend"},
		}
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
			Expect(writtenTask.Phase).To(Equal(""))
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
			Expect(writtenTask.ClaudeSessionID).To(Equal(""))
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
			Expect(writtenTask.Assignee).To(Equal(""))
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
			Expect(writtenTask.Status).To(Equal(domain.TaskStatus("")))
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
			Expect(writtenTask.Priority).To(Equal(domain.Priority(0)))
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
			Expect(writtenTask.DeferDate).To(BeNil())
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
			Expect(writtenTask.PlannedDate).To(BeNil())
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
			Expect(writtenTask.Recurring).To(Equal(""))
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
			Expect(writtenTask.LastCompleted).To(Equal(""))
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
			Expect(writtenTask.PageType).To(Equal(""))
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
			Expect(writtenTask.Goals).To(BeNil())
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
			Expect(writtenTask.Tags).To(BeNil())
		})
	})

	Context("unknown key", func() {
		BeforeEach(func() {
			key = "unknown_key"
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("unknown field: unknown_key")))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
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
