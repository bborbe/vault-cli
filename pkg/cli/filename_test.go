// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"context"
	"io"
	"os"
	"strings"

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
		// Registered on the current spec so a failing assertion between the
		// assignment and the restore cannot leak a patched stdout into the
		// rest of the suite.
		DeferCleanup(func() {
			os.Stdout = origStdout
		})
		runErr := run()
		Expect(w.Close()).To(Succeed())
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

	It("reads the name from stdin when the argument is -", func() {
		rootCmd := cli.NewRootCommand(ctx)
		rootCmd.SetArgs([]string{"filename", "sanitize", "-"})
		rootCmd.SetIn(strings.NewReader("NUL .txt\n"))
		out, err := captureStdout(func() error {
			return rootCmd.ExecuteContext(ctx)
		})
		Expect(err).To(BeNil())
		Expect(out).To(Equal("NUL_.txt\n"))
	})

	It("falls back to Untitled for an empty stdin", func() {
		rootCmd := cli.NewRootCommand(ctx)
		rootCmd.SetArgs([]string{"filename", "sanitize", "-"})
		rootCmd.SetIn(strings.NewReader("\n"))
		out, err := captureStdout(func() error {
			return rootCmd.ExecuteContext(ctx)
		})
		Expect(err).To(BeNil())
		Expect(out).To(Equal("Untitled\n"))
	})

	It("errors when the name argument is missing", func() {
		_, err := captureStdout(func() error {
			return cli.Run(ctx, []string{"filename", "sanitize"})
		})
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("exactly one filename argument"))
	})
})
