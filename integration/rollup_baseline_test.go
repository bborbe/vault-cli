// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

// createTempVaultWithBaseline creates a temporary vault whose config names
// baselineFile as the vault's baseline, writes baselineContent at that
// vault-relative path, and returns the vault path, the config path and a cleanup
// func. Modelled on createTempVaultWithTopics in integration/cli_test.go.
//
// An empty baselineFile omits the `baseline:` line entirely — the no-baseline
// fixture. An empty baselineContent writes no file at all, so a config that names
// baselineFile then points at a path with nothing at it.
func createTempVaultWithBaseline(
	tasks map[string]string,
	baselineFile string,
	baselineContent string,
) (vaultPath string, configPath string, cleanup func()) {
	var err error
	vaultPath, err = os.MkdirTemp("", "vault-*")
	Expect(err).NotTo(HaveOccurred())

	tasksDir := filepath.Join(vaultPath, "Tasks")
	err = os.MkdirAll(tasksDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	for name, content := range tasks {
		taskPath := filepath.Join(tasksDir, name+".md")
		err = os.WriteFile(taskPath, []byte(content), 0600)
		Expect(err).NotTo(HaveOccurred())
	}

	baselineLine := ""
	if baselineFile != "" {
		baselineLine = fmt.Sprintf("    baseline: %s\n", baselineFile)
	}

	configContent := fmt.Sprintf(`default_vault: test
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
%s`, vaultPath, baselineLine)

	configFile, err := os.CreateTemp("", "vault-config-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	_, err = configFile.WriteString(configContent)
	Expect(err).NotTo(HaveOccurred())
	err = configFile.Close()
	Expect(err).NotTo(HaveOccurred())

	if baselineContent != "" {
		baselinePath := filepath.Join(vaultPath, baselineFile)
		err = os.WriteFile(baselinePath, []byte(baselineContent), 0600)
		Expect(err).NotTo(HaveOccurred())
	}

	return vaultPath, configFile.Name(), func() {
		_ = os.RemoveAll(vaultPath)
		_ = os.Remove(configFile.Name())
	}
}

// baselineDocument renders a complete baseline markdown file: the five frozen
// frontmatter keys and a markdown body. weeks is the pre-rendered body of
// baseline_weeks, e.g. "  2026-W36: 25141\n  2026-W37: 26476\n".
func baselineDocument(
	captured string,
	humanTotal string,
	median string,
	weeks string,
	agentCoverage string,
) string {
	return fmt.Sprintf(`---
baseline_captured: %s
baseline_human_total: %s
baseline_median: %s
baseline_weeks:
%sbaseline_agent_coverage: "%s"
---

Baseline fixture body.
`, captured, humanTotal, median, weeks, agentCoverage)
}

const (
	realBaselineWeeks    = "  2026-W36: 25141\n  2026-W37: 26476\n"
	scratchBaselineWeeks = "  2026-W36: 222\n  2026-W37: 333\n"
)

// rollupBaselineFixtureTasks is the task set every spec in this file runs
// against: one task completed inside 2026-W37 carrying a recorded count, so the
// week has a computed figure for each label the delta block maps.
var rollupBaselineFixtureTasks = map[string]string{
	"Rollup Baseline Fixture - 2026-09-08": `---
status: completed
page_type: task
metrics_completed_at: "2026-09-08T10:00:00+02:00"
metrics_interaction_count: 5
---
Fixture body.
`,
}

var _ = Describe("vault-cli rollup weekly baseline", func() {
	// runRollup starts the real binary against the given config and returns the
	// finished session. It never asserts the exit code: the failing specs below
	// need the non-zero case.
	runRollup := func(configPath string, args ...string) *gexec.Session {
		fullArgs := append([]string{"--config", configPath, "--vault", "test"}, args...)
		cmd := exec.Command(binPath, fullArgs...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		return session
	}

	// computedFigure reads one headline figure out of a run's own stdout, so a
	// delta assertion compares against the number the same run printed rather
	// than against a constant baked into this file.
	computedFigure := func(stdout string, label string) string {
		re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(label) + `: (\S+)$`)
		matches := re.FindStringSubmatch(stdout)
		Expect(matches).NotTo(BeNil(), "no %q line in output:\n%s", label, stdout)
		return matches[1]
	}

	// minus renders the arithmetic a delta row must carry, in the same shortest
	// decimal form the report uses.
	minus := func(computed string, baseline string) string {
		computedValue, err := strconv.ParseFloat(computed, 64)
		Expect(err).NotTo(HaveOccurred())
		baselineValue, err := strconv.ParseFloat(baseline, 64)
		Expect(err).NotTo(HaveOccurred())
		return strconv.FormatFloat(computedValue-baselineValue, 'f', -1, 64)
	}

	It("prints the baseline block from the configured file", func() {
		_, configPath, cleanup := createTempVaultWithBaseline(
			rollupBaselineFixtureTasks,
			"baseline.md",
			baselineDocument("2026-09-12", "62485", "64", realBaselineWeeks, "1 of 420"),
		)
		defer cleanup()

		session := runRollup(configPath, "rollup", "weekly", "--week", "2026-W37")
		Eventually(session).Should(gexec.Exit(0))

		out := string(session.Out.Contents())
		Expect(out).To(ContainSubstring("Baseline (captured 2026-09-12)"))
		Expect(out).To(ContainSubstring("  Human interactions (total): 62485"))
		Expect(out).To(ContainSubstring("  Median: 64"))
		Expect(out).To(ContainSubstring("  Week 2026-W36: 25141"))
		Expect(out).To(ContainSubstring("  Week 2026-W37: 26476"))
		Expect(out).To(ContainSubstring("  Agent coverage: 1 of 420"))
	})

	It("prints the fixture's own figures and not the real ones", func() {
		_, configPath, cleanup := createTempVaultWithBaseline(
			rollupBaselineFixtureTasks,
			"baseline.md",
			baselineDocument("2026-01-02", "111", "7", scratchBaselineWeeks, "2 of 9"),
		)
		defer cleanup()

		session := runRollup(configPath, "rollup", "weekly", "--week", "2026-W37")
		Eventually(session).Should(gexec.Exit(0))

		out := string(session.Out.Contents())
		Expect(out).To(ContainSubstring("Baseline (captured 2026-01-02)"))
		Expect(out).To(ContainSubstring("  Human interactions (total): 111"))
		Expect(out).To(ContainSubstring("  Median: 7"))
		Expect(out).To(ContainSubstring("  Week 2026-W36: 222"))
		Expect(out).To(ContainSubstring("  Week 2026-W37: 333"))
		Expect(out).To(ContainSubstring("  Agent coverage: 2 of 9"))

		Expect(out).NotTo(ContainSubstring("62485"))
		Expect(out).NotTo(ContainSubstring("25141"))
		Expect(out).NotTo(ContainSubstring("26476"))
	})

	It("prints the delta block last with one row per analogue", func() {
		_, configPath, cleanup := createTempVaultWithBaseline(
			rollupBaselineFixtureTasks,
			"baseline.md",
			baselineDocument("2026-01-02", "111", "7", scratchBaselineWeeks, "2 of 9"),
		)
		defer cleanup()

		session := runRollup(configPath, "rollup", "weekly", "--week", "2026-W37")
		Eventually(session).Should(gexec.Exit(0))

		out := string(session.Out.Contents())
		humanComputed := computedFigure(out, "Human interactions")
		medianComputed := computedFigure(out, "Per-family median")

		Expect(out).To(ContainSubstring("Delta (vs baseline captured 2026-01-02)"))
		Expect(out).To(ContainSubstring(fmt.Sprintf(
			"  Human interactions: %s - 333 = %s\n",
			humanComputed,
			minus(humanComputed, "333"),
		)))
		Expect(out).To(ContainSubstring(fmt.Sprintf(
			"  Per-family median: %s - 7 = %s [definitional mismatch]\n",
			medianComputed,
			minus(medianComputed, "7"),
		)))

		// Block order: baseline header, then the computed figures, then the delta
		// header last.
		baselineIndex := strings.Index(out, "Baseline (captured 2026-01-02)")
		figuresIndex := strings.Index(out, "\nHuman interactions: ")
		deltaIndex := strings.Index(out, "Delta (vs baseline captured 2026-01-02)")
		Expect(baselineIndex).To(BeNumerically(">=", 0))
		Expect(figuresIndex).To(BeNumerically(">", baselineIndex))
		Expect(deltaIndex).To(BeNumerically(">", figuresIndex))

		// No-analogue assertion, scoped to the delta block: the unattended figure
		// legitimately prints above the block, so only the tail is checked.
		tail := out[deltaIndex:]
		Expect(tail).NotTo(ContainSubstring("Unattended deliveries"))
	})

	It("prints no baseline and no delta block when no baseline is configured", func() {
		_, configPath, cleanup := createTempVaultWithBaseline(
			rollupBaselineFixtureTasks,
			"",
			"",
		)
		defer cleanup()

		session := runRollup(configPath, "rollup", "weekly", "--week", "2026-W37")
		Eventually(session).Should(gexec.Exit(0))

		out := string(session.Out.Contents())
		Expect(out).To(ContainSubstring("Human interactions: "))
		Expect(out).To(ContainSubstring("Unattended deliveries: "))
		Expect(out).To(ContainSubstring("Per-family median: "))
		Expect(out).NotTo(ContainSubstring("Baseline ("))
		Expect(out).NotTo(ContainSubstring("Delta ("))
	})

	It("fails loudly when the baseline file is missing", func() {
		vaultPath, configPath, cleanup := createTempVaultWithBaseline(
			rollupBaselineFixtureTasks,
			"missing-baseline.md",
			"",
		)
		defer cleanup()

		session := runRollup(configPath, "rollup", "weekly", "--week", "2026-W37")
		Eventually(session).Should(gexec.Exit(1))

		Expect(string(session.Err.Contents())).To(ContainSubstring("test"))
		Expect(string(session.Err.Contents())).To(
			ContainSubstring(filepath.Join(vaultPath, "missing-baseline.md")),
		)
		Expect(string(session.Out.Contents())).NotTo(ContainSubstring("Baseline"))
	})

	It("fails loudly when a baseline key is missing", func() {
		document := baselineDocument("2026-01-02", "111", "7", scratchBaselineWeeks, "2 of 9")
		document = strings.Replace(document, "baseline_median: 7\n", "", 1)

		_, configPath, cleanup := createTempVaultWithBaseline(
			rollupBaselineFixtureTasks,
			"baseline.md",
			document,
		)
		defer cleanup()

		session := runRollup(configPath, "rollup", "weekly", "--week", "2026-W37")
		Eventually(session).Should(gexec.Exit(1))

		Expect(string(session.Err.Contents())).To(ContainSubstring("baseline_median"))
		Expect(string(session.Out.Contents())).NotTo(ContainSubstring("Baseline"))
		Expect(string(session.Out.Contents())).NotTo(ContainSubstring("Median"))
	})

	It("rejects a baseline path outside the vault", func() {
		for _, rejected := range []string{"/etc/passwd", "../../outside.md"} {
			_, configPath, cleanup := createTempVaultWithBaseline(
				rollupBaselineFixtureTasks,
				rejected,
				"",
			)

			session := runRollup(configPath, "rollup", "weekly", "--week", "2026-W37")
			Eventually(session).Should(gexec.Exit(1))

			out := string(session.Out.Contents())
			Expect(out).NotTo(ContainSubstring("Human interactions"), rejected)
			Expect(out).NotTo(ContainSubstring("Baseline"), rejected)
			Expect(out).NotTo(ContainSubstring("Delta"), rejected)

			cleanup()
		}
	})

	It("carries the baseline figures and the deltas in JSON", func() {
		type deltaJSON struct {
			Computed float64 `json:"computed"`
			Baseline float64 `json:"baseline"`
			Delta    float64 `json:"delta"`
			Mismatch bool    `json:"mismatch"`
		}
		type baselineJSON struct {
			Captured      string         `json:"captured"`
			HumanTotal    int            `json:"human_total"`
			Median        int            `json:"median"`
			Weeks         map[string]int `json:"weeks"`
			AgentCoverage string         `json:"agent_coverage"`
			Deltas        struct {
				HumanInteractions *deltaJSON `json:"human_interactions"`
				PerFamilyMedian   *deltaJSON `json:"per_family_median"`
			} `json:"deltas"`
		}
		type resultJSON struct {
			Baseline *baselineJSON `json:"baseline"`
		}

		_, configPath, cleanup := createTempVaultWithBaseline(
			rollupBaselineFixtureTasks,
			"baseline.md",
			baselineDocument("2026-09-12", "62485", "64", realBaselineWeeks, "1 of 420"),
		)
		defer cleanup()

		session := runRollup(configPath, "rollup", "weekly", "--week", "2026-W37", "--output", "json")
		Eventually(session).Should(gexec.Exit(0))

		var result resultJSON
		Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
		Expect(result.Baseline).NotTo(BeNil())
		Expect(result.Baseline.Captured).To(Equal("2026-09-12"))
		Expect(result.Baseline.HumanTotal).To(Equal(62485))
		Expect(result.Baseline.Median).To(Equal(64))
		Expect(result.Baseline.AgentCoverage).To(Equal("1 of 420"))
		Expect(result.Baseline.Weeks["2026-W37"]).To(Equal(26476))
		Expect(result.Baseline.Deltas.HumanInteractions).NotTo(BeNil())
		Expect(result.Baseline.Deltas.HumanInteractions.Baseline).To(Equal(26476.0))
		Expect(result.Baseline.Deltas.HumanInteractions.Delta).To(Equal(
			result.Baseline.Deltas.HumanInteractions.Computed -
				result.Baseline.Deltas.HumanInteractions.Baseline,
		))
		Expect(result.Baseline.Deltas.PerFamilyMedian).NotTo(BeNil())
		Expect(result.Baseline.Deltas.PerFamilyMedian.Mismatch).To(BeTrue())

		_, noBaselineConfig, noBaselineCleanup := createTempVaultWithBaseline(
			rollupBaselineFixtureTasks,
			"",
			"",
		)
		defer noBaselineCleanup()

		noBaselineSession := runRollup(
			noBaselineConfig, "rollup", "weekly", "--week", "2026-W37", "--output", "json",
		)
		Eventually(noBaselineSession).Should(gexec.Exit(0))

		var generic map[string]any
		Expect(json.Unmarshal(noBaselineSession.Out.Contents(), &generic)).To(Succeed())
		_, ok := generic["baseline"]
		Expect(ok).To(BeFalse())
	})

	It("prints the edited baseline figure after the file is rewritten", func() {
		sha256OfFile := func(path string) string {
			data, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			sum := sha256.Sum256(data)
			return fmt.Sprintf("%x", sum)
		}

		const baselineFile = "baseline.md"
		vaultPath, configPath, cleanup := createTempVaultWithBaseline(
			rollupBaselineFixtureTasks,
			baselineFile,
			baselineDocument("2026-01-02", "111", "7", scratchBaselineWeeks, "2 of 9"),
		)
		defer cleanup()

		baselinePath := filepath.Join(vaultPath, baselineFile)
		digestBefore := sha256OfFile(binPath)

		first := runRollup(configPath, "rollup", "weekly", "--week", "2026-W37")
		Eventually(first).Should(gexec.Exit(0))
		Expect(string(first.Out.Contents())).To(ContainSubstring("  Median: 7"))

		Expect(os.WriteFile(
			baselinePath,
			[]byte(baselineDocument("2026-01-02", "111", "8", scratchBaselineWeeks, "2 of 9")),
			0600,
		)).To(Succeed())

		second := runRollup(configPath, "rollup", "weekly", "--week", "2026-W37")
		Eventually(second).Should(gexec.Exit(0))
		secondOut := string(second.Out.Contents())
		Expect(secondOut).To(ContainSubstring("  Median: 8"))
		Expect(secondOut).NotTo(ContainSubstring("  Median: 7"))
		Expect(secondOut).To(ContainSubstring(fmt.Sprintf(
			"  Per-family median: %s - 8 = %s [definitional mismatch]\n",
			computedFigure(secondOut, "Per-family median"),
			minus(computedFigure(secondOut, "Per-family median"), "8"),
		)))

		Expect(os.WriteFile(
			baselinePath,
			[]byte(baselineDocument("2026-01-02", "111", "7", scratchBaselineWeeks, "2 of 9")),
			0600,
		)).To(Succeed())

		third := runRollup(configPath, "rollup", "weekly", "--week", "2026-W37")
		Eventually(third).Should(gexec.Exit(0))
		Expect(string(third.Out.Contents())).To(ContainSubstring("  Median: 7"))

		Expect(sha256OfFile(binPath)).To(Equal(digestBefore))
	})
})
