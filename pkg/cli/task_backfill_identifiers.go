// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"

	"github.com/bborbe/errors"
	"github.com/spf13/cobra"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// backfillResultJSON is the per-vault JSON output shape emitted by
// backfill-identifiers.
type backfillResultJSON struct {
	Vault    string `json:"vault"`
	Modified int    `json:"modified"`
	Skipped  int    `json:"skipped"`
}

// createTaskBackfillIdentifiersCommand builds the backfill-identifiers command.
// The operation depends on a vault's task storage config, so it is built per
// vault inside RunE via the injected newBackfillOp factory.
func createTaskBackfillIdentifiersCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
	newBackfillOp func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation,
) *cobra.Command {
	return &cobra.Command{
		Use:   "backfill-identifiers",
		Short: "Assign task_identifier to tasks that are missing one",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			isPlain := OutputFormat(*outputFormat).IsPlain()
			var jsonResults []backfillResultJSON

			for _, vault := range vaults {
				if len(vaults) > 1 && isPlain {
					fmt.Printf("=== %s ===\n", vault.Name)
				}

				storageConfig := storage.NewConfigFromVault(vault)
				result, err := newBackfillOp(storageConfig).Execute(ctx, vault.Path)
				if isPlain {
					fmt.Printf("modified: %d\n", len(result.ModifiedFiles))
					fmt.Printf("skipped: %d\n", result.SkippedFiles)
				}
				if err != nil {
					return errors.Wrapf(ctx, err, "backfill identifiers in vault %s", vault.Name)
				}
				if !isPlain {
					jsonResults = append(jsonResults, backfillResultJSON{
						Vault:    vault.Name,
						Modified: len(result.ModifiedFiles),
						Skipped:  result.SkippedFiles,
					})
				}
			}

			if OutputFormat(*outputFormat).IsJSON() {
				return PrintJSON(jsonResults)
			}
			return nil
		},
	}
}
