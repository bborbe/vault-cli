// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/bborbe/vault-cli/pkg/domain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/index"
)

// pageFilePath is the on-disk path of one page file inside a key's folder.
func pageFilePath(key index.Key, name string) string {
	return filepath.Join(key.VaultPath, key.PagesDir, name+".md")
}

var _ = Describe("Per-file write marks", func() {
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

	It("AC5(i) re-reads exactly the marked file and lists nothing", func() {
		fake.setPages(key, "Alpha", "Beta")
		Expect(os.WriteFile(pageFilePath(key, "Alpha"), []byte("x"), 0600)).To(Succeed())
		idx := newIndex(fake)
		_, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())

		fake.putPage(key, "Alpha", "# Alpha v2")
		listings := fake.lister.ListFilesCallCount()
		reads := fake.reader.ReadEntryCallCount()
		idx.MarkFileDirty(key, "Alpha")

		pages, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())
		Expect(fake.lister.ListFilesCallCount() - listings).To(Equal(0))
		Expect(fake.reader.ReadEntryCallCount() - reads).To(Equal(1))
		Expect(string(pages[0].Content)).To(Equal("# Alpha v2"))
	})

	It("AC5(i) shares one re-read between two concurrent readers", func() {
		fake.setPages(key, "Alpha", "Beta")
		Expect(os.WriteFile(pageFilePath(key, "Alpha"), []byte("x"), 0600)).To(Succeed())
		idx := newIndex(fake)
		_, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())

		fake.putPage(key, "Alpha", "# Alpha v2")
		entered, release := fake.blockReads("Alpha.md")
		reads := fake.reader.ReadEntryCallCount()
		idx.MarkFileDirty(key, "Alpha")

		results := make([][]*domain.Page, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				defer GinkgoRecover()
				pages, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
				Expect(err).To(BeNil())
				results[i] = pages
			}(i)
		}
		Eventually(entered).Should(Receive(Equal("Alpha.md")))
		Consistently(fake.reader.ReadEntryCallCount, "100ms").Should(Equal(reads + 1))

		release("Alpha.md", 0)
		wg.Wait()

		Expect(fake.reader.ReadEntryCallCount() - reads).To(Equal(1))
		for i := range results {
			Expect(string(results[i][0].Content)).To(Equal("# Alpha v2"))
		}
	})

	It("AC5(i) strips a [[wikilink]] wrapper from the id", func() {
		fake.setPages(key, "Alpha", "Beta")
		Expect(os.WriteFile(pageFilePath(key, "Alpha"), []byte("x"), 0600)).To(Succeed())
		idx := newIndex(fake)
		_, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())

		fake.putPage(key, "Alpha", "# Alpha v2")
		listings := fake.lister.ListFilesCallCount()
		reads := fake.reader.ReadEntryCallCount()
		idx.MarkFileDirty(key, "[[Alpha]]")

		pages, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())
		Expect(fake.lister.ListFilesCallCount() - listings).To(Equal(0))
		Expect(fake.reader.ReadEntryCallCount() - reads).To(Equal(1))
		Expect(string(pages[0].Content)).To(Equal("# Alpha v2"))
	})

	It("AC6(d) blocks a concurrent reader until the marked file is re-read", func() {
		fake.setPages(key, "Alpha", "Beta")
		Expect(os.WriteFile(pageFilePath(key, "Alpha"), []byte("x"), 0600)).To(Succeed())
		idx := newIndex(fake)
		_, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())

		fake.putPage(key, "Alpha", "# Alpha v2")
		entered, release := fake.blockReads("Alpha.md")
		idx.MarkFileDirty(key, "Alpha")

		triggered := make(chan []*domain.Page, 1)
		go func() {
			defer GinkgoRecover()
			pages, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
			Expect(err).To(BeNil())
			triggered <- pages
		}()
		Eventually(entered).Should(Receive(Equal("Alpha.md")))

		concurrent := make(chan []*domain.Page, 1)
		go func() {
			defer GinkgoRecover()
			pages, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
			Expect(err).To(BeNil())
			concurrent <- pages
		}()
		Consistently(concurrent, "150ms").ShouldNot(Receive())

		release("Alpha.md", 0)
		Expect(string((<-triggered)[0].Content)).To(Equal("# Alpha v2"))
		Expect(string((<-concurrent)[0].Content)).To(Equal("# Alpha v2"))
	})

	It("re-reads a file marked again while its read was in flight", func() {
		fake.setPages(key, "Alpha", "Beta")
		Expect(os.WriteFile(pageFilePath(key, "Alpha"), []byte("x"), 0600)).To(Succeed())
		idx := newIndex(fake)
		_, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())

		fake.putPage(key, "Alpha", "# Alpha v2")
		entered, release := fake.blockReads("Alpha.md")
		reads := fake.reader.ReadEntryCallCount()
		idx.MarkFileDirty(key, "Alpha")

		triggered := make(chan []*domain.Page, 1)
		go func() {
			defer GinkgoRecover()
			pages, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
			Expect(err).To(BeNil())
			triggered <- pages
		}()
		Eventually(entered).Should(Receive(Equal("Alpha.md")))

		// A second write lands while the first re-read is blocked.
		fake.putPage(key, "Alpha", "# Alpha v3")
		idx.MarkFileDirty(key, "Alpha")
		release("Alpha.md", 0)
		Expect(string((<-triggered)[0].Content)).To(Equal("# Alpha v2"))
		Expect(fake.reader.ReadEntryCallCount() - reads).To(Equal(1))

		// Stop gating so the follow-up re-read can finish.
		_, _ = fake.blockReads()
		pages, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())
		Expect(string(pages[0].Content)).To(Equal("# Alpha v3"))
		Expect(fake.reader.ReadEntryCallCount() - reads).To(Equal(2))
	})

	for _, name := range []string{"alpha", "sub/Alpha", "../Alpha", "[[]]", "Missing"} {
		name := name
		It("falls back to a folder-level mark for the id "+name, func() {
			fake.setPages(key, "Alpha", "Beta")
			idx := newIndex(fake)
			_, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
			Expect(err).To(BeNil())

			listings := fake.lister.ListFilesCallCount()
			reads := fake.reader.ReadEntryCallCount()
			idx.MarkFileDirty(key, name)

			pages, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
			Expect(err).To(BeNil())
			Expect(fake.lister.ListFilesCallCount() - listings).To(Equal(1))
			Expect(fake.reader.ReadEntryCallCount() - reads).To(Equal(0))
			Expect(titles(pages)).To(Equal([]string{"Alpha", "Beta"}))
		})
	}

	It("counts the write, rescan and reload read sites under their own reason", func() {
		fake.setPages(key, "Alpha", "Beta")
		Expect(os.WriteFile(pageFilePath(key, "Alpha"), []byte("x"), 0600)).To(Succeed())
		idx := newIndex(fake)
		_, err := idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())

		write := index.FilesReadTotal("write")
		fake.putPage(key, "Alpha", "# Alpha v2")
		idx.MarkFileDirty(key, "Alpha")
		_, err = idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())
		Expect(index.FilesReadTotal("write") - write).To(Equal(1.0))

		rescan := index.FilesReadTotal("rescan")
		fake.putPage(key, "Beta", "# Beta v2")
		Expect(idx.Refresh(ctx, key)).To(BeNil())
		Expect(index.FilesReadTotal("rescan") - rescan).To(Equal(1.0))

		reload := index.FilesReadTotal("reload")
		idx.ForceReload()
		_, err = idx.ListPages(ctx, key.VaultPath, key.PagesDir)
		Expect(err).To(BeNil())
		Expect(index.FilesReadTotal("reload") - reload).To(Equal(2.0))
	})

	It("AC5(iii) reads every file exactly once after ForceReload", func() {
		aTasks := index.NewKey("/vault-a", "24 Tasks")
		bTasks := index.NewKey("/vault-b", "24 Tasks")
		fake.setPages(aTasks, "a1", "a2", "a3")
		fake.setPages(bTasks, "b1", "b2")
		idx := newIndex(fake)
		Expect(idx.Build(ctx, []index.Key{aTasks, bTasks})).To(BeNil())

		reads := fake.reader.ReadEntryCallCount()
		idx.ForceReload()

		for _, k := range []index.Key{aTasks, bTasks} {
			_, err := idx.ListPages(ctx, k.VaultPath, k.PagesDir)
			Expect(err).To(BeNil())
		}
		// Five files across the two keys, each read exactly once.
		Expect(fake.reader.ReadEntryCallCount() - reads).To(Equal(5))
	})
})
