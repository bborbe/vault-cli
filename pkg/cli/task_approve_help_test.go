// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/bborbe/vault-cli/pkg/cli"
)

// approvalLong returns the constructed `task approve` command's Long help
// text. It reads the built command tree rather than the source literal: Long
// is assembled from adjacent string literals, so a source grep is not the
// same assertion as the text a reader sees from `--help`.
func approvalLong() string {
	var approveCmd *cobra.Command
	for _, top := range cli.NewRootCommand(context.Background()).Commands() {
		if top.Name() != "task" {
			continue
		}
		for _, sub := range top.Commands() {
			if sub.Name() == "approve" {
				approveCmd = sub
			}
		}
	}
	Expect(approveCmd).NotTo(BeNil(), "the root command must expose `task approve`")
	return approveCmd.Long
}

var _ = Describe("task approve help text", func() {
	It("names that an existing approved_at is preserved", func() {
		long := approvalLong()
		Expect(long).To(ContainSubstring("approved_at"))
		Expect(strings.ToLower(long)).To(ContainSubstring("preserv"))
	})

	It("does not describe re-approval with the fresh-timestamp phrase alone", func() {
		long := approvalLong()
		const phrase = "records approved_by and approved_at in one write"
		idx := strings.Index(long, phrase)
		if idx < 0 {
			// Removing the phrase is an acceptable fix: nothing is left to
			// mislead a reader about a fresh timestamp.
			return
		}
		rest := long[idx:]
		preservIdx := strings.Index(strings.ToLower(rest), "preserv")
		Expect(preservIdx).To(BeNumerically(">=", 0),
			"a preservation clause must appear at or after the write phrase")
		Expect(rest[preservIdx:]).To(ContainSubstring("approved_at"),
			"the preservation clause must name approved_at")
	})
})
