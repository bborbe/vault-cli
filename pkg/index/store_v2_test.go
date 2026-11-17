// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"time"

	libtime "github.com/bborbe/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	bolt "go.etcd.io/bbolt"

	"github.com/bborbe/vault-cli/pkg/index"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// fieldNames walks a type and returns every struct field name at any depth.
func fieldNames(t reflect.Type, seen map[reflect.Type]bool) []string {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return nil
	}
	seen[t] = true
	names := []string{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		names = append(names, field.Name)
		names = append(names, fieldNames(field.Type, seen)...)
	}
	return names
}

// writeLiteralStore writes a bolt file holding one identity and one entry
// bucket, using literal names rather than package constants.
func writeLiteralStore(path, identity, vaultDir, pagesDir string) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	Expect(err).NotTo(HaveOccurred())
	Expect(db.Update(func(tx *bolt.Tx) error {
		meta, err := tx.CreateBucket([]byte("vault-ui-page-index-meta"))
		if err != nil {
			return err
		}
		if err := meta.Put([]byte("identity"), []byte(identity)); err != nil {
			return err
		}
		entries, err := tx.CreateBucket(
			[]byte("vault-ui-page-index\x00" + vaultDir + "\x00" + pagesDir),
		)
		if err != nil {
			return err
		}
		return entries.Put([]byte("A.md"), []byte("v1 record with a body"))
	})).To(Succeed())
	Expect(db.Close()).To(Succeed())
}

// bucketNames lists the bolt file's top-level bucket names.
func bucketNames(path string) []string {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	Expect(err).NotTo(HaveOccurred())
	defer func() { Expect(db.Close()).To(Succeed()) }()
	names := []string{}
	Expect(db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			names = append(names, string(name))
			return nil
		})
	})).To(Succeed())
	return names
}

var _ = Describe("Store format version 2", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("persists no Content field in the store record", func() {
		names := fieldNames(index.PersistedRecordType(), map[reflect.Type]bool{})
		Expect(names).NotTo(ContainElement("Content"))
		Expect(names).To(ContainElement("OpenQuestions"))
	})

	It("writes no body bytes to the store file", func() {
		vaultDir := GinkgoT().TempDir()
		Expect(os.MkdirAll(filepath.Join(vaultDir, equivalenceFolder), 0750)).To(Succeed())
		writeFixtureFile(vaultDir, "Body.md",
			"---\ntitle: Body\n---\n## Open Questions\n\n- kept question\n\n## Notes\nBODY-SENTINEL-7f3a\n")
		path := filepath.Join(GinkgoT().TempDir(), "page-index.bolt")
		store := newStore(path, &warnSink{}, &vlogSink{})

		idx := index.NewPageIndexWithStore(
			index.NewPageReader(storage.NewPageStorage(nil)),
			index.NewDirectoryLister(),
			libtime.NewCurrentDateTime(), fastWaiter(), nil, store,
		)
		pages, err := idx.ListPages(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())
		Expect(pages).To(HaveLen(1))
		Expect(store.Close()).To(Succeed())

		raw, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(bytes.Contains(raw, []byte("kept question"))).To(BeTrue())
		Expect(bytes.Contains(raw, []byte("BODY-SENTINEL-7f3a"))).To(BeFalse())
	})

	It("discards a v1 store, deletes its entry buckets and warns store format mismatch with the path", func() {
		path := filepath.Join(GinkgoT().TempDir(), "page-index.bolt")
		writeLiteralStore(path, `{"StoreFormat":1,"ParserVersion":"v0.171.0"}`, "/vault", "24 Tasks")
		Expect(bucketNames(path)).To(HaveLen(2))

		warns := &warnSink{}
		store := newStore(path, warns, &vlogSink{})
		Expect(store.Close()).To(Succeed())

		Expect(bucketNames(path)).To(Equal([]string{"vault-ui-page-index-meta"}))
		mismatch := warns.naming("store format mismatch")
		Expect(mismatch).To(HaveLen(1))
		Expect(mismatch[0]).To(ContainSubstring("stored 1, current 2"))
		Expect(mismatch[0]).To(ContainSubstring(path))
	})

	It("reports parser version mismatch as a distinct reason", func() {
		path := filepath.Join(GinkgoT().TempDir(), "page-index.bolt")
		writeLiteralStore(path, `{"StoreFormat":2,"ParserVersion":"v0.0.0-stale"}`, "/vault", "24 Tasks")

		warns := &warnSink{}
		store := newStore(path, warns, &vlogSink{})
		Expect(store.Close()).To(Succeed())

		Expect(warns.naming("parser version mismatch")).To(HaveLen(1))
		Expect(warns.naming(path)).To(HaveLen(1))
		Expect(warns.naming("store format mismatch")).To(BeEmpty())
		Expect(bucketNames(path)).To(Equal([]string{"vault-ui-page-index-meta"}))
	})

	It("reports a damaged identity as a distinct reason", func() {
		path := filepath.Join(GinkgoT().TempDir(), "page-index.bolt")
		writeLiteralStore(path, `{not json`, "/vault", "24 Tasks")

		warns := &warnSink{}
		store := newStore(path, warns, &vlogSink{})
		Expect(store.Close()).To(Succeed())

		Expect(warns.naming("damaged")).To(HaveLen(1))
		Expect(bucketNames(path)).To(Equal([]string{"vault-ui-page-index-meta"}))
	})

	It("serves without a store when the bolt file is locked by another process", NodeTimeout(30*time.Second), func(specCtx SpecContext) {
		path := filepath.Join(GinkgoT().TempDir(), "page-index.bolt")
		holder, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(holder.Close()).To(Succeed()) }()

		vaultDir := buildEquivalenceVault()
		warns := &warnSink{}
		start := time.Now()
		store := index.NewBoltStore(specCtx, path, index.CurrentWriterIdentity(specCtx), warns.warnf, (&vlogSink{}).vlogf)
		Expect(time.Since(start)).To(BeNumerically("<", 10*time.Second))

		idx := index.NewPageIndexWithStoreAndWarnf(
			index.NewPageReader(storage.NewPageStorage(nil)),
			index.NewDirectoryLister(),
			libtime.NewCurrentDateTime(), fastWaiter(), store, warns.warnf,
		)
		pages, err := idx.ListPages(specCtx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())
		Expect(indexedNames(pages)).To(Equal([]string{"Inside", "Plain", "Wikilink", "Ünïcode"}))
		Expect(warns.naming("discarded")).NotTo(BeEmpty())
		Expect(warns.naming(path)).NotTo(BeEmpty())
	})

	It("resolves the store path under the user cache directory per app", func() {
		cacheDir, err := os.UserCacheDir()
		Expect(err).NotTo(HaveOccurred())
		path, err := index.DefaultStorePath("vault-api")
		Expect(err).NotTo(HaveOccurred())
		Expect(path).To(Equal(filepath.Join(cacheDir, "vault-api", "page-index.bolt")))
	})

	It("creates the store directory with mode 0700", func() {
		dir := filepath.Join(GinkgoT().TempDir(), "nested")
		store := newStore(filepath.Join(dir, "page-index.bolt"), &warnSink{}, &vlogSink{})
		defer closeStore(store)
		info, err := os.Stat(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0700)))
	})
})

