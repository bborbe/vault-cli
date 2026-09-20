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

var _ = Describe("task set refusal for blocked_by", func() {
	var (
		ctx             context.Context
		err             error
		setOp           ops.FrontmatterSetOperation
		mockTaskStorage *mocks.TaskStorage
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		setOp = ops.NewFrontmatterSetOperation(
			mockTaskStorage,
			ops.NewEscalationPublisher("", "", &mocks.NotificationSenderFactory{}),
			"personal",
			"25 Tasks",
		)
		mockTaskStorage.FindTaskByNameReturns(
			domain.NewTask(
				map[string]any{"status": "in_progress", "blocked_by": []any{"[[Blocker A]]"}},
				domain.FileMetadata{Name: "Alpha"},
				domain.Content(""),
			),
			nil,
		)
		mockTaskStorage.WriteTaskReturns(nil)
	})

	// Execute is called from inside each body: a DescribeTable entry body is the
	// spec body, so a container-level JustBeforeEach would run it with the
	// previous row's value.
	setValue := func(k, v string) {
		err = setOp.Execute(ctx, "/vault", "Alpha", k, v, "", "", false)
	}

	DescribeTable("refuses a non-empty blocked_by and writes nothing",
		func(v string) {
			setValue("blocked_by", v)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("blocked_by"))
			Expect(err.Error()).To(ContainSubstring("add"))
			Expect(err.Error()).To(ContainSubstring("scalar"))
			Expect(err.Error()).To(ContainSubstring("clear"))
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		},
		Entry("wikilink value", "[[Blocker A]]"),
		Entry("plain name value", "Blocker A"),
		Entry("comma-joined value", "A,B"),
	)

	It("accepts the empty blocked_by value — the documented clear", func() {
		setValue("blocked_by", "")
		Expect(err).To(BeNil())
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
		_, written := mockTaskStorage.WriteTaskArgsForCall(0)
		Expect(written.Get("blocked_by")).To(Equal(""))
		Expect(written.BlockedBy()).To(BeEmpty())
	})

	It("is not bypassed by force", func() {
		err = setOp.Execute(ctx, "/vault", "Alpha", "blocked_by", "[[Blocker A]]", "", "", true)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("blocked_by"))
		Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
	})

	DescribeTable("leaves the comma-split coercion of tags and goals untouched",
		func(k, v string, assert func(written *domain.Task)) {
			setValue(k, v)
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, written := mockTaskStorage.WriteTaskArgsForCall(0)
			assert(written)
		},
		Entry("tags comma-split into a list", "tags", "a,b",
			func(written *domain.Task) {
				Expect(written.Tags()).To(Equal([]string{"a", "b"}))
			},
		),
		Entry("goals comma-split into a list", "goals", "g1,g2",
			func(written *domain.Task) {
				Expect(written.Goals()).To(Equal([]string{"g1", "g2"}))
			},
		),
		Entry("an unrelated unknown field still passes through", "custom_field", "x",
			func(written *domain.Task) {
				Expect(written.Get("custom_field")).To(Equal("x"))
			},
		),
	)
})

var _ = Describe("goal set refusal for blocked_by", func() {
	var (
		ctx             context.Context
		err             error
		setOp           ops.EntitySetOperation
		mockGoalStorage *mocks.GoalStorage
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		setOp = ops.NewGoalSetOperation(mockGoalStorage)
		mockGoalStorage.FindGoalByNameReturns(
			domain.NewGoal(
				map[string]any{"status": "next", "blocked_by": []any{"[[Blocker E]]"}},
				domain.FileMetadata{Name: "Beta"},
				domain.Content(""),
			),
			nil,
		)
		mockGoalStorage.WriteGoalReturns(nil)
	})

	setValue := func(k, v string) {
		err = setOp.Execute(ctx, "/vault", "Beta", k, v, "", "")
	}

	It("refuses a non-empty blocked_by and writes nothing", func() {
		setValue("blocked_by", "[[Blocker D]]")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("blocked_by"))
		Expect(err.Error()).To(ContainSubstring("add"))
		Expect(err.Error()).To(ContainSubstring("scalar"))
		Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
	})

	It("accepts the empty blocked_by value", func() {
		setValue("blocked_by", "")
		Expect(err).To(BeNil())
		Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
		_, written := mockGoalStorage.WriteGoalArgsForCall(0)
		Expect(written.Get("blocked_by")).To(Equal(""))
	})

	It("leaves the goal tags comma-split coercion untouched", func() {
		setValue("tags", "a,b")
		Expect(err).To(BeNil())
		Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
		_, written := mockGoalStorage.WriteGoalArgsForCall(0)
		Expect(written.Tags()).To(Equal([]string{"a", "b"}))
	})
})
