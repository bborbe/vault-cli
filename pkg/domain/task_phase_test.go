// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("TaskPhase", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("Validate", func() {
		DescribeTable("valid phases",
			func(phase domain.TaskPhase) {
				Expect(phase.Validate(ctx)).To(BeNil())
			},
			Entry("todo", domain.TaskPhaseTodo),
			Entry("planning", domain.TaskPhasePlanning),
			Entry("in_progress", domain.TaskPhaseInProgress),
			Entry("ai_review", domain.TaskPhaseAIReview),
			Entry("human_review", domain.TaskPhaseHumanReview),
			Entry("done", domain.TaskPhaseDone),
		)

		Context("invalid phase", func() {
			It("returns an error", func() {
				phase := domain.TaskPhase("unknown_phase")
				err := phase.Validate(ctx)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("unknown task phase"))
			})
		})
	})

	Describe("String", func() {
		It("returns the string value", func() {
			Expect(domain.TaskPhasePlanning.String()).To(Equal("planning"))
			Expect(domain.TaskPhaseInProgress.String()).To(Equal("in_progress"))
			Expect(domain.TaskPhaseAIReview.String()).To(Equal("ai_review"))
			Expect(domain.TaskPhaseHumanReview.String()).To(Equal("human_review"))
		})
	})

	Describe("Ptr", func() {
		It("returns a non-nil pointer with the correct value", func() {
			ptr := domain.TaskPhasePlanning.Ptr()
			Expect(ptr).NotTo(BeNil())
			Expect(*ptr).To(Equal(domain.TaskPhasePlanning))
		})

		It("returns independent copies", func() {
			p1 := domain.TaskPhaseTodo.Ptr()
			p2 := domain.TaskPhaseTodo.Ptr()
			Expect(p1).NotTo(BeIdenticalTo(p2))
		})
	})

	Describe("AvailableTaskPhases.Contains", func() {
		It("returns true for valid phases", func() {
			Expect(domain.AvailableTaskPhases.Contains(domain.TaskPhaseTodo)).To(BeTrue())
			Expect(domain.AvailableTaskPhases.Contains(domain.TaskPhaseDone)).To(BeTrue())
		})

		It("returns false for invalid phases", func() {
			Expect(domain.AvailableTaskPhases.Contains(domain.TaskPhase("invalid"))).To(BeFalse())
			Expect(domain.AvailableTaskPhases.Contains(domain.TaskPhase(""))).To(BeFalse())
		})
	})

	Describe("YAML marshal/unmarshal with *TaskPhase", func() {
		type wrapper struct {
			Phase *domain.TaskPhase `yaml:"phase,omitempty"`
		}

		Context("nil phase is omitted", func() {
			It("marshals without phase field", func() {
				w := wrapper{Phase: nil}
				data, err := yaml.Marshal(w)
				Expect(err).To(BeNil())
				Expect(string(data)).NotTo(ContainSubstring("phase"))
			})
		})

		Context("non-nil phase round-trips", func() {
			It("marshals and unmarshals correctly", func() {
				phase := domain.TaskPhaseInProgress
				w := wrapper{Phase: &phase}
				data, err := yaml.Marshal(w)
				Expect(err).To(BeNil())
				Expect(string(data)).To(ContainSubstring("in_progress"))

				var w2 wrapper
				Expect(yaml.Unmarshal(data, &w2)).To(Succeed())
				Expect(w2.Phase).NotTo(BeNil())
				Expect(*w2.Phase).To(Equal(domain.TaskPhaseInProgress))
			})
		})
	})
})
