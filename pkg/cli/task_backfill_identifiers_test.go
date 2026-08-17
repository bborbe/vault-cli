// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"context"
	"io"
	"os"

	"github.com/bborbe/errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/cli"
	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("Task Backfill Identifiers Command", func() {
	var (
		ctx          context.Context
		fakeLoader   *mocks.Loader
		configLoader *config.Loader
		vaultName    string
		outputFormat string
		fakeOp       *mocks.EnsureAllTaskIdentifiersOperation
	)

	runCommand := func(newBackfillOp func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation) error {
		cmd := cli.CreateTaskBackfillIdentifiersCommandForTest(
			ctx, configLoader, &vaultName, &outputFormat, newBackfillOp,
		)
		return cmd.RunE(cmd, nil)
	}

	BeforeEach(func() {
		ctx = context.Background()
		fakeLoader = &mocks.Loader{}
		var loader config.Loader = fakeLoader
		configLoader = &loader
		vaultName = ""
		outputFormat = "plain"
		fakeOp = &mocks.EnsureAllTaskIdentifiersOperation{}
		fakeOp.ExecuteReturns(ops.BackfillResult{}, nil)
	})

	It("invokes the backfill operation once for each vault when no vault is selected", func() {
		fakeLoader.GetAllVaultsReturns([]*config.Vault{
			{Name: "alpha", Path: "/tmp/alpha"},
			{Name: "beta", Path: "/tmp/beta"},
		}, nil)
		vaultName = ""

		err := runCommand(func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation {
			return fakeOp
		})

		Expect(err).To(BeNil())
		Expect(fakeOp.ExecuteCallCount()).To(Equal(2))
		_, vaultPath := fakeOp.ExecuteArgsForCall(0)
		Expect(vaultPath).To(Equal("/tmp/alpha"))
		_, vaultPath = fakeOp.ExecuteArgsForCall(1)
		Expect(vaultPath).To(Equal("/tmp/beta"))
	})

	It("invokes the backfill operation exactly once for the selected vault", func() {
		fakeLoader.GetVaultReturns(&config.Vault{Name: "alpha", Path: "/tmp/alpha"}, nil)
		vaultName = "alpha"

		err := runCommand(func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation {
			return fakeOp
		})

		Expect(err).To(BeNil())
		Expect(fakeOp.ExecuteCallCount()).To(Equal(1))
		_, vaultPath := fakeOp.ExecuteArgsForCall(0)
		Expect(vaultPath).To(Equal("/tmp/alpha"))
	})

	It("builds the operation from the vault it is about to process", func() {
		fakeLoader.GetVaultReturns(&config.Vault{
			Name:     "alpha",
			Path:     "/tmp/alpha",
			TasksDir: "Custom Tasks",
		}, nil)
		vaultName = "alpha"
		var gotTasksDirs []string

		err := runCommand(func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation {
			gotTasksDirs = append(gotTasksDirs, cfg.TasksDir)
			return fakeOp
		})

		Expect(err).To(BeNil())
		Expect(fakeOp.ExecuteCallCount()).To(Equal(1))
		Expect(gotTasksDirs).To(Equal([]string{"Custom Tasks"}))
	})

	It("returns the error when the backfill operation fails", func() {
		fakeLoader.GetVaultReturns(&config.Vault{Name: "alpha", Path: "/tmp/alpha"}, nil)
		vaultName = "alpha"
		fakeOp.ExecuteReturns(
			ops.BackfillResult{},
			errors.Errorf(ctx, "list tasks: no such directory"),
		)

		err := runCommand(func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation {
			return fakeOp
		})

		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("alpha"))
	})

	It("prints the partial counts and still returns the error when the run is cancelled", func() {
		fakeLoader.GetVaultReturns(&config.Vault{Name: "alpha", Path: "/tmp/alpha"}, nil)
		vaultName = "alpha"
		fakeOp.ExecuteReturns(ops.BackfillResult{
			ModifiedFiles: []string{"/tmp/alpha/Tasks/A.md"},
			SkippedFiles:  0,
		}, context.Canceled)

		origStdout := os.Stdout
		r, w, err := os.Pipe()
		Expect(err).To(BeNil())
		os.Stdout = w
		runErr := runCommand(func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation {
			return fakeOp
		})
		Expect(w.Close()).To(Succeed())
		os.Stdout = origStdout
		out, err := io.ReadAll(r)
		Expect(err).To(BeNil())

		Expect(runErr).NotTo(BeNil())
		Expect(string(out)).To(ContainSubstring("modified: 1"))
	})

	It("reports the modified and skipped counts it received", func() {
		fakeLoader.GetVaultReturns(&config.Vault{Name: "alpha", Path: "/tmp/alpha"}, nil)
		vaultName = "alpha"
		fakeOp.ExecuteReturns(ops.BackfillResult{
			ModifiedFiles: []string{"/tmp/alpha/Tasks/A.md"},
			SkippedFiles:  2,
		}, nil)

		err := runCommand(func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation {
			return fakeOp
		})

		Expect(err).To(BeNil())
	})
})
