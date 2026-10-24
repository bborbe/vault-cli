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

var _ = Describe("PatchFrontmatter", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("with a block formatted unlike yaml.Marshal output", func() {
		// The whole point of the change: every byte outside the patched key must
		// survive. A substring assertion would pass while the block sequence, the
		// extra-spaced key and task_identifier were all re-rendered.
		It("rewrites only the named key and leaves every other byte untouched", func() {
			original := "---\n" +
				"task_identifier: abc-123\n" +
				"phase:   planning\n" +
				"themes:\n" +
				"  - \"Alpha\"\n" +
				"  - \"Beta\"\n" +
				"---\n" +
				"# Body\n"

			out, err := storage.PatchFrontmatter(ctx, map[string]any{"phase": "execution"}, original)
			Expect(err).To(BeNil())
			Expect(out).To(Equal("---\n" +
				"task_identifier: abc-123\n" +
				"phase: execution\n" +
				"themes:\n" +
				"  - \"Alpha\"\n" +
				"  - \"Beta\"\n" +
				"---\n" +
				"# Body\n"))
		})
	})

	Context("with a key absent from the block", func() {
		It("appends the key at the end of the block and leaves the body alone", func() {
			original := "---\nstatus: todo\n---\n# Body\n"

			out, err := storage.PatchFrontmatter(ctx, map[string]any{"priority": 2}, original)
			Expect(err).To(BeNil())
			Expect(out).To(Equal("---\nstatus: todo\npriority: 2\n---\n# Body\n"))
		})
	})

	Context("with a multi-line value", func() {
		It("replaces all of its indented lines rather than leaving orphans behind", func() {
			original := "---\nthemes:\n  - \"Alpha\"\n  - \"Beta\"\nstatus: todo\n---\n# Body\n"

			out, err := storage.PatchFrontmatter(
				ctx,
				map[string]any{"themes": []string{"Gamma", "Delta"}},
				original,
			)
			Expect(err).To(BeNil())
			Expect(out).To(Equal(
				"---\nthemes:\n    - Gamma\n    - Delta\nstatus: todo\n---\n# Body\n",
			))
		})
	})

	Context("with a blank line between keys", func() {
		It("keeps the blank line when a neighbouring key is patched", func() {
			original := "---\nstatus: todo\n\npriority: 2\n---\n# Body\n"

			out, err := storage.PatchFrontmatter(ctx, map[string]any{"status": "done"}, original)
			Expect(err).To(BeNil())
			Expect(out).To(Equal("---\nstatus: done\n\npriority: 2\n---\n# Body\n"))
		})
	})

	Context("with no frontmatter block", func() {
		It("returns an error rather than a rewritten string", func() {
			out, err := storage.PatchFrontmatter(
				ctx,
				map[string]any{"status": "done"},
				"# Just markdown\n\nNo frontmatter here.\n",
			)
			Expect(err).To(HaveOccurred())
			Expect(out).To(BeEmpty())
		})
	})

	Context("with a body that looks like frontmatter", func() {
		It("never touches the body", func() {
			original := "---\nstatus: todo\n---\n---\nstatus: not-real\n"

			out, err := storage.PatchFrontmatter(ctx, map[string]any{"status": "done"}, original)
			Expect(err).To(BeNil())
			Expect(out).To(Equal("---\nstatus: done\n---\n---\nstatus: not-real\n"))
		})
	})

	Context("after patching one key", func() {
		It("still parses through ParseFrontmatterMap", func() {
			original := "---\n" +
				"task_identifier: abc-123\n" +
				"phase:   planning\n" +
				"themes:\n" +
				"  - \"Alpha\"\n" +
				"  - \"Beta\"\n" +
				"---\n" +
				"# Body\n"

			out, err := storage.PatchFrontmatter(ctx, map[string]any{"phase": "execution"}, original)
			Expect(err).To(BeNil())

			parsed, err := storage.ParseFrontmatterMap(ctx, []byte(out))
			Expect(err).To(BeNil())
			Expect(parsed).To(HaveKeyWithValue("phase", "execution"))
			Expect(parsed).To(HaveKeyWithValue("task_identifier", "abc-123"))
			Expect(parsed).To(HaveKeyWithValue("themes", []any{"Alpha", "Beta"}))
		})
	})

	Context("with a value yaml.Marshal cannot encode", func() {
		It("returns an error rather than a rewritten string", func() {
			out, err := storage.PatchFrontmatter(
				ctx,
				map[string]any{"phase": func() {}},
				"---\nstatus: todo\n---\n# Body\n",
			)
			Expect(err).To(HaveOccurred())
			Expect(out).To(BeEmpty())
		})
	})
})
