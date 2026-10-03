// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/bborbe/errors"
	"github.com/spf13/cobra"

	"github.com/bborbe/vault-cli/pkg/domain"
)

// stdinNameArgument is the positional argument that tells `filename sanitize`
// to read the name from stdin rather than from argv, so an untrusted title is
// never spliced into a shell command line.
const stdinNameArgument = "-"

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
		Short: "Sanitize a filename so it checks out on Windows",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.Errorf(ctx, "sanitize requires exactly one filename argument")
			}

			raw, err := sanitizeInput(ctx, cmd, args[0])
			if err != nil {
				return err
			}

			name := domain.SanitizeFilename(raw)
			if OutputFormat(*outputFormat).IsJSON() {
				return PrintJSON(map[string]string{"filename": name})
			}

			fmt.Println(name)
			return nil
		},
	}
}

// sanitizeInput returns the name to sanitize: the argument itself, or, when the
// argument is "-", the whole of stdin with a single trailing newline removed.
// Reading stdin lets a caller pipe untrusted text in as data instead of splicing
// it into a shell command line. Empty input is not an error — it sanitizes to
// the "Untitled" fallback.
func sanitizeInput(ctx context.Context, cmd *cobra.Command, arg string) (string, error) {
	if arg != stdinNameArgument {
		return arg, nil
	}
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return "", errors.Wrap(ctx, err, "read filename from stdin failed")
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}
