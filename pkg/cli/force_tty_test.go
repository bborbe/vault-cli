// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"bytes"
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/bborbe/vault-cli/pkg/cli"
	"github.com/bborbe/vault-cli/pkg/ops"
)

// findSubcommand walks the built command tree to the named top-level command
// and then to the named subcommand.
func findSubcommand(top, sub string) *cobra.Command {
	for _, candidate := range cli.NewRootCommand(context.Background()).Commands() {
		if candidate.Name() != top {
			continue
		}
		for _, nested := range candidate.Commands() {
			if nested.Name() == sub {
				return nested
			}
		}
	}
	return nil
}

var _ = Describe("force requires a terminal", func() {
	Context("requireTTYForForce", func() {
		It("refuses a forced complete when stdin is not a terminal", func() {
			err := cli.RequireTTYForForceForTest(context.Background(), true, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("stdin is not a terminal"))
		})

		It("accepts a forced complete at a terminal", func() {
			Expect(cli.RequireTTYForForceForTest(context.Background(), true, true)).
				To(Succeed())
		})

		It("never gates a non-forced complete", func() {
			Expect(cli.RequireTTYForForceForTest(context.Background(), false, false)).
				To(Succeed())
		})
	})

	Context("through cli.Run", func() {
		It("refuses task complete --force when stdin is not a terminal", func() {
			err := cli.Run(context.Background(), []string{"task", "complete", "Some Task", "--force"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("stdin is not a terminal"))
		})

		It("refuses goal complete --force when stdin is not a terminal", func() {
			err := cli.Run(context.Background(), []string{"goal", "complete", "Some Goal", "--force"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("stdin is not a terminal"))
		})

		It("does not gate a non-forced task complete", func() {
			err := cli.Run(context.Background(), []string{"task", "complete", "Some Task"})
			Expect(err).To(HaveOccurred(), "no task store is reachable, so this must fail before any TTY gate")
			Expect(err.Error()).NotTo(ContainSubstring("stdin is not a terminal"))
		})
	})

	Context("printTaskCompleteResult", func() {
		It("prints warnings in the refusal branch", func() {
			var buf bytes.Buffer
			cli.PrintTaskCompleteResultForTest(&buf, ops.MutationResult{
				Reason:   "3 subtasks are incomplete",
				Warnings: []string{"first warning", "second warning"},
			})
			Expect(buf.String()).To(ContainSubstring("Cannot complete: 3 subtasks are incomplete"))
			Expect(buf.String()).To(ContainSubstring("first warning"))
			Expect(buf.String()).To(ContainSubstring("second warning"))
		})

		It("prints warnings in the success branch", func() {
			var buf bytes.Buffer
			cli.PrintTaskCompleteResultForTest(&buf, ops.MutationResult{
				Name:     "Some Task",
				Warnings: []string{"a warning"},
			})
			Expect(buf.String()).To(ContainSubstring("Task completed: Some Task"))
			Expect(buf.String()).To(ContainSubstring("a warning"))
		})
	})

	Context("flag surface", func() {
		It("does not expose --force on topic complete", func() {
			sub := findSubcommand("topic", "complete")
			Expect(sub).NotTo(BeNil(), "the root command must expose `topic complete`")
			Expect(sub.Flags().Lookup("force")).To(BeNil())
		})

		It("exposes --force on task complete", func() {
			sub := findSubcommand("task", "complete")
			Expect(sub).NotTo(BeNil(), "the root command must expose `task complete`")
			Expect(sub.Flags().Lookup("force")).NotTo(BeNil())
		})

		It("exposes --force on goal complete", func() {
			sub := findSubcommand("goal", "complete")
			Expect(sub).NotTo(BeNil(), "the root command must expose `goal complete`")
			Expect(sub.Flags().Lookup("force")).NotTo(BeNil())
		})
	})
})
