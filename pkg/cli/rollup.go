// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	"github.com/spf13/cobra"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// unattendedRuleSentence is the vault's written unattended-delivery rule, printed
// verbatim so a reader can check the figure without leaving the terminal. It is a
// frozen literal: rewording it is a behaviour change, not a style choice.
const unattendedRuleSentence = "A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0"

// groupingRuleSentence is the vault's written filename-stem grouping rule,
// printed verbatim. It is a frozen literal, like the unattended rule above.
const groupingRuleSentence = "A task family is the filename stem with dates, week numbers, versions and month names stripped, compared case-insensitively."

// createRollupCommands returns the parent "rollup" command.
func createRollupCommands(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollup",
		Short: "Weekly rollup of vault task metrics",
	}
	cmd.AddCommand(createRollupWeeklyCommand(ctx, configLoader, vaultName, outputFormat))
	return cmd
}

// createRollupWeeklyCommand returns the "rollup weekly" leaf command.
//
// The leaf deliberately diverges from the shared multi-vault resolver: every
// other command with a single vault name goes through `getVaults`, whose
// no-flag default is every configured vault, whereas this command resolves
// exactly one vault through the loader's `GetVault` with an empty name, which
// falls back to the config's default_vault key. The three figures are
// properties of one task population, so summing Personal with Brogrammers
// would produce a number that answers no question anyone asked. `watch` is the
// only other command that leaves the all-vaults default behind.
func createRollupWeeklyCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	var week string

	cmd := &cobra.Command{
		Use:   "weekly",
		Short: "Report one ISO week's task metrics for a single vault",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vault, err := (*configLoader).GetVault(ctx, *vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vault")
			}

			storageConfig := storage.NewConfigFromVault(vault)
			taskStore := storage.NewTaskStorage(storageConfig)
			rollupOp := ops.NewRollupWeeklyOperation(taskStore, libtime.NewCurrentDateTime())
			result, err := rollupOp.Execute(ctx, vault.Path, vault.Name, week)
			if err != nil {
				return err
			}
			if OutputFormat(*outputFormat).IsJSON() {
				return PrintJSON(result)
			}
			fmt.Print(formatRollupWeeklyPlain(result))
			return nil
		},
	}

	cmd.Flags().
		StringVar(&week, "week", "", "ISO week to report, e.g. 2026-W37 (default: the last complete ISO week)")
	return cmd
}

// formatRollupWeeklyPlain renders a rollup result as the plain-text report. It
// is pure: it builds and returns the text, and the caller owns printing.
func formatRollupWeeklyPlain(result ops.RollupWeeklyResult) string {
	var builder strings.Builder
	writeRollupWeeklyHeader(&builder, result)
	writeRollupWeeklyFigures(&builder, result)
	writeRollupWeeklyFamilies(&builder, result.Families)
	writeRollupWeeklyRules(&builder)
	return builder.String()
}

// writeRollupWeeklyHeader writes the week token and its Monday-to-Sunday range.
func writeRollupWeeklyHeader(builder *strings.Builder, result ops.RollupWeeklyResult) {
	fmt.Fprintf(
		builder,
		"Week: %d-W%02d (%s to %s)\n",
		result.Year,
		result.Week,
		result.WeekStart,
		result.WeekEnd,
	)
}

// writeRollupWeeklyFigures writes the three headline figures at column 0. Every
// value is written verbatim: a figure with no measurement behind it arrives as
// "undefined" or "no data" and is never replaced by a zero here.
func writeRollupWeeklyFigures(builder *strings.Builder, result ops.RollupWeeklyResult) {
	fmt.Fprintf(builder, "Human interactions: %s\n", result.HumanInteractions)
	fmt.Fprintf(builder, "Unattended deliveries: %s\n", result.UnattendedDeliveries)
	fmt.Fprintf(builder, "Per-family median: %s\n", result.PerFamilyMedian)
}

// writeRollupWeeklyFamilies writes one two-space-indented line per family,
// directly beneath the headline. An empty family list writes nothing.
func writeRollupWeeklyFamilies(builder *strings.Builder, families []ops.RollupFamily) {
	for _, family := range families {
		fmt.Fprintf(builder, "  %s: %s\n", family.Name, family.Median)
	}
}

// writeRollupWeeklyRules writes the two rule sentences the figures were computed
// under, each single-sourced from its constant.
func writeRollupWeeklyRules(builder *strings.Builder) {
	fmt.Fprintf(builder, "Unattended rule: %s\n", unattendedRuleSentence)
	fmt.Fprintf(builder, "Grouping rule: %s\n", groupingRuleSentence)
}
