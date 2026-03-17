// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("NewGoalGetOperation", func() {
	var (
		ctx             context.Context
		err             error
		result          string
		getOp           ops.EntityGetOperation
		mockGoalStorage *mocks.GoalStorage
		vaultPath       string
		goalName        string
		key             string
		goal            *domain.Goal
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		getOp = ops.NewGoalGetOperation(mockGoalStorage)
		vaultPath = "/path/to/vault"
		goalName = "my-goal"

		startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		goal = &domain.Goal{
			Name:     goalName,
			Status:   domain.GoalStatusActive,
			PageType: "goal",
			Theme:    "health",
			Priority: domain.Priority(2),
			Assignee: "alice",
			Tags:     []string{"urgent", "q1"},
			FilePath: "/vault/Goals/my-goal.md",
			Content:  "---\nstatus: active\n---\n",
		}
		goal.StartDate = &startDate
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
	})

	JustBeforeEach(func() {
		result, err = getOp.Execute(ctx, vaultPath, goalName, key)
	})

	Context("getting status field", func() {
		BeforeEach(func() {
			key = "status"
		})

		It("returns the status value", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("active"))
		})
	})

	Context("getting tags field", func() {
		BeforeEach(func() {
			key = "tags"
		})

		It("returns comma-joined tags", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("urgent,q1"))
		})
	})

	Context("getting unset optional field", func() {
		BeforeEach(func() {
			key = "assignee"
			goal.Assignee = ""
		})

		It("returns empty string with no error", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("unknown key", func() {
		BeforeEach(func() {
			key = "xyz"
		})

		It("returns unknown field error", func() {
			Expect(err).To(MatchError(ContainSubstring("unknown field")))
			Expect(result).To(Equal(""))
		})
	})

	Context("FindGoalByName fails", func() {
		BeforeEach(func() {
			key = "status"
			mockGoalStorage.FindGoalByNameReturns(nil, errors.New("not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find goal")))
		})
	})
})

