// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("blocked_by list write path", func() {
	var (
		ctx             context.Context
		err             error
		vaultPath       string
		mockTaskStorage *mocks.TaskStorage
		mockGoalStorage *mocks.GoalStorage
	)

	BeforeEach(func() {
		ctx = context.Background()
		vaultPath = "/path/to/vault"
		mockTaskStorage = &mocks.TaskStorage{}
		mockGoalStorage = &mocks.GoalStorage{}
		mockTaskStorage.WriteTaskReturns(nil)
		mockGoalStorage.WriteGoalReturns(nil)
	})

	DescribeTable("task add appends one entry and preserves what is already there",
		func(stored any, value string, expected []string) {
			task := domain.NewTask(
				map[string]any{"status": "in_progress", "blocked_by": stored},
				domain.FileMetadata{Name: "Alpha"},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			addOp := ops.NewTaskListAddOperation(mockTaskStorage)
			err = addOp.Execute(ctx, vaultPath, "Alpha", "blocked_by", value)
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, written := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(written.BlockedBy()).To(Equal(expected))
		},
		Entry("absent key yields a one-entry list", nil, "[[Blocker B]]", []string{"[[Blocker B]]"}),
		Entry("appends onto a one-entry list preserving order", []any{"[[Blocker A]]"}, "[[Blocker B]]", []string{"[[Blocker A]]", "[[Blocker B]]"}),
		Entry("empty list yields a one-entry list", []any{}, "[[Blocker B]]", []string{"[[Blocker B]]"}),
	)

	DescribeTable("add is refused when the current value is a non-empty scalar",
		func(stored any) {
			task := domain.NewTask(
				map[string]any{"status": "in_progress", "blocked_by": stored},
				domain.FileMetadata{Name: "Alpha"},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			addOp := ops.NewTaskListAddOperation(mockTaskStorage)
			err = addOp.Execute(ctx, vaultPath, "Alpha", "blocked_by", "[[Blocker B]]")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("blocked_by"))
			Expect(err.Error()).To(ContainSubstring("YAML list"))
			Expect(err.Error()).To(ContainSubstring("clear"))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		},
		Entry("scalar name", "Blocker A"),
		Entry("scalar comma-joined value", "A,B"),
	)

	DescribeTable("task remove drops exactly one entry",
		func(stored any, value string, expected []string) {
			task := domain.NewTask(
				map[string]any{"status": "in_progress", "blocked_by": stored},
				domain.FileMetadata{Name: "Alpha"},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			removeOp := ops.NewTaskListRemoveOperation(mockTaskStorage)
			err = removeOp.Execute(ctx, vaultPath, "Alpha", "blocked_by", value)
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, written := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(written.BlockedBy()).To(Equal(expected))
		},
		Entry(
			"removes the second of two entries",
			[]any{"[[Blocker A]]", "[[Blocker B]]"},
			"[[Blocker B]]",
			[]string{"[[Blocker A]]"},
		),
	)

	It("remove of the only entry deletes the key", func() {
		task := domain.NewTask(
			map[string]any{"status": "in_progress", "blocked_by": []any{"[[Blocker A]]"}},
			domain.FileMetadata{Name: "Alpha"},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		removeOp := ops.NewTaskListRemoveOperation(mockTaskStorage)
		err = removeOp.Execute(ctx, vaultPath, "Alpha", "blocked_by", "[[Blocker A]]")
		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		_, written := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(written.Get("blocked_by")).To(BeNil())
		Expect(written.BlockedBy()).To(BeEmpty())
	})

	DescribeTable("goal add appends one entry and preserves what is already there",
		func(stored any, value string, expected []string) {
			goal := domain.NewGoal(
				map[string]any{"status": "next", "blocked_by": stored},
				domain.FileMetadata{Name: "Beta"},
				domain.Content(""),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
			addOp := ops.NewGoalListAddOperation(mockGoalStorage)
			err = addOp.Execute(ctx, vaultPath, "Beta", "blocked_by", value)
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, written := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(written.BlockedBy()).To(Equal(expected))
		},
		Entry(
			"appends onto a one-entry list preserving order",
			[]any{"[[Blocker E]]"},
			"[[Blocker C]]",
			[]string{"[[Blocker E]]", "[[Blocker C]]"},
		),
	)

	DescribeTable("goal remove drops exactly one entry",
		func(stored any, value string, expected []string) {
			goal := domain.NewGoal(
				map[string]any{"status": "next", "blocked_by": stored},
				domain.FileMetadata{Name: "Beta"},
				domain.Content(""),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
			removeOp := ops.NewGoalListRemoveOperation(mockGoalStorage)
			err = removeOp.Execute(ctx, vaultPath, "Beta", "blocked_by", value)
			Expect(err).To(BeNil())
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			_, written := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(written.BlockedBy()).To(Equal(expected))
		},
		Entry(
			"removes the first of two entries",
			[]any{"[[Blocker C]]", "[[Blocker E]]"},
			"[[Blocker C]]",
			[]string{"[[Blocker E]]"},
		),
	)

	DescribeTable("goal add is refused when the current value is a non-empty scalar",
		func(stored any) {
			goal := domain.NewGoal(
				map[string]any{"status": "next", "blocked_by": stored},
				domain.FileMetadata{Name: "Beta"},
				domain.Content(""),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
			addOp := ops.NewGoalListAddOperation(mockGoalStorage)
			err = addOp.Execute(ctx, vaultPath, "Beta", "blocked_by", "[[Blocker C]]")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("blocked_by"))
			Expect(err.Error()).To(ContainSubstring("YAML list"))
			Expect(err.Error()).To(ContainSubstring("clear"))
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		},
		Entry("scalar name", "Blocker E"),
		Entry("scalar comma-joined value", "E,F"),
	)

	It("goal remove is refused on a non-empty scalar and writes nothing", func() {
		goal := domain.NewGoal(
			map[string]any{"status": "next", "blocked_by": "Blocker E"},
			domain.FileMetadata{Name: "Beta"},
			domain.Content(""),
		)
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
		removeOp := ops.NewGoalListRemoveOperation(mockGoalStorage)
		err = removeOp.Execute(ctx, vaultPath, "Beta", "blocked_by", "Blocker E")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("YAML list"))
		Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
	})

	It("task remove is refused on a non-empty scalar and writes nothing", func() {
		task := domain.NewTask(
			map[string]any{"status": "in_progress", "blocked_by": "Blocker A"},
			domain.FileMetadata{Name: "Alpha"},
			domain.Content(""),
		)
		mockTaskStorage.FindTaskByNameReturns(task, nil)
		removeOp := ops.NewTaskListRemoveOperation(mockTaskStorage)
		err = removeOp.Execute(ctx, vaultPath, "Alpha", "blocked_by", "Blocker A")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("YAML list"))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
	})

	DescribeTable("the empty string is the documented clear and is not refused",
		func(stored any) {
			task := domain.NewTask(
				map[string]any{"status": "in_progress", "blocked_by": stored},
				domain.FileMetadata{Name: "Alpha"},
				domain.Content(""),
			)
			mockTaskStorage.FindTaskByNameReturns(task, nil)
			addOp := ops.NewTaskListAddOperation(mockTaskStorage)
			err = addOp.Execute(ctx, vaultPath, "Alpha", "blocked_by", "[[Blocker B]]")
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, written := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(written.BlockedBy()).To(Equal([]string{"[[Blocker B]]"}))
		},
		Entry("empty scalar", ""),
		Entry("absent key", nil),
	)
})
