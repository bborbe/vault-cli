// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("Content", func() {
	It("round-trips through String()", func() {
		s := "# Hello\n\nWorld"
		c := domain.Content(s)
		Expect(c.String()).To(Equal(s))
	})
})
