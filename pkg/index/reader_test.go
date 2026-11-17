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

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/index"
	"github.com/bborbe/vault-cli/pkg/storage"
)

const readerFolder = "Tasks"

var _ = Describe("PageReader", func() {
	var (
		ctx      context.Context
		vaultDir string
		reader   index.PageReader
	)

	write := func(name, content string) {
		Expect(os.WriteFile(filepath.Join(vaultDir, readerFolder, name), []byte(content), 0600)).
			To(Succeed())
	}

	BeforeEach(func() {
		ctx = context.Background()
		vaultDir = GinkgoT().TempDir()
		Expect(os.MkdirAll(filepath.Join(vaultDir, readerFolder), 0750)).To(Succeed())
		reader = index.NewPageReader(storage.NewPageStorage(nil))
	})

	It("projects open questions from ParseOpenQuestions", func() {
		body := "---\nstatus: in_progress\n---\n## Open Questions\n\n- first question\n- second question → answer\n\n## Notes\n- not a question\n"
		write("Task A.md", body)

		entry, fingerprint, err := reader.ReadEntry(ctx, vaultDir, readerFolder, "Task A.md")
		Expect(err).NotTo(HaveOccurred())
		Expect(fingerprint.Size).To(Equal(int64(len(body))))

		items, err := storage.ParseOpenQuestions(ctx, body)
		Expect(err).NotTo(HaveOccurred())
		expected := make([]domain.OpenQuestion, 0, len(items))
		for _, item := range items {
			expected = append(expected, domain.OpenQuestion{Index: item.Index, Text: item.Question})
		}
		Expect(entry.OpenQuestions).To(Equal(expected))
		Expect(entry.OpenQuestions).To(HaveLen(2))
	})

	It("returns an empty non-nil slice when the page has no Open Questions section", func() {
		write("Task B.md", "---\nstatus: next\n---\n## Notes\n- nothing\n")

		entry, _, err := reader.ReadEntry(ctx, vaultDir, readerFolder, "Task B.md")
		Expect(err).NotTo(HaveOccurred())
		Expect(entry.OpenQuestions).NotTo(BeNil())
		Expect(entry.OpenQuestions).To(BeEmpty())
	})

	It("drops the body but keeps frontmatter, name, path and modification date", func() {
		write("Task C.md", "---\nstatus: next\njira: ABC-1\n---\nbody text\n")

		entry, _, err := reader.ReadEntry(ctx, vaultDir, readerFolder, "Task C.md")
		Expect(err).NotTo(HaveOccurred())
		reference, err := storage.NewPageStorage(nil).ReadPage(ctx, vaultDir, readerFolder, "Task C")
		Expect(err).NotTo(HaveOccurred())

		Expect(string(entry.Page.Content)).To(BeEmpty())
		Expect(entry.Page.RawMap()).To(Equal(reference.RawMap()))
		Expect(entry.Page.FileMetadata).To(Equal(reference.FileMetadata))
	})

	It("reads the full page including its body on ReadPage", func() {
		body := "---\nstatus: next\n---\nfull body\n"
		write("Task D.md", body)

		page, err := reader.ReadPage(ctx, vaultDir, readerFolder, "Task D")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(page.Content)).To(Equal(body))
	})

	It("reports an error and a zero fingerprint for a missing file", func() {
		_, fingerprint, err := reader.ReadEntry(ctx, vaultDir, readerFolder, "Missing.md")
		Expect(err).To(HaveOccurred())
		Expect(fingerprint).To(Equal(index.FileFingerprint{}))
	})
})

var _ = Describe("DirectoryLister", func() {
	It("lists .md files only, skipping directories, in directory order", func() {
		ctx := context.Background()
		vaultDir := GinkgoT().TempDir()
		dir := filepath.Join(vaultDir, readerFolder)
		Expect(os.MkdirAll(filepath.Join(dir, "sub.md"), 0750)).To(Succeed())
		for _, name := range []string{"b.md", "a.md", "c.txt"} {
			Expect(os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600)).To(Succeed())
		}

		entries, err := index.NewDirectoryLister().ListFiles(ctx, vaultDir, readerFolder)
		Expect(err).NotTo(HaveOccurred())
		names := []string{}
		for _, entry := range entries {
			names = append(names, entry.Name)
			Expect(entry.Fingerprint.Size).To(Equal(int64(1)))
		}
		Expect(names).To(Equal([]string{"a.md", "b.md"}))
	})

	It("returns nil for a missing folder", func() {
		entries, err := index.NewDirectoryLister().
			ListFiles(context.Background(), GinkgoT().TempDir(), "absent")
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeNil())
	})
})

var _ = Describe("NewKey", func() {
	It("cleans both parts so equivalent paths share a key", func() {
		Expect(index.NewKey("/vault/", "./Tasks/")).To(Equal(index.NewKey("/vault", "Tasks")))
	})
})

var _ = Describe("Metrics", func() {
	It("accepts calls on the no-op implementation", func() {
		metrics := index.NewNoopMetrics()
		metrics.FileRead(index.ReadReasonBuild)
		metrics.ChangedPagesUnbounded(index.UnboundedUnknownKey)
	})
})
