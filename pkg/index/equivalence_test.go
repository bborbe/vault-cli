// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	libtime "github.com/bborbe/time"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/index"
)

// equivalenceFolder is the fixture's vault-relative pages dir.
const equivalenceFolder = "24 Tasks"

// writeFixtureFile writes one file under the vault's pages folder.
func writeFixtureFile(vaultDir, name, content string) {
	Expect(os.WriteFile(
		filepath.Join(vaultDir, equivalenceFolder, name), []byte(content), 0600,
	)).To(Succeed())
}

// buildEquivalenceVault creates the awkward-files fixture: a plain page, a bare
// wikilink, a file without frontmatter, a file with invalid YAML, a non-.md
// file, a subdirectory, a symlink out of the vault, a broken symlink, a symlink
// to a page inside the vault and a non-ASCII filename.
func buildEquivalenceVault() string {
	vaultDir := GinkgoT().TempDir()
	folderDir := filepath.Join(vaultDir, equivalenceFolder)
	Expect(os.MkdirAll(folderDir, 0750)).To(Succeed())
	Expect(os.MkdirAll(filepath.Join(folderDir, "subdir"), 0750)).To(Succeed())

	writeFixtureFile(vaultDir, "Plain.md", "---\ntitle: Plain\nstatus: next\n---\n# Plain\n")
	writeFixtureFile(
		vaultDir, "Wikilink.md", "---\ntitle: Wikilink\nrelated: [[Other]]\n---\n# Wikilink\n",
	)
	writeFixtureFile(vaultDir, "NoFrontmatter.md", "# No frontmatter\n")
	writeFixtureFile(vaultDir, "BadYaml.md", "---\ntitle: [unclosed\n---\n# Bad\n")
	writeFixtureFile(vaultDir, "notes.txt", "not a page\n")
	writeFixtureFile(vaultDir, "Ünïcode.md", "---\ntitle: Ünïcode\n---\n# Ünïcode\n")

	// The outside target lives in its own temp dir, never under the vault.
	outsideDir := GinkgoT().TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.md")
	Expect(os.WriteFile(outsideFile, []byte("---\ntitle: Secret\n---\n# Secret\n"), 0600)).
		To(Succeed())
	Expect(os.Symlink(outsideFile, filepath.Join(folderDir, "Outside.md"))).To(Succeed())

	// A broken symlink counts as outside.
	Expect(os.Symlink(
		filepath.Join(folderDir, "does-not-exist.md"), filepath.Join(folderDir, "Broken.md"),
	)).To(Succeed())

	// A symlink to a page inside the vault stays in the listing.
	Expect(os.Symlink("Plain.md", filepath.Join(folderDir, "Inside.md"))).To(Succeed())

	return vaultDir
}

// indexedNames returns the pages' file names in listing order.
func indexedNames(pages []*domain.Page) []string {
	names := make([]string, 0, len(pages))
	for _, page := range pages {
		names = append(names, page.Name)
	}
	return names
}

// withoutContent returns copies of pages with Content zeroed, the reference
// shape the metadata-only index serves. A nil input stays nil.
func withoutContent(pages []*domain.Page) []*domain.Page {
	if pages == nil {
		return nil
	}
	result := make([]*domain.Page, 0, len(pages))
	for _, page := range pages {
		copied := *page
		copied.Content = ""
		result = append(result, &copied)
	}
	return result
}

// referenceListing is vault-cli's own folder listing with Content zeroed.
func referenceListing(ctx context.Context, vaultDir string) []*domain.Page {
	pages, err := storage.NewPageStorage(nil).ListPages(ctx, vaultDir, equivalenceFolder)
	Expect(err).NotTo(HaveOccurred())
	return withoutContent(pages)
}

// newEquivalenceIndex builds the index over the production reader and lister.
func newEquivalenceIndex() index.PageIndex {
	return index.NewPageIndex(
		index.NewPageReader(storage.NewPageStorage(nil)),
		index.NewDirectoryLister(),
		libtime.NewCurrentDateTime(),
		libtime.NewWaiterDuration(),
		index.TestMetrics,
	)
}

