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

var _ = Describe("ObjectiveStorage", func() {
	var (
		ctx           context.Context
		store         storage.Storage
		vaultPath     string
		objectivesDir string
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = storage.NewStorage(nil)

		var err error
		vaultPath, err = os.MkdirTemp("", "vault-objective-test-*")
		Expect(err).To(BeNil())

		objectivesDir = filepath.Join(vaultPath, "22 Objectives")
		Expect(os.MkdirAll(objectivesDir, 0755)).To(Succeed())
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	objectiveContent := func() string {
		return `---
status: active
page_type: objective
priority: 1
assignee: bborbe
tags:
  - growth
---
# Test Objective

This is a test objective.
`
	}

	Describe("ReadObjective", func() {
		It("reads an objective successfully", func() {
			objectivePath := filepath.Join(objectivesDir, "Test Objective.md")
			Expect(os.WriteFile(objectivePath, []byte(objectiveContent()), 0600)).To(Succeed())

			obj, err := store.ReadObjective(ctx, vaultPath, "Test Objective")
			Expect(err).To(BeNil())
			Expect(obj).NotTo(BeNil())
			Expect(obj.Name).To(Equal("Test Objective"))
			Expect(obj.Status).To(Equal(domain.ObjectiveStatusActive))
			Expect(obj.PageType).To(Equal("objective"))
			Expect(obj.Priority).To(Equal(domain.Priority(1)))
			Expect(obj.Assignee).To(Equal("bborbe"))
			Expect(obj.Tags).To(ContainElement("growth"))
		})

		It("returns error when objective file does not exist", func() {
			_, err := store.ReadObjective(ctx, vaultPath, "Nonexistent Objective")
			Expect(err).NotTo(BeNil())
		})
	})

	Describe("WriteObjective", func() {
		It("writes and reads back an objective correctly", func() {
			objectivePath := filepath.Join(objectivesDir, "New Objective.md")
			newObjective := &domain.Objective{
				Name:     "New Objective",
				FilePath: objectivePath,
				Status:   domain.ObjectiveStatusActive,
				PageType: "objective",
				Priority: 2,
				Assignee: "alice",
			}

			Expect(store.WriteObjective(ctx, newObjective)).To(Succeed())

			obj, err := store.ReadObjective(ctx, vaultPath, "New Objective")
			Expect(err).To(BeNil())
			Expect(obj.Name).To(Equal("New Objective"))
			Expect(obj.Status).To(Equal(domain.ObjectiveStatusActive))
			Expect(obj.Priority).To(Equal(domain.Priority(2)))
			Expect(obj.Assignee).To(Equal("alice"))
		})

		It("returns error when writing to read-only directory", func() {
			readOnlyVault, err := os.MkdirTemp("", "vault-readonly-*")
			Expect(err).To(BeNil())
			defer func() { _ = os.RemoveAll(readOnlyVault) }()

			readOnlyObjectivesDir := filepath.Join(readOnlyVault, "22 Objectives")
			Expect(os.MkdirAll(readOnlyObjectivesDir, 0755)).To(Succeed())
			Expect(os.Chmod(readOnlyObjectivesDir, 0444)).To(Succeed())

			objective := &domain.Objective{
				Name:     "Read-Only Objective",
				FilePath: filepath.Join(readOnlyObjectivesDir, "Read-Only Objective.md"),
				Status:   domain.ObjectiveStatusActive,
			}

			err = store.WriteObjective(ctx, objective)
			Expect(err).NotTo(BeNil())
		})

		It("excludes metadata fields from frontmatter", func() {
			objectivePath := filepath.Join(objectivesDir, "Metadata Test.md")
			objective := &domain.Objective{
				Name:     "Metadata Test",
				FilePath: objectivePath,
				Status:   domain.ObjectiveStatusCompleted,
				PageType: "objective",
				Content: `---
status: completed
page_type: objective
---
# Metadata Test
`,
			}

			Expect(store.WriteObjective(ctx, objective)).To(Succeed())

			rawBytes, err := os.ReadFile(objectivePath)
			Expect(err).To(BeNil())
			rawContent := string(rawBytes)

			Expect(rawContent).NotTo(ContainSubstring("name:"))
			Expect(rawContent).NotTo(ContainSubstring("content:"))
			Expect(rawContent).NotTo(ContainSubstring("filepath:"))
			Expect(rawContent).To(ContainSubstring("status: completed"))
		})
	})

	Describe("FindObjectiveByName", func() {
		BeforeEach(func() {
			objectivePath := filepath.Join(objectivesDir, "Test Objective.md")
			Expect(os.WriteFile(objectivePath, []byte(objectiveContent()), 0600)).To(Succeed())
		})

		It("finds objective by exact name", func() {
			obj, err := store.FindObjectiveByName(ctx, vaultPath, "Test Objective")
			Expect(err).To(BeNil())
			Expect(obj).NotTo(BeNil())
			Expect(obj.Name).To(Equal("Test Objective"))
			Expect(obj.Status).To(Equal(domain.ObjectiveStatusActive))
		})

		It("finds objective by partial name", func() {
			obj, err := store.FindObjectiveByName(ctx, vaultPath, "test")
			Expect(err).To(BeNil())
			Expect(obj).NotTo(BeNil())
			Expect(obj.Name).To(Equal("Test Objective"))
		})

		It("returns error when objective not found", func() {
			_, err := store.FindObjectiveByName(ctx, vaultPath, "Nonexistent")
			Expect(err).NotTo(BeNil())
		})

		It("round-trips objective with FindObjectiveByName after WriteObjective", func() {
			newObj := &domain.Objective{
				Name:     "Round Trip Objective",
				FilePath: filepath.Join(objectivesDir, "Round Trip Objective.md"),
				Status:   domain.ObjectiveStatusOnHold,
				PageType: "objective",
				Tags:     []string{"test", "roundtrip"},
			}

			Expect(store.WriteObjective(ctx, newObj)).To(Succeed())

			found, err := store.FindObjectiveByName(ctx, vaultPath, "Round Trip Objective")
			Expect(err).To(BeNil())
			Expect(found).NotTo(BeNil())
			Expect(found.Name).To(Equal("Round Trip Objective"))
			Expect(found.Status).To(Equal(domain.ObjectiveStatusOnHold))
			Expect(found.Tags).To(Equal([]string{"test", "roundtrip"}))
		})
	})

	Describe("NewObjectiveStorage", func() {
		It("creates a narrow ObjectiveStorage", func() {
			narrowStore := storage.NewObjectiveStorage(nil)
			Expect(narrowStore).NotTo(BeNil())
		})
	})
})
