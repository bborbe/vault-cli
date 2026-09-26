// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// writeRollupTask writes one task file into the vault's Tasks directory. The
// frontmatter argument is the raw YAML block between the --- delimiters.
func writeRollupTask(vaultPath string, name string, frontmatter string) {
	content := "---\n" + frontmatter + "---\n# " + name + "\n\nBody.\n"
	taskPath := filepath.Join(vaultPath, "Tasks", name+".md")
	Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
}

// rollupTaskFrontmatter renders a task frontmatter block. An empty count omits the
// metrics_interaction_count key entirely — an absent count, not a zero.
func rollupTaskFrontmatter(status string, completedAt string, count string) string {
	frontmatter := "status: " + status + "\n"
	if completedAt != "" {
		frontmatter += "metrics_completed_at: " + completedAt + "\n"
	}
	if count != "" {
		frontmatter += "metrics_interaction_count: " + count + "\n"
	}
	return frontmatter
}

// writeRollupBaseline writes a baseline file at a vault-relative path. The
// frontmatter argument is the raw YAML block between the --- delimiters.
func writeRollupBaseline(vaultPath string, relPath string, frontmatter string) {
	content := "---\n" + frontmatter + "---\n# Baseline\n\nCaptured figures.\n"
	full := filepath.Join(vaultPath, relPath)
	Expect(os.MkdirAll(filepath.Dir(full), 0755)).To(Succeed())
	Expect(os.WriteFile(full, []byte(content), 0600)).To(Succeed())
}

// baselineFrontmatter renders a baseline frontmatter block from its five figures.
// weeks is the pre-rendered body of baseline_weeks, e.g. "  2026-W36: 25141\n".
func baselineFrontmatter(
	captured string, humanTotal int, median int, weeks string, agentCoverage string,
) string {
	return "baseline_captured: " + captured + "\n" +
		"baseline_human_total: " + strconv.Itoa(humanTotal) + "\n" +
		"baseline_median: " + strconv.Itoa(median) + "\n" +
		"baseline_weeks:\n" + weeks +
		"baseline_agent_coverage: \"" + agentCoverage + "\"\n"
}

// realBaselineWeeks is the contract's own example week map: the 2026-09-12 capture.
const realBaselineWeeks = "  2026-W36: 25141\n  2026-W37: 26476\n"

// rollupOpWithBaseline builds a weekly rollup operation pointed at a
// vault-relative baseline path.
func rollupOpWithBaseline(
	currentDateTime libtime.CurrentDateTime,
	relPath string,
) ops.RollupWeeklyOperation {
	return ops.NewRollupWeeklyOperationWithBaseline(
		storage.NewTaskStorage(&storage.Config{TasksDir: "Tasks"}),
		currentDateTime,
		relPath,
	)
}

func assertRollupNoData(result ops.RollupWeeklyResult) {
	Expect(result.HumanInteractions).To(Equal("no data"))
	Expect(result.UnattendedDeliveries).To(Equal("no data"))
	Expect(result.PerFamilyMedian).To(Equal("no data"))
	Expect(result.Families).To(BeEmpty())
	Expect(result.HumanInteractions).NotTo(Equal("0"))
	Expect(result.UnattendedDeliveries).NotTo(Equal("0"))
	Expect(result.PerFamilyMedian).NotTo(Equal("0"))
}

func familyMedians(families []ops.RollupFamily) map[string]string {
	medians := make(map[string]string, len(families))
	for _, family := range families {
		medians[family.Name] = family.Median
	}
	return medians
}

