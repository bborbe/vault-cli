// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("Decision storage", func() {
	var (
		ctx       context.Context
		store     storage.Storage
		vaultPath string
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = storage.NewStorage(nil)

		var err error
		vaultPath, err = os.MkdirTemp("", "vault-decision-test-*")
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	Describe("ListDecisions", func() {
		It("returns only files with needs_review: true", func() {
			reviewContent := `---
needs_review: true
status: pending
type: architecture
---
# Decision A

Some decision body.
`
			noReviewContent := `---
needs_review: false
status: accepted
---
# Decision B

Not pending review.
`
			Expect(
				os.WriteFile(filepath.Join(vaultPath, "DecisionA.md"), []byte(reviewContent), 0600),
			).To(Succeed())
			Expect(
				os.WriteFile(
					filepath.Join(vaultPath, "DecisionB.md"),
					[]byte(noReviewContent),
					0600,
				),
			).To(Succeed())

			decisions, err := store.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).To(HaveLen(1))
			Expect(decisions[0].Name).To(Equal("DecisionA"))
			Expect(decisions[0].NeedsReview).To(BeTrue())
		})

		It("skips files with no frontmatter (warning, no error)", func() {
			noFrontmatter := `# Just a markdown file

No frontmatter here at all.
`
			Expect(
				os.WriteFile(
					filepath.Join(vaultPath, "NoFrontmatter.md"),
					[]byte(noFrontmatter),
					0600,
				),
			).To(Succeed())

			decisions, err := store.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).To(HaveLen(0))
		})

		It("returns empty slice when no decisions exist", func() {
			decisions, err := store.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).NotTo(BeNil())
			Expect(decisions).To(HaveLen(0))
		})

		It("scans recursively into subdirectories", func() {
			subDir := filepath.Join(vaultPath, "ADR", "2024")
			Expect(os.MkdirAll(subDir, 0755)).To(Succeed())

			reviewContent := `---
needs_review: true
type: adr
---
# ADR-001

Some architectural decision.
`
			Expect(
				os.WriteFile(filepath.Join(subDir, "adr-001.md"), []byte(reviewContent), 0600),
			).To(Succeed())

			decisions, err := store.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).To(HaveLen(1))
			Expect(decisions[0].Name).To(Equal("ADR/2024/adr-001"))
		})

		It("returns error when vault path does not exist", func() {
			_, err := store.ListDecisions(ctx, "/nonexistent/vault/path")
			Expect(err).NotTo(BeNil())
		})

		It("skips files in excluded directories", func() {
			templatesDir := filepath.Join(vaultPath, "90 Templates")
			Expect(os.MkdirAll(templatesDir, 0755)).To(Succeed())

			reviewContent := `---
needs_review: true
type: template
---
# Template Decision
`
			normalContent := `---
needs_review: true
type: architecture
---
# Normal Decision
`
			Expect(
				os.WriteFile(
					filepath.Join(templatesDir, "Template.md"),
					[]byte(reviewContent),
					0600,
				),
			).To(Succeed())
			Expect(
				os.WriteFile(filepath.Join(vaultPath, "Normal.md"), []byte(normalContent), 0600),
			).To(Succeed())

			storeWithExcludes := storage.NewStorage(&storage.Config{
				Excludes: []string{"90 Templates"},
			})
			decisions, err := storeWithExcludes.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).To(HaveLen(1))
			Expect(decisions[0].Name).To(Equal("Normal"))
		})

		It("returns all files when excludes list is empty", func() {
			subDir := filepath.Join(vaultPath, "90 Templates")
			Expect(os.MkdirAll(subDir, 0755)).To(Succeed())

			reviewContent := `---
needs_review: true
type: template
---
# Template Decision
`
			Expect(
				os.WriteFile(filepath.Join(subDir, "Template.md"), []byte(reviewContent), 0600),
			).To(Succeed())

			decisions, err := store.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).To(HaveLen(1))
		})

		It("skips entire subtree when exclude matches parent directory", func() {
			parentDir := filepath.Join(vaultPath, "90 Templates")
			subDir := filepath.Join(parentDir, "sub")
			Expect(os.MkdirAll(subDir, 0755)).To(Succeed())

			reviewContent := `---
needs_review: true
type: template
---
# Nested Template
`
			Expect(
				os.WriteFile(filepath.Join(subDir, "Nested.md"), []byte(reviewContent), 0600),
			).To(Succeed())

			storeWithExcludes := storage.NewStorage(&storage.Config{
				Excludes: []string{"90 Templates"},
			})
			decisions, err := storeWithExcludes.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).To(HaveLen(0))
		})
	})

	Describe("FindDecisionByName", func() {
		BeforeEach(func() {
			content1 := `---
needs_review: true
type: architecture
---
# Alpha Decision
`
			content2 := `---
needs_review: true
type: data
---
# Beta Decision
`
			Expect(
				os.WriteFile(filepath.Join(vaultPath, "Alpha Decision.md"), []byte(content1), 0600),
			).To(Succeed())
			Expect(
				os.WriteFile(filepath.Join(vaultPath, "Beta Decision.md"), []byte(content2), 0600),
			).To(Succeed())
		})

		It("returns decision on exact match", func() {
			d, err := store.FindDecisionByName(ctx, vaultPath, "Alpha Decision")
			Expect(err).To(BeNil())
			Expect(d).NotTo(BeNil())
			Expect(d.Name).To(Equal("Alpha Decision"))
		})

		It("returns decision on single partial match", func() {
			d, err := store.FindDecisionByName(ctx, vaultPath, "alpha")
			Expect(err).To(BeNil())
			Expect(d).NotTo(BeNil())
			Expect(d.Name).To(Equal("Alpha Decision"))
		})

		It("returns error for ambiguous partial match", func() {
			// "Decision" appears in both names
			_, err := store.FindDecisionByName(ctx, vaultPath, "Decision")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("ambiguous match"))
		})

		It("returns error when not found", func() {
			_, err := store.FindDecisionByName(ctx, vaultPath, "Nonexistent")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("decision not found"))
		})

		It("returns error for name containing ..", func() {
			_, err := store.FindDecisionByName(ctx, vaultPath, "../etc/passwd")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid decision name"))
		})

		Context("with ambiguous names in different paths", func() {
			BeforeEach(func() {
				tradingDir := filepath.Join(vaultPath, "40 Trading", "Weekly")
				periodicDir := filepath.Join(vaultPath, "60 Periodic Notes", "Weekly")
				Expect(os.MkdirAll(tradingDir, 0755)).To(Succeed())
				Expect(os.MkdirAll(periodicDir, 0755)).To(Succeed())

				content1 := "---\nneeds_review: true\ntype: architecture\n---\n# Review\n"
				content2 := "---\nneeds_review: true\ntype: data\n---\n# Review\n"

				Expect(os.WriteFile(
					filepath.Join(tradingDir, "2026-W12 - Review.md"),
					[]byte(content1), 0600,
				)).To(Succeed())
				Expect(os.WriteFile(
					filepath.Join(periodicDir, "2026-W12.md"),
					[]byte(content2), 0600,
				)).To(Succeed())
			})

			It("returns ambiguous error for short name matching multiple decisions", func() {
				_, err := store.FindDecisionByName(ctx, vaultPath, "2026-W12")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("ambiguous match"))
			})

			It("resolves with full path", func() {
				d, err := store.FindDecisionByName(
					ctx,
					vaultPath,
					"40 Trading/Weekly/2026-W12 - Review",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(d.Name).To(Equal("40 Trading/Weekly/2026-W12 - Review"))
			})

			It("resolves with partial path suffix", func() {
				d, err := store.FindDecisionByName(
					ctx,
					vaultPath,
					"Trading/Weekly/2026-W12 - Review",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(d.Name).To(Equal("40 Trading/Weekly/2026-W12 - Review"))
			})

			It("resolves with partial path prefix", func() {
				d, err := store.FindDecisionByName(ctx, vaultPath, "40 Trading/Weekly/2026-W12")
				Expect(err).NotTo(HaveOccurred())
				Expect(d.Name).To(Equal("40 Trading/Weekly/2026-W12 - Review"))
			})

			It("resolves the other decision with its path", func() {
				d, err := store.FindDecisionByName(
					ctx,
					vaultPath,
					"60 Periodic Notes/Weekly/2026-W12",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(d.Name).To(Equal("60 Periodic Notes/Weekly/2026-W12"))
			})
		})
	})

	Describe("WriteDecision", func() {
		It("preserves markdown body content and only changes frontmatter", func() {
			originalContent := `---
needs_review: true
status: pending
type: architecture
---
# My Decision

This is the decision body.

## Context

Some important context.
`
			filePath := filepath.Join(vaultPath, "My Decision.md")
			Expect(os.WriteFile(filePath, []byte(originalContent), 0600)).To(Succeed())

			// Read the decision
			decisions, err := store.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).To(HaveLen(1))

			d := decisions[0]
			d.NeedsReview = false
			d.Reviewed = true
			d.ReviewedDate = "2026-03-16"

			Expect(store.WriteDecision(ctx, d)).To(Succeed())

			// Read raw file and verify body preserved
			rawBytes, err := os.ReadFile(filePath)
			Expect(err).To(BeNil())
			rawContent := string(rawBytes)

			Expect(rawContent).To(ContainSubstring("# My Decision"))
			Expect(rawContent).To(ContainSubstring("This is the decision body."))
			Expect(rawContent).To(ContainSubstring("## Context"))
			Expect(rawContent).To(ContainSubstring("Some important context."))

			// Verify frontmatter is updated
			Expect(rawContent).To(ContainSubstring("needs_review: false"))
			Expect(rawContent).To(ContainSubstring("reviewed: true"))
			Expect(rawContent).To(ContainSubstring("reviewed_date: \"2026-03-16\""))
		})
	})
})
