// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("TopicPhase", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("Validate", func() {
		DescribeTable("valid topic phases",
			func(phase domain.TopicPhase) {
				Expect(phase.Validate(ctx)).To(BeNil())
			},
			Entry("todo", domain.TopicPhaseTodo),
			Entry("planning", domain.TopicPhasePlanning),
			Entry("execution", domain.TopicPhaseExecution),
			Entry("done", domain.TopicPhaseDone),
		)

		Context("invalid phase", func() {
			It("returns an error for an unknown topic phase", func() {
				phase := domain.TopicPhase("bogus")
				err := phase.Validate(ctx)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("unknown topic phase"))
			})

			It("returns an error for an empty topic phase", func() {
				phase := domain.TopicPhase("")
				err := phase.Validate(ctx)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("unknown topic phase"))
			})

			It("returns an error for the task-only phase in_progress", func() {
				phase := domain.TopicPhase("in_progress")
				err := phase.Validate(ctx)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("unknown topic phase"))
			})

			It("returns an error for the task-only phase ai_review", func() {
				phase := domain.TopicPhase("ai_review")
				err := phase.Validate(ctx)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("unknown topic phase"))
			})
		})
	})

	Describe("String", func() {
		It("returns the topic phase as its string value", func() {
			Expect(domain.TopicPhaseExecution.String()).To(Equal("execution"))
			Expect(domain.TopicPhasePlanning.String()).To(Equal("planning"))
			Expect(domain.TopicPhaseTodo.String()).To(Equal("todo"))
			Expect(domain.TopicPhaseDone.String()).To(Equal("done"))
		})
	})

	Describe("Ptr", func() {
		It("returns a non-nil pointer with the correct topic phase", func() {
			ptr := domain.TopicPhaseExecution.Ptr()
			Expect(ptr).NotTo(BeNil())
			Expect(*ptr).To(Equal(domain.TopicPhaseExecution))
		})

		It("returns independent topic phase copies", func() {
			p1 := domain.TopicPhaseTodo.Ptr()
			p2 := domain.TopicPhaseTodo.Ptr()
			Expect(p1).NotTo(BeIdenticalTo(p2))
		})
	})

	Describe("AvailableTopicPhases.Contains", func() {
		It("returns true for each valid topic phase", func() {
			Expect(domain.AvailableTopicPhases.Contains(domain.TopicPhaseTodo)).To(BeTrue())
			Expect(domain.AvailableTopicPhases.Contains(domain.TopicPhasePlanning)).To(BeTrue())
			Expect(domain.AvailableTopicPhases.Contains(domain.TopicPhaseExecution)).To(BeTrue())
			Expect(domain.AvailableTopicPhases.Contains(domain.TopicPhaseDone)).To(BeTrue())
		})

		It("returns false for invalid topic phases", func() {
			Expect(domain.AvailableTopicPhases.Contains(domain.TopicPhase("invalid"))).To(BeFalse())
			Expect(domain.AvailableTopicPhases.Contains(domain.TopicPhase(""))).To(BeFalse())
			Expect(
				domain.AvailableTopicPhases.Contains(domain.TopicPhase("in_progress")),
			).To(BeFalse())
		})
	})
})
