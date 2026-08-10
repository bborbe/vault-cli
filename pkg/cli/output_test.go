// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/cli"
)

var _ = Describe("OutputFormat", func() {
	Describe("Validate", func() {
		It("returns no error for OutputFormatPlain", func() {
			ctx := context.Background()
			err := cli.OutputFormatPlain.Validate(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns no error for OutputFormatJSON", func() {
			ctx := context.Background()
			err := cli.OutputFormatJSON.Validate(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error for an invalid value", func() {
			ctx := context.Background()
			err := cli.OutputFormat("invalid").Validate(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid"))
		})
	})

	Describe("IsJSON", func() {
		It("returns true for OutputFormatJSON", func() {
			Expect(cli.OutputFormatJSON.IsJSON()).To(BeTrue())
		})

		It("returns false for OutputFormatPlain", func() {
			Expect(cli.OutputFormatPlain.IsJSON()).To(BeFalse())
		})
	})

	Describe("IsPlain", func() {
		It("returns true for OutputFormatPlain", func() {
			Expect(cli.OutputFormatPlain.IsPlain()).To(BeTrue())
		})

		It("returns false for OutputFormatJSON", func() {
			Expect(cli.OutputFormatJSON.IsPlain()).To(BeFalse())
		})
	})

	Describe("AvailableOutputFormats", func() {
		It("contains plain and json", func() {
			Expect(cli.AvailableOutputFormats.Contains(cli.OutputFormatPlain)).To(BeTrue())
			Expect(cli.AvailableOutputFormats.Contains(cli.OutputFormatJSON)).To(BeTrue())
		})
	})
})
