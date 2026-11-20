// Copyright (c) 2026 Benjamin Borbe All rights reserved.
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

var _ = Describe("parent cascade single close path", func() {
	var (
		ctx                    context.Context
		mockTopicStorage       *mocks.TopicStorage
		mockGoalStorage        *mocks.GoalStorage
		mockTaskStorage        *mocks.TaskStorage
		mockDailyNoteStorage   *mocks.DailyNoteStorage
		mockInteractionCounter *mocks.InteractionCounter
		goalOp                 ops.GoalCompleteOperation
		completeOp             ops.CompleteOperation
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTopicStorage = &mocks.TopicStorage{}
		mockGoalStorage = &mocks.GoalStorage{}
		mockTaskStorage = &mocks.TaskStorage{}
		mockDailyNoteStorage = &mocks.DailyNoteStorage{}
		mockInteractionCounter = &mocks.InteractionCounter{}

		currentDateTime := libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))

		topicOp := ops.NewTopicCompleteOperation(mockTopicStorage, mockGoalStorage)
		goalOp = ops.NewGoalCompleteOperation(
			mockGoalStorage,
			mockTaskStorage,
			currentDateTime,
			topicOp,
			mockTopicStorage,
		)
		completeOp = ops.NewCompleteOperation(
			mockTaskStorage,
			mockDailyNoteStorage,
			currentDateTime,
			mockInteractionCounter,
			goalOp,
		)

		mockTaskStorage.WriteTaskReturns(nil)
		mockGoalStorage.WriteGoalReturns(nil)
		mockDailyNoteStorage.ReadDailyNoteReturns("", nil)
	})

	It("does not write the goal when the parent goal refuses", func() {
		task := domain.NewTask(
			map[string]any{"status": "todo", "goals": []string{"my-goal"}},
			domain.FileMetadata{Name: "my-task"},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		goal := domain.NewGoal(
			map[string]any{"status": "in_progress"},
			domain.FileMetadata{Name: "my-goal"},
			domain.Content("---\nstatus: in_progress\n---\n# Success Criteria\n"+
				"- [ ] an open criterion\n# Definition of Done\n- [x] done\n"),
		)
		mockGoalStorage.FindGoalByNameReturns(goal, nil)

		result, err := completeOp.Execute(ctx, "/vault", "my-task", "test-vault", false, "", "")

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Success).To(BeTrue())
		Expect(result.Parents).To(HaveLen(1))
		Expect(result.Parents[0].Result).To(Equal(ops.ParentResultRefused))
		Expect(result.Parents[0].OpenItems).To(ContainElement("an open criterion"))
		Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
	})

	It("does not write the topic when the parent topic refuses", func() {
		goal := domain.NewGoal(
			map[string]any{"status": "in_progress", "topics": []string{"my-topic"}},
			domain.FileMetadata{Name: "my-goal"},
			domain.Content("---\nstatus: in_progress\n---\n# Success Criteria\n"+
				"- [x] done\n# Definition of Done\n- [x] done\n"),
		)
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
		mockTaskStorage.ListTasksReturns(nil, nil)
		topic := domain.NewTopic(
			map[string]any{"status": "in_progress"},
			domain.FileMetadata{Name: "my-topic"},
			domain.Content("---\nstatus: in_progress\n---\n# Success Criteria\n"+
				"- [x] done\n# Completion Gate\n- [ ] an open gate item\n"),
		)
		mockTopicStorage.FindTopicByNameReturns(topic, nil)

		result, err := goalOp.Execute(ctx, "/vault", "my-goal", "test-vault", false, "", "")

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Success).To(BeTrue())
		Expect(result.Parents).To(HaveLen(1))
		Expect(result.Parents[0].Result).To(Equal(ops.ParentResultRefused))
		Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(0))
		Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
	})

	It("writes the parent goal exactly once on a successful close", func() {
		task := domain.NewTask(
			map[string]any{"status": "todo", "goals": []string{"my-goal"}},
			domain.FileMetadata{Name: "my-task"},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		goal := domain.NewGoal(
			map[string]any{"status": "in_progress"},
			domain.FileMetadata{Name: "my-goal"},
			domain.Content("---\nstatus: in_progress\n---\n# Success Criteria\n"+
				"- [x] done\n# Definition of Done\n- [x] done\n"),
		)
		mockGoalStorage.FindGoalByNameReturns(goal, nil)

		result, err := completeOp.Execute(ctx, "/vault", "my-task", "test-vault", false, "", "")

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Success).To(BeTrue())
		Expect(result.Parents).To(HaveLen(1))
		Expect(result.Parents[0].Result).To(Equal(ops.ParentResultCompleted))
		Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))

		_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
		Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusCompleted))
	})

	It("reports a missing parent goal as refused and still completes the task", func() {
		task := domain.NewTask(
			map[string]any{"status": "todo", "goals": []string{"my-goal"}},
			domain.FileMetadata{Name: "my-task"},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		mockGoalStorage.FindGoalByNameReturns(nil, ErrTest)

		result, err := completeOp.Execute(ctx, "/vault", "my-task", "test-vault", false, "", "")

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Success).To(BeTrue())
		Expect(result.Parents).To(HaveLen(1))
		Expect(result.Parents[0].Result).To(Equal(ops.ParentResultRefused))
		Expect(result.Parents[0].OpenItems).NotTo(BeEmpty())
		Expect(result.Parents[0].OpenItems[0]).To(ContainSubstring("test error"))
		Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
	})
})
