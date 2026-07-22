// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
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
				// "not in this vault" is a not-found-class error so the loop continues
				return fmt.Errorf("find task: %w", storage.ErrNotFound)
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
				return fmt.Errorf("find task: %w", storage.ErrNotFound)
			})
		})

		It("returns a wrapped error containing 'not found in any vault'", func() {
			Expect(err).To(MatchError(ContainSubstring("not found in any vault")))
		})

		It("calls fn for all vaults", func() {
			Expect(callCount).To(Equal(2))
		})

		It("the returned error satisfies errors.Is for storage.ErrNotFound", func() {
			Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())
		})
	})

	Context("multiple vaults, non-not-found error in first vault", func() {
		BeforeEach(func() {
			vaults = []*config.Vault{
				{Name: "vault-a", Path: "/a"},
				{Name: "vault-b", Path: "/b"},
			}
		})

		JustBeforeEach(func() {
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				callCount++
				return errors.New("incomplete subtasks: 3 pending")
			})
		})

		It("returns the error directly without wrapping it", func() {
			Expect(err).To(MatchError(ContainSubstring("incomplete subtasks: 3 pending")))
			Expect(err).NotTo(MatchError(ContainSubstring("not found in any vault")))
		})

		It("does not fall through to the second vault", func() {
			Expect(callCount).To(Equal(1))
		})

		It("the error does not satisfy errors.Is for storage.ErrNotFound", func() {
			Expect(errors.Is(err, storage.ErrNotFound)).To(BeFalse())
		})
	})

	Context("multiple vaults, precondition error when owning vault is not last", func() {
		BeforeEach(func() {
			vaults = []*config.Vault{
				{Name: "vault-a", Path: "/a"},
				{Name: "vault-b", Path: "/b"},
			}
		})

		JustBeforeEach(func() {
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				callCount++
				if vault.Name == "vault-a" {
					return fmt.Errorf("find task: %w", storage.ErrNotFound)
				}
				return errors.New("incomplete subtasks: 7 pending")
			})
		})

		It("returns the precondition error from the owning vault", func() {
			Expect(err).To(MatchError(ContainSubstring("incomplete subtasks: 7 pending")))
			Expect(err).NotTo(MatchError(ContainSubstring("not found in any vault")))
		})

		It("called fn for both vaults", func() {
			Expect(callCount).To(Equal(2))
		})
	})
})
