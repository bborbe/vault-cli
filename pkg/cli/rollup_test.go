// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/cli"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("rollup weekly plain report", func() {
	It("renders the three figures, the family block and both rules", func() {
		result := ops.RollupWeeklyResult{
			Year:                 2026,
			Week:                 37,
			WeekStart:            "2026-09-07",
			WeekEnd:              "2026-09-13",
			HumanInteractions:    "26476",
			UnattendedDeliveries: "12",
			PerFamilyMedian:      "52",
			Families: []ops.RollupFamily{
				{Name: "check prometheus alerts", Median: "52"},
				{Name: "start day", Median: "undefined"},
			},
		}

		Expect(cli.FormatRollupWeeklyPlainForTest(result)).To(Equal(`Week: 2026-W37 (2026-09-07 to 2026-09-13)
Human interactions: 26476
Unattended deliveries: 12
Per-family median: 52
  check prometheus alerts: 52
  start day: undefined
Unattended rule: A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0
Grouping rule: A task family is the filename stem with dates, week numbers, versions and month names stripped, compared case-insensitively.
`))
	})

	It("zero-pads the week number to two digits", func() {
		result := ops.RollupWeeklyResult{
			Year:      2026,
			Week:      5,
			WeekStart: "2026-01-26",
			WeekEnd:   "2026-02-01",
			Families:  []ops.RollupFamily{},
		}

		Expect(cli.FormatRollupWeeklyPlainForTest(result)).To(ContainSubstring("Week: 2026-W05 "))
	})

	It("renders undefined and no-data values without substituting zero", func() {
		result := ops.RollupWeeklyResult{
			Year:                 2026,
			Week:                 37,
			WeekStart:            "2026-09-07",
			WeekEnd:              "2026-09-13",
			HumanInteractions:    "12",
			UnattendedDeliveries: "1",
			PerFamilyMedian:      "undefined",
			Families: []ops.RollupFamily{
				{Name: "nightly cleanup", Median: "4"},
				{Name: "weekly review", Median: "undefined"},
			},
		}

		rendered := cli.FormatRollupWeeklyPlainForTest(result)

		Expect(rendered).To(ContainSubstring("  weekly review: undefined"))
		Expect(rendered).NotTo(ContainSubstring("  weekly review: 0"))
		Expect(rendered).NotTo(ContainSubstring("Per-family median: 0"))
	})

	It("renders a no-data week as three no-data lines and no family block", func() {
		result := ops.RollupWeeklyResult{
			Year:                 2026,
			Week:                 20,
			WeekStart:            "2026-05-11",
			WeekEnd:              "2026-05-17",
			HumanInteractions:    "no data",
			UnattendedDeliveries: "no data",
			PerFamilyMedian:      "no data",
			Families:             []ops.RollupFamily{},
		}

		rendered := cli.FormatRollupWeeklyPlainForTest(result)

		Expect(rendered).To(ContainSubstring("Human interactions: no data"))
		Expect(rendered).To(ContainSubstring("Unattended deliveries: no data"))
		Expect(rendered).To(ContainSubstring("Per-family median: no data"))
		Expect(rendered).NotTo(MatchRegexp(`(?m)^\s+[^:]+: 0$`))
		Expect(rendered).To(ContainSubstring("Unattended rule:"))
		Expect(rendered).To(ContainSubstring("Grouping rule:"))
	})

	It("prints the unattended rule sentence verbatim", func() {
		rendered := cli.FormatRollupWeeklyPlainForTest(ops.RollupWeeklyResult{})

		Expect(rendered).To(ContainSubstring(
			"A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0",
		))
		Expect(rendered).NotTo(ContainSubstring("`"))
	})

	It("prints the grouping rule sentence naming the four strip classes", func() {
		rendered := cli.FormatRollupWeeklyPlainForTest(ops.RollupWeeklyResult{})

		Expect(rendered).To(ContainSubstring("Grouping rule:"))
		Expect(rendered).To(ContainSubstring("filename stem"))
		Expect(rendered).To(ContainSubstring("dates"))
		Expect(rendered).To(ContainSubstring("week numbers"))
		Expect(rendered).To(ContainSubstring("versions"))
		Expect(rendered).To(ContainSubstring("month names"))
		Expect(rendered).To(ContainSubstring("case-insensitively"))
	})
})

