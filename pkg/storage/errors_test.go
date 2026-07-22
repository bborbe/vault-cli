// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("ErrNotFound sentinel", func() {
	var (
		ctx    context.Context
		b      *storage.BaseStorageForTest
		tmpDir string
	)

	BeforeEach(func() {
		ctx = context.Background()
		b = storage.NewBaseStorageForTest()
		var err error
		tmpDir, err = os.MkdirTemp("", "vault-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("findFileByName with non-existent name", func() {
		It("returns an error containing 'file not found'", func() {
			_, _, err := storage.FindFileByNameForTest(ctx, b, tmpDir, "nonexistent-task")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("file not found"))
		})

		It("the error satisfies errors.Is for storage.ErrNotFound", func() {
			_, _, err := storage.FindFileByNameForTest(ctx, b, tmpDir, "nonexistent-task")
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())
		})
	})

	Context("with a file that does not match", func() {
		BeforeEach(func() {
			// Create a different file so the directory is non-empty but has no match
			err := os.WriteFile(filepath.Join(tmpDir, "other.md"), []byte("---\n---\n"), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns ErrNotFound for a name that does not match any .md file", func() {
			_, _, err := storage.FindFileByNameForTest(ctx, b, tmpDir, "nonexistent")
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())
		})
	})
})
