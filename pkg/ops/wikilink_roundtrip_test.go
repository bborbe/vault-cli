// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("bare Wikilink survival through every entity write path", func() {
	var (
		ctx           context.Context
		vaultPath     string
		storageConfig *storage.Config
	)

	BeforeEach(func() {
		ctx = context.Background()

		var err error
		vaultPath, err = os.MkdirTemp("", "vault-wikilink-roundtrip-*")
		Expect(err).To(BeNil())

		storageConfig = &storage.Config{
			TasksDir:      "24 Tasks",
			GoalsDir:      "23 Goals",
			ThemesDir:     "21 Themes",
			ObjectivesDir: "22 Objectives",
			VisionDir:     "20 Vision",
		}
		for _, dir := range []string{
			"24 Tasks", "23 Goals", "21 Themes", "22 Objectives", "20 Vision", "40 Decisions",
		} {
			Expect(os.MkdirAll(filepath.Join(vaultPath, dir), 0755)).To(Succeed())
		}
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	const entityFixture = `---
priority: 2
related: '[[Already Quoted]]'
related_task: [[Some Other Task]]
status: in_progress
tags:
    - [[A Theme]]
themes: [[A Theme]]
---
Tags: [[Task]]

---
body
`

	// assertWikilinksSurvived re-reads the file from disk and pins every shape the
	// spec's Acceptance Criteria name: the bare scalar became a quoted scalar, the
	// bare list entry became a quoted entry, the already-quoted value is unchanged,
	// no nested block sequence exists anywhere, and the markdown body is intact.
	assertWikilinksSurvived := func(filePath string) {
		raw, err := os.ReadFile(filePath)
		Expect(err).To(BeNil())
		written := string(raw)

		Expect(written).To(ContainSubstring("\nrelated_task: '[[Some Other Task]]'\n"))
		// Scalar stays scalar — spec AC 1 + Desired Behavior 3. Not normalised to a list.
		Expect(written).To(ContainSubstring("\nthemes: '[[A Theme]]'\n"))
		// Block-sequence entry stays an entry.
		Expect(written).To(ContainSubstring("\n    - '[[A Theme]]'\n"))
		Expect(written).To(ContainSubstring("\nrelated: '[[Already Quoted]]'\n"))
		Expect(written).NotTo(MatchRegexp(`(?m)^ *- - `))
		Expect(written).NotTo(ContainSubstring("enzel('[["))
		Expect(written).To(ContainSubstring("\nTags: [[Task]]\n"))
		Expect(written).To(HaveSuffix("\n---\nbody\n"))
		Expect(strings.Count(written, "related_task: '[[Some Other Task]]'")).To(Equal(1))
	}

	DescribeTable("survives a set-and-write cycle",
		func(
			dir string,
			exec func(
				ctx context.Context,
				cfg *storage.Config,
				vaultPath, name, key, value string,
			) error,
		) {
			filePath := filepath.Join(vaultPath, dir, "Repro Entity.md")
			Expect(os.WriteFile(filePath, []byte(entityFixture), 0600)).To(Succeed())

			Expect(exec(ctx, storageConfig, vaultPath, "Repro Entity", "priority", "3")).
				To(Succeed())
			assertWikilinksSurvived(filePath)

			// Idempotence: a second write must leave the wikilink lines unchanged.
			Expect(exec(ctx, storageConfig, vaultPath, "Repro Entity", "priority", "4")).
				To(Succeed())
			assertWikilinksSurvived(filePath)
		},
		Entry("task", "24 Tasks", func(
			ctx context.Context, cfg *storage.Config, vaultPath, name, key, value string,
		) error {
			return ops.NewFrontmatterSetOperation(storage.NewTaskStorage(cfg)).
				Execute(ctx, vaultPath, name, key, value, "", "")
		}),
		Entry("goal", "23 Goals", func(
			ctx context.Context, cfg *storage.Config, vaultPath, name, key, value string,
		) error {
			return ops.NewGoalSetOperation(storage.NewGoalStorage(cfg)).
				Execute(ctx, vaultPath, name, key, value, "", "")
		}),
		Entry("theme", "21 Themes", func(
			ctx context.Context, cfg *storage.Config, vaultPath, name, key, value string,
		) error {
			return ops.NewThemeSetOperation(storage.NewThemeStorage(cfg)).
				Execute(ctx, vaultPath, name, key, value, "", "")
		}),
		Entry("objective", "22 Objectives", func(
			ctx context.Context, cfg *storage.Config, vaultPath, name, key, value string,
		) error {
			return ops.NewObjectiveSetOperation(storage.NewObjectiveStorage(cfg)).
				Execute(ctx, vaultPath, name, key, value, "", "")
		}),
		Entry("vision", "20 Vision", func(
			ctx context.Context, cfg *storage.Config, vaultPath, name, key, value string,
		) error {
			return ops.NewVisionSetOperation(storage.NewVisionStorage(cfg)).
				Execute(ctx, vaultPath, name, key, value, "", "")
		}),
	)

	It("survives a decision write", func() {
		const decisionFixture = `---
needs_review: true
page_type: decision
related: '[[Already Quoted]]'
related_task: [[Some Other Task]]
status: proposed
tags:
    - [[A Theme]]
themes: [[A Theme]]
type: Trading Decision Record
---
Tags: [[Task]]

---
body
`
		filePath := filepath.Join(vaultPath, "40 Decisions", "TDR 2026-01-09 - Repro.md")
		Expect(os.WriteFile(filePath, []byte(decisionFixture), 0600)).To(Succeed())

		decisionStore := storage.NewDecisionStorage(storageConfig)

		decisions, err := decisionStore.ListDecisions(ctx, vaultPath)
		Expect(err).To(BeNil())
		Expect(decisions).To(HaveLen(1))
		Expect(decisionStore.WriteDecision(ctx, decisions[0])).To(Succeed())
		assertWikilinksSurvived(filePath)

		// Idempotence: re-read and write again; the wikilink lines must not move.
		decisions, err = decisionStore.ListDecisions(ctx, vaultPath)
		Expect(err).To(BeNil())
		Expect(decisions).To(HaveLen(1))
		Expect(decisionStore.WriteDecision(ctx, decisions[0])).To(Succeed())
		assertWikilinksSurvived(filePath)
	})

	It("leaves a decision wikilink with a trailing comment broken", func() {
		const commentFixture = `---
needs_review: true
related_task: [[Some Other Task]]  # migrated 2026-01
status: proposed
---
Tags: [[Task]]

---
body
`
		filePath := filepath.Join(vaultPath, "40 Decisions", "TDR Comment.md")
		Expect(os.WriteFile(filePath, []byte(commentFixture), 0600)).To(Succeed())

		decisionStore := storage.NewDecisionStorage(storageConfig)
		decisions, err := decisionStore.ListDecisions(ctx, vaultPath)
		Expect(err).To(BeNil())
		Expect(decisions).To(HaveLen(1))
		Expect(decisionStore.WriteDecision(ctx, decisions[0])).To(Succeed())

		raw, err := os.ReadFile(filePath)
		Expect(err).To(BeNil())
		Expect(string(raw)).To(MatchRegexp(`(?m)^ *- - Some Other Task$`))
	})
})