var _ = Describe("RollupWeeklyOperation", func() {
	var (
		ctx             context.Context
		vaultPath       string
		vaultName       string
		currentDateTime libtime.CurrentDateTime
		rollupOp        ops.RollupWeeklyOperation
	)

	BeforeEach(func() {
		ctx = context.Background()
		vaultName = "test-vault"

		var err error
		vaultPath, err = os.MkdirTemp("", "vault-rollup-test-*")
		Expect(err).To(BeNil())
		Expect(os.MkdirAll(filepath.Join(vaultPath, "Tasks"), 0755)).To(Succeed())

		currentDateTime = libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-09-16T12:00:00Z"))
		rollupOp = ops.NewRollupWeeklyOperation(
			storage.NewTaskStorage(&storage.Config{TasksDir: "Tasks"}),
			currentDateTime,
		)
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	It("selects the tasks of a week by the local date of metrics_completed_at", func() {
		// Local Monday 00:30 in +02:00 is week 38; the same instant is Sunday in UTC.
		writeRollupTask(vaultPath, "Local Monday", rollupTaskFrontmatter(
			"completed", "2026-09-14T00:30:00+02:00", "5",
		))
		// Sunday 23:30 UTC is week 37.
		writeRollupTask(vaultPath, "Utc Sunday", rollupTaskFrontmatter(
			"completed", "2026-09-13T23:30:00Z", "3",
		))

		week37, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(week37.HumanInteractions).To(Equal("3"))

		week38, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W38")
		Expect(err).To(BeNil())
		Expect(week38.HumanInteractions).To(Equal("5"))
	})

	It("sums human interactions over the week's task set", func() {
		writeRollupTask(vaultPath, "In Week A", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "4",
		))
		writeRollupTask(vaultPath, "In Week B", rollupTaskFrontmatter(
			"completed", "2026-09-12T10:00:00Z", "6",
		))
		writeRollupTask(vaultPath, "Out Of Week", rollupTaskFrontmatter(
			"completed", "2026-09-20T10:00:00Z", "100",
		))

		result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.HumanInteractions).To(Equal("10"))
	})

	It("counts a completed task with a recorded zero as an unattended delivery", func() {
		writeRollupTask(vaultPath, "Unattended", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "0",
		))
		writeRollupTask(vaultPath, "Attended", rollupTaskFrontmatter(
			"completed", "2026-09-09T10:00:00Z", "4",
		))

		result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.UnattendedDeliveries).To(Equal("1"))
		// The recorded zero still contributes 0 to the sum — a measurement, not an absence.
		Expect(result.HumanInteractions).To(Equal("4"))
	})

	It("excludes a completed task with an absent count from unattended deliveries", func() {
		writeRollupTask(vaultPath, "Recorded Zero", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "0",
		))
		// No metrics_interaction_count key at all — indeterminate, counted in neither direction.
		writeRollupTask(vaultPath, "Absent Count", rollupTaskFrontmatter(
			"completed", "2026-09-09T10:00:00Z", "",
		))
		// A malformed count reads as absent through the accessor, never coerced to 0.
		writeRollupTask(vaultPath, "Malformed Count", rollupTaskFrontmatter(
			"completed", "2026-09-10T10:00:00Z", `"unknown"`,
		))
		// A recorded zero on a non-completed task is not a delivery.
		writeRollupTask(vaultPath, "Zero Not Completed", rollupTaskFrontmatter(
			"todo", "2026-09-11T10:00:00Z", "0",
		))

		result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.UnattendedDeliveries).To(Equal("1"))
		// Only the two recorded zeros are measurements, and both are zero.
		Expect(result.HumanInteractions).To(Equal("0"))
	})

	It("reports no recorded counts for a week with no recorded counts", func() {
		writeRollupTask(vaultPath, "No Count A", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "",
		))
		writeRollupTask(vaultPath, "No Count B", rollupTaskFrontmatter(
			"completed", "2026-09-09T10:00:00Z", "",
		))

		result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.HumanInteractions).To(Equal("no recorded counts"))
		Expect(result.UnattendedDeliveries).To(Equal("no recorded counts"))
		Expect(result.PerFamilyMedian).To(Equal("undefined"))
		Expect(result.Families).To(HaveLen(2))
		for _, family := range result.Families {
			Expect(family.Median).To(Equal("undefined"))
		}
	})

	It("reports no data for a week with no completed task", func() {
		empty, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		assertRollupNoData(empty)

		// A task with no metrics_completed_at at all is in no week.
		writeRollupTask(vaultPath, "No Completion", rollupTaskFrontmatter("completed", "", "3"))
		withoutCompletion, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		assertRollupNoData(withoutCompletion)
	})

	It("reports undefined for a family with no recorded count", func() {
		writeRollupTask(vaultPath, "Alpha Task One", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "4",
		))
		writeRollupTask(vaultPath, "Alpha Task Two", rollupTaskFrontmatter(
			"completed", "2026-09-09T10:00:00Z", "6",
		))
		writeRollupTask(vaultPath, "Beta Task", rollupTaskFrontmatter(
			"completed", "2026-09-10T10:00:00Z", "",
		))

		result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())

		medians := familyMedians(result.Families)
		Expect(medians).To(HaveKey("beta task"))
		Expect(medians["beta task"]).To(Equal("undefined"))
		Expect(medians["alpha task one"]).To(Equal("4"))
		Expect(medians["alpha task two"]).To(Equal("6"))
		// Headline is the median over the defined families only: median(4, 6) = 5.
		Expect(result.PerFamilyMedian).To(Equal("5"))
	})

	It("groups dated, versioned and month-named filenames into one family", func() {
		writeRollupTask(vaultPath, "Check Unassigned Tasks - 2026-09-16", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "2",
		))
		writeRollupTask(vaultPath, "check unassigned tasks W37", rollupTaskFrontmatter(
			"completed", "2026-09-09T10:00:00Z", "4",
		))
		writeRollupTask(vaultPath, "Check Unassigned Tasks v2.1", rollupTaskFrontmatter(
			"completed", "2026-09-10T10:00:00Z", "6",
		))

		result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.Families).To(HaveLen(1))
		Expect(result.Families[0].Name).To(Equal("check unassigned tasks"))
		Expect(result.Families[0].Median).To(Equal("4"))
	})

	It("lists families sorted by name", func() {
		writeRollupTask(vaultPath, "Zebra Task", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "1",
		))
		writeRollupTask(vaultPath, "Alpha Task", rollupTaskFrontmatter(
			"completed", "2026-09-09T10:00:00Z", "2",
		))
		writeRollupTask(vaultPath, "Middle Task", rollupTaskFrontmatter(
			"completed", "2026-09-10T10:00:00Z", "3",
		))

		result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.Families).To(HaveLen(3))
		Expect(result.Families[0].Name).To(Equal("alpha task"))
		Expect(result.Families[1].Name).To(Equal("middle task"))
		Expect(result.Families[2].Name).To(Equal("zebra task"))
	})

	DescribeTable("rejects a malformed week token",
		func(week string) {
			result, err := rollupOp.Execute(ctx, vaultPath, vaultName, week)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("YYYY-Wnn"))
			Expect(result).To(Equal(ops.RollupWeeklyResult{}))
		},
		Entry("one-digit week number", "2026-W5"),
		Entry("lowercase w", "2026-w37"),
		Entry("missing dash", "2026W37"),
		Entry("missing year", "W37"),
		Entry("missing w", "2026-37"),
		Entry("three-digit week number", "2026-W370"),
		Entry("week zero", "2026-W00"),
		Entry("week beyond the year's ISO week count", "2026-W54"),
	)

	It("accepts 2026-W53 because 2026 has 53 ISO weeks", func() {
		result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W53")
		Expect(err).To(BeNil())
		Expect(result.Year).To(Equal(2026))
		Expect(result.Week).To(Equal(53))
		Expect(result.WeekStart).To(Equal("2026-12-28"))
		Expect(result.WeekEnd).To(Equal("2027-01-03"))
	})

	It("defaults to the last complete ISO week", func() {
		// Wednesday of week 38 → the week before now is week 37.
		wednesday, err := rollupOp.Execute(ctx, vaultPath, vaultName, "")
		Expect(err).To(BeNil())
		Expect(wednesday.Year).To(Equal(2026))
		Expect(wednesday.Week).To(Equal(37))
		Expect(wednesday.WeekStart).To(Equal("2026-09-07"))
		Expect(wednesday.WeekEnd).To(Equal("2026-09-13"))

		// Monday of week 38 → still week 37, not the week containing now.
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-09-14T12:00:00Z"))
		monday, err := rollupOp.Execute(ctx, vaultPath, vaultName, "")
		Expect(err).To(BeNil())
		Expect(monday.Year).To(Equal(2026))
		Expect(monday.Week).To(Equal(37))
		Expect(monday.WeekStart).To(Equal("2026-09-07"))

		// Sunday of week 37 → the week before now is week 36, not week 37.
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-09-13T12:00:00Z"))
		sunday, err := rollupOp.Execute(ctx, vaultPath, vaultName, "")
		Expect(err).To(BeNil())
		Expect(sunday.Year).To(Equal(2026))
		Expect(sunday.Week).To(Equal(36))
		Expect(sunday.WeekStart).To(Equal("2026-08-31"))
		Expect(sunday.WeekEnd).To(Equal("2026-09-06"))
	})

	It("returns an error naming an unreadable task file", func() {
		if os.Geteuid() == 0 {
			Skip("running as root — file mode 0000 is not enforced")
		}
		writeRollupTask(vaultPath, "Unreadable Task", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "4",
		))
		taskPath := filepath.Join(vaultPath, "Tasks", "Unreadable Task.md")
		Expect(os.Chmod(taskPath, 0000)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(taskPath, 0600) })

		result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("Unreadable Task.md"))
		Expect(result).To(Equal(ops.RollupWeeklyResult{}))
	})

	It("renders the result under the frozen JSON keys", func() {
		payload, err := json.Marshal(ops.RollupWeeklyResult{
			Families: []ops.RollupFamily{},
		})
		Expect(err).To(BeNil())

		var decoded map[string]json.RawMessage
		Expect(json.Unmarshal(payload, &decoded)).To(Succeed())
		for _, key := range []string{
			"year",
			"week",
			"week_start",
			"week_end",
			"human_interactions",
			"unattended_deliveries",
			"per_family_median",
			"families",
		} {
			Expect(decoded).To(HaveKey(key))
		}
		Expect(string(decoded["families"])).To(Equal("[]"))
	})

	DescribeTable("reduces a filename stem to its family key",
		func(stem string, expected string) {
			Expect(ops.RollupFamilyName(stem)).To(Equal(expected))
		},
		Entry("bare ISO date", "Check Unassigned Tasks - 2026-09-16", "check unassigned tasks"),
		Entry("bare week number", "Check Unassigned Tasks W37", "check unassigned tasks"),
		Entry("bare version", "Check Unassigned Tasks v2.1", "check unassigned tasks"),
		Entry("bare month name", "Check Unassigned Tasks September", "check unassigned tasks"),
		Entry("case-insensitive pair", "CHECK UNASSIGNED TASKS", "check unassigned tasks"),
		Entry("year and week", "Check Unassigned Tasks 2026-W37", "check unassigned tasks"),
		Entry("all-date fallback", "2026-09-16", "2026 09 16"),
		Entry("already plain", "Check Unassigned Tasks", "check unassigned tasks"),
	)

	DescribeTable("returns the median of values",
		func(values []float64, expected float64) {
			Expect(ops.RollupMedian(values)).To(Equal(expected))
		},
		Entry("single element", []float64{5}, 5.0),
		Entry("odd count", []float64{1, 3, 5}, 3.0),
		Entry("even count with a whole mean", []float64{2, 4}, 3.0),
		Entry("even count with a half mean", []float64{1, 2, 3, 4}, 2.5),
		Entry("unsorted input", []float64{5, 1, 3}, 3.0),
	)

	It("reads the baseline figures from the configured file", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md", baselineFrontmatter(
			"2026-09-12", 62485, 64, realBaselineWeeks, "1 of 420",
		))
		writeRollupTask(vaultPath, "Alpha Task", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "4",
		))
		op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.Baseline).NotTo(BeNil())
		// The date is written unquoted in the file, so YAML resolves it as a
		// timestamp; it must render as the stored date, not as a Go time string.
		Expect(result.Baseline.Captured).To(Equal("2026-09-12"))
		Expect(result.Baseline.HumanTotal).To(Equal(62485))
		Expect(result.Baseline.Median).To(Equal(64))
		Expect(result.Baseline.Weeks["2026-W36"]).To(Equal(25141))
		Expect(result.Baseline.Weeks["2026-W37"]).To(Equal(26476))
		Expect(result.Baseline.AgentCoverage).To(Equal("1 of 420"))
	})

	It("reads distinct figures and carries no compiled-in constant", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md", baselineFrontmatter(
			"2026-01-02", 111, 7, "  2026-W36: 222\n  2026-W37: 333\n", "2 of 9",
		))
		writeRollupTask(vaultPath, "Alpha Task", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "4",
		))
		op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.Baseline).NotTo(BeNil())
		Expect(result.Baseline.Captured).To(Equal("2026-01-02"))
		Expect(result.Baseline.HumanTotal).To(Equal(111))
		Expect(result.Baseline.HumanTotal).NotTo(Equal(62485))
		Expect(result.Baseline.Median).To(Equal(7))
		Expect(result.Baseline.Weeks["2026-W36"]).To(Equal(222))
		Expect(result.Baseline.Weeks["2026-W37"]).To(Equal(333))
		Expect(result.Baseline.AgentCoverage).To(Equal("2 of 9"))
		Expect(result.Baseline.Weeks).NotTo(ContainElement(25141))
		Expect(result.Baseline.Weeks).NotTo(ContainElement(26476))
	})

	It("computes the human-interactions delta from the week's stored figure", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md", baselineFrontmatter(
			"2026-01-02", 111, 7, "  2026-W37: 222\n", "2 of 9",
		))
		writeRollupTask(vaultPath, "Alpha Task", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "300",
		))
		writeRollupTask(vaultPath, "Beta Task", rollupTaskFrontmatter(
			"completed", "2026-09-09T10:00:00Z", "200",
		))
		op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.HumanInteractions).To(Equal("500"))
		Expect(result.Baseline).NotTo(BeNil())
		Expect(result.Baseline.Deltas.HumanInteractions).NotTo(BeNil())
		Expect(result.Baseline.Deltas.HumanInteractions.Computed).To(Equal(500.0))
		Expect(result.Baseline.Deltas.HumanInteractions.Baseline).To(Equal(222.0))
		Expect(result.Baseline.Deltas.HumanInteractions.Delta).To(Equal(278.0))
	})

	It("computes the per-family median delta and marks it a definitional mismatch", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md", baselineFrontmatter(
			"2026-01-02", 111, 7, "  2026-W37: 222\n", "2 of 9",
		))
		writeRollupTask(vaultPath, "Alpha Task One", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "4",
		))
		writeRollupTask(vaultPath, "Alpha Task Two", rollupTaskFrontmatter(
			"completed", "2026-09-09T10:00:00Z", "6",
		))
		op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())

		computed, parseErr := strconv.ParseFloat(result.PerFamilyMedian, 64)
		Expect(parseErr).To(BeNil())
		Expect(result.Baseline).NotTo(BeNil())
		Expect(result.Baseline.Deltas.PerFamilyMedian).NotTo(BeNil())
		Expect(result.Baseline.Deltas.PerFamilyMedian.Baseline).To(Equal(7.0))
		Expect(result.Baseline.Deltas.PerFamilyMedian.Computed).To(Equal(computed))
		Expect(result.Baseline.Deltas.PerFamilyMedian.Delta).To(Equal(computed - 7))
		Expect(result.Baseline.Deltas.PerFamilyMedian.Mismatch).To(BeTrue())
	})

	It("omits the human-interactions delta when the requested week is absent from the baseline", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md", baselineFrontmatter(
			"2026-01-02", 111, 7, "  2026-W37: 222\n", "2 of 9",
		))
		writeRollupTask(vaultPath, "Alpha Task", rollupTaskFrontmatter(
			"completed", "2026-05-13T10:00:00Z", "9",
		))
		op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W20")
		Expect(err).To(BeNil())
		Expect(result.Baseline).NotTo(BeNil())
		// The omission is scoped to the delta: the computed figure is still real.
		Expect(result.HumanInteractions).To(Equal("9"))
		Expect(result.Baseline.Deltas.HumanInteractions).To(BeNil())
		Expect(result.Baseline.Deltas.PerFamilyMedian).NotTo(BeNil())
	})

	It("omits a delta row when the computed figure is not a number", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md", baselineFrontmatter(
			"2026-01-02", 111, 7, "  2026-W37: 222\n", "2 of 9",
		))
		writeRollupTask(vaultPath, "No Count Task", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "",
		))
		op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.HumanInteractions).To(Equal("no recorded counts"))
		Expect(result.PerFamilyMedian).To(Equal("undefined"))
		Expect(result.Baseline).NotTo(BeNil())
		Expect(result.Baseline.Deltas).To(Equal(ops.RollupBaselineDeltas{}))
		Expect(result.Baseline.Deltas.HumanInteractions).To(BeNil())
		Expect(result.Baseline.Deltas.PerFamilyMedian).To(BeNil())
	})

	It("carries no baseline when the operation is built without one", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md", baselineFrontmatter(
			"2026-01-02", 111, 7, "  2026-W37: 222\n", "2 of 9",
		))
		writeRollupTask(vaultPath, "Alpha Task", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "4",
		))

		result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.Baseline).To(BeNil())
	})

	It("fails naming the vault and the resolved path when the baseline file is missing", func() {
		op := rollupOpWithBaseline(currentDateTime, "Baselines/absent.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring(vaultName))
		Expect(err.Error()).To(ContainSubstring(filepath.Join(vaultPath, "Baselines/absent.md")))
		Expect(result).To(Equal(ops.RollupWeeklyResult{}))
	})

	It("fails naming the missing frontmatter key", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md",
			"baseline_captured: 2026-01-02\n"+
				"baseline_human_total: 111\n"+
				"baseline_weeks:\n  2026-W37: 222\n"+
				"baseline_agent_coverage: \"2 of 9\"\n",
		)
		op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("baseline_median"))
		Expect(result).To(Equal(ops.RollupWeeklyResult{}))
	})

	It("fails naming the key when a baseline figure is not an integer", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md",
			"baseline_captured: 2026-01-02\n"+
				"baseline_human_total: 111\n"+
				"baseline_median: not-a-number\n"+
				"baseline_weeks:\n  2026-W37: 222\n"+
				"baseline_agent_coverage: \"2 of 9\"\n",
		)
		op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("baseline_median"))
		Expect(result).To(Equal(ops.RollupWeeklyResult{}))
	})

	It("fails naming the key when baseline_weeks carries a non-integer", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md", baselineFrontmatter(
			"2026-01-02", 111, 7, "  2026-W37: many\n", "2 of 9",
		))
		op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("baseline_weeks"))
		Expect(err.Error()).To(ContainSubstring("2026-W37"))
		Expect(result).To(Equal(ops.RollupWeeklyResult{}))
	})

	It("echoes a quoted baseline capture date verbatim", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md", baselineFrontmatter(
			`"2026-01-02"`, 111, 7, "  2026-W37: 222\n", "2 of 9",
		))
		op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())
		Expect(result.Baseline).NotTo(BeNil())
		Expect(result.Baseline.Captured).To(Equal("2026-01-02"))
	})

	DescribeTable("fails naming the key for a malformed baseline figure",
		func(frontmatter string, key string) {
			writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md", frontmatter)
			op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

			result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring(key))
			Expect(result).To(Equal(ops.RollupWeeklyResult{}))
		},
		Entry("capture date absent",
			"baseline_human_total: 111\nbaseline_median: 7\n"+
				"baseline_weeks:\n  2026-W37: 222\nbaseline_agent_coverage: \"2 of 9\"\n",
			"baseline_captured"),
		Entry("capture date empty",
			"baseline_captured: \"\"\nbaseline_human_total: 111\nbaseline_median: 7\n"+
				"baseline_weeks:\n  2026-W37: 222\nbaseline_agent_coverage: \"2 of 9\"\n",
			"baseline_captured"),
		Entry("capture date not a date",
			"baseline_captured: 2026\nbaseline_human_total: 111\nbaseline_median: 7\n"+
				"baseline_weeks:\n  2026-W37: 222\nbaseline_agent_coverage: \"2 of 9\"\n",
			"baseline_captured"),
		Entry("human total absent",
			"baseline_captured: 2026-01-02\nbaseline_median: 7\n"+
				"baseline_weeks:\n  2026-W37: 222\nbaseline_agent_coverage: \"2 of 9\"\n",
			"baseline_human_total"),
		Entry("human total not an integer",
			"baseline_captured: 2026-01-02\nbaseline_human_total: abc\nbaseline_median: 7\n"+
				"baseline_weeks:\n  2026-W37: 222\nbaseline_agent_coverage: \"2 of 9\"\n",
			"baseline_human_total"),
		Entry("weeks absent",
			"baseline_captured: 2026-01-02\nbaseline_human_total: 111\nbaseline_median: 7\n"+
				"baseline_agent_coverage: \"2 of 9\"\n",
			"baseline_weeks"),
		Entry("weeks not a map",
			"baseline_captured: 2026-01-02\nbaseline_human_total: 111\nbaseline_median: 7\n"+
				"baseline_weeks: 5\nbaseline_agent_coverage: \"2 of 9\"\n",
			"baseline_weeks"),
		Entry("agent coverage absent",
			"baseline_captured: 2026-01-02\nbaseline_human_total: 111\nbaseline_median: 7\n"+
				"baseline_weeks:\n  2026-W37: 222\n",
			"baseline_agent_coverage"),
		Entry("agent coverage not a string",
			"baseline_captured: 2026-01-02\nbaseline_human_total: 111\nbaseline_median: 7\n"+
				"baseline_weeks:\n  2026-W37: 222\nbaseline_agent_coverage: 3\n",
			"baseline_agent_coverage"),
	)

	It("marshals the baseline figures and the deltas under a baseline key", func() {
		writeRollupBaseline(vaultPath, "Baselines/rollup-baseline.md", baselineFrontmatter(
			"2026-09-12", 62485, 64, realBaselineWeeks, "1 of 420",
		))
		writeRollupTask(vaultPath, "Alpha Task", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "4",
		))
		op := rollupOpWithBaseline(currentDateTime, "Baselines/rollup-baseline.md")

		result, err := op.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())

		payload, marshalErr := json.Marshal(result)
		Expect(marshalErr).To(BeNil())
		var decoded map[string]any
		Expect(json.Unmarshal(payload, &decoded)).To(Succeed())

		baseline, ok := decoded["baseline"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(baseline["captured"]).To(Equal("2026-09-12"))
		Expect(baseline["human_total"]).To(Equal(float64(62485)))
		Expect(baseline["median"]).To(Equal(float64(64)))
		Expect(baseline["agent_coverage"]).To(Equal("1 of 420"))
		weeks, ok := baseline["weeks"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(weeks["2026-W37"]).To(Equal(float64(26476)))

		deltas, ok := baseline["deltas"].(map[string]any)
		Expect(ok).To(BeTrue())
		medianDelta, ok := deltas["per_family_median"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(medianDelta["mismatch"]).To(Equal(true))
		humanDelta, ok := deltas["human_interactions"].(map[string]any)
		Expect(ok).To(BeTrue())
		_, hasMismatch := humanDelta["mismatch"]
		Expect(hasMismatch).To(BeFalse())
	})

	It("omits the baseline key from the JSON when no baseline is configured", func() {
		writeRollupTask(vaultPath, "Alpha Task", rollupTaskFrontmatter(
			"completed", "2026-09-08T10:00:00Z", "4",
		))

		result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")
		Expect(err).To(BeNil())

		payload, marshalErr := json.Marshal(result)
		Expect(marshalErr).To(BeNil())
		var decoded map[string]any
		Expect(json.Unmarshal(payload, &decoded)).To(Succeed())
		_, ok := decoded["baseline"]
		Expect(ok).To(BeFalse())
	})
})
