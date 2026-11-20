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

// completeGoalContent is a closure-complete goal body. The absent
// # Definition of Done section is grandfathered (no created), so it closes.
func completeGoalContent(status string) domain.Content {
	return domain.Content("---\nstatus: " + status + "\n---\n# Success Criteria\n- [x] done\n")
}

// goalWithTopics builds a closure-complete goal whose frontmatter declares the
// given topics, so the parent cascade has a relation to walk.
func goalWithTopics(status string, topics []string) *domain.Goal {
	return domain.NewGoal(
		map[string]any{"status": status, "topics": topics},
		domain.FileMetadata{Name: "my-goal"},
		completeGoalContent(status),
	)
}

var _ = Describe("GoalCompleteOperation", func() {
	var (
		ctx                        context.Context
		err                        error
		result                     ops.MutationResult
		op                         ops.GoalCompleteOperation
		mockGoalStorage            *mocks.GoalStorage
		mockTaskStorage            *mocks.TaskStorage
		mockTopicStorage           *mocks.TopicStorage
		mockTopicCompleteOperation *mocks.TopicCompleteOperation
		currentDateTime            libtime.CurrentDateTime
		vaultPath                  string
		goalName                   string
		vaultName                  string
		force                      bool
		reason                     string
		gateSuccessor              string
		goal                       *domain.Goal
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		mockTaskStorage = &mocks.TaskStorage{}
		mockTopicStorage = &mocks.TopicStorage{}
		mockTopicStorage.ListTopicsReturns(nil, nil)
		mockTopicCompleteOperation = &mocks.TopicCompleteOperation{}
		mockTopicCompleteOperation.ExecuteReturns(ops.MutationResult{Success: true}, nil)
		currentDateTime = libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-17T12:00:00Z"))
		op = ops.NewGoalCompleteOperation(
			mockGoalStorage,
			mockTaskStorage,
			currentDateTime,
			mockTopicCompleteOperation,
			mockTopicStorage,
		)
		vaultPath = "/path/to/vault"
		goalName = "my-goal"
		vaultName = "test-vault"
		force = false

		goal = domain.NewGoal(
			map[string]any{"status": "active", "aborted_reason": "test reason", "gate_successor": "none"},
			domain.FileMetadata{Name: goalName},
			completeGoalContent("active"),
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

		It("returns no error", func() {
			Expect(err).To(BeNil())
			Expect(result.Success).To(BeTrue())
			Expect(result.Name).To(Equal(goalName))
		})

		It("does not write goal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})

		It("reports no open items", func() {
			Expect(result.OpenItems).To(BeEmpty())
		})
	})

	Context("goal already aborted", func() {
		BeforeEach(func() {
			_ = goal.SetStatus(domain.GoalStatusAborted)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
			Expect(result.Success).To(BeTrue())
			Expect(result.Name).To(Equal(goalName))
		})

		It("does not write goal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("open criterion blocks completion", func() {
		BeforeEach(func() {
			goal = domain.NewGoal(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: goalName},
				domain.Content(
					"---\nstatus: in_progress\n---\n# Success Criteria\n- [ ] an open criterion\n"+
						"# Definition of Done\n- [x] done\n",
				),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		})

		It("returns an error naming the criterion", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("an open criterion"))
		})

		It("does not write goal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})

		It("reports the single open item", func() {
			Expect(result.OpenItems).To(HaveLen(1))
		})
	})

	Context("force does not bypass an open criterion", func() {
		BeforeEach(func() {
			force = true
			goal = domain.NewGoal(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: goalName},
				domain.Content(
					"---\nstatus: in_progress\n---\n# Success Criteria\n- [ ] an open criterion\n"+
						"# Definition of Done\n- [x] done\n",
				),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		})

		It("returns an error naming the criterion", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("an open criterion"))
		})

		It("does not write goal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})

		It("reports the open item", func() {
			Expect(result.OpenItems).To(ContainElement(ContainSubstring("an open criterion")))
		})
	})

	Context("bracketed goal link blocks completion", func() {
		BeforeEach(func() {
			task := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "linked-task"},
				domain.Content(""),
			)
			task.SetGoals([]string{"[[" + goalName + "]]"})
			mockTaskStorage.ListTasksReturns([]*domain.Task{task}, nil)
		})

		It("returns an error naming the task", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("linked-task"))
		})

		It("does not write goal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("bare goal link blocks completion", func() {
		BeforeEach(func() {
			task := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "linked-task"},
				domain.Content(""),
			)
			task.SetGoals([]string{goalName})
			mockTaskStorage.ListTasksReturns([]*domain.Task{task}, nil)
		})

		It("returns an error naming the task", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("linked-task"))
		})

		It("does not write goal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("aliased goal link blocks completion", func() {
		BeforeEach(func() {
			task := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "linked-task"},
				domain.Content(""),
			)
			task.SetGoals([]string{"[[" + goalName + "|label]]"})
			mockTaskStorage.ListTasksReturns([]*domain.Task{task}, nil)
		})

		It("returns an error naming the task", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("linked-task"))
		})

		It("does not write goal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("heading-suffixed goal link blocks completion", func() {
		BeforeEach(func() {
			task := domain.NewTask(
				map[string]any{"status": "todo"},
				domain.FileMetadata{Name: "linked-task"},
				domain.Content(""),
			)
			task.SetGoals([]string{"[[" + goalName + "#heading]]"})
			mockTaskStorage.ListTasksReturns([]*domain.Task{task}, nil)
		})

		It("returns an error naming the task", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("linked-task"))
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

	Context("exempt open task does not block completion", func() {
		BeforeEach(func() {
			tasks := []*domain.Task{
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "next", "goal_close_exempt": true},
						domain.FileMetadata{Name: "repair-row"},
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

		It("does not name the exempt task in a refusal", func() {
			Expect(result.OpenItems).To(BeEmpty())
		})

		It("writes goal as completed", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusCompleted))
		})
	})

	Context("exempt rows do not mask a genuine open task", func() {
		BeforeEach(func() {
			tasks := []*domain.Task{
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "next", "goal_close_exempt": true},
						domain.FileMetadata{Name: "repair-row"},
						domain.Content(""),
					)
					t.SetGoals([]string{goalName})
					return t
				}(),
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "in_progress"},
						domain.FileMetadata{Name: "re-check"},
						domain.Content(""),
					)
					t.SetGoals([]string{goalName})
					return t
				}(),
			}
			mockTaskStorage.ListTasksReturns(tasks, nil)
		})

		It("refuses and names only the genuine task", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("cannot complete goal"))
			Expect(err.Error()).To(ContainSubstring("re-check"))
			Expect(err.Error()).NotTo(ContainSubstring("repair-row"))
			Expect(result.OpenItems).To(HaveLen(1))
		})

		It("does not write goal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("marker on a task linked to another goal does not affect this goal", func() {
		BeforeEach(func() {
			tasks := []*domain.Task{
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "next", "goal_close_exempt": true},
						domain.FileMetadata{Name: "other-exempt"},
						domain.Content(""),
					)
					t.SetGoals([]string{"other-goal"})
					return t
				}(),
				func() *domain.Task {
					t := domain.NewTask(
						map[string]any{"status": "todo"},
						domain.FileMetadata{Name: "own-task"},
						domain.Content(""),
					)
					t.SetGoals([]string{goalName})
					return t
				}(),
			}
			mockTaskStorage.ListTasksReturns(tasks, nil)
		})

		It("still refuses on the unmarked task of this goal", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("own-task"))
			Expect(err.Error()).NotTo(ContainSubstring("other-exempt"))
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
				completeGoalContent("active"),
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

		It("persists phase done", func() {
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Phase()).NotTo(BeNil())
			Expect(*writtenGoal.Phase()).To(Equal(domain.GoalPhaseDone))
		})
	})

	Context("one-step close-out writes reason and successor with the status", func() {
		BeforeEach(func() {
			goal = domain.NewGoal(
				map[string]any{"status": "active"},
				domain.FileMetadata{Name: goalName},
				completeGoalContent("active"),
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
			Expect(writtenGoal.Phase()).NotTo(BeNil())
			Expect(*writtenGoal.Phase()).To(Equal(domain.GoalPhaseDone))
		})
	})

	Context("goal at phase planning completes to phase done", func() {
		BeforeEach(func() {
			goal = domain.NewGoal(
				map[string]any{"status": "in_progress", "phase": "planning"},
				domain.FileMetadata{Name: goalName},
				completeGoalContent("in_progress"),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		})

		It("writes once with completed status and done phase", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusCompleted))
			Expect(writtenGoal.Phase()).NotTo(BeNil())
			Expect(*writtenGoal.Phase()).To(Equal(domain.GoalPhaseDone))
		})
	})

	Context("goal at phase execution completes to phase done", func() {
		BeforeEach(func() {
			goal = domain.NewGoal(
				map[string]any{"status": "in_progress", "phase": "execution"},
				domain.FileMetadata{Name: goalName},
				completeGoalContent("in_progress"),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		})

		It("writes once with completed status and done phase", func() {
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusCompleted))
			Expect(writtenGoal.Phase()).NotTo(BeNil())
			Expect(*writtenGoal.Phase()).To(Equal(domain.GoalPhaseDone))
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

		It("writes goal with done phase", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Phase()).NotTo(BeNil())
			Expect(*writtenGoal.Phase()).To(Equal(domain.GoalPhaseDone))
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

	Context("parent cascade", func() {
		Context("goal with no topics", func() {
			It("does not cascade", func() {
				Expect(mockTopicCompleteOperation.ExecuteCallCount()).To(Equal(0))
				Expect(result.Parents).To(BeEmpty())
			})
		})

		Context("goal declaring a topic", func() {
			BeforeEach(func() {
				mockGoalStorage.FindGoalByNameReturns(
					goalWithTopics("active", []string{"My Topic"}),
					nil,
				)
			})

			It("cascades to the declared topic", func() {
				Expect(mockTopicCompleteOperation.ExecuteCallCount()).To(Equal(1))
				ctxArg, vaultPathArg, topicNameArg, vaultNameArg :=
					mockTopicCompleteOperation.ExecuteArgsForCall(0)
				Expect(ctxArg).To(Equal(ctx))
				Expect(vaultPathArg).To(Equal(vaultPath))
				Expect(topicNameArg).To(Equal("My Topic"))
				Expect(vaultNameArg).To(Equal(vaultName))
				Expect(result.Parents).To(HaveLen(1))
				Expect(result.Parents[0].Result).To(Equal(ops.ParentResultCompleted))
			})
		})

		Context("topic refuses", func() {
			BeforeEach(func() {
				mockGoalStorage.FindGoalByNameReturns(
					goalWithTopics("active", []string{"My Topic"}),
					nil,
				)
				mockTopicCompleteOperation.ExecuteReturns(
					ops.MutationResult{Success: false, OpenItems: []string{"gate item"}},
					ErrTest,
				)
			})

			It("reports a refused topic and still completes the goal", func() {
				Expect(err).To(BeNil())
				Expect(result.Success).To(BeTrue())
				Expect(result.Parents).To(HaveLen(1))
				Expect(result.Parents[0].Result).To(Equal(ops.ParentResultRefused))
				Expect(result.Parents[0].OpenItems).To(Equal([]string{"gate item"}))
				Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			})
		})

		Context("topic declared as a wikilink", func() {
			BeforeEach(func() {
				mockGoalStorage.FindGoalByNameReturns(
					goalWithTopics("active", []string{"[[My Topic|label]]"}),
					nil,
				)
			})

			It("normalises the wikilink topic name", func() {
				Expect(mockTopicCompleteOperation.ExecuteCallCount()).To(Equal(1))
				_, _, topicNameArg, _ := mockTopicCompleteOperation.ExecuteArgsForCall(0)
				Expect(topicNameArg).To(Equal("My Topic"))
			})
		})

		Context("goal already completed", func() {
			BeforeEach(func() {
				mockGoalStorage.FindGoalByNameReturns(
					goalWithTopics("completed", []string{"My Topic"}),
					nil,
				)
			})

			It("cascades from an already-completed goal", func() {
				Expect(mockTopicCompleteOperation.ExecuteCallCount()).To(Equal(1))
				Expect(result.Parents).To(HaveLen(1))
				Expect(result.Parents[0].Result).To(Equal(ops.ParentResultCompleted))
				Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
			})
		})

		Context("goal refuses", func() {
			BeforeEach(func() {
				mockGoalStorage.FindGoalByNameReturns(
					domain.NewGoal(
						map[string]any{"status": "in_progress", "topics": []string{"My Topic"}},
						domain.FileMetadata{Name: goalName},
						domain.Content(
							"---\nstatus: in_progress\n---\n# Success Criteria\n- [ ] an open criterion\n"+
								"# Definition of Done\n- [x] done\n",
						),
					),
					nil,
				)
			})

			It("does not cascade", func() {
				Expect(err).NotTo(BeNil())
				Expect(mockTopicCompleteOperation.ExecuteCallCount()).To(Equal(0))
			})
		})
	})
})