var _ = Describe("rollup weekly plain report with a baseline", func() {
	// baselineResult is the fully-populated result the first three specs render:
	// the figures of section 2d's frozen report plus the real 2026-09-12 capture.
	baselineResult := func() ops.RollupWeeklyResult {
		return ops.RollupWeeklyResult{
			Year:                 2026,
			Week:                 37,
			WeekStart:            "2026-09-07",
			WeekEnd:              "2026-09-13",
			HumanInteractions:    "39055",
			UnattendedDeliveries: "12",
			PerFamilyMedian:      "113",
			Families: []ops.RollupFamily{
				{Name: "check prometheus alerts", Median: "52"},
				{Name: "start day", Median: "undefined"},
			},
			Baseline: &ops.RollupBaseline{
				Captured:      "2026-09-12",
				HumanTotal:    62485,
				Median:        64,
				Weeks:         map[string]int{"2026-W37": 26476, "2026-W36": 25141},
				AgentCoverage: "1 of 420",
				Deltas: ops.RollupBaselineDeltas{
					HumanInteractions: &ops.RollupBaselineDelta{
						Computed: 39055,
						Baseline: 26476,
						Delta:    12579,
					},
					PerFamilyMedian: &ops.RollupBaselineDelta{
						Computed: 113,
						Baseline: 64,
						Delta:    49,
						Mismatch: true,
					},
				},
			},
		}
	}

	It("renders the baseline block between the week header and the computed figures", func() {
		Expect(cli.FormatRollupWeeklyPlainForTest(baselineResult())).To(Equal(`Week: 2026-W37 (2026-09-07 to 2026-09-13)
Baseline (captured 2026-09-12)
  Human interactions (total): 62485
  Median: 64
  Week 2026-W36: 25141
  Week 2026-W37: 26476
  Agent coverage: 1 of 420
Human interactions: 39055
Unattended deliveries: 12
Per-family median: 113
  check prometheus alerts: 52
  start day: undefined
Unattended rule: A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0
Grouping rule: A task family is the filename stem with dates, week numbers, versions and month names stripped, compared case-insensitively.
Delta (vs baseline captured 2026-09-12)
  Human interactions: 39055 - 26476 = 12579
  Per-family median: 113 - 64 = 49 [definitional mismatch]
`))
	})

	It("renders the delta block last with the frozen marker on the median row", func() {
		rendered := cli.FormatRollupWeeklyPlainForTest(baselineResult())

		Expect(rendered).To(ContainSubstring("Delta (vs baseline captured 2026-09-12)\n"))
		Expect(rendered).To(ContainSubstring("  Human interactions: 39055 - 26476 = 12579\n"))
		Expect(rendered).To(ContainSubstring(
			"  Per-family median: 113 - 64 = 49 [definitional mismatch]\n",
		))
	})

	It("renders no baseline and no delta block when the result carries no baseline", func() {
		result := baselineResult()
		result.Baseline = nil

		rendered := cli.FormatRollupWeeklyPlainForTest(result)

		Expect(rendered).NotTo(MatchRegexp(`(?m)^Baseline`))
		Expect(rendered).NotTo(MatchRegexp(`(?m)^Delta`))
		Expect(rendered).To(HaveSuffix(
			"Grouping rule: A task family is the filename stem with dates, week numbers, versions and month names stripped, compared case-insensitively.\n",
		))
	})

	It("omits the human-interactions row when the week has no stored figure", func() {
		result := baselineResult()
		result.Baseline.Deltas.HumanInteractions = nil

		rendered := cli.FormatRollupWeeklyPlainForTest(result)

		Expect(rendered).To(ContainSubstring("Delta (vs baseline captured"))
		Expect(rendered).To(ContainSubstring(
			"  Per-family median: 113 - 64 = 49 [definitional mismatch]",
		))
		Expect(rendered).NotTo(MatchRegexp(`(?m)^  Human interactions: `))
	})
})

var _ = Describe("rollup weekly command wiring", func() {
	It("wires the rollup weekly command in-process", func() {
		ctx := context.Background()

		vaultDir, err := os.MkdirTemp("", "vault-rollup-*")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = os.RemoveAll(vaultDir) }()

		Expect(os.MkdirAll(filepath.Join(vaultDir, "Tasks"), 0750)).To(Succeed())

		configContent := fmt.Sprintf(`default_vault: test
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
`, vaultDir)
		configFile, err := os.CreateTemp("", "vault-config-*.yaml")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = os.Remove(configFile.Name()) }()
		Expect(os.WriteFile(configFile.Name(), []byte(configContent), 0600)).To(Succeed())

		err = cli.Run(ctx, []string{
			"--config", configFile.Name(),
			"rollup", "weekly",
			"--week", "2026-W37",
		})

		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error for a malformed --week", func() {
		ctx := context.Background()

		vaultDir, err := os.MkdirTemp("", "vault-rollup-*")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = os.RemoveAll(vaultDir) }()

		Expect(os.MkdirAll(filepath.Join(vaultDir, "Tasks"), 0750)).To(Succeed())

		configContent := fmt.Sprintf(`default_vault: test
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
`, vaultDir)
		configFile, err := os.CreateTemp("", "vault-config-*.yaml")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = os.Remove(configFile.Name()) }()
		Expect(os.WriteFile(configFile.Name(), []byte(configContent), 0600)).To(Succeed())

		err = cli.Run(ctx, []string{
			"--config", configFile.Name(),
			"rollup", "weekly",
			"--week", "2026-W5",
		})

		Expect(err).To(HaveOccurred())
		Expect(strings.ToLower(err.Error())).To(ContainSubstring("yyyy-wnn"))
	})
})
