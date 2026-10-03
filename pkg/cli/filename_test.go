// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"context"
	"io"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/cli"
)

var _ = Describe("filename sanitize command", func() {
	var ctx context.Context

	captureStdout := func(run func() error) (string, error) {
		origStdout := os.Stdout
		r, w, err := os.Pipe()
		Expect(err).To(BeNil())
		os.Stdout = w
		runErr := run()
		Expect(w.Close()).To(Succeed())
		os.Stdout = origStdout
		out, err := io.ReadAll(r)
		Expect(err).To(BeNil())
		return string(out), runErr
	}

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("prints the sanitized name and nothing else", func() {
		out, err := captureStdout(func() error {
			return cli.Run(ctx, []string{"filename", "sanitize", "Fix Build-Fix Planning: Model Prose"})
		})
		Expect(err).To(BeNil())
		Expect(out).To(Equal("Fix Build-Fix Planning - Model Prose\n"))
	})

	It("prints the sanitized name as JSON", func() {
		out, err := captureStdout(func() error {
			return cli.Run(ctx, []string{"--output", "json", "filename", "sanitize", "NUL.txt"})
		})
		Expect(err).To(BeNil())
		Expect(out).To(Equal("{\n  \"filename\": \"NUL_.txt\"\n}\n"))
	})

	It("errors when the name argument is missing", func() {
		_, err := captureStdout(func() error {
			return cli.Run(ctx, []string{"filename", "sanitize"})
		})
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("exactly one filename argument"))
	})
})
