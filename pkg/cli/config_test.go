// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/cli"
	"github.com/bborbe/vault-cli/pkg/config"
)

var _ = Describe("vault-cli config set-baseline", func() {
	var (
		ctx        context.Context
		tempDir    string
		vaultDir   string
		configPath string
	)

	readBytes := func(path string) []byte {
		b, err := os.ReadFile(path)
		Expect(err).To(BeNil())
		return b
	}

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		tempDir, err = os.MkdirTemp("", "vault-cli-set-baseline-test-*")
		Expect(err).To(BeNil())

		vaultDir = filepath.Join(tempDir, "vault")
		Expect(os.MkdirAll(vaultDir, 0750)).To(Succeed())

		configPath = filepath.Join(tempDir, "config.yaml")
		configData := fmt.Sprintf(`vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
`, vaultDir)
		Expect(os.WriteFile(configPath, []byte(configData), 0600)).To(Succeed())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	It("persists the baseline key for a named vault", func() {
		err := cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-baseline", "test", "60 Baseline.md"},
		)
		Expect(err).To(BeNil())

		vault, err := config.NewLoader(configPath).GetVault(ctx, "test")
		Expect(err).To(BeNil())
		Expect(vault.Baseline).To(Equal("60 Baseline.md"))
	})

	It("returns an error for a vault absent from the config and leaves the file unchanged", func() {
		before := readBytes(configPath)

		err := cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-baseline", "nope", "b.md"},
		)
		Expect(err).NotTo(BeNil())
		Expect(readBytes(configPath)).To(Equal(before))
	})

	It("returns an error for an absolute path and leaves the file unchanged", func() {
		before := readBytes(configPath)

		err := cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-baseline", "test", "/etc/passwd"},
		)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("/etc/passwd"))
		Expect(readBytes(configPath)).To(Equal(before))
	})

	It("returns an error for a path escaping the vault root and leaves the file unchanged", func() {
		before := readBytes(configPath)

		err := cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-baseline", "test", "../../outside.md"},
		)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("../../outside.md"))
		Expect(readBytes(configPath)).To(Equal(before))
	})

	It("requires a vault name and a path", func() {
		err := cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-baseline", "test"},
		)
		Expect(err).NotTo(BeNil())
	})
})
