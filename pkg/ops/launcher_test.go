// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	stderrors "errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("ResolveLauncher", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("resolves the launcher path",
		func(vaultScript string, taskLauncher string, goalLaunchers []string, expected string) {
			resolved, err := ops.ResolveLauncher(ctx, vaultScript, taskLauncher, goalLaunchers)
			Expect(err).To(BeNil())
			Expect(resolved).To(Equal(expected))
		},
		Entry("bare task name resolves next to the vault script",
			"/s/cc-private", "cc-private-claude", nil, "/s/cc-private-claude"),
		Entry("nothing set falls back to the vault script",
			"/s/cc-private", "", nil, "/s/cc-private"),
		Entry("bare vault script and bare task name stay relative",
			"claude", "cc-x", nil, "cc-x"),
		Entry("goal launcher is inherited when the task sets none",
			"/s/cc-private", "", []string{"cc-private-claude"}, "/s/cc-private-claude"),
		Entry("task launcher wins over the goal launcher",
			"/s/cc-private", "cc-private", []string{"cc-private-claude"}, "/s/cc-private"),
		Entry("two agreeing goals resolve to that one",
			"/s/cc-private", "", []string{"cc-private-claude", "cc-private-claude"}, "/s/cc-private-claude"),
		Entry("a value containing a slash is used as given",
			"/s/cc-private", "/other/cc-x", nil, "/other/cc-x"),
		Entry("surrounding whitespace is trimmed",
			"/s/cc-private", "  cc-private-claude  ", nil, "/s/cc-private-claude"),
		Entry("empty goal values are ignored",
			"/s/cc-private", "", []string{"", "cc-a"}, "/s/cc-a"),
	)

	DescribeTable("refuses an unsafe launcher value",
		func(taskLauncher string) {
			resolved, err := ops.ResolveLauncher(ctx, "/s/cc-private", taskLauncher, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(taskLauncher))
			Expect(resolved).To(Equal(""))
		},
		Entry("shell command separator", "cc-x; rm -rf /"),
		Entry("command substitution", "cc-$(id)"),
		Entry("space", "cc x"),
		Entry("parent directory segment", "../cc-x"),
		Entry("parent segment inside a path", "/other/../cc-x"),
	)

	It("refuses an unsafe goal launcher value", func() {
		resolved, err := ops.ResolveLauncher(ctx, "/s/cc-private", "", []string{"cc-$(id)"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cc-$(id)"))
		Expect(resolved).To(Equal(""))
	})

	It("names every distinct goal launcher on a conflict", func() {
		resolved, err := ops.ResolveLauncher(
			ctx, "/s/cc-private", "", []string{"cc-a", "cc-b", "cc-a"},
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cc-a"))
		Expect(err.Error()).To(ContainSubstring("cc-b"))
		Expect(resolved).To(Equal(""))
	})
})

var _ = Describe("ResolveTaskLauncher", func() {
	var (
		ctx          context.Context
		goalStorage  *mocks.GoalStorage
		vaultPath    string
		vaultScript  string
		task         *domain.Task
		resolved     string
		warnings     []string
		err          error
		newTask      func(data map[string]any) *domain.Task
		newGoalNamed func(name string, launcher string) *domain.Goal
	)

	BeforeEach(func() {
		ctx = context.Background()
		goalStorage = &mocks.GoalStorage{}
		vaultPath = "/path/to/vault"
		vaultScript = "/s/cc-private"
		newTask = func(data map[string]any) *domain.Task {
			return domain.NewTask(
				data,
				domain.FileMetadata{Name: "my-task", FilePath: "/path/to/vault/Tasks/my-task.md"},
				domain.Content(""),
			)
		}
		newGoalNamed = func(name string, launcher string) *domain.Goal {
			return domain.NewGoal(
				map[string]any{"launcher": launcher},
				domain.FileMetadata{Name: name, FilePath: "/path/to/vault/Goals/" + name + ".md"},
				domain.Content(""),
			)
		}
	})

	JustBeforeEach(func() {
		resolved, warnings, err = ops.ResolveTaskLauncher(
			ctx, goalStorage, vaultPath, vaultScript, task,
		)
	})

	Context("task launcher set", func() {
		BeforeEach(func() {
			task = newTask(map[string]any{"launcher": "cc-x", "goals": []any{"[[Goal A]]"}})
		})

		It("resolves the task's own launcher", func() {
			Expect(err).To(BeNil())
			Expect(resolved).To(Equal("/s/cc-x"))
		})

		It("makes no goal lookup", func() {
			Expect(goalStorage.FindGoalByNameCallCount()).To(Equal(0))
		})

		It("emits no warning", func() {
			Expect(warnings).To(BeEmpty())
		})
	})

	Context("goal launcher inherited", func() {
		BeforeEach(func() {
			task = newTask(map[string]any{"goals": []any{"[[Goal A]]"}})
			goalStorage.FindGoalByNameReturns(newGoalNamed("Goal A", "cc-private-claude"), nil)
		})

		It("resolves the goal's launcher", func() {
			Expect(err).To(BeNil())
			Expect(resolved).To(Equal("/s/cc-private-claude"))
		})

		It("looks the goal up by its bare name", func() {
			Expect(goalStorage.FindGoalByNameCallCount()).To(Equal(1))
			_, actualVaultPath, actualName := goalStorage.FindGoalByNameArgsForCall(0)
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualName).To(Equal("Goal A"))
		})

		It("emits no warning", func() {
			Expect(warnings).To(BeEmpty())
		})
	})

	Context("goal link no longer resolves", func() {
		BeforeEach(func() {
			task = newTask(map[string]any{"goals": []any{"[[Gone Goal]]"}})
			goalStorage.FindGoalByNameReturns(nil, errFakeGoalLookup)
		})

		It("returns the vault default", func() {
			Expect(err).To(BeNil())
			Expect(resolved).To(Equal(vaultScript))
		})

		It("warns naming the missing goal", func() {
			Expect(warnings).To(HaveLen(1))
			Expect(warnings[0]).To(ContainSubstring("Gone Goal"))
		})
	})

	Context("empty goal link", func() {
		BeforeEach(func() {
			task = newTask(map[string]any{"goals": []any{"[[]]"}})
		})

		It("is skipped without a lookup or a warning", func() {
			Expect(err).To(BeNil())
			Expect(resolved).To(Equal(vaultScript))
			Expect(warnings).To(BeEmpty())
			Expect(goalStorage.FindGoalByNameCallCount()).To(Equal(0))
		})
	})

	Context("no launcher anywhere", func() {
		BeforeEach(func() {
			task = newTask(map[string]any{"status": "next"})
		})

		It("returns the vault default", func() {
			Expect(err).To(BeNil())
			Expect(resolved).To(Equal(vaultScript))
		})
	})
})

var _ = Describe("ResolveTaskLauncher boundary", func() {
	It("resolves to a path NewClaudeSessionStarter accepts", func() {
		ctx := context.Background()
		dir := GinkgoT().TempDir()
		vaultScript := dir + "/cc-private"
		launcherPath := dir + "/cc-private-claude"
		Expect(writeExecutableFile(launcherPath)).To(Succeed())

		resolved, err := ops.ResolveLauncher(ctx, vaultScript, "cc-private-claude", nil)
		Expect(err).To(BeNil())
		Expect(resolved).To(Equal(launcherPath))

		Expect(
			ops.NewClaudeSessionStarter(resolved, ops.NewSessionLocker()),
		).NotTo(BeNil())
	})
})

// errFakeGoalLookup stands in for a goal link that no longer resolves.
var errFakeGoalLookup = stderrors.New("goal not found")

// writeExecutableFile writes an empty executable file at path, so exec.LookPath
// can resolve it in the boundary test.
func writeExecutableFile(path string) error {
	return os.WriteFile(path, []byte("#!/bin/sh\n"), 0700)
}