var _ = Describe("Snapshot equivalence", func() {
	It("lists the same pages as vault-cli storage with only Content excluded", func() {
		ctx := context.Background()
		vaultDir := buildEquivalenceVault()

		want := referenceListing(ctx, vaultDir)
		// Positive control: a fixture that silently excluded everything would
		// still deep-equal an equally empty snapshot.
		Expect(indexedNames(want)).To(Equal([]string{"Inside", "Plain", "Wikilink", "Ünïcode"}))

		got, err := newEquivalenceIndex().ListPages(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())

		Expect(reflect.DeepEqual(got, want)).To(BeTrue())
	})

	It("matches vault-cli on an existing folder with no page files", func() {
		ctx := context.Background()
		vaultDir := GinkgoT().TempDir()
		Expect(os.MkdirAll(
			filepath.Join(vaultDir, equivalenceFolder), 0750,
		)).To(Succeed())

		want, err := storage.NewPageStorage(nil).ListPages(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())
		Expect(want).NotTo(BeNil())
		Expect(want).To(BeEmpty())

		got, err := newEquivalenceIndex().ListPages(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())

		Expect(reflect.DeepEqual(got, want)).To(BeTrue())
	})

	It("AC3 stays equal to vault-cli through create, modify, delete and rename", func() {
		ctx := context.Background()
		vaultDir := buildEquivalenceVault()
		idx := newEquivalenceIndex()
		key := index.NewKey(vaultDir, equivalenceFolder)

		// Every step is checked against vault-cli's own listing of the same dir.
		assertEquivalent := func() {
			want := referenceListing(ctx, vaultDir)
			got, err := idx.ListPages(ctx, vaultDir, equivalenceFolder)
			Expect(err).NotTo(HaveOccurred())
			Expect(reflect.DeepEqual(got, want)).To(
				BeTrue(),
				"index %v != vault-cli %v",
				indexedNames(got),
				indexedNames(want),
			)
		}

		// The cold build is the starting point.
		assertEquivalent()

		// Create, through a per-file update.
		writeFixtureFile(vaultDir, "Created.md", "---\ntitle: Created\n---\n# Created\n")
		Expect(idx.RefreshFile(ctx, key, "Created.md")).To(Succeed())
		assertEquivalent()

		// Modify Plain.md, whose symlink Inside.md shares the same bytes.
		writeFixtureFile(vaultDir, "Plain.md", "---\ntitle: Plain\nstatus: done\n---\n# Plain v2\n")
		Expect(idx.RefreshFile(ctx, key, "Plain.md")).To(Succeed())
		Expect(idx.RefreshFile(ctx, key, "Inside.md")).To(Succeed())
		assertEquivalent()

		// Delete, through a per-file update.
		Expect(os.Remove(filepath.Join(vaultDir, equivalenceFolder, "NoFrontmatter.md"))).
			To(Succeed())
		Expect(idx.RefreshFile(ctx, key, "NoFrontmatter.md")).To(Succeed())
		assertEquivalent()

		// Rename is a delete plus a create.
		Expect(os.Rename(
			filepath.Join(vaultDir, equivalenceFolder, "Wikilink.md"),
			filepath.Join(vaultDir, equivalenceFolder, "Renamed.md"),
		)).To(Succeed())
		Expect(idx.RefreshFile(ctx, key, "Wikilink.md")).To(Succeed())
		Expect(idx.RefreshFile(ctx, key, "Renamed.md")).To(Succeed())
		assertEquivalent()

		// The in-vault symlink's target is rewritten with no event delivered; a
		// stat-diff must pick up both the target and the symlink.
		writeFixtureFile(
			vaultDir, "Plain.md", "---\ntitle: Plain\nstatus: blocked\n---\n# Plain v3 longer\n",
		)
		Expect(idx.Refresh(ctx, key)).To(BeNil())
		assertEquivalent()

		// And the stat-diff of an unchanged folder changes nothing.
		Expect(idx.Refresh(ctx, key)).To(BeNil())
		assertEquivalent()
	})

	It("matches vault-cli on a missing folder", func() {
		ctx := context.Background()
		vaultDir := GinkgoT().TempDir()

		want, err := storage.NewPageStorage(nil).ListPages(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())
		Expect(want).To(BeNil())

		got, err := newEquivalenceIndex().ListPages(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())

		Expect(reflect.DeepEqual(got, want)).To(BeTrue())
	})

	It("returns pages and entries with empty Content and populated frontmatter, name, path and modification date", func() {
		ctx := context.Background()
		vaultDir := buildEquivalenceVault()
		idx := newEquivalenceIndex()
		key := index.NewKey(vaultDir, equivalenceFolder)
		Expect(idx.Build(ctx, []index.Key{key})).To(Succeed())

		pages, err := idx.ListPages(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())
		entries, err := idx.ListEntries(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(len(pages)))
		Expect(pages).NotTo(BeEmpty())

		for i, page := range pages {
			Expect(entries[i].Page).To(BeIdenticalTo(page))
			Expect(string(page.Content)).To(BeEmpty())
			path := filepath.Join(vaultDir, equivalenceFolder, page.Name+".md")
			info, err := os.Stat(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(page.Name).To(Equal(strings.TrimSuffix(filepath.Base(path), ".md")))
			Expect(page.FileMetadata.FilePath).To(Equal(path))
			Expect(page.FileMetadata.ModifiedDate).NotTo(BeNil())
			Expect(page.FileMetadata.ModifiedDate.Equal(info.ModTime())).To(BeTrue())
			if page.Name == "Plain" {
				Expect(page.RawMap()).To(HaveKeyWithValue("title", "Plain"))
			}
			Expect(entries[i].OpenQuestions).NotTo(BeNil())
		}
	})

	It("produces the same list-operation items as vault-cli storage", func() {
		ctx := context.Background()
		vaultDir := buildEquivalenceVault()

		want, err := ops.NewListOperation(storage.NewPageStorage(nil)).
			Execute(ctx, vaultDir, "vault", equivalenceFolder, nil, true, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(want).NotTo(BeEmpty())
		got, err := ops.NewListOperation(newEquivalenceIndex()).
			Execute(ctx, vaultDir, "vault", equivalenceFolder, nil, true, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(want))
	})

	It("reads the full body on ReadPage without caching it in the snapshot", func() {
		ctx := context.Background()
		vaultDir := buildEquivalenceVault()
		idx := newEquivalenceIndex()

		before, err := idx.ListPages(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())
		raw, err := os.ReadFile(filepath.Join(vaultDir, equivalenceFolder, "Plain.md"))
		Expect(err).NotTo(HaveOccurred())

		page, err := idx.ReadPage(ctx, vaultDir, equivalenceFolder, "Plain")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(page.Content)).To(Equal(string(raw)))

		after, err := idx.ListPages(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())
		Expect(after).To(HaveLen(len(before)))
		for i := range after {
			Expect(after[i]).To(BeIdenticalTo(before[i]))
			Expect(string(after[i].Content)).To(BeEmpty())
		}
	})

	It("refuses a ReadPage name that must never become a file path", func() {
		ctx := context.Background()
		idx := newEquivalenceIndex()
		for _, name := range []string{"", "sub/Page", ".."} {
			_, err := idx.ReadPage(ctx, GinkgoT().TempDir(), equivalenceFolder, name)
			Expect(err).To(HaveOccurred())
		}
	})

	It("republishes a body-only edit and names it in ChangedPagesSince", func() {
		ctx := context.Background()
		vaultDir := buildEquivalenceVault()
		key := index.NewKey(vaultDir, equivalenceFolder)

		for _, viaRefreshFile := range []bool{true, false} {
			writeFixtureFile(vaultDir, "Questions.md", "---\ntitle: Questions\n---\n## Open Questions\n\n- first\n")
			idx := newEquivalenceIndex()
			Expect(idx.Build(ctx, []index.Key{key})).To(Succeed())
			before := findEntry(ctx, idx, vaultDir, "Questions")
			Expect(before.OpenQuestions).To(Equal([]domain.OpenQuestion{{Index: 1, Text: "first"}}))
			revision := idx.Revision(key)

			settle()
			body := "---\ntitle: Questions\n---\n## Open Questions\n\n- first\n- second edited\n"
			if !viaRefreshFile {
				body += "<!-- rescan -->\n"
			}
			writeFixtureFile(vaultDir, "Questions.md", body)
			if viaRefreshFile {
				Expect(idx.RefreshFile(ctx, key, "Questions.md")).To(Succeed())
			} else {
				Expect(idx.Refresh(ctx, key)).To(Succeed())
			}

			after := findEntry(ctx, idx, vaultDir, "Questions")
			Expect(after.Page).NotTo(BeIdenticalTo(before.Page))
			Expect(after.OpenQuestions).To(Equal([]domain.OpenQuestion{
				{Index: 1, Text: "first"},
				{Index: 2, Text: "second edited"},
			}))
			names, ok := idx.ChangedPagesSince(key, revision)
			Expect(ok).To(BeTrue())
			Expect(names).To(ContainElement("Questions.md"))
		}
	})
})

// findEntry returns the listed entry whose page has the given name.
func findEntry(ctx context.Context, idx index.PageIndex, vaultDir, name string) *index.Entry {
	entries, err := idx.ListEntries(ctx, vaultDir, equivalenceFolder)
	Expect(err).NotTo(HaveOccurred())
	for _, entry := range entries {
		if entry.Page.Name == name {
			return entry
		}
	}
	Fail("no entry named " + name)
	return nil
}
