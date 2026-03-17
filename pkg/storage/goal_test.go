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

	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("GoalStorage", func() {
	var (
		ctx      context.Context
		store    storage.Storage
		vaultDir string
		goalsDir string
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = storage.NewStorage(nil)

		var err error
		vaultDir, err = os.MkdirTemp("", "vault-goal-test-*")
		Expect(err).To(BeNil())

		goalsDir = filepath.Join(vaultDir, "Goals")
		Expect(os.MkdirAll(goalsDir, 0755)).To(Succeed())
	})

	AfterEach(func() {
		if vaultDir != "" {
			_ = os.RemoveAll(vaultDir)
		}
	})

	goalContent := func() string {
		return `---
status: active
page_type: goal
---
# Test Goal

This is a test goal.
`
	}

	Describe("FindGoalByName", func() {
		BeforeEach(func() {
			goalPath := filepath.Join(goalsDir, "Test Goal.md")
			Expect(os.WriteFile(goalPath, []byte(goalContent()), 0600)).To(Succeed())
		})

		It("finds goal by bare name", func() {
			goal, err := store.FindGoalByName(ctx, vaultDir, "Test Goal")
			Expect(err).To(BeNil())
			Expect(goal).NotTo(BeNil())
			Expect(goal.Name).To(Equal("Test Goal"))
		})

		It("finds goal by bracket-wrapped name", func() {
			goal, err := store.FindGoalByName(ctx, vaultDir, "[[Test Goal]]")
			Expect(err).To(BeNil())
			Expect(goal).NotTo(BeNil())
			Expect(goal.Name).To(Equal("Test Goal"))
		})

		It("returns error for nonexistent bracket-wrapped name", func() {
			_, err := store.FindGoalByName(ctx, vaultDir, "[[Nonexistent]]")
			Expect(err).NotTo(BeNil())
		})
	})
})
