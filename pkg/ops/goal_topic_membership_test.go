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

// topicWithGoals builds a closable topic whose `## Goals` section lists the
// given entries. The prose argument is placed above the Goals heading so a case
// can name a goal outside the section.
func topicWithGoals(name, prose string, goals ...string) *domain.Topic {
	content := "---\nstatus: in_progress\n---\n# " + name + "\n" + prose +
		"## Goals\n"
	for _, goal := range goals {
		content += "- [[" + goal + "]]\n"
	}
	content += "# Success Criteria\n- [x] done\n# Completion Gate\n- [x] done\n"
	return domain.NewTopic(
		map[string]any{"status": "in_progress"},
		domain.FileMetadata{Name: name},
		domain.Content(content),
	)
}

var _ = Describe("goal to topic cascade membership", func() {
	var (
		ctx          context.Context
		err          error
		result       ops.MutationResult
		op           ops.GoalCompleteOperation
		mockTopics   *mocks.TopicStorage
		mockGoals    *mocks.GoalStorage
		mockTasks    *mocks.TaskStorage
		goal         *domain.Goal
		vaultPath    string
		vaultName    string
		goalName     string
		declaredOnly *domain.Goal
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTopics = &mocks.TopicStorage{}
		mockGoals = &mocks.GoalStorage{}
		mockTasks = &mocks.TaskStorage{}

		currentDateTime := libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-11-20T12:00:00Z"))

		topicOp := ops.NewTopicCompleteOperation(mockTopics, mockGoals)
		op = ops.NewGoalCompleteOperation(
			mockGoals,
			mockTasks,
			currentDateTime,
			topicOp,
			mockTopics,
		)

		vaultPath = "/path/to/vault"
		vaultName = "test-vault"
		goalName = "my-goal"

		goal = domain.NewGoal(
			map[string]any{"status": "in_progress"},
			domain.FileMetadata{Name: goalName},
			completeGoalContent("in_progress"),
		)
		declaredOnly = domain.NewGoal(
			map[string]any{"status": "in_progress", "topics": []string{"[[declared-topic]]"}},
			domain.FileMetadata{Name: goalName},
			completeGoalContent("in_progress"),
		)

		mockGoals.FindGoalByNameReturns(goal, nil)
		mockGoals.WriteGoalReturns(nil)
		mockTasks.ListTasksReturns(nil, nil)
		mockTopics.ListTopicsReturns(nil, nil)
	})

	execute := func() {
		result, err = op.Execute(ctx, vaultPath, goalName, vaultName, false, "", "")
	}

	It("completes a topic that lists the goal in its Goals section", func() {
		topic := topicWithGoals("my-topic", "", goalName)
		mockTopics.ListTopicsReturns([]*domain.Topic{topic}, nil)
		mockTopics.FindTopicByNameReturns(topic, nil)

		execute()

		Expect(err).To(BeNil())
		Expect(mockTopics.WriteTopicCallCount()).To(Equal(1))
		_, writtenTopic := mockTopics.WriteTopicArgsForCall(0)
		Expect(writtenTopic.GetField("status")).To(Equal(domain.TopicStatusCompleted))
		Expect(result.Parents).To(HaveLen(1))
		Expect(result.Parents[0]).To(Equal(ops.ParentAttempt{
			Name:   "my-topic",
			Result: ops.ParentResultCompleted,
		}))
	})

	It("completes a topic that lists the goal as a path-qualified wikilink", func() {
		topic := topicWithGoals("my-topic", "", "24 Goals/"+goalName)
		mockTopics.ListTopicsReturns([]*domain.Topic{topic}, nil)
		mockTopics.FindTopicByNameReturns(topic, nil)

		execute()

		Expect(err).To(BeNil())
		Expect(mockTopics.WriteTopicCallCount()).To(Equal(1))
		_, writtenTopic := mockTopics.WriteTopicArgsForCall(0)
		Expect(writtenTopic.GetField("status")).To(Equal(domain.TopicStatusCompleted))
		Expect(result.Parents).To(HaveLen(1))
		Expect(result.Parents[0]).To(Equal(ops.ParentAttempt{
			Name:   "my-topic",
			Result: ops.ParentResultCompleted,
		}))
	})

	It("does not treat a goal named outside the Goals section as a member", func() {
		topic := topicWithGoals("my-topic", "This goal mentions [[my-goal]] in prose.\n")
		mockTopics.ListTopicsReturns([]*domain.Topic{topic}, nil)

		execute()

		Expect(err).To(BeNil())
		Expect(mockTopics.WriteTopicCallCount()).To(Equal(0))
		Expect(result.Parents).To(BeEmpty())
	})

	It("does not cascade when no topic lists the goal", func() {
		topic := topicWithGoals("other-topic", "", "other-goal")
		mockTopics.ListTopicsReturns([]*domain.Topic{topic}, nil)

		execute()

		Expect(err).To(BeNil())
		Expect(mockTopics.WriteTopicCallCount()).To(Equal(0))
		Expect(result.Parents).To(BeEmpty())
	})

	It("attempts declared topics before discovered ones, without duplicates", func() {
		mockGoals.FindGoalByNameReturns(declaredOnly, nil)
		declaredTopic := topicWithGoals("declared-topic", "", goalName)
		discoveredTopic := topicWithGoals("discovered-topic", "", goalName)
		mockTopics.ListTopicsReturns(
			[]*domain.Topic{discoveredTopic, declaredTopic},
			nil,
		)
		completedTopic := domain.NewTopic(
			map[string]any{"status": "completed"},
			domain.FileMetadata{Name: "any"},
			domain.Content("---\nstatus: completed\n---\n"),
		)
		mockTopics.FindTopicByNameReturns(completedTopic, nil)

		execute()

		Expect(err).To(BeNil())
		Expect(result.Parents).To(HaveLen(2))
		Expect(result.Parents[0].Name).To(Equal("declared-topic"))
		Expect(result.Parents[1].Name).To(Equal("discovered-topic"))
	})

	It("records a topic-scan failure and still completes the goal", func() {
		mockTopics.ListTopicsReturns(nil, ErrTest)

		execute()

		Expect(err).To(BeNil())
		Expect(result.Success).To(BeTrue())
		Expect(mockGoals.WriteGoalCallCount()).To(Equal(1))
		Expect(result.Parents).NotTo(BeEmpty())
		last := result.Parents[len(result.Parents)-1]
		Expect(last.Name).To(Equal("<topics scan>"))
		Expect(last.Result).To(Equal(ops.ParentResultRefused))
		Expect(last.OpenItems[0]).To(ContainSubstring("test error"))
	})
})
