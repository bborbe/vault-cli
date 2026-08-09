// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("Decision", func() {
	Describe("YAML marshaling", func() {
		Context("with all fields set", func() {
			var (
				decision domain.Decision
				data     []byte
				err      error
			)

			BeforeEach(func() {
				decision = domain.Decision{
					NeedsReview: true,
					Reviewed:    true,
					// ReviewedDate is managed by the storage layer (yaml:"-"); not set here
					Status:   "approved",
					Type:     "architecture",
					PageType: "decision",
					Name:     "10 Decisions/Some Page Name",
					Content:  "---\nneeds_review: true\n---\nsome content",
					FilePath: "/vault/10 Decisions/Some Page Name.md",
				}
				data, err = yaml.Marshal(decision)
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("marshals frontmatter fields", func() {
				Expect(string(data)).To(ContainSubstring("needs_review: true"))
				Expect(string(data)).To(ContainSubstring("reviewed: true"))
				Expect(string(data)).To(ContainSubstring("status: approved"))
				Expect(string(data)).To(ContainSubstring("type: architecture"))
				Expect(string(data)).To(ContainSubstring("page_type: decision"))
			})

			It("does not marshal reviewed_date (managed by storage layer)", func() {
				Expect(string(data)).NotTo(ContainSubstring("reviewed_date:"))
			})

			It("does not marshal metadata fields", func() {
				Expect(string(data)).NotTo(ContainSubstring("10 Decisions/Some Page Name"))
				Expect(string(data)).NotTo(ContainSubstring("some content"))
				Expect(string(data)).NotTo(ContainSubstring("/vault/"))
			})

			It("round-trips correctly for YAML-managed fields", func() {
				var result domain.Decision
				Expect(yaml.Unmarshal(data, &result)).To(Succeed())
				Expect(result.NeedsReview).To(BeTrue())
				Expect(result.Reviewed).To(BeTrue())
				Expect(result.Status).To(Equal("approved"))
				Expect(result.Type).To(Equal("architecture"))
				Expect(result.PageType).To(Equal("decision"))
			})
		})

		Context("with only needs_review set", func() {
			var (
				decision domain.Decision
				data     []byte
				err      error
			)

			BeforeEach(func() {
				decision = domain.Decision{
					NeedsReview: true,
				}
				data, err = yaml.Marshal(decision)
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("marshals needs_review", func() {
				Expect(string(data)).To(ContainSubstring("needs_review: true"))
			})

			It("omits empty optional fields", func() {
				Expect(string(data)).NotTo(ContainSubstring("reviewed:"))
				Expect(string(data)).NotTo(ContainSubstring("reviewed_date:"))
				Expect(string(data)).NotTo(ContainSubstring("status:"))
				Expect(string(data)).NotTo(ContainSubstring("type:"))
				Expect(string(data)).NotTo(ContainSubstring("page_type:"))
			})
		})
	})

	Describe("DecisionID", func() {
		It("returns the string representation", func() {
			id := domain.DecisionID("10 Decisions/Some Page Name")
			Expect(id.String()).To(Equal("10 Decisions/Some Page Name"))
		})
	})

	Describe("NewDecision", func() {
		It("projects the six managed keys onto struct fields", func() {
			d := domain.NewDecision(
				map[string]any{
					"needs_review": true,
					"reviewed":     false,
					"status":       "proposed",
					"type":         "Trading Decision Record",
					"page_type":    "decision",
				},
				"TDR",
				"content",
				"/vault/TDR.md",
			)
			Expect(d.NeedsReview).To(BeTrue())
			Expect(d.Reviewed).To(BeFalse())
			Expect(d.Status).To(Equal("proposed"))
			Expect(d.Type).To(Equal("Trading Decision Record"))
			Expect(d.PageType).To(Equal("decision"))
			Expect(d.Name).To(Equal("TDR"))
			Expect(d.Content).To(Equal("content"))
			Expect(d.FilePath).To(Equal("/vault/TDR.md"))
		})

		It("retains keys it has no field for", func() {
			d := domain.NewDecision(
				map[string]any{"selected_option": "B", "decision_confidence": "high"},
				"n",
				"c",
				"p",
			)
			Expect(d.Get("selected_option")).To(Equal("B"))
			Expect(d.RawMap()).To(HaveLen(2))
		})

		It("reads a hand-quoted reviewed value as true", func() {
			d := domain.NewDecision(map[string]any{"reviewed": "true"}, "n", "c", "p")
			Expect(d.Reviewed).To(BeTrue())
		})

		It("parses reviewed_date from a YAML date value", func() {
			d := domain.NewDecision(
				map[string]any{"reviewed_date": time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)},
				"n",
				"c",
				"p",
			)
			Expect(d.ReviewedDate).NotTo(BeNil())
			Expect(
				d.ReviewedDate.Time().UTC(),
			).To(Equal(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)))
		})

		It("leaves ReviewedDate nil when the key is absent", func() {
			d := domain.NewDecision(map[string]any{}, "n", "c", "p")
			Expect(d.ReviewedDate).To(BeNil())
		})

		It("does not emit the embedded map when the struct is marshaled", func() {
			data, err := yaml.Marshal(*domain.NewDecision(
				map[string]any{"needs_review": true, "selected_option": "B"},
				"n",
				"c",
				"p",
			))
			Expect(err).To(BeNil())
			Expect(string(data)).NotTo(ContainSubstring("frontmattermap"))
			Expect(string(data)).NotTo(ContainSubstring("selected_option"))
		})
	})
})
