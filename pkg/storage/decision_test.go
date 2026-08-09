// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	libtime "github.com/bborbe/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/domain"
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

	Describe("reviewed_date round-trip", func() {
		It("reads YAML date literal as date-only DateOrDateTime", func() {
			content := "---\nneeds_review: true\nreviewed_date: 2025-01-15\n---\n# Decision\n"
			filePath := filepath.Join(vaultPath, "DateLiteral.md")
			Expect(os.WriteFile(filePath, []byte(content), 0600)).To(Succeed())

			decisions, err := store.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).To(HaveLen(1))

			d := decisions[0]
			Expect(d.ReviewedDate).NotTo(BeNil())
			t := d.ReviewedDate.Time().UTC()
			Expect(t.Format("2006-01-02")).To(Equal("2025-01-15"))
			Expect(t.Hour()).To(Equal(0))
		})

		It("reads RFC3339 string as DateOrDateTime with time component", func() {
			content := "---\nneeds_review: true\nreviewed_date: \"2025-01-15T14:30:00Z\"\n---\n# Decision\n"
			filePath := filepath.Join(vaultPath, "RFC3339.md")
			Expect(os.WriteFile(filePath, []byte(content), 0600)).To(Succeed())

			decisions, err := store.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).To(HaveLen(1))

			d := decisions[0]
			Expect(d.ReviewedDate).NotTo(BeNil())
			t := d.ReviewedDate.Time().UTC()
			Expect(t.Hour()).To(Equal(14))
			Expect(t.Minute()).To(Equal(30))
		})

		It("leaves ReviewedDate nil when reviewed_date is absent", func() {
			content := "---\nneeds_review: true\nstatus: pending\n---\n# Decision\n"
			filePath := filepath.Join(vaultPath, "NoDate.md")
			Expect(os.WriteFile(filePath, []byte(content), 0600)).To(Succeed())

			decisions, err := store.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).To(HaveLen(1))
			Expect(decisions[0].ReviewedDate).To(BeNil())
		})

		It("writes midnight-UTC DateOrDateTime as YYYY-MM-DD in frontmatter", func() {
			content := "---\nneeds_review: true\n---\n# Decision\n"
			filePath := filepath.Join(vaultPath, "WriteDate.md")
			Expect(os.WriteFile(filePath, []byte(content), 0600)).To(Succeed())

			decisions, err := store.ListDecisions(ctx, vaultPath)
			Expect(err).To(BeNil())
			Expect(decisions).To(HaveLen(1))

			d := decisions[0]
			reviewedDate := libtime.DateOrDateTime(
				time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			)
			d.ReviewedDate = &reviewedDate
			Expect(store.WriteDecision(ctx, d)).To(Succeed())

			rawBytes, err := os.ReadFile(filePath)
			Expect(err).To(BeNil())
			Expect(string(rawBytes)).To(ContainSubstring("reviewed_date: \"2025-01-15\""))
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
			reviewedDate := libtime.DateOrDateTime(
				time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			)
			d.ReviewedDate = &reviewedDate

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

	Describe("frontmatter preservation", func() {
		// splitDecisionFrontmatter returns the YAML frontmatter block and the markdown
		// body of a decision file's content.
		splitDecisionFrontmatter := func(content string) (string, string) {
			parts := strings.SplitN(content, "---\n", 3)
			Expect(parts).To(HaveLen(3))
			return parts[1], parts[2]
		}
		Context("after an ack-style write", func() {
			var (
				writtenContent  string
				parsed          map[string]any
				originalContent string
			)

			BeforeEach(func() {
				parsed = nil
				originalContent = `---
date: 2026-08-09
decision_confidence: high
decision_status: proposed
needs_review: true
page_type: decision
related:
    - '[[Some Page]]'
    - '[[Another Page]]'
related_task: '[[Some Task]]'
review_date: 2026-08-15
selected_option: B
status: proposed
supersedes: '[[Older TDR]]'
type: Trading Decision Record
unknown_count: 7
unknown_flag: true
---
# TDR 2026-08-09 - GBPJPY V6 Pause Continuation

Body text here.

## Options

Option A, Option B.
`
				filePath := filepath.Join(vaultPath, "TDR.md")
				Expect(os.WriteFile(filePath, []byte(originalContent), 0600)).To(Succeed())

				decisions, err := store.ListDecisions(ctx, vaultPath)
				Expect(err).To(BeNil())
				Expect(decisions).To(HaveLen(1))

				d := decisions[0]
				d.Reviewed = true
				reviewedDate := libtime.DateOrDateTime(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC))
				d.ReviewedDate = &reviewedDate
				d.NeedsReview = false
				Expect(store.WriteDecision(ctx, d)).To(Succeed())

				rawBytes, err := os.ReadFile(filePath)
				Expect(err).To(BeNil())
				writtenContent = string(rawBytes)

				fmYAML, _ := splitDecisionFrontmatter(writtenContent)
				Expect(yaml.Unmarshal([]byte(fmYAML), &parsed)).To(Succeed())
			})

			It("preserves every non-managed frontmatter key", func() {
				Expect(parsed).To(HaveKeyWithValue("decision_confidence", "high"))
				Expect(parsed).To(HaveKeyWithValue("decision_status", "proposed"))
				Expect(parsed).To(HaveKeyWithValue("selected_option", "B"))
				Expect(parsed).To(HaveKeyWithValue("related_task", "[[Some Task]]"))
				Expect(parsed).To(HaveKeyWithValue("supersedes", "[[Older TDR]]"))
				Expect(parsed).To(HaveKey("date"))
				Expect(parsed).To(HaveKey("review_date"))
				Expect(parsed).To(HaveKey("related"))
			})

			It("preserves date-valued keys as the same instant", func() {
				reviewVal, ok := parsed["review_date"].(time.Time)
				Expect(ok).To(BeTrue(), "review_date must stay a YAML timestamp")
				Expect(reviewVal.UTC()).To(Equal(time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)))

				dateVal, ok := parsed["date"].(time.Time)
				Expect(ok).To(BeTrue(), "date must stay a YAML timestamp")
				Expect(dateVal.UTC()).To(Equal(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)))
			})

			It("re-renders a bare date in RFC3339 form without changing the instant", func() {
				Expect(writtenContent).To(ContainSubstring("\nreview_date: 2026-08-15T00:00:00Z\n"))
				Expect(writtenContent).NotTo(ContainSubstring("review_date: \"2026-08-15\""))
			})

			It("round-trips a list-valued key as a list", func() {
				relatedVal, ok := parsed["related"].([]any)
				Expect(
					ok,
				).To(BeTrue(), "related must round-trip as a sequence, not a flattened string")
				Expect(relatedVal).To(HaveLen(2))
				Expect(relatedVal[0]).To(Equal("[[Some Page]]"))
				Expect(relatedVal[1]).To(Equal("[[Another Page]]"))
			})

			It("does not coerce an unknown key to a different YAML type", func() {
				countVal, ok := parsed["unknown_count"].(int)
				Expect(ok).To(BeTrue(), "unknown_count must round-trip as int, not string")
				Expect(countVal).To(Equal(7))

				flagVal, ok := parsed["unknown_flag"].(bool)
				Expect(ok).To(BeTrue(), "unknown_flag must round-trip as bool, not string")
				Expect(flagVal).To(BeTrue())
			})

			It("updates the six managed fields", func() {
				Expect(parsed).To(HaveKeyWithValue("needs_review", false))
				Expect(parsed).To(HaveKeyWithValue("reviewed", true))
				Expect(parsed).To(HaveKeyWithValue("reviewed_date", "2026-08-09"))
				Expect(parsed).To(HaveKeyWithValue("status", "proposed"))
				Expect(parsed).To(HaveKeyWithValue("type", "Trading Decision Record"))
				Expect(parsed).To(HaveKeyWithValue("page_type", "decision"))
			})

			It("leaves the markdown body byte-identical", func() {
				_, originalBody := splitDecisionFrontmatter(originalContent)
				_, writtenBody := splitDecisionFrontmatter(writtenContent)
				Expect(writtenBody).To(Equal(originalBody))
			})

			It("loses no frontmatter key", func() {
				Expect(parsed).To(HaveLen(16))
			})
		})

		Context("managed-value precedence", func() {
			var writtenContent string
			var parsed map[string]any

			BeforeEach(func() {
				parsed = nil
				originalContent := `---
needs_review: true
status: proposed
---
# Test
`
				filePath := filepath.Join(vaultPath, "Precedence.md")
				Expect(os.WriteFile(filePath, []byte(originalContent), 0600)).To(Succeed())

				decisions, err := store.ListDecisions(ctx, vaultPath)
				Expect(err).To(BeNil())
				Expect(decisions).To(HaveLen(1))

				d := decisions[0]
				d.Status = "accepted"
				Expect(store.WriteDecision(ctx, d)).To(Succeed())

				rawBytes, err := os.ReadFile(filePath)
				Expect(err).To(BeNil())
				writtenContent = string(rawBytes)

				fmYAML, _ := splitDecisionFrontmatter(writtenContent)
				Expect(yaml.Unmarshal([]byte(fmYAML), &parsed)).To(Succeed())
			})

			It("lets a managed value win over the preserved key of the same name", func() {
				Expect(parsed).To(HaveKeyWithValue("status", "accepted"))
				Expect(strings.Count(writtenContent, "\nstatus:")).To(Equal(1))
			})
		})

		Context("empty preserved map", func() {
			It("writes the managed keys when the preserved map is empty", func() {
				d := &domain.Decision{
					NeedsReview: true,
					Status:      "proposed",
					Name:        "Empty",
					Content:     "---\nneeds_review: true\n---\n# Empty\n\nBody.\n",
					FilePath:    filepath.Join(vaultPath, "Empty.md"),
				}
				Expect(d.RawMap()).To(BeNil())
				Expect(store.WriteDecision(ctx, d)).To(Succeed())

				rawBytes, err := os.ReadFile(d.FilePath)
				Expect(err).To(BeNil())

				fmYAML, body := splitDecisionFrontmatter(string(rawBytes))
				Expect(body).To(Equal("# Empty\n\nBody.\n"))

				var parsed map[string]any
				Expect(yaml.Unmarshal([]byte(fmYAML), &parsed)).To(Succeed())
				Expect(parsed).To(HaveLen(2))
				Expect(parsed).To(HaveKeyWithValue("needs_review", true))
				Expect(parsed).To(HaveKeyWithValue("status", "proposed"))
			})
		})

		Context("unparseable frontmatter", func() {
			It("skips a file with unparseable frontmatter and leaves it byte-identical", func() {
				malformedContent := `---
needs_review: true
  bad: [unclosed
---
# Malformed
`
				filePath := filepath.Join(vaultPath, "Malformed.md")
				Expect(os.WriteFile(filePath, []byte(malformedContent), 0600)).To(Succeed())

				decisions, err := store.ListDecisions(ctx, vaultPath)
				Expect(err).To(BeNil())

				var found bool
				for _, d := range decisions {
					if d.Name == "Malformed" {
						found = true
						break
					}
				}
				Expect(found).To(BeFalse())

				rawBytes, err := os.ReadFile(filePath)
				Expect(err).To(BeNil())
				Expect(string(rawBytes)).To(Equal(malformedContent))
			})
		})

		Context("unwritable path", func() {
			It("returns a wrapped error when the file cannot be written", func() {
				d := &domain.Decision{
					NeedsReview: true,
					Name:        "X",
					Content:     "---\nneeds_review: true\n---\n# X\n",
					FilePath:    filepath.Join(vaultPath, "no-such-dir", "X.md"),
				}
				err := store.WriteDecision(ctx, d)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("write file"))
			})
		})
	})
})
