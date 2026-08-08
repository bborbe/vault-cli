// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("IsOwnDailyNoteEntry", func() {
	Describe("IsOwnDailyNoteEntry accepts what the daily-note writers produce", func() {
		It("accepts both writer formats", func() {
			for _, line := range []string{
				fmt.Sprintf("- [/] [[%s]]", "Turn on hell - 2026W32-sat"), // workon.appendTaskToDaily
				fmt.Sprintf("- [ ] [[%s]]", "Turn on hell - 2026W32-sat"), // defer.addToDailyNote
			} {
				matches := storage.CheckboxRegex.FindStringSubmatch(line)
				Expect(matches).To(HaveLen(4))
				Expect(
					ops.IsOwnDailyNoteEntry(matches[3], "Turn on hell - 2026W32-sat"),
				).To(BeTrue())
			}
		})
	})

	DescribeTable(
		"IsOwnDailyNoteEntry",
		func(checkboxText string, taskName string, expected bool) {
			Expect(ops.IsOwnDailyNoteEntry(checkboxText, taskName)).To(Equal(expected))
		},
		Entry(
			"bare wikilink is the own entry",
			"[[Turn on hell - 2026W32-sat]]",
			"Turn on hell - 2026W32-sat",
			true,
		),
		Entry(
			"wikilink with trailing prose is the own entry",
			"[[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today",
			"Turn on hell - 2026W32-sat",
			true,
		),
		Entry(
			"prose before the wikilink is a mention",
			"🔧 Nuke-reboot chain — [[Turn on hell - 2026W32-sat]].",
			"Turn on hell - 2026W32-sat",
			false,
		),
		Entry(
			"second wikilink in a chain summary is a mention",
			"🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].",
			"Turn on hell - 2026W32-sat",
			false,
		),
		Entry(
			"alias wikilink is the own entry",
			"[[Turn on hell - 2026W32-sat|hell]] — due today",
			"Turn on hell - 2026W32-sat",
			true,
		),
		Entry(
			"heading wikilink is the own entry",
			"[[Turn on hell - 2026W32-sat#Steps]]",
			"Turn on hell - 2026W32-sat",
			true,
		),
		Entry(
			"prefix task name does not match a longer task",
			"[[Plan Weekend - 2026W32-sat]]",
			"Plan Week",
			false,
		),
		Entry(
			"task name with regex metacharacters matches literally",
			"[[Fix (urgent) c++ build]]",
			"Fix (urgent) c++ build",
			true,
		),
		Entry(
			"regex metacharacters are not treated as a pattern",
			"[[Fix XurgentX cXX build]]",
			"Fix (urgent) c++ build",
			false,
		),
		Entry(
			"lowercase task name matches a capitalised wikilink",
			"[[Turn on hell - 2026W32-sat]]",
			"turn on hell - 2026w32-sat",
			true,
		),
		Entry("empty checkbox text is not an own entry", "", "Turn on hell - 2026W32-sat", false),
		Entry(
			"unterminated wikilink is not an own entry",
			"[[Turn on hell - 2026W32-sat",
			"Turn on hell - 2026W32-sat",
			false,
		),
		Entry(
			"leading whitespace before the wikilink is tolerated",
			"  [[Turn on hell - 2026W32-sat]]",
			"Turn on hell - 2026W32-sat",
			true,
		),
	)
})
