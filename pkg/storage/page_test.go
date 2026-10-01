// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

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

	It("emits one bounded record per unreadable file regardless of error count", func() {
		pagesDir := filepath.Join(vaultPath, "UnreadablePages")
		Expect(os.MkdirAll(pagesDir, 0755)).To(Succeed())

		healthy := "---\nstatus: in_progress\npage_type: task\ntask_identifier: 11111111-1111-4111-a111-111111111111\n---\n# Healthy\n"
		Expect(
			os.WriteFile(filepath.Join(pagesDir, "Short.md"), []byte(duplicateKeyPage(3)), 0600),
		).To(Succeed())
		Expect(
			os.WriteFile(filepath.Join(pagesDir, "Huge.md"), []byte(duplicateKeyPage(2992)), 0600),
		).To(Succeed())
		Expect(
			os.WriteFile(filepath.Join(pagesDir, "Healthy.md"), []byte(healthy), 0600),
		).To(Succeed())

		pages, err := store.ListPages(ctx, vaultPath, "UnreadablePages")

		Expect(err).To(BeNil())
		Expect(pages).To(HaveLen(1))
		Expect(pages[0].Name).To(Equal("Healthy"))

		log := logBuf.String()
		Expect(strings.Count(log, "skipping unreadable page")).To(Equal(2))
		for _, line := range strings.Split(strings.TrimRight(log, "\n"), "\n") {
			Expect(len(line)).To(BeNumerically("<=", 1024))
		}
		Expect(log).ToNot(ContainSubstring("Healthy.md"))
	})

	It("bounds the logged error field to 200 bytes and keeps the head", func() {
		jsonBuf := &bytes.Buffer{}
		slog.SetDefault(
			slog.New(slog.NewJSONHandler(jsonBuf, &slog.HandlerOptions{Level: slog.LevelWarn})),
		)

		pagesDir := filepath.Join(vaultPath, "UnreadablePages")
		Expect(os.MkdirAll(pagesDir, 0755)).To(Succeed())
		Expect(
			os.WriteFile(filepath.Join(pagesDir, "Huge.md"), []byte(duplicateKeyPage(2992)), 0600),
		).To(Succeed())

		pages, err := store.ListPages(ctx, vaultPath, "UnreadablePages")

		Expect(err).To(BeNil())
		Expect(pages).To(BeEmpty())

		var rec map[string]any
		Expect(
			json.Unmarshal([]byte(strings.TrimRight(jsonBuf.String(), "\n")), &rec),
		).To(Succeed())
		errField, ok := rec["error"].(string)
		Expect(ok).To(BeTrue())
		Expect(len(errField)).To(BeNumerically("<=", 200))
		Expect(utf8.ValidString(errField)).To(BeTrue())
		Expect(strings.HasSuffix(errField, "…")).To(BeTrue())
		Expect(strings.Contains(errField, "already defined")).To(BeTrue())
	})

	It("cuts on a rune boundary when the cause contains multibyte runes", func() {
		jsonBuf := &bytes.Buffer{}
		slog.SetDefault(
			slog.New(slog.NewJSONHandler(jsonBuf, &slog.HandlerOptions{Level: slog.LevelWarn})),
		)

		pagesDir := filepath.Join(vaultPath, "UnreadablePages")
		Expect(os.MkdirAll(pagesDir, 0755)).To(Succeed())

		key := "k" + strings.Repeat("ü", 100)
		midRune := fmt.Sprintf("---\n%s: 1\n%s: 2\n---\n# MidRune\n", key, key)
		Expect(
			os.WriteFile(filepath.Join(pagesDir, "MidRune.md"), []byte(midRune), 0600),
		).To(Succeed())

		pages, err := store.ListPages(ctx, vaultPath, "UnreadablePages")

		Expect(err).To(BeNil())
		Expect(pages).To(BeEmpty())

		var rec map[string]any
		Expect(
			json.Unmarshal([]byte(strings.TrimRight(jsonBuf.String(), "\n")), &rec),
		).To(Succeed())
		errField, ok := rec["error"].(string)
		Expect(ok).To(BeTrue())
		Expect(len(errField)).To(BeNumerically("<=", 200))
		Expect(utf8.ValidString(errField)).To(BeTrue())
		Expect(strings.HasSuffix(errField, "…")).To(BeTrue())
	})

	It("logs one short stack-free line for a page without frontmatter", func() {
		pagesDir := filepath.Join(vaultPath, "UnreadablePages")
		Expect(os.MkdirAll(pagesDir, 0755)).To(Succeed())
		Expect(
			os.WriteFile(filepath.Join(pagesDir, "NoFm.md"), []byte("# No Frontmatter\n"), 0600),
		).To(Succeed())

		pages, err := store.ListPages(ctx, vaultPath, "UnreadablePages")

		Expect(err).To(BeNil())
		Expect(pages).To(BeEmpty())

		log := logBuf.String()
		Expect(strings.Count(log, "\n")).To(Equal(1))
		Expect(len(log)).To(BeNumerically("<=", 300))
		Expect(log).To(ContainSubstring("no frontmatter found"))
		Expect(log).ToNot(ContainSubstring("runtime.goexit"))
		Expect(log).ToNot(ContainSubstring("errors_new.go"))
		Expect(log).ToNot(ContainSubstring("errors_wrap.go"))
		Expect(log).ToNot(ContainSubstring("\n\t"))
	})
})

// duplicateKeyPage returns a page whose frontmatter declares task_identifier n
// times, so the YAML parse raises n(n-1)/2 duplicate-key errors.
func duplicateKeyPage(n int) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	for i := 0; i < n; i++ {
		sb.WriteString("task_identifier: 11111111-1111-4111-a111-111111111111\n")
	}
	sb.WriteString("---\n# Broken\n")
	return sb.String()
}
