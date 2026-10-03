// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"

	"github.com/bborbe/errors"
	"github.com/spf13/cobra"

	"github.com/bborbe/vault-cli/pkg/domain"
)

// createFilenameCommands returns the parent "filename" command.
func createFilenameCommands(
	ctx context.Context,
	outputFormat *string,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filename",
		Short: "Filename helpers",
	}
	cmd.AddCommand(createFilenameSanitizeCommand(ctx, outputFormat))
	return cmd
}

// createFilenameSanitizeCommand returns the "filename sanitize" leaf command.
//
// It is pure: it neither loads nor resolves a vault, so it works with no vault
// configured and its plain output is the bare name, usable in a shell
// substitution.
func createFilenameSanitizeCommand(
	ctx context.Context,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "sanitize <name>",
		Short: "Sanitize a filename stem so it checks out on Windows",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.Errorf(ctx, "sanitize requires exactly one filename argument")
			}

			name := domain.SanitizeFilename(args[0])
			if OutputFormat(*outputFormat).IsJSON() {
				return PrintJSON(map[string]string{"filename": name})
			}

			fmt.Println(name)
			return nil
		},
	}
}
