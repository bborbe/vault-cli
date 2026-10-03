// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("SanitizeFilename", func() {
	DescribeTable("sanitizes",
		func(input, expected string) {
			Expect(domain.SanitizeFilename(input)).To(Equal(expected))
		},
		// One entry per forbidden character.
		Entry("colon becomes a spaced dash", "a:b", "a -b"),
		Entry("forward slash becomes a dash", "a/b", "a-b"),
		Entry("backslash becomes a dash", `a\b`, "a-b"),
		Entry("asterisk becomes x", "a*b", "axb"),
		Entry("question mark is removed", "a?b", "ab"),
		Entry("double quote is removed", `a"b`, "ab"),
		Entry("less-than is removed", "a<b", "ab"),
		Entry("greater-than is removed", "a>b", "ab"),
		Entry("pipe is removed", "a|b", "ab"),
		// Rule interactions from the manual cleanup.
		Entry("colon in a title", "Fix Build-Fix Planning: Model Prose", "Fix Build-Fix Planning - Model Prose"),
		Entry("colon with surrounding spaces collapses", "Fix A : B", "Fix A - B"),
		Entry("colon before a word", "Vault-cli Resolve Stops At First Vault On found:false", "Vault-cli Resolve Stops At First Vault On found -false"),
		Entry("colon inside a time", "vault-cli topic defer Test Fails Between Midnight and 02:00 CEST", "vault-cli topic defer Test Fails Between Midnight and 02 -00 CEST"),
		Entry("slash becomes a dash", "The Board Offers No Yes/No Control", "The Board Offers No Yes-No Control"),
		Entry("asterisk becomes x", "Settle Whether the *.needs.json Store Is Retired", "Settle Whether the x.needs.json Store Is Retired"),
		Entry("question mark is removed from a flag", "Support Repeated ?scope= Parameters", "Support Repeated scope= Parameters"),
		// Names that are already safe pass through unchanged.
		Entry("safe name is unchanged", "Build vault-cli Go Tool", "Build vault-cli Go Tool"),
		Entry("em-dash is unchanged", "emdash — ok", "emdash — ok"),
		Entry("umlaut is unchanged", "Über Größe", "Über Größe"),
		Entry("emoji is unchanged", "Ship 🚀 Now", "Ship 🚀 Now"),
		// Reserved device names gain an underscore on the stem.
		Entry("NUL gains an underscore", "NUL", "NUL_"),
		Entry("lowercase nul gains an underscore", "nul", "nul_"),
		Entry("NUL with extension gains an underscore", "NUL.txt", "NUL_.txt"),
		Entry("CON with extension gains an underscore", "CON.md", "CON_.md"),
		Entry("COM1 gains an underscore", "COM1", "COM1_"),
		Entry("LPT9 gains an underscore", "LPT9", "LPT9_"),
		// Non-reserved near-misses are untouched.
		Entry("Console is not reserved", "Console", "Console"),
		Entry("COM0 is not reserved", "COM0", "COM0"),
		Entry("LPT0 is not reserved", "LPT0", "LPT0"),
		Entry("NULx is not reserved", "NULx", "NULx"),
		// Empty results fall back to a placeholder.
		Entry("only removed characters yields Untitled", "???", "Untitled"),
		Entry("only dots yields Untitled", "...", "Untitled"),
		Entry("a lone colon is not empty", ":", "-"),
		Entry("two colons yield two dashes", "::", "- -"),
		Entry("three colons yield three dashes", ":::", "- - -"),
		// Whitespace and dot trimming.
		Entry("whitespace runs collapse", "a   b\tc", "a b c"),
		Entry("leading and trailing spaces are trimmed", "  spaced  ", "spaced"),
		Entry("trailing dot is trimmed", "trailing.", "trailing"),
	)
})
