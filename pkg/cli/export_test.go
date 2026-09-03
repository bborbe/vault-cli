// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// Test-only exports for package cli.
// These are only visible to _test.go files in package cli_test.

// CreateTaskBackfillIdentifiersCommandForTest exposes
// createTaskBackfillIdentifiersCommand for testing.
func CreateTaskBackfillIdentifiersCommandForTest(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
	newBackfillOp func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation,
) *cobra.Command {
	return createTaskBackfillIdentifiersCommand(
		ctx, configLoader, vaultName, outputFormat, newBackfillOp,
	)
}

// CreateResolveCommandForTest exposes createResolveCommand for testing.
func CreateResolveCommandForTest(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
	newResolveOp func(cfg *storage.Config) ops.ResolveOperation,
) *cobra.Command {
	return createResolveCommand(ctx, configLoader, vaultName, outputFormat, newResolveOp)
}
