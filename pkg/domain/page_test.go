// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("Page Flag", func() {
	DescribeTable("coerces the stored value via GetBool",
		func(stored any, expected bool) {
			page := domain.NewPage(
				map[string]any{"status": "todo", "flag": stored},
				domain.FileMetadata{Name: "Task"},
				domain.Content(""),
			)
			Expect(page.Flag()).To(Equal(expected))
		},
		Entry("bool true", true, true),
		Entry("bool false", false, false),
		Entry("string true", "true", true),
		Entry("string no", "no", false),
		Entry("nil value", nil, false),
	)

	It("returns false for a missing key", func() {
		page := domain.NewPage(
			map[string]any{"status": "todo"},
			domain.FileMetadata{Name: "Task"},
			domain.Content(""),
		)
		Expect(page.Flag()).To(BeFalse())
	})
})

var _ = Describe("Page BlockedBy", func() {
	DescribeTable("reads the stored blocked_by value via blockedByList",
		func(stored any, expected []string) {
			page := domain.NewPage(
				map[string]any{"status": "todo", "blocked_by": stored},
				domain.FileMetadata{Name: "Task"},
				domain.Content(""),
			)
			Expect(page.BlockedBy()).To(Equal(expected))
		},
		Entry("plain names", []any{"A", "B"}, []string{"A", "B"}),
		Entry("wikilink names", []any{"[[A]]"}, []string{"[[A]]"}),
		Entry("nil value", nil, nil),
		Entry("scalar string is malformed", "A", nil),
	)

	It("returns an empty list for a missing key", func() {
		page := domain.NewPage(
			map[string]any{"status": "todo"},
			domain.FileMetadata{Name: "Task"},
			domain.Content(""),
		)
		Expect(page.BlockedBy()).To(BeEmpty())
	})
})
