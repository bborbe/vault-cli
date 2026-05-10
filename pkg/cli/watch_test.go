// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/bborbe/vault-cli/pkg/cli"
)

var _ = Describe("vault-cli watch --types", func() {
	It("returns an error for an unknown type value", func() {
		ctx := context.Background()
		err := cli.Run(ctx, []string{"watch", "--types", "unknown"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown"))
	})

	It("returns an error for multiple values where one is unknown", func() {
		ctx := context.Background()
		err := cli.Run(ctx, []string{"watch", "--types", "task,foo"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("foo"))
	})

	It("returns an error when --types flag is explicitly set to empty string", func() {
		ctx := context.Background()
		err := cli.Run(ctx, []string{"watch", "--types", ""})
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("vault-cli task watch deprecation", func() {
	It(
		"writes a deprecation warning to stderr before streaming and stdout stays JSON-clean",
		func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			vaultDir, err := os.MkdirTemp("", "vault-deprecation-test-*")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.RemoveAll(vaultDir) }()

			tasksDir := filepath.Join(vaultDir, "Tasks")
			Expect(os.MkdirAll(tasksDir, 0750)).To(Succeed())

			configContent := fmt.Sprintf(`vaults:
  test:
    name: test
    path: %s
`, vaultDir)
			configFile, err := os.CreateTemp("", "vault-config-*.yaml")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.Remove(configFile.Name()) }()
			_, err = configFile.WriteString(configContent)
			Expect(err).NotTo(HaveOccurred())
			Expect(configFile.Close()).To(Succeed())

			rootCmd := cli.NewRootCommand(ctx)
			stderrBuf := gbytes.NewBuffer()
			var stdoutBuf bytes.Buffer
			rootCmd.SetErr(stderrBuf)
			rootCmd.SetOut(&stdoutBuf)
			rootCmd.SetArgs([]string{"--config", configFile.Name(), "task", "watch"})

			runDone := make(chan struct{})
			go func() {
				defer close(runDone)
				_ = rootCmd.ExecuteContext(ctx)
			}()

			Eventually(
				stderrBuf,
				2*time.Second,
				20*time.Millisecond,
			).Should(gbytes.Say("deprecated"))

			cancel()
			<-runDone

			Expect(
				stdoutBuf.String(),
			).NotTo(ContainSubstring("deprecated"), "deprecation warning leaked to stdout")
		},
	)
})
