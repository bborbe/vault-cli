// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("ResolveResult", func() {
	It("marshals task match correctly", func() {
		result := domain.ResolveResult{Type: "task", Name: "Existing Task Name", Found: true}
		bytes, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(bytes)).To(Equal(`{"type":"task","name":"Existing Task Name","found":true}`))
	})

	It("marshals goal match correctly", func() {
		result := domain.ResolveResult{Type: "goal", Name: "Existing Goal Name", Found: true}
		bytes, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(bytes)).To(Equal(`{"type":"goal","name":"Existing Goal Name","found":true}`))
	})

	It("marshals not found correctly", func() {
		result := domain.ResolveResult{Type: "", Name: "Does Not Exist", Found: false}
		bytes, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(bytes)).To(Equal(`{"type":"","name":"Does Not Exist","found":false}`))
	})
})
