// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("ParseCheckboxes", func() {
	It("returns empty slice for content with no checkboxes", func() {
		items := storage.ParseCheckboxes("# Title\n\nJust prose.")
		Expect(items).To(BeEmpty())
	})

	It("parses a checked checkbox", func() {
		items := storage.ParseCheckboxes("- [x] Done item")
		Expect(items).To(HaveLen(1))
		Expect(items[0].Checked).To(BeTrue())
		Expect(items[0].InProgress).To(BeFalse())
		Expect(items[0].Text).To(Equal("Done item"))
		Expect(items[0].Line).To(Equal(0))
	})

	It("parses an unchecked checkbox", func() {
		items := storage.ParseCheckboxes("- [ ] Pending item")
		Expect(items).To(HaveLen(1))
		Expect(items[0].Checked).To(BeFalse())
		Expect(items[0].InProgress).To(BeFalse())
		Expect(items[0].Text).To(Equal("Pending item"))
	})

	It("parses an in-progress checkbox", func() {
		items := storage.ParseCheckboxes("- [/] WIP item")
		Expect(items).To(HaveLen(1))
		Expect(items[0].Checked).To(BeFalse())
		Expect(items[0].InProgress).To(BeTrue())
		Expect(items[0].Text).To(Equal("WIP item"))
	})

	It("parses mixed states and returns correct line numbers", func() {
		content := "- [x] Done\n- [ ] Pending\n- [/] WIP"
		items := storage.ParseCheckboxes(content)
		Expect(items).To(HaveLen(3))
		Expect(items[0].Checked).To(BeTrue())
		Expect(items[0].Line).To(Equal(0))
		Expect(items[1].Checked).To(BeFalse())
		Expect(items[1].InProgress).To(BeFalse())
		Expect(items[1].Line).To(Equal(1))
		Expect(items[2].InProgress).To(BeTrue())
		Expect(items[2].Line).To(Equal(2))
	})

	It("preserves RawLine", func() {
		raw := "- [x] My task"
		items := storage.ParseCheckboxes(raw)
		Expect(items).To(HaveLen(1))
		Expect(items[0].RawLine).To(Equal(raw))
	})
})

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
