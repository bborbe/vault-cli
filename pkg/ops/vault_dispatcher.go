// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/config"
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
// Multiple vaults wraps the last error with "not found in any vault" if all fail.
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
		if err := fn(vault); err == nil {
			return nil
		} else { //nolint:revive // else after return is clearer here
			lastErr = err
		}
	}
	return errors.Wrap(ctx, lastErr, "not found in any vault")
}
