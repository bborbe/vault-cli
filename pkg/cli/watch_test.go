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

	"github.com/bborbe/errors"
	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/cli"
	"github.com/bborbe/vault-cli/pkg/config"
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

var _ = Describe("vault-cli watch --vault", func() {
	var (
		ctx          context.Context
		fakeLoader   *mocks.Loader
		configLoader *config.Loader
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeLoader = &mocks.Loader{}
		var loader config.Loader = fakeLoader
		configLoader = &loader
	})

	It("watches every configured vault when the value is empty", func() {
		fakeLoader.GetAllVaultsReturns([]*config.Vault{
			{Name: "alpha", Path: "/tmp/alpha"},
			{Name: "beta", Path: "/tmp/beta"},
		}, nil)
		vaultName := ""

		vaults, err := cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)

		Expect(err).To(BeNil())
		Expect(vaults).To(HaveLen(2))
		Expect(fakeLoader.GetAllVaultsCallCount()).To(Equal(1))
		Expect(fakeLoader.GetVaultCallCount()).To(Equal(0))
	})

	It("resolves a single name to exactly that vault", func() {
		fakeLoader.GetVaultReturns(&config.Vault{Name: "alpha", Path: "/tmp/alpha"}, nil)
		vaultName := "alpha"

		vaults, err := cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)

		Expect(err).To(BeNil())
		Expect(vaults).To(HaveLen(1))
		Expect(vaults[0].Name).To(Equal("alpha"))
		Expect(fakeLoader.GetVaultCallCount()).To(Equal(1))
		_, firstName := fakeLoader.GetVaultArgsForCall(0)
		Expect(firstName).To(Equal("alpha"))
		Expect(fakeLoader.GetAllVaultsCallCount()).To(Equal(0))
	})

	It("resolves a comma list to both vaults in order", func() {
		fakeLoader.GetVaultReturnsOnCall(0, &config.Vault{Name: "alpha"}, nil)
		fakeLoader.GetVaultReturnsOnCall(1, &config.Vault{Name: "beta"}, nil)
		vaultName := "alpha,beta"

		vaults, err := cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)

		Expect(err).To(BeNil())
		Expect(vaults).To(HaveLen(2))
		Expect(vaults[0].Name).To(Equal("alpha"))
		Expect(vaults[1].Name).To(Equal("beta"))
		_, firstName := fakeLoader.GetVaultArgsForCall(0)
		_, secondName := fakeLoader.GetVaultArgsForCall(1)
		Expect(firstName).To(Equal("alpha"))
		Expect(secondName).To(Equal("beta"))
		Expect(fakeLoader.GetAllVaultsCallCount()).To(Equal(0))
	})

	It("ignores whitespace around each name", func() {
		fakeLoader.GetVaultReturnsOnCall(0, &config.Vault{Name: "alpha"}, nil)
		fakeLoader.GetVaultReturnsOnCall(1, &config.Vault{Name: "beta"}, nil)
		vaultName := " alpha , beta "

		vaults, err := cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)

		Expect(err).To(BeNil())
		Expect(vaults).To(HaveLen(2))
		_, firstName := fakeLoader.GetVaultArgsForCall(0)
		_, secondName := fakeLoader.GetVaultArgsForCall(1)
		Expect(firstName).To(Equal("alpha"))
		Expect(secondName).To(Equal("beta"))
	})

	It("skips empty entries between commas", func() {
		fakeLoader.GetVaultReturnsOnCall(0, &config.Vault{Name: "alpha"}, nil)
		fakeLoader.GetVaultReturnsOnCall(1, &config.Vault{Name: "beta"}, nil)
		vaultName := "alpha,,beta"

		vaults, err := cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)

		Expect(err).To(BeNil())
		Expect(vaults).To(HaveLen(2))
		Expect(fakeLoader.GetVaultCallCount()).To(Equal(2))
		_, firstName := fakeLoader.GetVaultArgsForCall(0)
		_, secondName := fakeLoader.GetVaultArgsForCall(1)
		Expect(firstName).To(Equal("alpha"))
		Expect(secondName).To(Equal("beta"))
	})

	It("tolerates the same name twice without deduplicating", func() {
		fakeLoader.GetVaultReturns(&config.Vault{Name: "alpha", Path: "/tmp/alpha"}, nil)
		vaultName := "alpha,alpha"

		vaults, err := cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)

		Expect(err).To(BeNil())
		Expect(vaults).To(HaveLen(2))
		Expect(vaults[0].Name).To(Equal("alpha"))
		Expect(vaults[1].Name).To(Equal("alpha"))
		Expect(fakeLoader.GetVaultCallCount()).To(Equal(2))
	})

	It("fails when the value is only a comma", func() {
		vaultName := ","

		vaults, err := cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)

		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring(","))
		Expect(fakeLoader.GetVaultCallCount()).To(Equal(0))
		Expect(fakeLoader.GetAllVaultsCallCount()).To(Equal(0))
		Expect(vaults).To(BeEmpty())
	})

	It("fails when the value is only whitespace", func() {
		vaultName := " "

		vaults, err := cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)

		Expect(err).NotTo(BeNil())
		Expect(fakeLoader.GetVaultCallCount()).To(Equal(0))
		Expect(fakeLoader.GetAllVaultsCallCount()).To(Equal(0))
		Expect(vaults).To(BeEmpty())
	})

	It("fails when the value is only separators", func() {
		vaultName := ",,"

		vaults, err := cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)

		Expect(err).NotTo(BeNil())
		Expect(fakeLoader.GetVaultCallCount()).To(Equal(0))
		Expect(fakeLoader.GetAllVaultsCallCount()).To(Equal(0))
		Expect(vaults).To(BeEmpty())
	})

	It("fails the whole call when one name is unresolvable", func() {
		fakeLoader.GetVaultReturnsOnCall(0, &config.Vault{Name: "alpha"}, nil)
		fakeLoader.GetVaultReturnsOnCall(1, nil, errors.Errorf(ctx, "vault not found: nope"))
		vaultName := "alpha,nope"

		vaults, err := cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)

		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("nope"))
		Expect(vaults).To(BeEmpty())
	})

	It("composes trimming with the real case-insensitive config lookup", func() {
		vaultDirA, err := os.MkdirTemp("", "vault-watch-a-*")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = os.RemoveAll(vaultDirA) }()

		vaultDirB, err := os.MkdirTemp("", "vault-watch-b-*")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = os.RemoveAll(vaultDirB) }()

		configContent := fmt.Sprintf(`vaults:
  alpha:
    name: alpha
    path: %s
  beta:
    name: beta
    path: %s
`, vaultDirA, vaultDirB)
		configFile, err := os.CreateTemp("", "vault-config-*.yaml")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = os.Remove(configFile.Name()) }()
		Expect(os.WriteFile(configFile.Name(), []byte(configContent), 0600)).To(Succeed())

		loader := config.NewLoader(configFile.Name())
		vaultName := "ALPHA, beta"

		vaults, err := cli.GetWatchVaultsForTest(ctx, &loader, &vaultName)

		Expect(err).To(BeNil())
		Expect(vaults).To(HaveLen(2))
		Expect(vaults[0].Name).To(Equal("alpha"))
		Expect(vaults[1].Name).To(Equal("beta"))
	})
})