var _ = Describe("NewGoalSetOperation", func() {
	var (
		ctx             context.Context
		err             error
		setOp           ops.EntitySetOperation
		mockGoalStorage *mocks.GoalStorage
		vaultPath       string
		goalName        string
		key             string
		value           string
		goal            *domain.Goal
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		setOp = ops.NewGoalSetOperation(mockGoalStorage)
		vaultPath = "/path/to/vault"
		goalName = "my-goal"

		goal = &domain.Goal{
			Name:   goalName,
			Status: domain.GoalStatusActive,
		}
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
		mockGoalStorage.WriteGoalReturns(nil)
	})

	JustBeforeEach(func() {
		err = setOp.Execute(ctx, vaultPath, goalName, key, value)
	})

	Context("setting a string field", func() {
		BeforeEach(func() {
			key = "status"
			value = "completed"
		})

		It("sets the field and calls WriteGoal", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(string(writtenGoal.Status)).To(Equal("completed"))
		})
	})

	Context("setting a *time.Time date field", func() {
		BeforeEach(func() {
			key = "start_date"
			value = "2025-06-15"
		})

		It("sets the date and calls WriteGoal", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.StartDate).NotTo(BeNil())
			Expect(writtenGoal.StartDate.Format("2006-01-02")).To(Equal("2025-06-15"))
		})
	})

	Context("invalid date format", func() {
		BeforeEach(func() {
			key = "start_date"
			value = "2025-13-45"
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("invalid date format")))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("setting a []string field", func() {
		BeforeEach(func() {
			key = "tags"
			value = "tag-a,tag-b"
		})

		It("sets the slice and calls WriteGoal", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Tags).To(Equal([]string{"tag-a", "tag-b"}))
		})
	})

	Context("setting nil slice with empty string", func() {
		BeforeEach(func() {
			key = "tags"
			value = ""
			goal.Tags = []string{"old"}
		})

		It("sets tags to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Tags).To(BeNil())
		})
	})

	Context("unknown field", func() {
		BeforeEach(func() {
			key = "nonexistent"
			value = "val"
		})

		It("returns unknown field error", func() {
			Expect(err).To(MatchError(ContainSubstring("unknown field")))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("read-only field", func() {
		BeforeEach(func() {
			// yaml:"-" is the tag value for metadata fields
			key = "-"
			value = "some-value"
		})

		It("returns read-only error", func() {
			Expect(err).To(MatchError(ContainSubstring("read-only")))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("FindGoalByName fails", func() {
		BeforeEach(func() {
			key = "status"
			value = "active"
			mockGoalStorage.FindGoalByNameReturns(nil, errors.New("not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find goal")))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("WriteGoal fails", func() {
		BeforeEach(func() {
			key = "status"
			value = "active"
			mockGoalStorage.WriteGoalReturns(errors.New("write failed"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("write goal")))
		})
	})
})

var _ = Describe("NewGoalClearOperation", func() {
	var (
		ctx             context.Context
		err             error
		clearOp         ops.EntityClearOperation
		mockGoalStorage *mocks.GoalStorage
		vaultPath       string
		goalName        string
		key             string
		goal            *domain.Goal
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		clearOp = ops.NewGoalClearOperation(mockGoalStorage)
		vaultPath = "/path/to/vault"
		goalName = "my-goal"

		startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		goal = &domain.Goal{
			Name:     goalName,
			Status:   domain.GoalStatusActive,
			Assignee: "alice",
			Tags:     []string{"urgent"},
		}
		goal.StartDate = &startDate
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
		mockGoalStorage.WriteGoalReturns(nil)
	})

	JustBeforeEach(func() {
		err = clearOp.Execute(ctx, vaultPath, goalName, key)
	})

	Context("clearing string field", func() {
		BeforeEach(func() {
			key = "assignee"
		})

		It("sets the field to empty string", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Assignee).To(Equal(""))
		})
	})

	Context("clearing pointer field", func() {
		BeforeEach(func() {
			key = "start_date"
		})

		It("sets the field to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.StartDate).To(BeNil())
		})
	})

	Context("clearing slice field", func() {
		BeforeEach(func() {
			key = "tags"
		})

		It("sets the field to nil", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Tags).To(BeNil())
		})
	})

	Context("unknown field", func() {
		BeforeEach(func() {
			key = "nonexistent"
		})

		It("returns unknown field error", func() {
			Expect(err).To(MatchError(ContainSubstring("unknown field")))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})
})

var _ = Describe("NewGoalListAddOperation", func() {
	var (
		ctx             context.Context
		err             error
		addOp           ops.EntityListAddOperation
		mockGoalStorage *mocks.GoalStorage
		vaultPath       string
		goalName        string
		field           string
		value           string
		goal            *domain.Goal
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		addOp = ops.NewGoalListAddOperation(mockGoalStorage)
		vaultPath = "/path/to/vault"
		goalName = "my-goal"

		goal = &domain.Goal{
			Name:   goalName,
			Status: domain.GoalStatusActive,
			Tags:   []string{"existing"},
		}
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
		mockGoalStorage.WriteGoalReturns(nil)
	})

	JustBeforeEach(func() {
		err = addOp.Execute(ctx, vaultPath, goalName, field, value)
	})

	Context("successfully adding value to tags field", func() {
		BeforeEach(func() {
			field = "tags"
			value = "new-tag"
		})

		It("calls FindGoalByName and WriteGoal with updated tags", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.FindGoalByNameCallCount()).To(Equal(1))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Tags).To(ContainElement("new-tag"))
			Expect(writtenGoal.Tags).To(ContainElement("existing"))
		})
	})

	Context("value already in list", func() {
		BeforeEach(func() {
			field = "tags"
			value = "existing"
		})

		It("returns error containing 'already exists' and does not call WriteGoal", func() {
			Expect(err).To(MatchError(ContainSubstring("already exists")))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("field is scalar (not a list)", func() {
		BeforeEach(func() {
			field = "status"
			value = "active"
		})

		It("returns error containing 'not a list field' and does not call WriteGoal", func() {
			Expect(err).To(MatchError(ContainSubstring("not a list field")))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("unknown field", func() {
		BeforeEach(func() {
			field = "nonexistent"
			value = "val"
		})

		It("returns error containing 'unknown field' and does not call WriteGoal", func() {
			Expect(err).To(MatchError(ContainSubstring("unknown field")))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("FindGoalByName fails", func() {
		BeforeEach(func() {
			field = "tags"
			value = "new-tag"
			mockGoalStorage.FindGoalByNameReturns(nil, errors.New("not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find goal")))
		})
	})

	Context("WriteGoal fails", func() {
		BeforeEach(func() {
			field = "tags"
			value = "new-tag"
			mockGoalStorage.WriteGoalReturns(errors.New("write failed"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("write goal")))
		})
	})
})

var _ = Describe("NewGoalListRemoveOperation", func() {
	var (
		ctx             context.Context
		err             error
		removeOp        ops.EntityListRemoveOperation
		mockGoalStorage *mocks.GoalStorage
		vaultPath       string
		goalName        string
		field           string
		value           string
		goal            *domain.Goal
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		removeOp = ops.NewGoalListRemoveOperation(mockGoalStorage)
		vaultPath = "/path/to/vault"
		goalName = "my-goal"

		goal = &domain.Goal{
			Name:   goalName,
			Status: domain.GoalStatusActive,
			Tags:   []string{"existing", "other"},
		}
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
		mockGoalStorage.WriteGoalReturns(nil)
	})

	JustBeforeEach(func() {
		err = removeOp.Execute(ctx, vaultPath, goalName, field, value)
	})

	Context("successfully removing value from tags field", func() {
		BeforeEach(func() {
			field = "tags"
			value = "existing"
		})

		It("calls WriteGoal with updated tags", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Tags).NotTo(ContainElement("existing"))
			Expect(writtenGoal.Tags).To(ContainElement("other"))
		})
	})

	Context("value not in list", func() {
		BeforeEach(func() {
			field = "tags"
			value = "absent"
		})

		It("returns error containing 'not found' and does not call WriteGoal", func() {
			Expect(err).To(MatchError(ContainSubstring("not found")))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("field is scalar (not a list)", func() {
		BeforeEach(func() {
			field = "status"
			value = "active"
		})

		It("returns error containing 'not a list field' and does not call WriteGoal", func() {
			Expect(err).To(MatchError(ContainSubstring("not a list field")))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("unknown field", func() {
		BeforeEach(func() {
			field = "nonexistent"
			value = "val"
		})

		It("returns error containing 'unknown field' and does not call WriteGoal", func() {
			Expect(err).To(MatchError(ContainSubstring("unknown field")))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("FindGoalByName fails", func() {
		BeforeEach(func() {
			field = "tags"
			value = "existing"
			mockGoalStorage.FindGoalByNameReturns(nil, errors.New("not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find goal")))
		})
	})

	Context("WriteGoal fails", func() {
		BeforeEach(func() {
			field = "tags"
			value = "existing"
			mockGoalStorage.WriteGoalReturns(errors.New("write failed"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("write goal")))
		})
	})
})

var _ = Describe("NewTaskListAddOperation", func() {
	var (
		ctx             context.Context
		err             error
		addOp           ops.EntityListAddOperation
		mockTaskStorage *mocks.TaskStorage
		vaultPath       string
		taskName        string
		field           string
		value           string
		task            *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		addOp = ops.NewTaskListAddOperation(mockTaskStorage)
		vaultPath = "/path/to/vault"
		taskName = "my-task"

		task = &domain.Task{
			Name:   taskName,
			Status: domain.TaskStatusTodo,
			Goals:  []string{"existing-goal"},
		}
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	JustBeforeEach(func() {
		err = addOp.Execute(ctx, vaultPath, taskName, field, value)
	})

	Context("successfully adding value to goals field", func() {
		BeforeEach(func() {
			field = "goals"
			value = "new-goal"
		})

		It("calls FindTaskByName and WriteTask with updated goals", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.FindTaskByNameCallCount()).To(Equal(1))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Goals).To(ContainElement("new-goal"))
			Expect(writtenTask.Goals).To(ContainElement("existing-goal"))
		})
	})

	Context("value already in list", func() {
		BeforeEach(func() {
			field = "goals"
			value = "existing-goal"
		})

		It("returns error containing 'already exists' and does not call WriteTask", func() {
			Expect(err).To(MatchError(ContainSubstring("already exists")))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("field is scalar (not a list)", func() {
		BeforeEach(func() {
			field = "status"
			value = "todo"
		})

		It("returns error containing 'not a list field' and does not call WriteTask", func() {
			Expect(err).To(MatchError(ContainSubstring("not a list field")))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("unknown field", func() {
		BeforeEach(func() {
			field = "nonexistent"
			value = "val"
		})

		It("returns error containing 'unknown field' and does not call WriteTask", func() {
			Expect(err).To(MatchError(ContainSubstring("unknown field")))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("FindTaskByName fails", func() {
		BeforeEach(func() {
			field = "goals"
			value = "new-goal"
			mockTaskStorage.FindTaskByNameReturns(nil, errors.New("not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find task")))
		})
	})

	Context("WriteTask fails", func() {
		BeforeEach(func() {
			field = "goals"
			value = "new-goal"
			mockTaskStorage.WriteTaskReturns(errors.New("write failed"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("write task")))
		})
	})
})

var _ = Describe("NewTaskListRemoveOperation", func() {
	var (
		ctx             context.Context
		err             error
		removeOp        ops.EntityListRemoveOperation
		mockTaskStorage *mocks.TaskStorage
		vaultPath       string
		taskName        string
		field           string
		value           string
		task            *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		removeOp = ops.NewTaskListRemoveOperation(mockTaskStorage)
		vaultPath = "/path/to/vault"
		taskName = "my-task"

		task = &domain.Task{
			Name:   taskName,
			Status: domain.TaskStatusTodo,
			Goals:  []string{"goal-a", "goal-b"},
		}
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	JustBeforeEach(func() {
		err = removeOp.Execute(ctx, vaultPath, taskName, field, value)
	})

	Context("successfully removing value from goals field", func() {
		BeforeEach(func() {
			field = "goals"
			value = "goal-a"
		})

		It("calls WriteTask with updated goals", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Goals).NotTo(ContainElement("goal-a"))
			Expect(writtenTask.Goals).To(ContainElement("goal-b"))
		})
	})

	Context("value not in list", func() {
		BeforeEach(func() {
			field = "goals"
			value = "absent"
		})

		It("returns error containing 'not found' and does not call WriteTask", func() {
			Expect(err).To(MatchError(ContainSubstring("not found")))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("field is scalar (not a list)", func() {
		BeforeEach(func() {
			field = "status"
			value = "todo"
		})

		It("returns error containing 'not a list field' and does not call WriteTask", func() {
			Expect(err).To(MatchError(ContainSubstring("not a list field")))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("unknown field", func() {
		BeforeEach(func() {
			field = "nonexistent"
			value = "val"
		})

		It("returns error containing 'unknown field' and does not call WriteTask", func() {
			Expect(err).To(MatchError(ContainSubstring("unknown field")))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})

	Context("FindTaskByName fails", func() {
		BeforeEach(func() {
			field = "goals"
			value = "goal-a"
			mockTaskStorage.FindTaskByNameReturns(nil, errors.New("not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find task")))
		})
	})

	Context("WriteTask fails", func() {
		BeforeEach(func() {
			field = "goals"
			value = "goal-a"
			mockTaskStorage.WriteTaskReturns(errors.New("write failed"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("write task")))
		})
	})
})

var _ = Describe("NewGoalShowOperation", func() {
	var (
		ctx             context.Context
		err             error
		showOp          ops.EntityShowOperation
		mockGoalStorage *mocks.GoalStorage
		vaultPath       string
		vaultName       string
		goalName        string
		outputFormat    string
		goal            *domain.Goal
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		showOp = ops.NewGoalShowOperation(mockGoalStorage)
		vaultPath = "/path/to/vault"
		vaultName = "my-vault"
		goalName = "my-goal"
		outputFormat = "plain"

		goal = &domain.Goal{
			Name:     goalName,
			Status:   domain.GoalStatusActive,
			PageType: "goal",
			Theme:    "health",
			Tags:     []string{"important"},
			FilePath: "/vault/Goals/my-goal.md",
			Content:  "---\nstatus: active\n---\n",
		}
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
	})

	JustBeforeEach(func() {
		err = showOp.Execute(ctx, vaultPath, vaultName, goalName, outputFormat)
	})

	Context("plain output", func() {
		BeforeEach(func() {
			outputFormat = "plain"
		})

		It("succeeds without error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("json output", func() {
		BeforeEach(func() {
			outputFormat = "json"
		})

		It("succeeds without error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("find fails", func() {
		BeforeEach(func() {
			mockGoalStorage.FindGoalByNameReturns(nil, errors.New("not found"))
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("find goal")))
		})
	})
})
