// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/cli"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("printParents", func() {
	It("prints one completed line per completed attempt", func() {
		var buf bytes.Buffer
		cli.PrintParentsForTest(&buf, []ops.ParentAttempt{
			{Name: "Goal A", Result: ops.ParentResultCompleted},
			{Name: "Goal B", Result: ops.ParentResultCompleted},
		})

		Expect(buf.String()).To(Equal("parent Goal A: completed\nparent Goal B: completed\n"))
	})

	It("prints a refused line with every open item joined by semicolons", func() {
		var buf bytes.Buffer
		cli.PrintParentsForTest(&buf, []ops.ParentAttempt{
			{
				Name:      "Goal A",
				Result:    ops.ParentResultRefused,
				OpenItems: []string{"box one", "box two"},
			},
		})

		Expect(buf.String()).To(Equal("parent Goal A: refused — open: box one; box two\n"))
	})

	It("prints nothing for an empty attempt list", func() {
		var buf bytes.Buffer
		cli.PrintParentsForTest(&buf, nil)

		Expect(buf.String()).To(BeEmpty())
	})
})
