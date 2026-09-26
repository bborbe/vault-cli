// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"

	"github.com/bborbe/errors"
	"github.com/spf13/cobra"

	"github.com/bborbe/vault-cli/pkg/config"
)

// createConfigSetBaselineCommand returns the "config set-baseline" leaf command.
// It writes the vault-relative path of a vault's baseline file onto the vault's
// config entry, so `rollup weekly` can find the file the vault's figures are
// compared against.
func createConfigSetBaselineCommand(
	ctx context.Context,
	configPath *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "set-baseline <vault> <path>",
		Short: "Set a vault's baseline file path",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			writer := config.NewBaselineWriter(*configPath)
			if err := writer.SetBaseline(ctx, args[0], args[1]); err != nil {
				return errors.Wrap(ctx, err, "set baseline")
			}
			return nil
		},
	}
}
