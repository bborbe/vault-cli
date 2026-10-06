// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("baseStorage map methods", func() {
	var (
		ctx context.Context
		b   *storage.BaseStorageForTest
	)

	BeforeEach(func() {
		ctx = context.Background()
		b = storage.NewBaseStorageForTest()
	})

	Describe("parseToFrontmatterMap", func() {
		Context("with valid frontmatter", func() {
			It("returns expected map entries", func() {
				content := []byte("---\nstatus: todo\npage_type: task\n---\n# Body\n")
				m, err := storage.ParseToFrontmatterMapForTest(ctx, b, content)
				Expect(err).To(BeNil())
				Expect(m["status"]).To(Equal("todo"))
				Expect(m["page_type"]).To(Equal("task"))
			})
		})

		Context("with an unknown field", func() {
			It("preserves the unknown field in the map", func() {
				content := []byte("---\nstatus: todo\nunknown_field: somevalue\n---\n")
				m, err := storage.ParseToFrontmatterMapForTest(ctx, b, content)
				Expect(err).To(BeNil())
				Expect(m["unknown_field"]).To(Equal("somevalue"))
			})
		})

		Context("with no frontmatter block", func() {
			It("returns an error", func() {
				content := []byte("# Just a markdown file\n\nNo frontmatter here.\n")
				_, err := storage.ParseToFrontmatterMapForTest(ctx, b, content)
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Describe("serializeMapAsFrontmatter", func() {
		Context("with a simple map", func() {
			It("produces --- wrapped YAML block", func() {
				data := map[string]any{"status": "todo"}
				result, err := storage.SerializeMapAsFrontmatterForTest(ctx, b, data, "")
				Expect(err).To(BeNil())
				Expect(result).To(HavePrefix("---\n"))
				Expect(result).To(ContainSubstring("status: todo"))
				Expect(result).To(ContainSubstring("\n---\n"))
			})
		})

		Context("with originalContent containing a body", func() {
			It("preserves the markdown body", func() {
				orig := "---\nstatus: old\n---\n# My Body\n\nSome content.\n"
				data := map[string]any{"status": "done"}
				result, err := storage.SerializeMapAsFrontmatterForTest(ctx, b, data, orig)
				Expect(err).To(BeNil())
				Expect(result).To(ContainSubstring("# My Body"))
				Expect(result).To(ContainSubstring("Some content."))
			})
		})

		Context("round-trip", func() {
			It("re-parses to the same map", func() {
				original := "---\nstatus: todo\npage_type: task\n---\n# Body\n"
				parsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(original))
				Expect(err).To(BeNil())

				serialized, err := storage.SerializeMapAsFrontmatterForTest(
					ctx,
					b,
					parsed,
					original,
				)
				Expect(err).To(BeNil())

				reparsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(serialized))
				Expect(err).To(BeNil())

				Expect(reparsed["status"]).To(Equal(parsed["status"]))
				Expect(reparsed["page_type"]).To(Equal(parsed["page_type"]))
			})
		})
	})
})

var _ = Describe("bare Wikilink quoting on the parse path", func() {
	var (
		ctx context.Context
		b   *storage.BaseStorageForTest
	)

	BeforeEach(func() {
		ctx = context.Background()
		b = storage.NewBaseStorageForTest()
	})

	// wrap builds a full markdown document from a frontmatter block. The body
	// deliberately contains a bare wikilink so every spec also proves the pass
	// never reaches past the closing fence.
	wrap := func(frontmatter string) string {
		return "---\n" + frontmatter + "\n---\nTags: [[Task]]\n\n---\nbody\n"
	}

	parse := func(frontmatter string) (map[string]any, error) {
		return storage.ParseToFrontmatterMapForTest(ctx, b, []byte(wrap(frontmatter)))
	}

	// roundTrip parses a frontmatter block and re-serializes it, returning the
	// full document as it would be written back to disk.
	roundTrip := func(frontmatter string) string {
		original := wrap(frontmatter)
		parsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(original))
		Expect(err).To(BeNil())
		serialized, err := storage.SerializeMapAsFrontmatterForTest(ctx, b, parsed, original)
		Expect(err).To(BeNil())
		return serialized
	}

	It("reads a bare wikilink scalar as a string", func() {
		m, err := parse("related_task: [[Some Other Task]]")
		Expect(err).To(BeNil())
		Expect(m).To(HaveKeyWithValue("related_task", "[[Some Other Task]]"))
	})

	It("writes a bare wikilink scalar back as a quoted scalar", func() {
		out := roundTrip("priority: 2\nrelated_task: [[Some Other Task]]\nstatus: in_progress")
		Expect(out).To(ContainSubstring("\nrelated_task: '[[Some Other Task]]'\n"))
		Expect(out).NotTo(MatchRegexp(`(?m)^ *- - `))
	})

	It("keeps a bare wikilink under a list-style key as a scalar", func() {
		out := roundTrip("themes: [[A Theme]]")
		Expect(out).To(ContainSubstring("\nthemes: '[[A Theme]]'\n"))
		Expect(out).NotTo(ContainSubstring("themes:\n"))
	})

	It("reads a bare wikilink block-sequence entry as a string", func() {
		m, err := parse("themes:\n    - [[A Theme]]\n    - [[B Theme]]")
		Expect(err).To(BeNil())
		Expect(m).To(HaveKeyWithValue("themes", []any{"[[A Theme]]", "[[B Theme]]"}))
	})

	It("writes a bare wikilink block-sequence entry back as a quoted entry", func() {
		out := roundTrip("themes:\n    - [[A Theme]]")
		Expect(out).To(ContainSubstring("\n    - '[[A Theme]]'\n"))
		Expect(out).NotTo(MatchRegexp(`(?m)^ *- - `))
	})

	It("leaves an already single-quoted wikilink byte-identical", func() {
		out := roundTrip("related_task: '[[Some Task]]'")
		Expect(out).To(ContainSubstring("\nrelated_task: '[[Some Task]]'\n"))
		Expect(out).NotTo(ContainSubstring("星期一"))
	})

	It("does not re-quote an already double-quoted wikilink", func() {
		m, err := parse(`related_task: "[[Some Task]]"`)
		Expect(err).To(BeNil())
		Expect(m).To(HaveKeyWithValue("related_task", "[[Some Task]]"))
	})

	It("escapes a single quote in the wikilink title", func() {
		out := roundTrip("related_task: [[Ben's Task]]")
		Expect(out).To(ContainSubstring("\nrelated_task: '[[Ben''s Task]]'\n"))

		reparsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(out))
		Expect(err).To(BeNil())
		Expect(reparsed).To(HaveKeyWithValue("related_task", "[[Ben's Task]]"))
	})

	It("preserves an alias wikilink verbatim", func() {
		out := roundTrip("related_task: [[X|alias]]")
		Expect(out).To(ContainSubstring("[[X|alias]]"))

		reparsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(out))
		Expect(err).To(BeNil())
		Expect(reparsed).To(HaveKeyWithValue("related_task", "[[X|alias]]"))
	})

	It("preserves a heading-anchor wikilink verbatim", func() {
		out := roundTrip("related_task: [[X#Section]]")
		Expect(out).To(ContainSubstring("[[X#Section]]"))

		reparsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(out))
		Expect(err).To(BeNil())
		Expect(reparsed).To(HaveKeyWithValue("related_task", "[[X#Section]]"))
	})

	It("leaves a value that merely contains a wikilink unchanged", func() {
		out := roundTrip("title: see [[X]] for details")
		Expect(out).To(ContainSubstring("\ntitle: see [[X]] for details\n"))

		reparsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(out))
		Expect(err).To(BeNil())
		Expect(reparsed).To(HaveKeyWithValue("title", "see [[X]] for details"))
	})

	It("does not quote a line holding two wikilinks", func() {
		_, err := parse("related: [[A]] and [[B]]")
		Expect(err).NotTo(BeNil())
	})

	It("leaves a wikilink carrying a trailing comment unchanged", func() {
		out := roundTrip("related_task: [[X]]  # note")
		Expect(out).To(MatchRegexp(`(?m)^ *- - X$`))
	})

	It("leaves a wikilink inside a block scalar unchanged", func() {
		m, err := parse("description: |\n    related_task: [[X]]\n\n    second line\nother: [[Y]]")
		Expect(err).To(BeNil())
		Expect(m).To(HaveKeyWithValue("description", "related_task: [[X]]\n\nsecond line\n"))
		Expect(m).To(HaveKeyWithValue("other", "[[Y]]"))
	})

	It("quotes a bare wikilink nested under a parent key", func() {
		m, err := parse("nested:\n    inner: [[Deep Link]]")
		Expect(err).To(BeNil())
		Expect(m).To(HaveKeyWithValue("nested", map[string]any{"inner": "[[Deep Link]]"}))
	})

	It("leaves an already-destroyed nested list unchanged", func() {
		out := roundTrip("related_task:\n    - - Some Other Task")
		Expect(out).To(MatchRegexp(`(?m)^ *- - Some Other Task$`))
	})

	It("quotes a wikilink title containing square brackets", func() {
		out := roundTrip("related_task: [[Foo [bar]]]")
		Expect(out).To(ContainSubstring("\nrelated_task: '[[Foo [bar]]]'\n"))

		reparsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(out))
		Expect(err).To(BeNil())
		Expect(reparsed).To(HaveKeyWithValue("related_task", "[[Foo [bar]]]"))
	})

	It("does not touch wikilinks in the markdown body", func() {
		out := roundTrip("related_task: [[Some Other Task]]")
		Expect(out).To(ContainSubstring("\nTags: [[Task]]\n"))
		Expect(out).To(HaveSuffix("\n---\nbody\n"))
	})

	It("is idempotent across two write cycles", func() {
		first := roundTrip(
			"priority: 2\nrelated_task: [[Some Other Task]]\nthemes:\n    - [[A Theme]]",
		)
		parsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(first))
		Expect(err).To(BeNil())
		second, err := storage.SerializeMapAsFrontmatterForTest(ctx, b, parsed, first)
		Expect(err).To(BeNil())
		Expect(second).To(Equal(first))
		Expect(strings.Count(second, "related_task: '[[Some Other Task]]'")).To(Equal(1))
	})

	It("leaves frontmatter without wikilinks unchanged", func() {
		out := roundTrip("priority: 2\nstatus: in_progress")
		Expect(out).To(ContainSubstring("\npriority: 2\n"))
		Expect(out).To(ContainSubstring("\nstatus: in_progress\n"))
	})

	It("still reports a pre-existing YAML syntax error", func() {
		_, err := parse("status: in_progress\n  bad: [unclosed")
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("unmarshal yaml frontmatter"))
	})
})

