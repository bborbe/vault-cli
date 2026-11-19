// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("MutationResult parents JSON", func() {
	decode := func(result ops.MutationResult) map[string]any {
		payload, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		var decoded map[string]any
		Expect(json.Unmarshal(payload, &decoded)).To(Succeed())
		return decoded
	}

	It("omits the parents key when no attempt ran", func() {
		decoded := decode(ops.MutationResult{Success: true, Name: "t"})
		Expect(decoded).NotTo(HaveKey("parents"))
	})

	It("omits open_items on a completed entry", func() {
		decoded := decode(ops.MutationResult{
			Parents: []ops.ParentAttempt{
				{Name: "Goal A", Result: ops.ParentResultCompleted},
			},
		})

		parents, ok := decoded["parents"].([]any)
		Expect(ok).To(BeTrue())
		Expect(parents).To(HaveLen(1))

		entry, ok := parents[0].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(entry["name"]).To(Equal("Goal A"))
		Expect(entry["result"]).To(Equal("completed"))
		Expect(entry).NotTo(HaveKey("open_items"))
	})

	It("carries open_items on a refused entry", func() {
		decoded := decode(ops.MutationResult{
			Parents: []ops.ParentAttempt{
				{
					Name:      "Goal A",
					Result:    ops.ParentResultRefused,
					OpenItems: []string{"box one"},
				},
			},
		})

		parents, ok := decoded["parents"].([]any)
		Expect(ok).To(BeTrue())
		Expect(parents).To(HaveLen(1))

		entry, ok := parents[0].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(entry["result"]).To(Equal("refused"))
		Expect(entry["open_items"]).To(Equal([]any{"box one"}))
	})

	It("keeps both entries in document order", func() {
		decoded := decode(ops.MutationResult{
			Parents: []ops.ParentAttempt{
				{Name: "Goal A", Result: ops.ParentResultCompleted},
				{
					Name:      "Goal B",
					Result:    ops.ParentResultRefused,
					OpenItems: []string{"box two"},
				},
			},
		})

		parents, ok := decoded["parents"].([]any)
		Expect(ok).To(BeTrue())
		Expect(parents).To(HaveLen(2))

		first, ok := parents[0].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(first["name"]).To(Equal("Goal A"))

		second, ok := parents[1].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(second["name"]).To(Equal("Goal B"))
	})
})
