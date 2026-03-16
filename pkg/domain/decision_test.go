// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
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
					NeedsReview:  true,
					Reviewed:     true,
					ReviewedDate: "2025-06-01",
					Status:       "approved",
					Type:         "architecture",
					PageType:     "decision",
					Name:         "10 Decisions/Some Page Name",
					Content:      "---\nneeds_review: true\n---\nsome content",
					FilePath:     "/vault/10 Decisions/Some Page Name.md",
				}
				data, err = yaml.Marshal(decision)
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("marshals frontmatter fields", func() {
				Expect(string(data)).To(ContainSubstring("needs_review: true"))
				Expect(string(data)).To(ContainSubstring("reviewed: true"))
				Expect(string(data)).To(ContainSubstring("reviewed_date:"))
				Expect(string(data)).To(ContainSubstring("status: approved"))
				Expect(string(data)).To(ContainSubstring("type: architecture"))
				Expect(string(data)).To(ContainSubstring("page_type: decision"))
			})

			It("does not marshal metadata fields", func() {
				Expect(string(data)).NotTo(ContainSubstring("10 Decisions/Some Page Name"))
				Expect(string(data)).NotTo(ContainSubstring("some content"))
				Expect(string(data)).NotTo(ContainSubstring("/vault/"))
			})

			It("round-trips correctly", func() {
				var result domain.Decision
				Expect(yaml.Unmarshal(data, &result)).To(Succeed())
				Expect(result.NeedsReview).To(BeTrue())
				Expect(result.Reviewed).To(BeTrue())
				Expect(result.ReviewedDate).To(Equal("2025-06-01"))
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
})
