// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/vault-dispatcher.go --fake-name VaultDispatcher . VaultDispatcher

// VaultDispatcher tries a callback against each vault until one succeeds.
type VaultDispatcher interface {
	FirstSuccess(
		ctx context.Context,
		vaults []*config.Vault,
		fn func(vault *config.Vault) error,
	) error
}

// NewVaultDispatcher creates a new VaultDispatcher.
func NewVaultDispatcher() VaultDispatcher {
	return &vaultDispatcher{}
}

type vaultDispatcher struct{}

// FirstSuccess calls fn for each vault in order, returning nil on the first success.
// Empty vaults returns an error. Single vault calls fn directly without wrapping the error.
// Multiple vaults: only a storage.ErrNotFound-class error allows the loop to continue;
// any other error (e.g. a precondition failure) is returned immediately, unwrapped.
// If all vaults return ErrNotFound-class errors, the last one is wrapped as "not found in any vault".
func (d *vaultDispatcher) FirstSuccess(
	ctx context.Context,
	vaults []*config.Vault,
	fn func(vault *config.Vault) error,
) error {
	if len(vaults) == 0 {
		return errors.Errorf(ctx, "no vaults configured")
	}
	if len(vaults) == 1 {
		return fn(vaults[0])
	}
	var lastErr error
	for _, vault := range vaults {
		err := fn(vault)
		if err == nil {
			return nil
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return err
		}
		lastErr = err
	}
	return errors.Wrap(ctx, lastErr, "not found in any vault")
}
