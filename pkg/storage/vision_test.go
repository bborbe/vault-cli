// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("VisionStorage", func() {
	var (
		ctx       context.Context
		store     storage.Storage
		vaultPath string
		visionDir string
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = storage.NewStorage(nil)

		var err error
		vaultPath, err = os.MkdirTemp("", "vault-vision-test-*")
		Expect(err).To(BeNil())

		visionDir = filepath.Join(vaultPath, "20 Vision")
		Expect(os.MkdirAll(visionDir, 0755)).To(Succeed())
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	visionContent := func() string {
		return `---
status: active
page_type: vision
priority: 1
assignee: bborbe
tags:
  - longterm
---
# Test Vision

This is a test vision.
`
	}

	Describe("ReadVision", func() {
		It("reads a vision successfully", func() {
			visionPath := filepath.Join(visionDir, "Test Vision.md")
			Expect(os.WriteFile(visionPath, []byte(visionContent()), 0600)).To(Succeed())

			vis, err := store.ReadVision(ctx, vaultPath, "Test Vision")
			Expect(err).To(BeNil())
			Expect(vis).NotTo(BeNil())
			Expect(vis.Name).To(Equal("Test Vision"))
			Expect(vis.Status).To(Equal(domain.VisionStatusActive))
			Expect(vis.PageType).To(Equal("vision"))
			Expect(vis.Priority).To(Equal(domain.Priority(1)))
			Expect(vis.Assignee).To(Equal("bborbe"))
			Expect(vis.Tags).To(ContainElement("longterm"))
		})

		It("returns error when vision file does not exist", func() {
			_, err := store.ReadVision(ctx, vaultPath, "Nonexistent Vision")
			Expect(err).NotTo(BeNil())
		})
	})

	Describe("WriteVision", func() {
		It("writes and reads back a vision correctly", func() {
			visionPath := filepath.Join(visionDir, "New Vision.md")
			newVision := &domain.Vision{
				Name:     "New Vision",
				FilePath: visionPath,
				Status:   domain.VisionStatusActive,
				PageType: "vision",
				Priority: 2,
				Assignee: "alice",
			}

			Expect(store.WriteVision(ctx, newVision)).To(Succeed())

			vis, err := store.ReadVision(ctx, vaultPath, "New Vision")
			Expect(err).To(BeNil())
			Expect(vis.Name).To(Equal("New Vision"))
			Expect(vis.Status).To(Equal(domain.VisionStatusActive))
			Expect(vis.Priority).To(Equal(domain.Priority(2)))
			Expect(vis.Assignee).To(Equal("alice"))
		})

		It("returns error when writing to read-only directory", func() {
			readOnlyVault, err := os.MkdirTemp("", "vault-readonly-*")
			Expect(err).To(BeNil())
			defer func() { _ = os.RemoveAll(readOnlyVault) }()

			readOnlyVisionDir := filepath.Join(readOnlyVault, "20 Vision")
			Expect(os.MkdirAll(readOnlyVisionDir, 0755)).To(Succeed())
			Expect(os.Chmod(readOnlyVisionDir, 0444)).To(Succeed())

			vision := &domain.Vision{
				Name:     "Read-Only Vision",
				FilePath: filepath.Join(readOnlyVisionDir, "Read-Only Vision.md"),
				Status:   domain.VisionStatusActive,
			}

			err = store.WriteVision(ctx, vision)
			Expect(err).NotTo(BeNil())
		})

		It("excludes metadata fields from frontmatter", func() {
			visionPath := filepath.Join(visionDir, "Metadata Test.md")
			vision := &domain.Vision{
				Name:     "Metadata Test",
				FilePath: visionPath,
				Status:   domain.VisionStatusArchived,
				PageType: "vision",
				Content: `---
status: archived
page_type: vision
---
# Metadata Test
`,
			}

			Expect(store.WriteVision(ctx, vision)).To(Succeed())

			rawBytes, err := os.ReadFile(visionPath)
			Expect(err).To(BeNil())
			rawContent := string(rawBytes)

			Expect(rawContent).NotTo(ContainSubstring("name:"))
			Expect(rawContent).NotTo(ContainSubstring("content:"))
			Expect(rawContent).NotTo(ContainSubstring("filepath:"))
			Expect(rawContent).To(ContainSubstring("status: archived"))
		})
	})

	Describe("FindVisionByName", func() {
		BeforeEach(func() {
			visionPath := filepath.Join(visionDir, "Test Vision.md")
			Expect(os.WriteFile(visionPath, []byte(visionContent()), 0600)).To(Succeed())
		})

		It("finds vision by exact name", func() {
			vis, err := store.FindVisionByName(ctx, vaultPath, "Test Vision")
			Expect(err).To(BeNil())
			Expect(vis).NotTo(BeNil())
			Expect(vis.Name).To(Equal("Test Vision"))
			Expect(vis.Status).To(Equal(domain.VisionStatusActive))
		})

		It("finds vision by partial name", func() {
			vis, err := store.FindVisionByName(ctx, vaultPath, "test")
			Expect(err).To(BeNil())
			Expect(vis).NotTo(BeNil())
			Expect(vis.Name).To(Equal("Test Vision"))
		})

		It("returns error when vision not found", func() {
			_, err := store.FindVisionByName(ctx, vaultPath, "Nonexistent")
			Expect(err).NotTo(BeNil())
		})

		It("round-trips vision with FindVisionByName after WriteVision", func() {
			newVis := &domain.Vision{
				Name:     "Round Trip Vision",
				FilePath: filepath.Join(visionDir, "Round Trip Vision.md"),
				Status:   domain.VisionStatusCompleted,
				PageType: "vision",
				Tags:     []string{"test", "roundtrip"},
			}

			Expect(store.WriteVision(ctx, newVis)).To(Succeed())

			found, err := store.FindVisionByName(ctx, vaultPath, "Round Trip Vision")
			Expect(err).To(BeNil())
			Expect(found).NotTo(BeNil())
			Expect(found.Name).To(Equal("Round Trip Vision"))
			Expect(found.Status).To(Equal(domain.VisionStatusCompleted))
			Expect(found.Tags).To(Equal([]string{"test", "roundtrip"}))
		})
	})

	Describe("NewVisionStorage", func() {
		It("creates a narrow VisionStorage", func() {
			narrowStore := storage.NewVisionStorage(nil)
			Expect(narrowStore).NotTo(BeNil())
		})
	})
})
