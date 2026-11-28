// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

// mustTime parses an RFC3339 instant for file-time fixtures.
func mustTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	Expect(err).NotTo(HaveOccurred())
	return parsed
}

var _ = Describe("ScriptManagerResolver", func() {
	var (
		ctx    context.Context
		script string
	)

	BeforeEach(func() {
		ctx = context.Background()
		script = "/usr/local/bin/manager-for.py"
	})

	// fakeRunner returns the configured result and records the args it saw.
	fakeRunner := func(
		result ops.ManagerResolverCommandResult,
		runErr error,
		seen *[]string,
	) func(context.Context, []string) (ops.ManagerResolverCommandResult, error) {
		return func(_ context.Context, args []string) (ops.ManagerResolverCommandResult, error) {
			if seen != nil {
				*seen = args
			}
			return result, runErr
		}
	}

	It("returns the manager from a non-empty answer", func() {
		resolver := ops.NewScriptManagerResolverWithRunner(
			script,
			fakeRunner(ops.ManagerResolverCommandResult{
				Stdout: []byte(`{"manager":"goal-manager"}`),
			}, nil, nil),
		)

		name, found, err := resolver.LiveManagerForGoal(ctx, "Some Goal", "/vault")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(name).To(Equal("goal-manager"))
	})

	It("treats a null manager as no manager", func() {
		resolver := ops.NewScriptManagerResolverWithRunner(
			script,
			fakeRunner(ops.ManagerResolverCommandResult{
				Stdout: []byte(`{"manager":null}`),
			}, nil, nil),
		)

		name, found, err := resolver.LiveManagerForGoal(ctx, "Some Goal", "/vault")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(name).To(BeEmpty())
	})

	It("treats an empty manager string as no manager", func() {
		resolver := ops.NewScriptManagerResolverWithRunner(
			script,
			fakeRunner(ops.ManagerResolverCommandResult{
				Stdout: []byte(`{"manager":""}`),
			}, nil, nil),
		)

		name, found, err := resolver.LiveManagerForGoal(ctx, "Some Goal", "/vault")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(name).To(BeEmpty())
	})

	It("passes the goal, vault and json flag to the script", func() {
		var seen []string
		resolver := ops.NewScriptManagerResolverWithRunner(
			script,
			fakeRunner(ops.ManagerResolverCommandResult{
				Stdout: []byte(`{"manager":"m"}`),
			}, nil, &seen),
		)

		_, _, err := resolver.LiveManagerForGoal(ctx, "Some Goal", "/vault")
		Expect(err).NotTo(HaveOccurred())
		Expect(seen).To(Equal([]string{
			script, "--goal", "Some Goal", "--vault", "/vault", "--json",
		}))
	})

	It("refuses an unreadable fleet on exit 2", func() {
		resolver := ops.NewScriptManagerResolverWithRunner(
			script,
			fakeRunner(ops.ManagerResolverCommandResult{
				Stderr:   []byte("fleet down"),
				ExitCode: 2,
			}, nil, nil),
		)

		name, found, err := resolver.LiveManagerForGoal(ctx, "Some Goal", "/vault")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("fleet unreadable"))
		Expect(err.Error()).To(ContainSubstring("fleet down"))
		Expect(found).To(BeFalse())
		Expect(name).To(BeEmpty())
	})

	It("refuses any other non-zero exit", func() {
		resolver := ops.NewScriptManagerResolverWithRunner(
			script,
			fakeRunner(ops.ManagerResolverCommandResult{
				Stderr:   []byte("bad args"),
				ExitCode: 1,
			}, nil, nil),
		)

		name, found, err := resolver.LiveManagerForGoal(ctx, "Some Goal", "/vault")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("bad args"))
		Expect(found).To(BeFalse())
		Expect(name).To(BeEmpty())
	})

	It("refuses malformed JSON", func() {
		resolver := ops.NewScriptManagerResolverWithRunner(
			script,
			fakeRunner(ops.ManagerResolverCommandResult{
				Stdout: []byte("not json"),
			}, nil, nil),
		)

		name, found, err := resolver.LiveManagerForGoal(ctx, "Some Goal", "/vault")
		Expect(err).To(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(name).To(BeEmpty())
	})

	It("refuses a runner timeout rather than reporting no manager", func() {
		resolver := ops.NewScriptManagerResolverWithRunner(
			script,
			fakeRunner(ops.ManagerResolverCommandResult{}, context.DeadlineExceeded, nil),
		)

		name, found, err := resolver.LiveManagerForGoal(ctx, "Some Goal", "/vault")
		Expect(err).To(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(name).To(BeEmpty())
	})

	Describe("default search", func() {
		var home string

		BeforeEach(func() {
			home = GinkgoT().TempDir()
			GinkgoT().Setenv("HOME", home)
		})

		It("finds the fallback marketplace script", func() {
			fallback := filepath.Join(
				home, ".claude", "plugins", "marketplaces", "claude-supervisor",
				"scripts", "manager-for.py",
			)
			Expect(os.MkdirAll(filepath.Dir(fallback), 0o750)).To(Succeed())
			Expect(os.WriteFile(fallback, []byte("#!/bin/sh\n"), 0o600)).To(Succeed())

			var seen []string
			resolver := ops.NewScriptManagerResolverWithRunner(
				"",
				fakeRunner(ops.ManagerResolverCommandResult{
					Stdout: []byte(`{"manager":"m"}`),
				}, nil, &seen),
			)

			_, _, err := resolver.LiveManagerForGoal(ctx, "Some Goal", "/vault")
			Expect(err).NotTo(HaveOccurred())
			Expect(seen[0]).To(Equal(fallback))
		})

		It("prefers the newest cached plugin script over the marketplace fallback", func() {
			oldScript := filepath.Join(
				home, ".claude", "plugins", "cache", "market", "supervisor", "1.0.0",
				"scripts", "manager-for.py",
			)
			newScript := filepath.Join(
				home, ".claude", "plugins", "cache", "market", "supervisor", "2.0.0",
				"scripts", "manager-for.py",
			)
			for _, path := range []string{oldScript, newScript} {
				Expect(os.MkdirAll(filepath.Dir(path), 0o750)).To(Succeed())
				Expect(os.WriteFile(path, []byte("#!/bin/sh\n"), 0o600)).To(Succeed())
			}
			// The newest match is chosen by the script's parent directory mtime.
			Expect(os.Chtimes(
				filepath.Dir(oldScript),
				mustTime("2026-01-01T00:00:00Z"),
				mustTime("2026-01-01T00:00:00Z"),
			)).To(Succeed())
			Expect(os.Chtimes(
				filepath.Dir(newScript),
				mustTime("2026-02-01T00:00:00Z"),
				mustTime("2026-02-01T00:00:00Z"),
			)).To(Succeed())

			var seen []string
			resolver := ops.NewScriptManagerResolverWithRunner(
				"",
				fakeRunner(ops.ManagerResolverCommandResult{
					Stdout: []byte(`{"manager":"m"}`),
				}, nil, &seen),
			)

			_, _, err := resolver.LiveManagerForGoal(ctx, "Some Goal", "/vault")
			Expect(err).NotTo(HaveOccurred())
			Expect(seen[0]).To(Equal(newScript))
		})

		It("names the search when nothing is found", func() {
			resolver := ops.NewScriptManagerResolverWithRunner(
				"",
				fakeRunner(ops.ManagerResolverCommandResult{}, nil, nil),
			)

			_, _, err := resolver.LiveManagerForGoal(ctx, "Some Goal", "/vault")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("manager resolver script not found"))
			Expect(err.Error()).To(ContainSubstring(home))
		})
	})
})
