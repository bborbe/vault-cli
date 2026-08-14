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

	Describe("findFileByName with a non-existent directory", func() {
		var missingDir string

		BeforeEach(func() {
			missingDir = filepath.Join(tmpDir, "does-not-exist")
		})

		// A vault configured with a goals_dir that was never created must look like
		// "not here" rather than a hard failure — ops.VaultDispatcher.FirstSuccess only
		// continues to the next vault when the error satisfies errors.Is(err, ErrNotFound).
		It("the error satisfies errors.Is for storage.ErrNotFound", func() {
			_, _, err := storage.FindFileByNameForTest(ctx, b, missingDir, "some-goal")
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())
		})

		It("does not surface the missing directory path as a walk error", func() {
			_, _, err := storage.FindFileByNameForTest(ctx, b, missingDir, "some-goal")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).NotTo(ContainSubstring("walk directory"))
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