var _ = Describe("Store hydration", func() {
	It("restores open questions from the store without re-reading the file", func() {
		ctx := context.Background()
		vaultDir := GinkgoT().TempDir()
		Expect(os.MkdirAll(filepath.Join(vaultDir, equivalenceFolder), 0750)).To(Succeed())
		writeFixtureFile(vaultDir, "Questions.md",
			"---\ntitle: Questions\n---\n## Open Questions\n\n- first\n- second → answered\n")
		writeFixtureFile(vaultDir, "Plain.md", "---\ntitle: Plain\n---\n# Plain\n")
		path := filepath.Join(GinkgoT().TempDir(), "page-index.bolt")

		storeA := newStore(path, &warnSink{}, &vlogSink{})
		a := index.NewPageIndexWithStore(
			index.NewPageReader(storage.NewPageStorage(nil)), index.NewDirectoryLister(),
			libtime.NewCurrentDateTime(), fastWaiter(), nil, storeA,
		)
		entriesA, err := a.ListEntries(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())
		Expect(entriesA).To(HaveLen(2))
		Expect(storeA.Close()).To(Succeed())

		storeB := newStore(path, &warnSink{}, &vlogSink{})
		defer closeStore(storeB)
		metricsB := index.NewCountingMetrics()
		b := index.NewPageIndexWithStore(
			index.NewPageReader(storage.NewPageStorage(nil)), index.NewDirectoryLister(),
			libtime.NewCurrentDateTime(), fastWaiter(), metricsB, storeB,
		)
		entriesB, err := b.ListEntries(ctx, vaultDir, equivalenceFolder)
		Expect(err).NotTo(HaveOccurred())
		Expect(entriesB).To(HaveLen(2))
		for i := range entriesA {
			Expect(entriesB[i].OpenQuestions).To(Equal(entriesA[i].OpenQuestions))
		}
		Expect(entriesB[1].OpenQuestions).To(HaveLen(2))
		Expect(metricsB.TotalReads()).To(Equal(0))
	})
})
