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
