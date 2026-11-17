// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/index"
)

var _ = Describe("ChangedPagesSince", func() {
	var (
		ctx      context.Context
		fake     *storageFake
		vaultDir string
		key      index.Key
	)

	BeforeEach(func() {
		ctx = context.Background()
		fake = newStorageFake()
		vaultDir = GinkgoT().TempDir()
		Expect(os.MkdirAll(filepath.Join(vaultDir, "24 Tasks"), 0750)).To(Succeed())
		key = index.NewKey(vaultDir, "24 Tasks")
	})

	// build publishes a key holding Alpha and Beta and returns the index with
	// the revision that publication left, which is the revision a derived
	// snapshot would have recorded.
	build := func() (index.PageIndex, uint64) {
		fake.setPages(key, "Alpha", "Beta")
		for _, name := range []string{"Alpha", "Beta"} {
			Expect(os.WriteFile(pageFilePath(key, name), []byte("x"), 0600)).To(Succeed())
		}
		idx := newIndex(fake)
		_, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())
		return idx, idx.Revision(key)
	}

	DescribeTable("bounds the set of pages that moved",
		func(mutate func(idx index.PageIndex), expected []string, ok bool) {
			idx, revision := build()
			mutate(idx)

			names, got := idx.ChangedPagesSince(key, revision)
			Expect(got).To(Equal(ok))
			if !ok {
				Expect(names).To(BeEmpty())
				return
			}
			elements := make([]any, 0, len(expected))
			for _, name := range expected {
				elements = append(elements, name)
			}
			Expect(names).To(ConsistOf(elements...))
		},
		Entry("a per-file write mark reports exactly that page with its .md suffix",
			func(idx index.PageIndex) { idx.MarkFileDirty(key, "Alpha") },
			[]string{"Alpha.md"}, true),
		Entry("a per-file write mark given as a wikilink reports the same name",
			func(idx index.PageIndex) { idx.MarkFileDirty(key, "[[Alpha]]") },
			[]string{"Alpha.md"}, true),
		Entry("two per-file marks report both pages",
			func(idx index.PageIndex) {
				idx.MarkFileDirty(key, "Alpha")
				idx.MarkFileDirty(key, "Beta")
			},
			[]string{"Alpha.md", "Beta.md"}, true),
		Entry("a single-file re-read that changed the page reports it",
			func(idx index.PageIndex) {
				fake.putPage(key, "Alpha", "# Alpha v2")
				Expect(idx.RefreshFile(ctx, key, "Alpha.md")).To(Succeed())
			},
			[]string{"Alpha.md"}, true),
		Entry("a stat-diff that changed one page reports it",
			func(idx index.PageIndex) {
				fake.putPage(key, "Alpha", "# Alpha v2")
				Expect(idx.Refresh(ctx, key)).To(Succeed())
			},
			[]string{"Alpha.md"}, true),
		Entry("a stat-diff that changed nothing reports nothing",
			func(idx index.PageIndex) {
				Expect(idx.Refresh(ctx, key)).To(Succeed())
			},
			nil, true),
		Entry("an unchanged key reports nothing",
			func(index.PageIndex) {},
			nil, true),
		Entry("a folder-level mark cannot be bounded",
			func(idx index.PageIndex) { idx.MarkDirty(key) },
			nil, false),
		Entry("a forced reload cannot be bounded",
			func(idx index.PageIndex) { idx.ForceReload() },
			nil, false),
		Entry("a revision the key has advanced past its retained window cannot be placed",
			func(idx index.PageIndex) {
				for i := 0; i < 70; i++ {
					idx.MarkFileDirty(key, "Alpha")
				}
			},
			nil, false),
	)

	It("reports unknown for a key it has never seen", func() {
		idx, _ := build()
		_, ok := idx.ChangedPagesSince(index.NewKey(vaultDir, "23 Goals"), 0)
		Expect(ok).To(BeFalse())
	})

	It("reports unknown for a revision the key has never reached", func() {
		idx, revision := build()
		_, ok := idx.ChangedPagesSince(key, revision+5)
		Expect(ok).To(BeFalse())
	})

	It("reports unknown once an unbounded mark falls inside the range", func() {
		idx, revision := build()
		idx.MarkFileDirty(key, "Alpha")
		idx.MarkDirty(key)
		names, ok := idx.ChangedPagesSince(key, revision)
		Expect(ok).To(BeFalse())
		Expect(names).To(BeEmpty())
	})

	It("reports unknown after a cold build publishes the whole snapshot", func() {
		// A key's first publication is the whole snapshot, so a caller holding the
		// revision from before it (0, the value an unknown key reports) cannot be
		// told a bounded set of rows moved.
		idx, _ := build()
		names, ok := idx.ChangedPagesSince(key, 0)
		Expect(ok).To(BeFalse())
		Expect(names).To(BeEmpty())
	})
})
