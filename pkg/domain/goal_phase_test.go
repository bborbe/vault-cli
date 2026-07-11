// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("GoalPhase", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("Validate", func() {
		DescribeTable("valid phases",
			func(phase domain.GoalPhase) {
				Expect(phase.Validate(ctx)).To(BeNil())
			},
			Entry("todo", domain.GoalPhaseTodo),
			Entry("planning", domain.GoalPhasePlanning),
			Entry("execution", domain.GoalPhaseExecution),
			Entry("done", domain.GoalPhaseDone),
		)

		Context("invalid phase", func() {
			It("returns an error for unknown phase", func() {
				phase := domain.GoalPhase("bogus")
				err := phase.Validate(ctx)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("unknown goal phase"))
			})

			It("returns an error for empty phase", func() {
				phase := domain.GoalPhase("")
				err := phase.Validate(ctx)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("unknown goal phase"))
			})

			It("returns an error for task-only phase in_progress", func() {
				phase := domain.GoalPhase("in_progress")
				err := phase.Validate(ctx)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("unknown goal phase"))
			})
		})
	})

	Describe("String", func() {
		It("returns the string value", func() {
			Expect(domain.GoalPhaseExecution.String()).To(Equal("execution"))
			Expect(domain.GoalPhasePlanning.String()).To(Equal("planning"))
			Expect(domain.GoalPhaseTodo.String()).To(Equal("todo"))
			Expect(domain.GoalPhaseDone.String()).To(Equal("done"))
		})
	})

	Describe("Ptr", func() {
		It("returns a non-nil pointer with the correct value", func() {
			ptr := domain.GoalPhaseExecution.Ptr()
			Expect(ptr).NotTo(BeNil())
			Expect(*ptr).To(Equal(domain.GoalPhaseExecution))
		})

		It("returns independent copies", func() {
			p1 := domain.GoalPhaseTodo.Ptr()
			p2 := domain.GoalPhaseTodo.Ptr()
			Expect(p1).NotTo(BeIdenticalTo(p2))
		})
	})

	Describe("AvailableGoalPhases.Contains", func() {
		It("returns true for valid phases", func() {
			Expect(domain.AvailableGoalPhases.Contains(domain.GoalPhaseTodo)).To(BeTrue())
			Expect(domain.AvailableGoalPhases.Contains(domain.GoalPhasePlanning)).To(BeTrue())
			Expect(domain.AvailableGoalPhases.Contains(domain.GoalPhaseExecution)).To(BeTrue())
			Expect(domain.AvailableGoalPhases.Contains(domain.GoalPhaseDone)).To(BeTrue())
		})

		It("returns false for invalid phases", func() {
			Expect(domain.AvailableGoalPhases.Contains(domain.GoalPhase("invalid"))).To(BeFalse())
			Expect(domain.AvailableGoalPhases.Contains(domain.GoalPhase(""))).To(BeFalse())
			Expect(
				domain.AvailableGoalPhases.Contains(domain.GoalPhase("in_progress")),
			).To(BeFalse())
		})
	})
})
