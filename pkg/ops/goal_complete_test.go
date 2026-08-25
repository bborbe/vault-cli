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

var _ = Describe("GoalCompleteOperation", func() {
	var (
		ctx             context.Context
		err             error
		result          ops.MutationResult
		op              ops.GoalCompleteOperation
		mockGoalStorage *mocks.GoalStorage
		mockTaskStorage *mocks.TaskStorage
		currentDateTime libtime.CurrentDateTime
		vaultPath       string
		goalName        string
		vaultName       string
		force           bool
		reason          string
		gateSuccessor   string
		goal            *domain.Goal
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		mockTaskStorage = &mocks.TaskStorage{}
		currentDateTime = libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-17T12:00:00Z"))
		op = ops.NewGoalCompleteOperation(mockGoalStorage, mockTaskStorage, currentDateTime)
		vaultPath = "/path/to/vault"
		goalName = "my-goal"
		vaultName = "test-vault"
		force = false

		goal = domain.NewGoal(
			map[string]any{"status": "active", "aborted_reason": "test reason", "gate_successor": "none"},
			domain.FileMetadata{Name: goalName},
			domain.Content(""),
		)
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
		mockGoalStorage.WriteGoalReturns(nil)
		mockTaskStorage.ListTasksReturns(nil, nil)
		reason = ""
		gateSuccessor = ""
	})

	JustBeforeEach(func() {
		result, err = op.Execute(
			ctx,
			vaultPath,
			goalName,
			vaultName,
			force,
			reason,
			gateSuccessor,
		)
	})

	Context("goal not found", func() {
		BeforeEach(func() {
			mockGoalStorage.FindGoalByNameReturns(nil, ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})

		It("does not write goal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("goal already completed", func() {
		BeforeEach(func() {
			_ = goal.SetStatus(domain.GoalStatusCompleted)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("already completed"))
		})

		It("does not write goal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("open todo task blocks completion", func() {
		BeforeEach(func() {
			tasks := []*domain.Task{
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "todo"},
						domain.FileMetadata{Name: "open-task"},
						domain.Content(""),
					)
					t.SetGoals([]string{goalName})
					return t
				}(),
			}
			mockTaskStorage.ListTasksReturns(tasks, nil)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("cannot complete goal"))
			Expect(err.Error()).To(ContainSubstring("open-task"))
		})

		It("does not write goal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("open in_progress task blocks completion", func() {
		BeforeEach(func() {
			tasks := []*domain.Task{
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "in_progress"},
						domain.FileMetadata{Name: "active-task"},
						domain.Content(""),
					)
					t.SetGoals([]string{goalName})
					return t
				}(),
			}
			mockTaskStorage.ListTasksReturns(tasks, nil)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("cannot complete goal"))
			Expect(err.Error()).To(ContainSubstring("active-task"))
		})
	})

	Context("completed tasks do not block", func() {
		BeforeEach(func() {
			tasks := []*domain.Task{
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "completed"},
						domain.FileMetadata{Name: "done-task"},
						domain.Content(""),
					)
					t.SetGoals([]string{goalName})
					return t
				}(),
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "aborted"},
						domain.FileMetadata{Name: "aborted-task"},
						domain.Content(""),
					)
					t.SetGoals([]string{goalName})
					return t
				}(),
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "hold"},
						domain.FileMetadata{Name: "hold-task"},
						domain.Content(""),
					)
					t.SetGoals([]string{goalName})
					return t
				}(),
			}
			mockTaskStorage.ListTasksReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("writes goal as completed", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusCompleted))
		})
	})

	Context("tasks linked to other goals do not block", func() {
		BeforeEach(func() {
			tasks := []*domain.Task{
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "todo"},
						domain.FileMetadata{Name: "other-task"},
						domain.Content(""),
					)
					t.SetGoals([]string{"other-goal"})
					return t
				}(),
			}
			mockTaskStorage.ListTasksReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("zero linked tasks", func() {
		BeforeEach(func() {
			mockTaskStorage.ListTasksReturns(nil, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("force bypasses open task check", func() {
		BeforeEach(func() {
			force = true
			tasks := []*domain.Task{
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "todo"},
						domain.FileMetadata{Name: "open-task"},
						domain.Content(""),
					)
					t.SetGoals([]string{goalName})
					return t
				}(),
			}
			mockTaskStorage.ListTasksReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("does not call ListTasks", func() {
			Expect(mockTaskStorage.ListTasksCallCount()).To(Equal(0))
		})

		It("writes goal as completed", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
		})
	})

	Context("goal completes without close-out fields", func() {
		BeforeEach(func() {
			goal = domain.NewGoal(
				map[string]any{"status": "active"},
				domain.FileMetadata{Name: goalName},
				domain.Content(""),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("writes the goal in a single write", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
		})

		It("persists status completed without close-out fields", func() {
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusCompleted))
			Expect(writtenGoal.GetString("aborted_reason")).To(Equal(""))
			Expect(writtenGoal.GetString("gate_successor")).To(Equal(""))
		})
	})

	Context("one-step close-out writes reason and successor with the status", func() {
		BeforeEach(func() {
			goal = domain.NewGoal(
				map[string]any{"status": "active"},
				domain.FileMetadata{Name: goalName},
				domain.Content(""),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
			reason = "achieved"
			gateSuccessor = "none"
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("writes the goal in a single write", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
		})

		It("persists reason and successor with the status", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.GetString("aborted_reason")).To(Equal("achieved"))
			Expect(writtenGoal.GetString("gate_successor")).To(Equal("none"))
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusCompleted))
		})
	})

	Context("success", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("writes goal with completed status", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusCompleted))
			Expect(writtenGoal.Completed()).NotTo(BeNil())
		})

		It("returns result with success true", func() {
			Expect(result.Success).To(BeTrue())
			Expect(result.Name).To(Equal(goalName))
			Expect(result.Vault).To(Equal(vaultName))
		})
	})

	Context("WriteGoal error", func() {
		BeforeEach(func() {
			mockGoalStorage.WriteGoalReturns(ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})
	})
})
