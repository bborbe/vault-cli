// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"context"
	"strings"

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
			Entry("ai_review", domain.TopicPhaseAIReview),
			Entry("human_review", domain.TopicPhaseHumanReview),
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

			It("names the value and lists the accepted set", func() {
				err := domain.TopicPhase("bogus").Validate(ctx)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("bogus"))
				parts := make([]string, 0, len(domain.AvailableTopicPhases))
				for _, p := range domain.AvailableTopicPhases {
					parts = append(parts, string(p))
				}
				Expect(err.Error()).To(ContainSubstring(strings.Join(parts, ", ")))
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
			Expect(domain.AvailableTopicPhases.Contains(domain.TopicPhaseAIReview)).To(BeTrue())
			Expect(domain.AvailableTopicPhases.Contains(domain.TopicPhaseHumanReview)).To(BeTrue())
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

	Describe("AvailableTopicPhases", func() {
		It("matches the canonical task phase set element for element", func() {
			Expect(domain.AvailableTopicPhases).To(HaveLen(6))
			Expect(domain.AvailableTaskPhases).To(HaveLen(6))
			for i := range domain.AvailableTopicPhases {
				Expect(string(domain.AvailableTopicPhases[i])).
					To(Equal(string(domain.AvailableTaskPhases[i])))
			}
		})

		It("renders the accepted set comma-separated in declaration order", func() {
			parts := make([]string, 0, len(domain.AvailableTaskPhases))
			for _, p := range domain.AvailableTaskPhases {
				parts = append(parts, string(p))
			}
			Expect(domain.AvailableTopicPhases.String()).To(Equal(strings.Join(parts, ", ")))
		})
	})
})
