// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("VaultDispatcher", func() {
	var (
		ctx        context.Context
		dispatcher ops.VaultDispatcher
		err        error
		vaults     []*config.Vault
		callCount  int
	)

	BeforeEach(func() {
		ctx = context.Background()
		dispatcher = ops.NewVaultDispatcher()
		callCount = 0
		vaults = nil
	})

	Context("no vaults", func() {
		BeforeEach(func() {
			vaults = []*config.Vault{}
		})

		JustBeforeEach(func() {
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				callCount++
				return nil
			})
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("no vaults configured")))
		})

		It("never calls the fn", func() {
			Expect(callCount).To(Equal(0))
		})
	})

	Context("single vault, fn succeeds", func() {
		BeforeEach(func() {
			vaults = []*config.Vault{{Name: "vault-a", Path: "/a"}}
		})

		JustBeforeEach(func() {
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				callCount++
				return nil
			})
		})

		It("returns nil", func() {
			Expect(err).To(BeNil())
		})

		It("calls fn once", func() {
			Expect(callCount).To(Equal(1))
		})
	})

	Context("single vault, fn fails", func() {
		var fnErr error

		BeforeEach(func() {
			vaults = []*config.Vault{{Name: "vault-a", Path: "/a"}}
			fnErr = errors.New("op failed")
		})

		JustBeforeEach(func() {
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				callCount++
				return fnErr
			})
		})

		It("returns the error directly (not wrapped with 'not found in any vault')", func() {
			Expect(err).To(MatchError(ContainSubstring("op failed")))
			Expect(err).NotTo(MatchError(ContainSubstring("not found in any vault")))
		})

		It("calls fn once", func() {
			Expect(callCount).To(Equal(1))
		})
	})

	Context("multiple vaults, first succeeds", func() {
		BeforeEach(func() {
			vaults = []*config.Vault{
				{Name: "vault-a", Path: "/a"},
				{Name: "vault-b", Path: "/b"},
			}
		})

		JustBeforeEach(func() {
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				callCount++
				return nil // first vault always succeeds
			})
		})

		It("returns nil", func() {
			Expect(err).To(BeNil())
		})

		It("only calls fn for the first vault", func() {
			Expect(callCount).To(Equal(1))
		})
	})

	Context("multiple vaults, second succeeds", func() {
		BeforeEach(func() {
			vaults = []*config.Vault{
				{Name: "vault-a", Path: "/a"},
				{Name: "vault-b", Path: "/b"},
			}
		})

		JustBeforeEach(func() {
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				callCount++
				if vault.Name == "vault-b" {
					return nil
				}
				return errors.New("not in this vault")
			})
		})

		It("returns nil", func() {
			Expect(err).To(BeNil())
		})

		It("calls fn for both vaults", func() {
			Expect(callCount).To(Equal(2))
		})
	})

	Context("multiple vaults, all fail", func() {
		BeforeEach(func() {
			vaults = []*config.Vault{
				{Name: "vault-a", Path: "/a"},
				{Name: "vault-b", Path: "/b"},
			}
		})

		JustBeforeEach(func() {
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				callCount++
				return errors.New("not found")
			})
		})

		It("returns a wrapped error containing 'not found in any vault'", func() {
			Expect(err).To(MatchError(ContainSubstring("not found in any vault")))
		})

		It("calls fn for all vaults", func() {
			Expect(callCount).To(Equal(2))
		})
	})
})
