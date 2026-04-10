// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("FileMetadata", func() {
	It("is zero-valued correctly", func() {
		var fm domain.FileMetadata
		Expect(fm.Name).To(Equal(""))
		Expect(fm.FilePath).To(Equal(""))
		Expect(fm.ModifiedDate).To(BeNil())
	})
})
