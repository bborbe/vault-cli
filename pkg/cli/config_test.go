// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

var _ = Describe("vault-cli config set-a2a-default-cluster", func() {
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

	captureStdout := func(run func() error) (string, error) {
		origStdout := os.Stdout
		r, w, err := os.Pipe()
		Expect(err).To(BeNil())
		os.Stdout = w
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
		var err error
		tempDir, err = os.MkdirTemp("", "vault-cli-set-a2a-default-cluster-test-*")
		Expect(err).To(BeNil())

		vaultDir = filepath.Join(tempDir, "vault")
		Expect(os.MkdirAll(vaultDir, 0750)).To(Succeed())

		configPath = filepath.Join(tempDir, "config.yaml")
		configData := fmt.Sprintf(`current_user: user@example.com
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
  other:
    name: other
    path: %s
    tasks_dir: "25 Tasks"
`, vaultDir, vaultDir)
		Expect(os.WriteFile(configPath, []byte(configData), 0600)).To(Succeed())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	It("persists the cluster name for a named vault", func() {
		err := cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-a2a-default-cluster", "test", "nukedev"},
		)
		Expect(err).To(BeNil())

		vault, err := config.NewLoader(configPath).GetVault(ctx, "test")
		Expect(err).To(BeNil())
		Expect(vault.A2ADefaultCluster).To(Equal("nukedev"))
	})

	It("writes the literal on-disk key", func() {
		Expect(cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-a2a-default-cluster", "test", "nukedev"},
		)).To(BeNil())

		Expect(string(readBytes(configPath))).
			To(ContainSubstring("a2a_default_cluster: nukedev"))
	})

	It("emits the key in config list --output json", func() {
		Expect(cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-a2a-default-cluster", "test", "nukedev"},
		)).To(BeNil())

		out, err := captureStdout(func() error {
			return cli.Run(ctx, []string{"--config", configPath, "config", "list", "--output", "json"})
		})
		Expect(err).To(BeNil())
		Expect(out).To(ContainSubstring(`"a2a_default_cluster": "nukedev"`))
	})

	It("omits the key for a vault that names no default", func() {
		out, err := captureStdout(func() error {
			return cli.Run(ctx, []string{"--config", configPath, "config", "list", "--output", "json"})
		})
		Expect(err).To(BeNil())
		// The positive guard is asserted before the absence, so an empty or
		// failed run cannot satisfy this spec.
		Expect(out).To(ContainSubstring(`"name": "test"`))
		Expect(out).NotTo(ContainSubstring("a2a_default_cluster"))
	})

	It("emits a hand-edited malformed value verbatim rather than refusing it", func() {
		handEdited := fmt.Sprintf(`vaults:
  test:
    name: test
    path: %s
    a2a_default_cluster: "not a cluster!"
`, vaultDir)
		Expect(os.WriteFile(configPath, []byte(handEdited), 0600)).To(Succeed())

		out, err := captureStdout(func() error {
			return cli.Run(ctx, []string{"--config", configPath, "config", "list", "--output", "json"})
		})
		Expect(err).To(BeNil())
		Expect(out).To(ContainSubstring(`"not a cluster!"`))
	})

	It("refuses a target string rather than a bare name", func() {
		before := readBytes(configPath)

		err := cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-a2a-default-cluster", "test", "k8s:nukedev"},
		)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("k8s:nukedev"))
		Expect(readBytes(configPath)).To(Equal(before))
	})

	It("refuses a vault absent from the config and leaves the file unchanged", func() {
		before := readBytes(configPath)

		err := cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-a2a-default-cluster", "nosuchvault", "nukedev"},
		)
		Expect(err).NotTo(BeNil())
		Expect(readBytes(configPath)).To(Equal(before))
	})

	It("leaves the other vault untouched", func() {
		Expect(cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-a2a-default-cluster", "test", "nukedev"},
		)).To(BeNil())

		Expect(strings.Count(string(readBytes(configPath)), "a2a_default_cluster")).To(Equal(1))

		other, err := config.NewLoader(configPath).GetVault(ctx, "other")
		Expect(err).To(BeNil())
		Expect(other.A2ADefaultCluster).To(BeEmpty())
		Expect(other.TasksDir).To(Equal("25 Tasks"))
	})

	It("requires a vault name and a cluster", func() {
		err := cli.Run(
			ctx,
			[]string{"--config", configPath, "config", "set-a2a-default-cluster", "test"},
		)
		Expect(err).NotTo(BeNil())
	})
})
