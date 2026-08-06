// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("pageStorage.ListPages diagnostics", func() {
	var (
		ctx        context.Context
		vaultPath  string
		store      storage.PageStorage
		logBuf     *bytes.Buffer
		prevLogger *slog.Logger
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		vaultPath, err = os.MkdirTemp("", "vault-test")
		Expect(err).To(BeNil())

		logBuf = &bytes.Buffer{}
		prevLogger = slog.Default()
		slog.SetDefault(
			slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})),
		)

		store = storage.NewPageStorage(storage.NewConfigFromVault(&config.Vault{}))
	})

	AfterEach(func() {
		slog.SetDefault(prevLogger)
		os.RemoveAll(vaultPath)
	})

	It("warns with full path and parse error when skipping a corrupt page", func() {
		pagesDir := filepath.Join(vaultPath, "UnreadablePages")
		Expect(os.MkdirAll(pagesDir, 0755)).To(Succeed())

		healthy := "---\nstatus: in_progress\npage_type: task\ntask_identifier: 11111111-1111-4111-a111-111111111111\n---\n# Healthy\n"
		broken := "---\ntask_identifier: e1bc4321-7570-41f9-bfc6-a783d7aa4371\nassignee: bborbe\nstatus: completed\ntask_identifier: 9fba815b-e1bb-442d-bc3e-87722f767a1f\n---\n# Broken\n"

		Expect(
			os.WriteFile(filepath.Join(pagesDir, "Healthy.md"), []byte(healthy), 0600),
		).To(Succeed())
		Expect(
			os.WriteFile(filepath.Join(pagesDir, "Broken.md"), []byte(broken), 0600),
		).To(Succeed())

		pages, err := store.ListPages(ctx, vaultPath, "UnreadablePages")

		Expect(err).To(BeNil())
		Expect(pages).To(HaveLen(1))
		Expect(pages[0].Name).To(Equal("Healthy"))

		log := logBuf.String()
		Expect(log).To(ContainSubstring("skipping unreadable page"))
		Expect(log).To(ContainSubstring(filepath.Join(pagesDir, "Broken.md")))
		Expect(log).To(ContainSubstring("already defined"))
		Expect(log).ToNot(ContainSubstring("Healthy.md"))
		Expect(log).ToNot(ContainSubstring("github.com/bborbe/errors.Wrap"))
		Expect(log).ToNot(ContainSubstring("errors_wrap.go"))
		Expect(log).ToNot(ContainSubstring("runtime.goexit"))
	})

	It("produces no warning for a directory with only parseable pages", func() {
		pagesDir := filepath.Join(vaultPath, "UnreadablePages")
		Expect(os.MkdirAll(pagesDir, 0755)).To(Succeed())

		healthy := "---\nstatus: in_progress\npage_type: task\ntask_identifier: 11111111-1111-4111-a111-111111111111\n---\n# Healthy\n"
		Expect(
			os.WriteFile(filepath.Join(pagesDir, "Healthy.md"), []byte(healthy), 0600),
		).To(Succeed())

		pages, err := store.ListPages(ctx, vaultPath, "UnreadablePages")

		Expect(err).To(BeNil())
		Expect(pages).To(HaveLen(1))
		Expect(logBuf.String()).ToNot(ContainSubstring("skipping unreadable page"))
	})

	It("returns empty list without warning for a missing directory", func() {
		pages, err := store.ListPages(ctx, vaultPath, "DoesNotExist")

		Expect(err).To(BeNil())
		Expect(pages).To(BeEmpty())
		Expect(logBuf.String()).ToNot(ContainSubstring("skipping unreadable page"))
	})
})
