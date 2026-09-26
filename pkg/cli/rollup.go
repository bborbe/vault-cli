// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
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

// rollupDefinitionalMismatchMarker is the frozen marker the per-family median's
// delta row carries: the stored figure is a per-task median and the computed one
// is a median of per-family medians. It is a frozen literal — rewording it is a
// behaviour change, not a style choice — and it is the plain-report form of the
// JSON's `mismatch: true`.
const rollupDefinitionalMismatchMarker = "[definitional mismatch]"

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

			if vault.Baseline != "" {
				if err := config.ValidateBaselinePath(ctx, vault.Baseline); err != nil {
					return errors.Wrapf(ctx, err, "baseline of vault %s", vault.Name)
				}
			}

			storageConfig := storage.NewConfigFromVault(vault)
			taskStore := storage.NewTaskStorage(storageConfig)
			rollupOp := ops.NewRollupWeeklyOperationWithBaseline(
				taskStore,
				libtime.NewCurrentDateTime(),
				vault.Baseline,
			)
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
	writeRollupWeeklyBaseline(&builder, result)
	writeRollupWeeklyFigures(&builder, result)
	writeRollupWeeklyFamilies(&builder, result.Families)
	writeRollupWeeklyRules(&builder)
	writeRollupWeeklyDeltas(&builder, result)
	return builder.String()
}

// writeRollupWeeklyBaseline writes the stored baseline figures directly beneath
// the week header and above the computed figures. A result with no baseline
// writes nothing at all, which is what keeps the no-baseline report identical to
// the pre-baseline report. Every value is echoed exactly as the file stores it:
// nothing here recomputes, rounds or reformats a baseline figure.
func writeRollupWeeklyBaseline(builder *strings.Builder, result ops.RollupWeeklyResult) {
	if result.Baseline == nil {
		return
	}
	fmt.Fprintf(builder, "Baseline (captured %s)\n", result.Baseline.Captured)
	fmt.Fprintf(builder, "  Human interactions (total): %d\n", result.Baseline.HumanTotal)
	fmt.Fprintf(builder, "  Median: %d\n", result.Baseline.Median)

	// Weeks is a map, so its keys are copied and sorted: iterating it directly
	// would make the report's line order non-deterministic. The YYYY-Wnn keys are
	// zero-padded, so a lexicographic sort is also a chronological one.
	tokens := make([]string, 0, len(result.Baseline.Weeks))
	for token := range result.Baseline.Weeks {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	for _, token := range tokens {
		fmt.Fprintf(builder, "  Week %s: %d\n", token, result.Baseline.Weeks[token])
	}

	fmt.Fprintf(builder, "  Agent coverage: %s\n", result.Baseline.AgentCoverage)
}

// writeRollupWeeklyDeltas writes the delta block as the report's final block: one
// row per computed figure that has a baseline analogue. A figure with no analogue
// has no row, and a row whose pointer is nil is omitted entirely rather than
// rendered as a zero. A result with no baseline writes nothing.
func writeRollupWeeklyDeltas(builder *strings.Builder, result ops.RollupWeeklyResult) {
	if result.Baseline == nil {
		return
	}
	fmt.Fprintf(builder, "Delta (vs baseline captured %s)\n", result.Baseline.Captured)

	if delta := result.Baseline.Deltas.HumanInteractions; delta != nil {
		fmt.Fprintf(
			builder,
			"  Human interactions: %s - %s = %s\n",
			formatRollupNumber(delta.Computed),
			formatRollupNumber(delta.Baseline),
			formatRollupNumber(delta.Delta),
		)
	}
	if delta := result.Baseline.Deltas.PerFamilyMedian; delta != nil {
		marker := ""
		if delta.Mismatch {
			marker = " " + rollupDefinitionalMismatchMarker
		}
		fmt.Fprintf(
			builder,
			"  Per-family median: %s - %s = %s%s\n",
			formatRollupNumber(delta.Computed),
			formatRollupNumber(delta.Baseline),
			formatRollupNumber(delta.Delta),
			marker,
		)
	}
}

// formatRollupNumber renders a delta value the same way the computed figures are
// rendered elsewhere: the shortest decimal form, so an integer prints without a
// decimal point and a fractional median prints as it was computed. Using one
// formatter for the computed value, the stored value and the delta is what keeps
// the row's `<computed>` equal to the figure the same run prints above it.
func formatRollupNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
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
