// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"

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
		op              ops.GoalCompleteOperation
		mockGoalStorage *mocks.GoalStorage
		mockTaskStorage *mocks.TaskStorage
		currentDateTime libtime.CurrentDateTime
		vaultPath       string
		goalName        string
		vaultName       string
		outputFormat    string
		force           bool
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
		outputFormat = "plain"
		force = false

		goal = &domain.Goal{
			Name:   goalName,
			Status: domain.GoalStatusActive,
		}
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
		mockGoalStorage.WriteGoalReturns(nil)
		mockTaskStorage.ListTasksReturns(nil, nil)
	})

	JustBeforeEach(func() {
		err = op.Execute(ctx, vaultPath, goalName, vaultName, outputFormat, force)
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
			goal.Status = domain.GoalStatusCompleted
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
				{Name: "open-task", Status: domain.TaskStatusTodo, Goals: []string{goalName}},
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
				{
					Name:   "active-task",
					Status: domain.TaskStatusInProgress,
					Goals:  []string{goalName},
				},
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
				{Name: "done-task", Status: domain.TaskStatusCompleted, Goals: []string{goalName}},
				{Name: "aborted-task", Status: domain.TaskStatusAborted, Goals: []string{goalName}},
				{Name: "hold-task", Status: domain.TaskStatusHold, Goals: []string{goalName}},
			}
			mockTaskStorage.ListTasksReturns(tasks, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("writes goal as completed", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status).To(Equal(domain.GoalStatusCompleted))
		})
	})

	Context("tasks linked to other goals do not block", func() {
		BeforeEach(func() {
			tasks := []*domain.Task{
				{Name: "other-task", Status: domain.TaskStatusTodo, Goals: []string{"other-goal"}},
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
				{Name: "open-task", Status: domain.TaskStatusTodo, Goals: []string{goalName}},
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

	Context("success plain mode", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("writes goal with completed status", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status).To(Equal(domain.GoalStatusCompleted))
			Expect(writtenGoal.Completed).NotTo(BeNil())
		})
	})

	Context("success JSON mode", func() {
		var (
			pipeReader *os.File
			pipeWriter *os.File
			origStdout *os.File
			output     string
		)

		BeforeEach(func() {
			outputFormat = "json"
			pipeReader, pipeWriter, _ = os.Pipe()
			origStdout = os.Stdout
			os.Stdout = pipeWriter
		})

		JustBeforeEach(func() {
			pipeWriter.Close()
			os.Stdout = origStdout
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(pipeReader)
			output = buf.String()
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("outputs valid JSON with success true", func() {
			var result ops.GoalCompleteResult
			Expect(json.Unmarshal([]byte(output), &result)).To(Succeed())
			Expect(result.Success).To(BeTrue())
			Expect(result.Status).To(Equal("completed"))
			Expect(result.Completed).To(Equal("2026-03-17"))
			Expect(result.Name).To(Equal(goalName))
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
