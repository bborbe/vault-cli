// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/cli"
	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("parseOpenAnswers", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	// The flag grammar is pinned here rather than only end-to-end, so a malformed
	// value is a spec failure instead of a subprocess exit code.
	DescribeTable("refuses a malformed --answer",
		func(raw []string, wantErr string) {
			answers, err := cli.ParseOpenAnswersForTest(ctx, raw)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(wantErr))
			Expect(answers).To(BeNil())
		},
		Entry("no --answer at all", []string{}, "at least one"),
		Entry("no separator", []string{"1"}, "expected <index>=<text>"),
		Entry("index zero", []string{"0=x"}, "not a positive integer"),
		Entry("negative index", []string{"-1=x"}, "not a positive integer"),
		Entry("non-numeric index", []string{"abc=x"}, "not a positive integer"),
		Entry("empty text", []string{"1="}, "must not be empty"),
		Entry("whitespace-only text", []string{"1=   "}, "must not be empty"),
		Entry("a line break in the text", []string{"1=a\nb"}, "must be a single line"),
		Entry("a carriage return in the text", []string{"1=a\rb"}, "must be a single line"),
		Entry("one malformed value in a batch", []string{"1=ok", "bad"}, "expected <index>=<text>"),
	)

	It("keeps everything after the first = as the answer", func() {
		answers, err := cli.ParseOpenAnswersForTest(ctx, []string{"1=a=b"})
		Expect(err).NotTo(HaveOccurred())
		Expect(answers).To(Equal([]domain.OpenAnswer{{Index: 1, Answer: "a=b"}}))
	})

	It("parses a batch in the order given", func() {
		answers, err := cli.ParseOpenAnswersForTest(ctx, []string{"2=second", "1=first"})
		Expect(err).NotTo(HaveOccurred())
		Expect(answers).To(Equal([]domain.OpenAnswer{
			{Index: 2, Answer: "second"},
			{Index: 1, Answer: "first"},
		}))
	})

	It("tolerates whitespace around the index", func() {
		answers, err := cli.ParseOpenAnswersForTest(ctx, []string{" 3 = third "})
		Expect(err).NotTo(HaveOccurred())
		Expect(answers).To(Equal([]domain.OpenAnswer{{Index: 3, Answer: "third"}}))
	})
})
