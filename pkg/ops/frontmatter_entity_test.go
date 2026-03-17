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
