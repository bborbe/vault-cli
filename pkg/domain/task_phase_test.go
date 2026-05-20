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
			Entry("execution", domain.TaskPhaseExecution),
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
			Expect(domain.AvailableTaskPhases.Contains(domain.TaskPhaseExecution)).To(BeTrue())
		})

		It("returns false for invalid phases", func() {
			Expect(domain.AvailableTaskPhases.Contains(domain.TaskPhase("invalid"))).To(BeFalse())
			Expect(domain.AvailableTaskPhases.Contains(domain.TaskPhase(""))).To(BeFalse())
		})

		It("excludes alias phases from canonical set", func() {
			Expect(domain.AvailableTaskPhases.Contains(domain.TaskPhaseInProgress)).To(BeFalse())
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

var _ = Describe("NormalizeTaskPhase", func() {
	var ctx context.Context
	BeforeEach(func() {
		ctx = context.Background()
		_ = ctx
	})

	Context("canonical values round-trip", func() {
		DescribeTable("returns the canonical value unchanged",
			func(raw string, expected domain.TaskPhase) {
				phase, ok := domain.NormalizeTaskPhase(raw)
				Expect(ok).To(BeTrue())
				Expect(phase).To(Equal(expected))
			},
			Entry("todo", "todo", domain.TaskPhaseTodo),
			Entry("planning", "planning", domain.TaskPhasePlanning),
			Entry("execution", "execution", domain.TaskPhaseExecution),
			Entry("ai_review", "ai_review", domain.TaskPhaseAIReview),
			Entry("human_review", "human_review", domain.TaskPhaseHumanReview),
			Entry("done", "done", domain.TaskPhaseDone),
		)
	})

	Context("alias values", func() {
		It("normalizes in_progress to execution", func() {
			phase, ok := domain.NormalizeTaskPhase("in_progress")
			Expect(ok).To(BeTrue())
			Expect(phase).To(Equal(domain.TaskPhaseExecution))
		})
	})

	Context("invalid values", func() {
		It("returns false for garbage", func() {
			phase, ok := domain.NormalizeTaskPhase("garbage")
			Expect(ok).To(BeFalse())
			Expect(phase).To(Equal(domain.TaskPhase("")))
		})

		It("returns false for empty string", func() {
			phase, ok := domain.NormalizeTaskPhase("")
			Expect(ok).To(BeFalse())
			Expect(phase).To(Equal(domain.TaskPhase("")))
		})
	})
})
