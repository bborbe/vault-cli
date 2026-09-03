// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/bborbe/errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/cli"
	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("Resolve Command", func() {
	var (
		ctx          context.Context
		fakeLoader   *mocks.Loader
		configLoader *config.Loader
		vaultName    string
		outputFormat string
		fakeOp       *mocks.ResolveOperation
	)

	runCommand := func(newResolveOp func(cfg *storage.Config) ops.ResolveOperation) error {
		cmd := cli.CreateResolveCommandForTest(
			ctx, configLoader, &vaultName, &outputFormat, newResolveOp,
		)
		return cmd.RunE(cmd, []string{"my-entity"})
	}

	BeforeEach(func() {
		ctx = context.Background()
		fakeLoader = &mocks.Loader{}
		var loader config.Loader = fakeLoader
		configLoader = &loader
		vaultName = ""
		outputFormat = "plain"
		fakeOp = &mocks.ResolveOperation{}
	})

	It("tries every vault when a vault resolves found:false", func() {
		fakeLoader.GetAllVaultsReturns([]*config.Vault{
			{Name: "alpha", Path: "/tmp/alpha"},
			{Name: "beta", Path: "/tmp/beta"},
		}, nil)
		fakeOp.ExecuteReturns(domain.ResolveResult{Type: "", Name: "my-entity", Found: false}, nil)

		err := runCommand(func(cfg *storage.Config) ops.ResolveOperation {
			return fakeOp
		})

		Expect(err).To(BeNil())
		Expect(fakeOp.ExecuteCallCount()).To(Equal(2))
	})

	It("stops at the first vault that resolves found:true", func() {
		fakeLoader.GetAllVaultsReturns([]*config.Vault{
			{Name: "alpha", Path: "/tmp/alpha"},
			{Name: "beta", Path: "/tmp/beta"},
		}, nil)
		fakeOp.ExecuteReturns(domain.ResolveResult{Type: "task", Name: "my-entity", Found: true}, nil)

		err := runCommand(func(cfg *storage.Config) ops.ResolveOperation {
			return fakeOp
		})

		Expect(err).To(BeNil())
		Expect(fakeOp.ExecuteCallCount()).To(Equal(1))
	})

	It("emits a single found:false JSON document after every vault missed", func() {
		fakeLoader.GetAllVaultsReturns([]*config.Vault{
			{Name: "alpha", Path: "/tmp/alpha"},
			{Name: "beta", Path: "/tmp/beta"},
		}, nil)
		fakeOp.ExecuteReturns(domain.ResolveResult{Type: "", Name: "my-entity", Found: false}, nil)
		outputFormat = "json"

		origStdout := os.Stdout
		r, w, err := os.Pipe()
		Expect(err).To(BeNil())
		os.Stdout = w
		runErr := runCommand(func(cfg *storage.Config) ops.ResolveOperation {
			return fakeOp
		})
		Expect(w.Close()).To(Succeed())
		os.Stdout = origStdout
		out, err := io.ReadAll(r)
		Expect(err).To(BeNil())

		Expect(runErr).To(BeNil())
		Expect(strings.Count(string(out), `"found"`)).To(Equal(1))
		Expect(string(out)).To(ContainSubstring(`"found": false`))
		Expect(string(out)).To(ContainSubstring(`"type": ""`))
	})

	It("propagates a hard vault error and emits no JSON", func() {
		fakeLoader.GetAllVaultsReturns([]*config.Vault{
			{Name: "alpha", Path: "/tmp/alpha"},
			{Name: "beta", Path: "/tmp/beta"},
		}, nil)
		fakeOp.ExecuteReturns(
			domain.ResolveResult{},
			errors.Errorf(ctx, "list tasks: no such directory"),
		)
		outputFormat = "json"

		origStdout := os.Stdout
		r, w, err := os.Pipe()
		Expect(err).To(BeNil())
		os.Stdout = w
		runErr := runCommand(func(cfg *storage.Config) ops.ResolveOperation {
			return fakeOp
		})
		Expect(w.Close()).To(Succeed())
		os.Stdout = origStdout
		out, err := io.ReadAll(r)
		Expect(err).To(BeNil())

		Expect(runErr).NotTo(BeNil())
		Expect(string(out)).To(BeEmpty())
	})
})