// indexesAndTexts projects parsed items onto the domain shape the API exposes,
// so the table asserts the reader's contract — order, 1-based index, text —
// without restating every item's physical line.
func indexesAndTexts(items []storage.OpenQuestionItem) []domain.OpenQuestion {
	out := make([]domain.OpenQuestion, 0, len(items))
	for _, item := range items {
		out = append(out, domain.OpenQuestion{Index: item.Index, Text: item.Text})
	}
	return out
}

var _ = Describe("storage ParseOpenQuestions", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("returns the section's top-level items in section order",
		func(content string, expected []domain.OpenQuestion) {
			Expect(indexesAndTexts(storage.ParseOpenQuestions(ctx, content))).To(Equal(expected))
		},
		Entry("several dash bullets",
			"---\nstatus: todo\n---\n# Open Questions\n\n- First?\n- Second?\n- Third?\n",
			[]domain.OpenQuestion{
				{Index: 1, Text: "First?"},
				{Index: 2, Text: "Second?"},
				{Index: 3, Text: "Third?"},
			},
		),
		Entry("numbered items keep their own marker out of the text",
			"# Open Questions\n\n1. Alpha?\n2. Beta?\n",
			[]domain.OpenQuestion{
				{Index: 1, Text: "Alpha?"},
				{Index: 2, Text: "Beta?"},
			},
		),
		Entry("a section under a ## heading",
			"## Open Questions\n\n- Only?\n",
			[]domain.OpenQuestion{{Index: 1, Text: "Only?"}},
		),
		Entry("the section ends at the next heading of the same level",
			"# Open Questions\n\n- One?\n- Two?\n\n# Progress\n\n- not a question\n",
			[]domain.OpenQuestion{
				{Index: 1, Text: "One?"},
				{Index: 2, Text: "Two?"},
			},
		),
		Entry("the section ends at a higher-level heading",
			"### Open Questions\n\n- One?\n\n## Progress\n\n- not a question\n",
			[]domain.OpenQuestion{{Index: 1, Text: "One?"}},
		),
		Entry("a nested item is not a top-level item",
			"# Open Questions\n\n- Parent?\n  - Nested\n",
			[]domain.OpenQuestion{{Index: 1, Text: "Parent?"}},
		),
		Entry("a blank item is skipped and does not consume an index",
			"# Open Questions\n\n- First?\n- \n- Third?\n",
			[]domain.OpenQuestion{
				{Index: 1, Text: "First?"},
				{Index: 2, Text: "Third?"},
			},
		),
		Entry("an existing answer is part of the item text",
			"# Open Questions\n\n- First? → **Yes**\n",
			[]domain.OpenQuestion{{Index: 1, Text: "First? → **Yes**"}},
		),
		Entry("only the first Open Questions heading is read",
			"# Open Questions\n\n- First?\n\n# Open Questions\n\n- Second?\n",
			[]domain.OpenQuestion{{Index: 1, Text: "First?"}},
		),
		// A CRLF file leaves a trailing \r on every line split on "\n". Go's `.`
		// matches \r and `$` without (?m) is end-of-text, so an unstripped capture
		// would compare "Open Questions\r" against "Open Questions", never match,
		// and silently report a section-less task — no error, an empty list.
		Entry("a CRLF file parses the same as an LF one",
			"# Open Questions\r\n\r\n- First?\r\n- Second?\r\n",
			[]domain.OpenQuestion{
				{Index: 1, Text: "First?"},
				{Index: 2, Text: "Second?"},
			},
		),
	)

	It("reports the physical line each item occupies", func() {
		// The rewriter addresses items by this line rather than re-deriving the
		// section, so the value is part of the contract, not an implementation
		// detail: 0=# Summary, 1=blank, 2=# Open Questions, 3=blank, 4/5=the items.
		items := storage.ParseOpenQuestions(
			ctx,
			"# Summary\n\n# Open Questions\n\n- First?\n- Second?\n",
		)
		Expect(items).To(HaveLen(2))
		Expect(items[0].Line).To(Equal(4))
		Expect(items[1].Line).To(Equal(5))
	})

	It("returns an empty, non-nil slice when the section is absent", func() {
		questions := storage.ParseOpenQuestions(
			ctx,
			"# Summary\n\n- not a question\n\n# Progress\n",
		)
		Expect(questions).NotTo(BeNil())
		Expect(questions).To(BeEmpty())
	})

	It("returns an empty, non-nil slice for empty content", func() {
		questions := storage.ParseOpenQuestions(ctx, "")
		Expect(questions).NotTo(BeNil())
		Expect(questions).To(BeEmpty())
	})
})
