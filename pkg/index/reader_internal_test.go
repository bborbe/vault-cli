// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index

import (
	"context"
	"unsafe"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("projectOpenQuestions", func() {
	ginkgo.It("does not retain open-question text that shares memory with the file body", func() {
		body := "## Open Questions\n\n- first retained question\n- second retained question\n"
		questions, err := projectOpenQuestions(context.Background(), body)
		Expect(err).NotTo(HaveOccurred())
		Expect(questions).To(HaveLen(2))

		bodyStart := uintptr(unsafe.Pointer(unsafe.StringData(body)))
		bodyEnd := bodyStart + uintptr(len(body))
		for _, question := range questions {
			Expect(question.Text).NotTo(BeEmpty())
			textStart := uintptr(unsafe.Pointer(unsafe.StringData(question.Text)))
			outside := textStart+uintptr(len(question.Text)) <= bodyStart || textStart >= bodyEnd
			Expect(outside).To(BeTrue(), "text %q shares memory with the body", question.Text)
		}
	})
})
