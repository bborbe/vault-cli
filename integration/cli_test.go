// Copyright (c) 2025 Benjamin Borbe All rights reserved.
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
	"sort"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v3"
)

// createTempVault creates a temporary vault with tasks, goals, and config file
func createTempVault(
	tasks map[string]string,
) (vaultPath string, configPath string, cleanup func()) {
	return createTempVaultWithGoals(tasks, nil)
}

// createTempVaultWithGoals creates a temporary vault with tasks, goals, and config file
func createTempVaultWithGoals(
	tasks map[string]string,
	goals map[string]string,
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

	if goals != nil {
		goalsDir := filepath.Join(vaultPath, "Goals")
		err = os.MkdirAll(goalsDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		for name, content := range goals {
			goalPath := filepath.Join(goalsDir, name+".md")
			err = os.WriteFile(goalPath, []byte(content), 0600)
			Expect(err).NotTo(HaveOccurred())
		}
	}

	configContent := fmt.Sprintf(`default_vault: test
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
    goals_dir: Goals
`, vaultPath)

	configFile, err := os.CreateTemp("", "vault-config-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	_, err = configFile.WriteString(configContent)
	Expect(err).NotTo(HaveOccurred())
	err = configFile.Close()
	Expect(err).NotTo(HaveOccurred())

	return vaultPath, configFile.Name(), func() {
		_ = os.RemoveAll(vaultPath)
		_ = os.Remove(configFile.Name())
	}
}

// createTempVaultWithCurrentUser creates a temporary vault with tasks, a config
// that names current_user, and a claude_script that is deliberately not installed.
//
// current_user is load-bearing: `task work-on` calls config.GetCurrentUser and
// errors out when it is unset, so the work-on specs need this shape.
// The uninstalled claude_script keeps the work-on path's starter nil, which is the
// same guard createTempVaultWithTopicPages uses — these specs drive the
// cached-session branch, which spawns nothing.
func createTempVaultWithCurrentUser(
	tasks map[string]string,
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

	configContent := fmt.Sprintf(`default_vault: test
current_user: tester@example.com
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
    goals_dir: Goals
    claude_script: "claude-not-installed-for-tests"
`, vaultPath)

	configFile, err := os.CreateTemp("", "vault-config-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	_, err = configFile.WriteString(configContent)
	Expect(err).NotTo(HaveOccurred())
	err = configFile.Close()
	Expect(err).NotTo(HaveOccurred())

	return vaultPath, configFile.Name(), func() {
		_ = os.RemoveAll(vaultPath)
		_ = os.Remove(configFile.Name())
	}
}

// createTempVaultWithTopics creates a temporary vault whose config sets
// topics_dir, and returns the vault path, the config path and a cleanup func.
func createTempVaultWithTopics(topicsDir string) (vaultPath string, configPath string, cleanup func()) {
	var err error
	vaultPath, err = os.MkdirTemp("", "vault-*")
	Expect(err).NotTo(HaveOccurred())

	tasksDir := filepath.Join(vaultPath, "Tasks")
	err = os.MkdirAll(tasksDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	configContent := fmt.Sprintf(`default_vault: test
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
    topics_dir: "%s"
`, vaultPath, topicsDir)

	configFile, err := os.CreateTemp("", "vault-config-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	_, err = configFile.WriteString(configContent)
	Expect(err).NotTo(HaveOccurred())
	err = configFile.Close()
	Expect(err).NotTo(HaveOccurred())

	return vaultPath, configFile.Name(), func() {
		_ = os.RemoveAll(vaultPath)
		_ = os.Remove(configFile.Name())
	}
}

// createTempVaultWithTopicPages creates a temporary vault whose config sets
// topics_dir, current_user and a claude_script that is deliberately not installed,
// and whose topics and goals directories contain the given pages.
//
// The uninstalled claude_script is load-bearing: ops.NewClaudeSessionStarter calls
// exec.LookPath on it, which fails, so the starter is nil and the work-on path takes
// ErrStarterUnavailable as a soft warning instead of spawning a real headless turn.
func createTempVaultWithTopicPages(
	topicsDir string,
	topics map[string]string,
	goals map[string]string,
) (vaultPath string, configPath string, cleanup func()) {
	var err error
	vaultPath, err = os.MkdirTemp("", "vault-*")
	Expect(err).NotTo(HaveOccurred())

	tasksDir := filepath.Join(vaultPath, "Tasks")
	err = os.MkdirAll(tasksDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	topicsDirPath := filepath.Join(vaultPath, topicsDir)
	err = os.MkdirAll(topicsDirPath, 0755)
	Expect(err).NotTo(HaveOccurred())

	for name, content := range topics {
		topicPath := filepath.Join(topicsDirPath, name+".md")
		err = os.WriteFile(topicPath, []byte(content), 0600)
		Expect(err).NotTo(HaveOccurred())
	}

	goalsDir := filepath.Join(vaultPath, "Goals")
	err = os.MkdirAll(goalsDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	for name, content := range goals {
		goalPath := filepath.Join(goalsDir, name+".md")
		err = os.WriteFile(goalPath, []byte(content), 0600)
		Expect(err).NotTo(HaveOccurred())
	}

	configContent := fmt.Sprintf(`default_vault: test
current_user: tester@example.com
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
    goals_dir: Goals
    topics_dir: "%s"
    claude_script: "claude-not-installed-for-tests"
`, vaultPath, topicsDir)

	configFile, err := os.CreateTemp("", "vault-config-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	_, err = configFile.WriteString(configContent)
	Expect(err).NotTo(HaveOccurred())
	err = configFile.Close()
	Expect(err).NotTo(HaveOccurred())

	return vaultPath, configFile.Name(), func() {
		_ = os.RemoveAll(vaultPath)
		_ = os.Remove(configFile.Name())
	}
}

// helpLeafNames parses the `Available Commands:` block out of a cobra help output
// and returns the leaf names in the order cobra prints them.
//
// The block runs from the `Available Commands:` line to the next blank line; only
// lines beginning with two spaces and a lowercase letter are leaves. Returning the
// names in print order lets a caller compare two command groups member for member.
func helpLeafNames(help string) []string {
	lines := strings.Split(help, "\n")
	inBlock := false
	var leaves []string
	for _, line := range lines {
		if strings.HasPrefix(line, "Available Commands:") {
			inBlock = true
			continue
		}
		if !inBlock {
			continue
		}
		if strings.TrimSpace(line) == "" {
			break
		}
		if len(line) < 3 || !strings.HasPrefix(line, "  ") {
			continue
		}
		if line[2] < 'a' || line[2] > 'z' {
			continue
		}
		leaves = append(leaves, strings.Fields(line)[0])
	}
	return leaves
}

// showPhaseField runs `<entityType> show <name> --output json` against the given
// config and returns the value at `.fields.phase`.
//
// It fails the spec if the field is absent, so a caller comparing a topic's phase to
// a goal's phase cannot pass vacuously on two missing keys.
func showPhaseField(configPath, entityType, name string) string {
	cmd := exec.Command(
		binPath, "--config", configPath, "--vault", "test",
		entityType, "show", name, "--output", "json",
	)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session).Should(gexec.Exit(0))

	var parsed map[string]any
	Expect(json.Unmarshal(session.Out.Contents(), &parsed)).To(Succeed())
	fields, ok := parsed["fields"].(map[string]any)
	Expect(ok).To(BeTrue(), "%s show emitted no fields object", entityType)
	phase, present := fields["phase"]
	Expect(present).To(BeTrue(), "%s show emitted no phase key", entityType)
	phaseStr, ok := phase.(string)
	Expect(ok).To(BeTrue(), "%s show phase is not a string", entityType)
	return phaseStr
}

// countTagEntries counts the entries under the `tags:` frontmatter key of a page
// file as written on disk, in both the inline (`tags: [a, b]`) and the block
// (`tags:` followed by indented `- a` lines) shapes.
//
// Asserting against the file rather than against the command's own output is what
// makes the add/remove spec a state-transition test.
func countTagEntries(filePath string) int {
	content, err := os.ReadFile(filePath) //#nosec G304 -- test file
	Expect(err).NotTo(HaveOccurred())

	count := 0
	inBlock := false
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "tags:") {
			inline := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "tags:")), "[]")
			inBlock = inline == ""
			count += countCommaSeparated(inline)
			continue
		}
		if inBlock && strings.HasPrefix(trimmed, "- ") {
			count++
			continue
		}
		if trimmed != "" {
			inBlock = false
		}
	}
	return count
}

// countCommaSeparated counts the non-empty comma-separated items in s.
func countCommaSeparated(s string) int {
	count := 0
	for _, item := range strings.Split(s, ",") {
		if strings.TrimSpace(item) != "" {
			count++
		}
	}
	return count
}

// createTempVaultWithBrokers creates a temporary vault whose config carries a
// notification section naming the given brokers, and returns the vault path, the
// config path and a cleanup func.
func createTempVaultWithBrokers(
	brokers, topicPrefix string,
) (vaultPath string, configPath string, cleanup func()) {
	var err error
	vaultPath, err = os.MkdirTemp("", "vault-*")
	Expect(err).NotTo(HaveOccurred())

	tasksDir := filepath.Join(vaultPath, "Tasks")
	err = os.MkdirAll(tasksDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	configContent := fmt.Sprintf(`default_vault: test
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
notification:
  brokers: "%s"
  topic_prefix: "%s"
`, vaultPath, brokers, topicPrefix)

	configFile, err := os.CreateTemp("", "vault-config-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	_, err = configFile.WriteString(configContent)
	Expect(err).NotTo(HaveOccurred())
	err = configFile.Close()
	Expect(err).NotTo(HaveOccurred())

	return vaultPath, configFile.Name(), func() {
		_ = os.RemoveAll(vaultPath)
		_ = os.Remove(configFile.Name())
	}
}

// createTwoTempVaults creates two temporary vaults (alpha and beta) each with
// Tasks and Goals directories, plus a shared config file with
// default_vault: alpha and both vaults configured.
func createTwoTempVaults(
	tasksA, goalsA, tasksB, goalsB map[string]string,
) (vaultPathA, vaultPathB, configPath string, cleanup func()) {
	var err error
	vaultPathA, err = os.MkdirTemp("", "vault-alpha-*")
	Expect(err).NotTo(HaveOccurred())
	vaultPathB, err = os.MkdirTemp("", "vault-beta-*")
	Expect(err).NotTo(HaveOccurred())

	writeVaultFiles := func(vaultPath string, tasks, goals map[string]string) {
		tasksDir := filepath.Join(vaultPath, "Tasks")
		err = os.MkdirAll(tasksDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		for name, content := range tasks {
			taskPath := filepath.Join(tasksDir, name+".md")
			err = os.WriteFile(taskPath, []byte(content), 0600)
			Expect(err).NotTo(HaveOccurred())
		}

		goalsDir := filepath.Join(vaultPath, "Goals")
		err = os.MkdirAll(goalsDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		for name, content := range goals {
			goalPath := filepath.Join(goalsDir, name+".md")
			err = os.WriteFile(goalPath, []byte(content), 0600)
			Expect(err).NotTo(HaveOccurred())
		}
	}

	writeVaultFiles(vaultPathA, tasksA, goalsA)
	writeVaultFiles(vaultPathB, tasksB, goalsB)

	configContent := fmt.Sprintf(`default_vault: alpha
vaults:
  alpha:
    name: alpha
    path: %s
    tasks_dir: Tasks
    goals_dir: Goals
  beta:
    name: beta
    path: %s
    tasks_dir: Tasks
    goals_dir: Goals
`, vaultPathA, vaultPathB)

	configFile, err := os.CreateTemp("", "vault-config-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	_, err = configFile.WriteString(configContent)
	Expect(err).NotTo(HaveOccurred())
	err = configFile.Close()
	Expect(err).NotTo(HaveOccurred())

	return vaultPathA, vaultPathB, configFile.Name(), func() {
		_ = os.RemoveAll(vaultPathA)
		_ = os.RemoveAll(vaultPathB)
		_ = os.Remove(configFile.Name())
	}
}

var _ = Describe("vault-cli integration tests", func() {
	Describe("vault-cli --help", func() {
		It("exits 0 and shows help text", func() {
			cmd := exec.Command(binPath, "--help")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("vault-cli"))
			Expect(
				string(session.Out.Contents()),
			).To(ContainSubstring("/.config/vault-cli/config.yaml"))
		})
	})

	Describe("command registration", func() {
		DescribeTable("exits 0 for --help",
			func(args ...string) {
				helpArgs := append(args, "--help")
				cmd := exec.Command(binPath, helpArgs...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
			},
			// Task subcommands
			Entry("task list", "task", "list"),
			Entry("task show", "task", "show"),
			Entry("task complete", "task", "complete"),
			Entry("task defer", "task", "defer"),
			Entry("task update", "task", "update"),
			Entry("task work-on", "task", "work-on"),
			Entry("task get", "task", "get"),
			Entry("task set", "task", "set"),
			Entry("task clear", "task", "clear"),
			Entry("task lint", "task", "lint"),
			Entry("task validate", "task", "validate"),
			Entry("task backfill-identifiers", "task", "backfill-identifiers"),
			Entry("task search", "task", "search"),
			Entry("task watch", "task", "watch"),
			Entry("task add", "task", "add"),
			Entry("task remove", "task", "remove"),
			Entry("task append-metrics-session", "task", "append-metrics-session"),
			// Goal subcommands
			Entry("goal list", "goal", "list"),
			Entry("goal lint", "goal", "lint"),
			Entry("goal search", "goal", "search"),
			Entry("goal show", "goal", "show"),
			Entry("goal get", "goal", "get"),
			Entry("goal set", "goal", "set"),
			Entry("goal clear", "goal", "clear"),
			Entry("goal complete", "goal", "complete"),
			Entry("goal add", "goal", "add"),
			Entry("goal remove", "goal", "remove"),
			Entry("goal work-on", "goal", "work-on"),
			// Topic subcommands
			Entry("topic list", "topic", "list"),
			Entry("topic lint", "topic", "lint"),
			Entry("topic search", "topic", "search"),
			Entry("topic show", "topic", "show"),
			Entry("topic get", "topic", "get"),
			Entry("topic set", "topic", "set"),
			Entry("topic clear", "topic", "clear"),
			Entry("topic complete", "topic", "complete"),
			Entry("topic defer", "topic", "defer"),
			Entry("topic add", "topic", "add"),
			Entry("topic remove", "topic", "remove"),
			Entry("topic work-on", "topic", "work-on"),
			// Theme subcommands
			Entry("theme list", "theme", "list"),
			Entry("theme lint", "theme", "lint"),
			Entry("theme search", "theme", "search"),
			Entry("theme show", "theme", "show"),
			Entry("theme get", "theme", "get"),
			Entry("theme set", "theme", "set"),
			Entry("theme clear", "theme", "clear"),
			Entry("theme add", "theme", "add"),
			Entry("theme remove", "theme", "remove"),
			// Objective subcommands
			Entry("objective list", "objective", "list"),
			Entry("objective lint", "objective", "lint"),
			Entry("objective search", "objective", "search"),
			Entry("objective show", "objective", "show"),
			Entry("objective get", "objective", "get"),
			Entry("objective set", "objective", "set"),
			Entry("objective clear", "objective", "clear"),
			Entry("objective complete", "objective", "complete"),
			Entry("objective add", "objective", "add"),
			Entry("objective remove", "objective", "remove"),
			// Vision subcommands
			Entry("vision list", "vision", "list"),
			Entry("vision lint", "vision", "lint"),
			Entry("vision search", "vision", "search"),
			Entry("vision show", "vision", "show"),
			Entry("vision get", "vision", "get"),
			Entry("vision set", "vision", "set"),
			Entry("vision clear", "vision", "clear"),
			Entry("vision add", "vision", "add"),
			Entry("vision remove", "vision", "remove"),
			// Decision subcommands
			Entry("decision list", "decision", "list"),
			Entry("decision ack", "decision", "ack"),
			// Root-level commands
			Entry("search", "search"),
			Entry("resolve", "resolve"),
			Entry("watch", "watch"),
			// Rollup subcommands
			Entry("rollup weekly", "rollup", "weekly"),
			// Config subcommands
			Entry("config list", "config", "list"),
			Entry("config current-user", "config", "current-user"),
		)
	})

	Describe("vault-cli config list --output json topics_dir", func() {
		It("config list --output json includes topics_dir for a vault that sets it", func() {
			_, configPath, cleanup := createTempVaultWithTopics("23 Topics")
			defer cleanup()

			cmd := exec.Command(binPath, "--config", configPath, "config", "list", "--output", "json")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			out := string(session.Out.Contents())
			Expect(out).To(ContainSubstring(`"name": "test"`))
			Expect(out).To(ContainSubstring(`"topics_dir": "23 Topics"`))
		})

		It("config list still lists a vault without topics_dir", func() {
			vaultPath, configPath, cleanup := createTempVault(map[string]string{})
			defer cleanup()

			jsonCmd := exec.Command(binPath, "--config", configPath, "config", "list", "--output", "json")
			jsonSession, err := gexec.Start(jsonCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(jsonSession).Should(gexec.Exit(0))
			jsonOut := string(jsonSession.Out.Contents())
			Expect(jsonOut).To(ContainSubstring(`"name": "test"`))
			Expect(jsonOut).NotTo(ContainSubstring("topics_dir"))

			plainCmd := exec.Command(binPath, "--config", configPath, "config", "list")
			plainSession, err := gexec.Start(plainCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(plainSession).Should(gexec.Exit(0))
			Expect(string(plainSession.Out.Contents())).To(ContainSubstring("test\t" + vaultPath))
		})
	})

	Describe("vault-cli rollup weekly", func() {
		rollupFixtureTasks := map[string]string{
			"Rollup Fixture A - 2026-09-08": `---
status: completed
page_type: task
metrics_completed_at: "2026-09-08T10:00:00+02:00"
metrics_interaction_count: 7
---
Fixture body.
`,
			"Rollup Fixture B - 2026-09-09": `---
status: completed
page_type: task
metrics_completed_at: "2026-09-09T10:00:00+02:00"
metrics_interaction_count: 0
---
Fixture body.
`,
			"Rollup Fixture C - 2026-09-10": `---
status: completed
page_type: task
metrics_completed_at: "2026-09-10T10:00:00+02:00"
---
Fixture body.
`,
		}

		It("rollup weekly prints the three figures with the rule and the grouping", func() {
			_, configPath, cleanup := createTempVault(rollupFixtureTasks)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath,
				"rollup", "weekly", "--week", "2026-W37",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			out := string(session.Out.Contents())
			Expect(out).To(MatchRegexp(`(?m)^Human interactions: `))
			Expect(out).To(MatchRegexp(`(?m)^Unattended deliveries: `))
			Expect(out).To(MatchRegexp(`(?m)^Per-family median: `))
			Expect(out).To(ContainSubstring(
				"A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0",
			))
			Expect(out).To(MatchRegexp(`(?m)^Grouping rule: `))
			Expect(out).To(MatchRegexp(`(?m)^  [^:]+: .+$`))

			// The absent-count task is excluded: only the recorded zero counts.
			Expect(out).To(ContainSubstring("Unattended deliveries: 1"))
		})

		It("rollup weekly reports no data for a week with no metrics", func() {
			_, configPath, cleanup := createTempVault(map[string]string{
				"Rollup Plain Task - 2026-05-12": `---
status: completed
page_type: task
---
Fixture body.
`,
			})
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath,
				"rollup", "weekly", "--week", "2026-W20",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			out := string(session.Out.Contents())
			Expect(out).To(ContainSubstring("no data"))
			Expect(out).NotTo(MatchRegexp(`(?m)^Human interactions: 0$`))
			Expect(out).NotTo(MatchRegexp(`(?m)^Human interactions: no recorded counts$`))
			Expect(out).NotTo(MatchRegexp(`(?m)^\s+[^:]+: 0$`))
		})

		It("rollup weekly prints the same bytes on two runs", func() {
			_, configPath, cleanup := createTempVault(rollupFixtureTasks)
			defer cleanup()

			run := func(args ...string) string {
				cmd := exec.Command(binPath, append([]string{"--config", configPath}, args...)...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				return string(session.Out.Contents())
			}

			args := []string{"rollup", "weekly", "--week", "2026-W37"}
			Expect(run(args...)).To(Equal(run(args...)))

			jsonArgs := []string{"rollup", "weekly", "--week", "2026-W37", "--output", "json"}
			Expect(run(jsonArgs...)).To(Equal(run(jsonArgs...)))
		})

		It("rollup weekly --output json carries the same three figures as plain", func() {
			_, configPath, cleanup := createTempVault(rollupFixtureTasks)
			defer cleanup()

			run := func(args ...string) []byte {
				cmd := exec.Command(binPath, append([]string{"--config", configPath}, args...)...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				return session.Out.Contents()
			}

			plain := string(run("rollup", "weekly", "--week", "2026-W37"))
			rawJSON := run("rollup", "weekly", "--week", "2026-W37", "--output", "json")

			var figures struct {
				HumanInteractions    string `json:"human_interactions"`
				UnattendedDeliveries string `json:"unattended_deliveries"`
				PerFamilyMedian      string `json:"per_family_median"`
			}
			Expect(json.Unmarshal(rawJSON, &figures)).To(Succeed())

			Expect(figures.HumanInteractions).NotTo(BeEmpty())
			Expect(figures.UnattendedDeliveries).NotTo(BeEmpty())
			Expect(figures.PerFamilyMedian).NotTo(BeEmpty())

			Expect(plain).To(ContainSubstring("Human interactions: " + figures.HumanInteractions))
			Expect(plain).To(ContainSubstring("Unattended deliveries: " + figures.UnattendedDeliveries))
			Expect(plain).To(ContainSubstring("Per-family median: " + figures.PerFamilyMedian))
		})

		It("rollup weekly without --vault reads only the default vault", func() {
			_, _, configPath, cleanup := createTwoTempVaults(
				map[string]string{
					"Alpha Rollup Task - 2026-09-08": `---
status: completed
page_type: task
metrics_completed_at: "2026-09-08T10:00:00+02:00"
metrics_interaction_count: 5
---
Alpha body.
`,
				},
				nil,
				map[string]string{
					"Beta Rollup Task - 2026-09-08": `---
status: completed
page_type: task
metrics_completed_at: "2026-09-08T10:00:00+02:00"
metrics_interaction_count: 999
---
Beta body.
`,
				},
				nil,
			)
			defer cleanup()

			run := func(args ...string) string {
				cmd := exec.Command(binPath, append([]string{"--config", configPath}, args...)...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				return string(session.Out.Contents())
			}

			defaultOut := run("rollup", "weekly", "--week", "2026-W37")
			Expect(defaultOut).To(ContainSubstring("Human interactions: 5"))
			Expect(defaultOut).NotTo(ContainSubstring("Human interactions: 1004"))
			Expect(defaultOut).NotTo(ContainSubstring("999"))

			betaOut := run("rollup", "weekly", "--vault", "beta", "--week", "2026-W37")
			Expect(betaOut).To(ContainSubstring("Human interactions: 999"))
		})

		It("rollup weekly rejects a malformed --week", func() {
			_, configPath, cleanup := createTempVault(rollupFixtureTasks)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath,
				"rollup", "weekly", "--week", "2026-W5",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))

			Expect(string(session.Err.Contents())).To(ContainSubstring("YYYY-Wnn"))
			out := string(session.Out.Contents())
			Expect(out).NotTo(ContainSubstring("Human interactions:"))
			Expect(out).NotTo(ContainSubstring("Unattended deliveries:"))
			Expect(out).NotTo(ContainSubstring("Per-family median:"))

			// 2026 has 53 ISO weeks, so W53 is a valid token, not an out-of-range one.
			validCmd := exec.Command(
				binPath, "--config", configPath,
				"rollup", "weekly", "--week", "2026-W53",
			)
			validSession, err := gexec.Start(validCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(validSession).Should(gexec.Exit(0))
		})
	})

	Describe("frontmatter round-trip", func() {
		var vaultPath, configPath string
		var cleanup func()

		AfterEach(func() {
			cleanup()
		})

		It("preserves known fields through set operations", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"roundtrip-task": `---
status: todo
priority: 2
task_identifier: 10101010-1010-4101-a010-101010101010
---
# Roundtrip Task
Body content here.
`,
			})

			// Set status to in_progress
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set",
				"roundtrip-task",
				"status", "in_progress",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			// Verify status changed and other fields preserved
			taskPath := filepath.Join(vaultPath, "Tasks", "roundtrip-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: in_progress"))
			Expect(string(content)).To(ContainSubstring("priority: 2"))
			Expect(
				string(content),
			).To(ContainSubstring("task_identifier: 10101010-1010-4101-a010-101010101010"))
			Expect(string(content)).To(ContainSubstring("Body content here."))
		})

		It("preserves unknown frontmatter fields through set operations", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"unknown-fields-task": `---
status: todo
priority: 1
custom_field: my-custom-value
another_field: 42
task_identifier: 20202020-2020-4202-a020-202020202020
---
# Task with unknown fields
`,
			})

			// Set a known field
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set",
				"unknown-fields-task",
				"status", "in_progress",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			// Verify unknown fields are preserved
			taskPath := filepath.Join(
				vaultPath,
				"Tasks",
				"unknown-fields-task.md",
			)
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("custom_field: my-custom-value"))
			Expect(string(content)).To(ContainSubstring("another_field: 42"))
		})

		It("preserves markdown content through set operations", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"content-task": `---
status: todo
priority: 1
task_identifier: 30303030-3030-4303-a030-303030303030
---
# Content Task

This has **bold** and _italic_ text.

- bullet 1
- bullet 2

` + "```go\nfmt.Println(\"hello\")\n```\n",
			})

			// Set status
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set",
				"content-task",
				"status", "in_progress",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			// Verify markdown content preserved
			taskPath := filepath.Join(vaultPath, "Tasks", "content-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("**bold**"))
			Expect(string(content)).To(ContainSubstring("- bullet 1"))
			Expect(string(content)).To(ContainSubstring("fmt.Println"))
		})
	})

	Describe("task get/set", func() {
		var vaultPath, configPath string
		var cleanup func()

		AfterEach(func() {
			cleanup()
		})

		It("gets a known field value", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"get-task": `---
status: todo
priority: 3
task_identifier: 40404040-4040-4404-a040-404040404040
---
# Get Task
`,
			})

			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "get",
				"get-task",
				"status",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("next"))
		})

		It("sets a known field with valid value", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"set-task": `---
status: todo
priority: 1
task_identifier: 50505050-5050-4505-a050-505050505050
---
# Set Task
`,
			})

			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set",
				"set-task",
				"priority", "5",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			taskPath := filepath.Join(vaultPath, "Tasks", "set-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("priority: 5"))
		})
	})

	Describe("status normalization", func() {
		var configPath string
		var cleanup func()

		AfterEach(func() {
			cleanup()
		})

		It("normalizes legacy status 'todo' to 'next' on list", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"legacy-task": `---
status: todo
priority: 1
---
# Legacy Task
`,
			})

			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "list",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			// Task with status "todo" should appear (normalized to next)
			Expect(session.Out).To(gbytes.Say("legacy-task"))
		})
	})

	Describe("vault-cli list", func() {
		var configPath string
		var cleanup func()

		BeforeEach(func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"todo-task": `---
status: todo
priority: 2
---
# Todo Task
This is a todo task.
`,
				"done-task": `---
status: done
priority: 1
---
# Done Task
This is a done task.
`,
			})
		})

		AfterEach(func() {
			cleanup()
		})

		It("shows only non-completed tasks by default", func() {
			cmd := exec.Command(binPath, "--config", configPath, "--vault", "test", "task", "list")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("todo-task"))
			Expect(session.Out).NotTo(gbytes.Say("done-task"))
		})

		It("shows all tasks with --all flag", func() {
			cmd := exec.Command(
				binPath,
				"--config",
				configPath,
				"--vault",
				"test",
				"task",
				"list",
				"--all",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("todo-task"))
			Expect(session.Out).To(gbytes.Say("done-task"))
		})
	})

	Describe("vault-cli lint", func() {
		var configPath string
		var cleanup func()

		Context("with clean vault", func() {
			BeforeEach(func() {
				_, configPath, cleanup = createTempVault(map[string]string{
					"valid-task": `---
status: todo
priority: 2
task_identifier: 60606060-6060-4606-a060-606060606060
---
# Valid Task
This task has valid frontmatter.
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 0 and reports no issues", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"lint",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say("No lint issues found"))
			})
		})

		Context("with invalid status", func() {
			BeforeEach(func() {
				_, configPath, cleanup = createTempVault(map[string]string{
					"invalid-status-task": `---
status: garbage
priority: 2
task_identifier: 70707070-7070-4707-a070-707070707070
---
# Task with invalid status
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 1 and reports INVALID_STATUS", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"lint",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Out).To(gbytes.Say("INVALID_STATUS"))
			})
		})

		Context("with invalid priority", func() {
			BeforeEach(func() {
				_, configPath, cleanup = createTempVault(map[string]string{
					"high-priority-task": `---
status: todo
priority: high
---
# Task with string priority
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 1 and reports INVALID_PRIORITY", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"lint",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Out).To(gbytes.Say("INVALID_PRIORITY"))
			})
		})
	})

	Describe("vault-cli lint --fix", func() {
		var vaultPath, configPath string
		var cleanup func()

		Context("with legacy status: todo (silently accepted alias)", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"legacy-todo-task": `---
status: todo
priority: 2
task_identifier: 80808080-8080-4808-a080-808080808080
---
# Task with legacy todo status
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 0, reports no issues, and leaves file unchanged", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"lint",
					"--fix",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say("No lint issues found"))
				Expect(session.Out).NotTo(gbytes.Say("FIXED"))

				// Verify file was NOT rewritten — alias preserved on disk
				taskPath := filepath.Join(vaultPath, "Tasks", "legacy-todo-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("status: todo"))
				Expect(string(content)).NotTo(ContainSubstring("status: next"))
			})
		})

		Context("with priority: high", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"high-priority-task": `---
status: todo
priority: high
task_identifier: 90909090-9090-4909-a090-909090909090
---
# Task with string priority
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 0 and updates file to priority: 1", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"lint",
					"--fix",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				// Verify file was updated
				taskPath := filepath.Join(vaultPath, "Tasks", "high-priority-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("priority: 1"))
				Expect(string(content)).NotTo(ContainSubstring("priority: high"))
			})
		})
	})

	Describe("vault-cli blocked_by scalar detector", func() {
		var vaultPath, configPath string
		var cleanup func()

		AfterEach(func() {
			cleanup()
		})

		// scalarTaskFixture is the AC7 fixture: base keys only, so the only issue
		// any run can report is the scalar blocked_by under test.
		scalarTaskFixture := `---
status: in_progress
page_type: task
priority: 1
task_identifier: 22222222-2222-4222-8222-222222222222
blocked_by: Blocker A
---
# Alpha
`

		// listTaskFixture is byte-identical except that blocked_by is a YAML list.
		listTaskFixture := `---
status: in_progress
page_type: task
priority: 1
task_identifier: 22222222-2222-4222-8222-222222222222
blocked_by:
  - "Blocker A"
---
# Alpha
`

		runValidate := func(taskName string, extraArgs ...string) *gexec.Session {
			args := make([]string, 0, 7+len(extraArgs))
			args = append(
				args,
				"--config", configPath,
				"--vault", "test",
				"task", "validate", taskName,
			)
			args = append(args, extraArgs...)
			session, err := gexec.Start(
				exec.Command(binPath, args...),
				GinkgoWriter,
				GinkgoWriter,
			)
			Expect(err).NotTo(HaveOccurred())
			return session
		}

		runTaskLint := func(extraArgs ...string) *gexec.Session {
			args := make([]string, 0, 6+len(extraArgs))
			args = append(
				args,
				"--config", configPath,
				"--vault", "test",
				"task", "lint",
			)
			args = append(args, extraArgs...)
			session, err := gexec.Start(
				exec.Command(binPath, args...),
				GinkgoWriter,
				GinkgoWriter,
			)
			Expect(err).NotTo(HaveOccurred())
			return session
		}

		runGoalLint := func() *gexec.Session {
			session, err := gexec.Start(
				exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"goal", "lint",
				),
				GinkgoWriter,
				GinkgoWriter,
			)
			Expect(err).NotTo(HaveOccurred())
			return session
		}

		fileSHA256 := func(path string) string {
			content, err := os.ReadFile(path) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			sum := sha256.Sum256(content)
			return fmt.Sprintf("%x", sum)
		}

		It("AC7: task validate on a scalar blocked_by exits non-zero with exactly one issue line", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": scalarTaskFixture,
			})

			session := runValidate("Alpha")
			Eventually(session).Should(gexec.Exit(1))

			lines := strings.Split(strings.TrimSpace(string(session.Out.Contents())), "\n")
			Expect(lines).To(HaveLen(1))
			Expect(lines[0]).To(ContainSubstring("blocked_by"))
			Expect(lines[0]).To(ContainSubstring("YAML list"))

			// Negative evidence: the scalar still reads as unblocked, so neither
			// the raw list key nor the computed boolean is emitted.
			listCmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "list",
				"--output", "json",
			)
			listSession, err := gexec.Start(listCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(listSession).Should(gexec.Exit(0))
			var items []map[string]any
			Expect(json.Unmarshal(listSession.Out.Contents(), &items)).To(Succeed())
			Expect(items).To(HaveLen(1))
			Expect(items[0]).NotTo(HaveKey("blocked_by"))
			Expect(items[0]).NotTo(HaveKey("blocked"))
		})

		It("AC8a: task validate on a list-shaped blocked_by exits 0 with no blocked_by line", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": listTaskFixture,
			})

			session := runValidate("Alpha")
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).NotTo(ContainSubstring("blocked_by"))
		})

		It("AC8b: task validate on the empty scalar blocked_by exits 0", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": `---
status: in_progress
page_type: task
priority: 1
task_identifier: 22222222-2222-4222-8222-222222222222
blocked_by: ""
---
# Alpha
`,
			})

			session := runValidate("Alpha")
			Eventually(session).Should(gexec.Exit(0))
		})

		It("AC8c: task validate --output json exits 0 and lists the issue", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": scalarTaskFixture,
			})

			session := runValidate("Alpha", "--output", "json")
			Eventually(session).Should(gexec.Exit(0))

			var result struct {
				Name   string `json:"name"`
				Vault  string `json:"vault"`
				Issues []struct {
					Type        string `json:"type"`
					IssueType   string `json:"issue_type"`
					Description string `json:"description"`
				} `json:"issues"`
			}
			Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
			Expect(result.Issues).To(HaveLen(1))
			Expect(result.Issues[0].IssueType).To(Equal("BLOCKED_BY_SCALAR"))
			Expect(result.Issues[0].Type).To(Equal("ERROR"))
			Expect(result.Issues[0].Description).To(ContainSubstring("blocked_by"))
		})

		It("AC9: task lint reports the scalar blocked_by, and --fix leaves the file byte-identical", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": scalarTaskFixture,
			})
			taskPath := filepath.Join(vaultPath, "Tasks", "Alpha.md")

			session := runTaskLint()
			Eventually(session).Should(gexec.Exit(1))
			Expect(string(session.Out.Contents())).To(ContainSubstring("blocked_by"))

			before := fileSHA256(taskPath)

			fixSession := runTaskLint("--fix")
			Eventually(fixSession).Should(gexec.Exit(1))

			Expect(fileSHA256(taskPath)).To(Equal(before))
		})

		It("AC10a: goal lint reports a scalar blocked_by", func() {
			_, configPath, cleanup = createTempVaultWithGoals(
				map[string]string{},
				map[string]string{
					"Beta": `---
status: next
page_type: goal
priority: 1
blocked_by: Blocker C
---
# Beta
`,
				},
			)

			session := runGoalLint()
			Eventually(session).Should(gexec.Exit(1))
			Expect(string(session.Out.Contents())).To(ContainSubstring("blocked_by"))
		})

		It("AC10b: goal lint passes a list-shaped blocked_by", func() {
			_, configPath, cleanup = createTempVaultWithGoals(
				map[string]string{},
				map[string]string{
					"Beta": `---
status: next
page_type: goal
priority: 1
blocked_by:
  - "Blocker C"
---
# Beta
`,
				},
			)

			session := runGoalLint()
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).NotTo(ContainSubstring("blocked_by"))
		})
	})

	Describe("vault-cli complete", func() {
		var vaultPath, configPath string
		var cleanup func()

		Context("when task exists", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"my-task": `---
status: todo
priority: 2
aborted_reason: test reason
gate_successor: none
---
# My Task
This is my task.
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 0 and updates status to completed", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"complete",
					"my-task",
					"--reason",
					"test reason",
					"--gate-successor",
					"none",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				// Verify file was updated
				taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("status: completed"))
			})
		})

		Context("when task does not exist", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 1", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"complete",
					"non-existent-task",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
			})
		})
	})

	Describe("vault-cli task set status aborted gating", func() {
		var vaultPath, configPath string
		var cleanup func()

		BeforeEach(func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"my-task": "---\nstatus: in_progress\n---\n# My Task\n",
			})
		})

		AfterEach(func() {
			cleanup()
		})

		It("rejects aborted without reason and gate-successor", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set", "my-task", "status", "aborted",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(string(session.Err.Contents())).To(ContainSubstring("aborted_reason"))

			taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).NotTo(ContainSubstring("status: aborted"))
		})

		It("accepts aborted with --reason and --gate-successor", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set", "my-task", "status", "aborted",
				"--reason", "gate moved to X",
				"--gate-successor", "none",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: aborted"))
			Expect(string(content)).To(ContainSubstring("aborted_reason: gate moved to X"))
			Expect(string(content)).To(ContainSubstring("gate_successor: none"))
		})

		It("names the missing field and the succeeding command form in JSON error output", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set", "my-task", "status", "aborted",
				"--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// JSON-mode mutation commands print the error body and return exit 0
			// by pre-existing design; only the body is asserted here.
			Eventually(session.Out).Should(gbytes.Say(`"error"`))
			var parsed map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &parsed)).To(Succeed())
			Expect(parsed).To(HaveKeyWithValue("success", false))
			errStr, ok := parsed["error"].(string)
			Expect(ok).To(BeTrue())
			Expect(errStr).To(ContainSubstring("aborted_reason"))
			Expect(errStr).To(ContainSubstring("trigger / gate / threshold / recurring check"))
			Expect(errStr).To(ContainSubstring(`vault-cli task set "my-task" status aborted --reason`))
		})
	})

	Describe("vault-cli task complete gating", func() {
		var vaultPath, configPath string
		var cleanup func()

		Context("field-less task with no incomplete subtasks", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"my-task": "---\nstatus: in_progress\n---\n# My Task\n",
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("completes without close-out fields", func() {
				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"task", "complete", "my-task",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("status: completed"))
				Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
				Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
			})

			It("completes via task set status completed without close-out fields", func() {
				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"task", "set", "my-task", "status", "completed",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("status: completed"))
				Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
				Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
			})

			It("accepts complete with --reason and --gate-successor", func() {
				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"task", "complete", "my-task",
					"--reason", "all done",
					"--gate-successor", "none",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("status: completed"))
				Expect(string(content)).To(ContainSubstring("aborted_reason: all done"))
				Expect(string(content)).To(ContainSubstring("gate_successor: none"))
			})

			It("persists YAML-special characters in reason via the serializer", func() {
				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"task", "complete", "my-task",
					"--reason", "multi\nline: quoted",
					"--gate-successor", "none",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				getCmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"task", "get", "my-task", "aborted_reason",
					"--output", "json",
				)
				getSession, err := gexec.Start(getCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(getSession).Should(gexec.Exit(0))

				var parsed map[string]any
				Expect(json.Unmarshal(getSession.Out.Contents(), &parsed)).To(Succeed())
				Expect(parsed).To(HaveKeyWithValue("value", "multi\nline: quoted"))
			})
		})

		Context("field-less task with an incomplete subtask", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"my-task": "---\nstatus: in_progress\n---\n# My Task\n\n- [ ] not done\n",
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("completes with --force despite incomplete subtasks and without close-out fields", func() {
				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"task", "complete", "my-task", "--force",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("status: completed"))
				Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
				Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
			})
		})
	})

	Describe("vault-cli task update close-out gating", func() {
		var vaultPath, configPath string
		var cleanup func()

		BeforeEach(func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"my-task": "---\nstatus: in_progress\n---\n# My Task\n\n- [x] item one\n- [x] item two\n",
			})
		})

		AfterEach(func() {
			cleanup()
		})

		It("completes via checkbox sync without close-out fields", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "update", "my-task",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: completed"))
			Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
			Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
		})

		It("accepts checkbox-sync completion once the fields are present", func() {
			for _, args := range [][]string{
				{"task", "set", "my-task", "aborted_reason", "all done"},
				{"task", "set", "my-task", "gate_successor", "none"},
			} {
				setArgs := append([]string{"--config", configPath, "--vault", "test"}, args...)
				setCmd := exec.Command(binPath, setArgs...)
				setSession, err := gexec.Start(setCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(setSession).Should(gexec.Exit(0))
			}

			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "update", "my-task",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: completed"))
		})
	})

	Describe("vault-cli goal close-out gating", func() {
		var vaultPath, configPath string
		var cleanup func()

		BeforeEach(func() {
			vaultPath, configPath, cleanup = createTempVaultWithGoals(
				map[string]string{},
				map[string]string{
					"my-goal": "---\nstatus: in_progress\n---\n# My Goal\n",
				},
			)
		})

		AfterEach(func() {
			cleanup()
		})

		It("rejects goal aborted without reason", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"goal", "set", "my-goal", "status", "aborted",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(string(session.Err.Contents())).To(ContainSubstring("aborted_reason"))

			goalPath := filepath.Join(vaultPath, "Goals", "my-goal.md")
			content, err := os.ReadFile(goalPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).NotTo(ContainSubstring("status: aborted"))
		})

		It("accepts goal aborted with --reason and --gate-successor", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"goal", "set", "my-goal", "status", "aborted",
				"--reason", "no longer needed",
				"--gate-successor", "none",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			goalPath := filepath.Join(vaultPath, "Goals", "my-goal.md")
			content, err := os.ReadFile(goalPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: aborted"))
			Expect(string(content)).To(ContainSubstring("aborted_reason: no longer needed"))
			Expect(string(content)).To(ContainSubstring("gate_successor: none"))
		})

		It("accepts goal complete with --reason and --gate-successor", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"goal", "complete", "my-goal",
				"--reason", "achieved",
				"--gate-successor", "none",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			goalPath := filepath.Join(vaultPath, "Goals", "my-goal.md")
			content, err := os.ReadFile(goalPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: completed"))
			Expect(string(content)).To(ContainSubstring("aborted_reason: achieved"))
			Expect(string(content)).To(ContainSubstring("gate_successor: none"))
		})

		It("accepts goal set status completed without close-out fields", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"goal", "set", "my-goal", "status", "completed",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			goalPath := filepath.Join(vaultPath, "Goals", "my-goal.md")
			content, err := os.ReadFile(goalPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: completed"))
			Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
			Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
		})

		It("accepts goal complete without close-out fields", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"goal", "complete", "my-goal",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			goalPath := filepath.Join(vaultPath, "Goals", "my-goal.md")
			content, err := os.ReadFile(goalPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: completed"))
			Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
			Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
		})
	})

	Describe("vault-cli non-close-out transitions unaffected", func() {
		var vaultPath, configPath string
		var cleanup func()

		BeforeEach(func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"my-task": "---\nstatus: todo\n---\n# My Task\n",
			})
		})

		AfterEach(func() {
			cleanup()
		})

		It("task set status in_progress works without flags", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set", "my-task", "status", "in_progress",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: in_progress"))
			Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
		})

		It("task set status hold works without flags", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "set", "my-task", "status", "hold",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: hold"))
		})
	})

	Describe("vault-cli task show with YAML date literal", func() {
		var vaultPath, configPath string
		var cleanup func()

		BeforeEach(func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"aqua": `---
status: todo
priority: 2
defer_date: 2026-04-13
---
# Aqua
`,
			})
		})

		AfterEach(func() {
			cleanup()
		})

		It("outputs defer_date in JSON when YAML has a native date literal", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "show", "aqua",
				"--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say(`"defer_date":\s*"2026-04-13"`))
			Expect(string(session.Out.Contents())).NotTo(ContainSubstring("00:00:00 +0000 UTC"))
		})

		// vaultPath is assigned in BeforeEach to avoid unused variable lint error
		_ = &vaultPath
	})

	Describe("vault-cli task JSON schema", func() {
		var configPath string
		var cleanup func()

		BeforeEach(func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"schema-task": `---
status: in_progress
priority: 2
assignee: bborbe
recurring: weekly
phase: todo
defer_date: 2026-04-13
planned_date: "2026-04-15"
due_date: 2026-04-20T10:30:00Z
completed_date: "2026-03-09T12:30:00Z"
last_completed: 2026-03-08
task_identifier: 043d9cac-d56b-4a36-921e-b0e35819fb66
goals:
  - "[[Example Goal]]"
tags:
  - alpha
---
body
`,
			})
		})

		AfterEach(func() {
			cleanup()
		})

		It("includes all date fields with correct values in task show --output json", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "show", "schema-task",
				"--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			var parsed map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &parsed)).To(Succeed())
			Expect(parsed).To(HaveKeyWithValue("defer_date", "2026-04-13"))
			Expect(parsed).To(HaveKeyWithValue("planned_date", "2026-04-15"))
			Expect(parsed).To(HaveKeyWithValue("due_date", "2026-04-20T10:30:00Z"))
			Expect(parsed).To(HaveKeyWithValue("completed_date", "2026-03-09T12:30:00Z"))
			Expect(string(session.Out.Contents())).NotTo(ContainSubstring("00:00:00 +0000 UTC"))
		})

		It("includes all date fields with correct values in task list --output json", func() {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "list",
				"--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			var items []map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &items)).To(Succeed())
			Expect(items).To(HaveLen(1))
			item := items[0]
			Expect(item).To(HaveKeyWithValue("defer_date", "2026-04-13"))
			Expect(item).To(HaveKeyWithValue("planned_date", "2026-04-15"))
			Expect(item).To(HaveKeyWithValue("due_date", "2026-04-20T10:30:00Z"))
			Expect(item).To(HaveKeyWithValue("completed_date", "2026-03-09T12:30:00Z"))
			Expect(string(session.Out.Contents())).NotTo(ContainSubstring("00:00:00 +0000 UTC"))
		})
	})

	Describe("vault-cli blocked_by JSON surface", func() {
		var vaultPath, configPath string
		var cleanup func()

		runTaskListJSON := func() []map[string]any {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "list",
				"--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			var items []map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &items)).To(Succeed())
			return items
		}

		AfterEach(func() {
			cleanup()
		})

		It("AC1: task list emits the raw blocked_by list and the computed blocked flag", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"dep-a": `---
status: todo
blocked_by:
  - "A"
  - "B"
---
`,
			})
			items := runTaskListJSON()
			Expect(items).To(HaveLen(1))
			item := items[0]
			Expect(item).To(HaveKeyWithValue("name", "dep-a"))
			Expect(item).To(HaveKeyWithValue("blocked_by", []any{"A", "B"}))
			Expect(item).To(HaveKeyWithValue("blocked", true))
		})

		It("AC2: goal list emits both fields", func() {
			_, configPath, cleanup = createTempVaultWithGoals(
				map[string]string{},
				map[string]string{
					"dep-goal": `---
status: next
blocked_by:
  - "Absent Goal"
---
`,
				},
			)
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"goal", "list",
				"--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			var items []map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &items)).To(Succeed())
			Expect(items).To(HaveLen(1))
			item := items[0]
			Expect(item).To(HaveKeyWithValue("name", "dep-goal"))
			Expect(item).To(HaveKeyWithValue("blocked_by", []any{"Absent Goal"}))
			Expect(item).To(HaveKeyWithValue("blocked", true))
		})

		It("AC3: no dependency list means no new keys", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"plain-task": `---
status: todo
---
`,
			})
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				"task", "list",
				"--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			var items []map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &items)).To(Succeed())
			Expect(items).To(HaveLen(1))
			item := items[0]
			Expect(item).To(HaveLen(4))
			Expect(item).To(HaveKey("name"))
			Expect(item).To(HaveKey("status"))
			Expect(item).To(HaveKey("vault"))
			Expect(item).To(HaveKey("modified_date"))
			Expect(string(session.Out.Contents())).NotTo(ContainSubstring("blocked"))
		})

		It("AC5: blocked transitions to unblocked once the blocker is completed", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"dependent": `---
status: todo
blocked_by:
  - "Blocker"
---
`,
			})
			blocked := runTaskListJSON()
			Expect(blocked).To(HaveLen(1))
			Expect(blocked[0]).To(HaveKeyWithValue("blocked", true))

			// Complete the blocker by writing its file directly into the Tasks dir.
			err := os.WriteFile(
				filepath.Join(vaultPath, "Tasks", "Blocker.md"),
				[]byte("---\nstatus: completed\n---\n"),
				0600,
			)
			Expect(err).NotTo(HaveOccurred())

			unblocked := runTaskListJSON()
			Expect(unblocked).To(HaveLen(1))
			Expect(unblocked[0]).To(HaveKey("blocked"))
			Expect(unblocked[0]).To(HaveKeyWithValue("blocked", false))
		})

		It("AC6: wikilink and case-insensitive blocker names resolve through the real binary", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"dependent": `---
status: todo
blocked_by:
  - "[[blocker-task]]"
---
`,
				"Blocker-Task": `---
status: completed
---
`,
			})
			items := runTaskListJSON()
			Expect(items).To(HaveLen(1))
			Expect(items[0]).To(HaveKeyWithValue("name", "dependent"))
			Expect(items[0]).To(HaveKeyWithValue("blocked", false))
		})

		It("malformed scalar blocked_by is inert", func() {
			_, configPath, cleanup = createTempVault(map[string]string{
				"scalar-dep": `---
status: todo
blocked_by: A
---
`,
			})
			items := runTaskListJSON()
			Expect(items).To(HaveLen(1))
			item := items[0]
			Expect(item).NotTo(HaveKey("blocked"))
			Expect(item).NotTo(HaveKey("blocked_by"))
		})
	})

	Describe("vault-cli blocked_by list add and remove", func() {
		var vaultPath, configPath string
		var cleanup func()

		AfterEach(func() {
			cleanup()
		})

		runListJSON := func(entityType string) []map[string]any {
			cmd := exec.Command(
				binPath,
				"--config", configPath,
				"--vault", "test",
				entityType, "list",
				"--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			var items []map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &items)).To(Succeed())
			return items
		}

		runEntityCommand := func(args ...string) *gexec.Session {
			fullArgs := append(
				[]string{"--config", configPath, "--vault", "test"},
				args...,
			)
			session, err := gexec.Start(exec.Command(binPath, fullArgs...), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			return session
		}

		sha256OfFile := func(path string) string {
			data, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			sum := sha256.Sum256(data)
			return fmt.Sprintf("%x", sum)
		}

		It("AC1: task add appends to an existing blocked_by list without clobbering it", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": `---
status: in_progress
page_type: task
priority: 1
task_identifier: 22222222-2222-4222-8222-222222222222
blocked_by:
  - "[[Blocker A]]"
---
`,
			})

			session := runEntityCommand("task", "add", "Alpha", "blocked_by", "[[Blocker B]]")
			Eventually(session).Should(gexec.Exit(0))

			items := runListJSON("task")
			Expect(items).To(HaveLen(1))
			Expect(items[0]).To(HaveKeyWithValue("name", "Alpha"))
			Expect(items[0]).To(HaveKeyWithValue(
				"blocked_by",
				[]any{"[[Blocker A]]", "[[Blocker B]]"},
			))

			raw, err := os.ReadFile(filepath.Join(vaultPath, "Tasks", "Alpha.md"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(raw)).To(ContainSubstring("Blocker A"))
			Expect(string(raw)).To(ContainSubstring("Blocker B"))
		})

		It("AC2: task remove drops exactly one entry from a two-entry blocked_by list", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": `---
status: in_progress
page_type: task
priority: 1
task_identifier: 22222222-2222-4222-8222-222222222222
blocked_by:
  - "[[Blocker A]]"
  - "[[Blocker B]]"
---
`,
			})

			session := runEntityCommand("task", "remove", "Alpha", "blocked_by", "[[Blocker B]]")
			Eventually(session).Should(gexec.Exit(0))

			items := runListJSON("task")
			Expect(items).To(HaveLen(1))
			Expect(items[0]).To(HaveKeyWithValue("blocked_by", []any{"[[Blocker A]]"}))

			raw, err := os.ReadFile(filepath.Join(vaultPath, "Tasks", "Alpha.md"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(raw)).NotTo(ContainSubstring("Blocker B"))
		})

		It("AC3a: goal add appends to an existing blocked_by list", func() {
			_, configPath, cleanup = createTempVaultWithGoals(
				map[string]string{},
				map[string]string{
					"Beta": `---
status: next
page_type: goal
priority: 1
blocked_by:
  - "[[Blocker E]]"
---
`,
				},
			)

			session := runEntityCommand("goal", "add", "Beta", "blocked_by", "[[Blocker C]]")
			Eventually(session).Should(gexec.Exit(0))

			items := runListJSON("goal")
			Expect(items).To(HaveLen(1))
			Expect(items[0]).To(HaveKeyWithValue("name", "Beta"))
			Expect(items[0]).To(HaveKeyWithValue(
				"blocked_by",
				[]any{"[[Blocker E]]", "[[Blocker C]]"},
			))
		})

		It("AC4: task add on a scalar blocked_by is refused and writes nothing", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": `---
status: in_progress
page_type: task
priority: 1
task_identifier: 22222222-2222-4222-8222-222222222222
blocked_by: Blocker A
---
`,
			})

			taskFile := filepath.Join(vaultPath, "Tasks", "Alpha.md")
			before := sha256OfFile(taskFile)

			session := runEntityCommand("task", "add", "Alpha", "blocked_by", "[[Blocker B]]")
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(Equal(0))

			stderr := string(session.Err.Contents())
			Expect(stderr).To(ContainSubstring("blocked_by"))
			Expect(stderr).To(ContainSubstring("YAML list"))
			Expect(stderr).To(ContainSubstring("clear"))

			Expect(sha256OfFile(taskFile)).To(Equal(before))
		})
	})

	Describe("vault-cli blocked_by set refusal", func() {
		var vaultPath, configPath string
		var cleanup func()

		AfterEach(func() {
			cleanup()
		})

		runEntityCommand := func(args ...string) *gexec.Session {
			fullArgs := append(
				[]string{"--config", configPath, "--vault", "test"},
				args...,
			)
			session, err := gexec.Start(exec.Command(binPath, fullArgs...), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			return session
		}

		runTaskListJSON := func() []map[string]any {
			session := runEntityCommand("task", "list", "--output", "json")
			Eventually(session).Should(gexec.Exit(0))
			var items []map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &items)).To(Succeed())
			return items
		}

		sha256OfFile := func(path string) string {
			data, err := os.ReadFile(path) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			sum := sha256.Sum256(data)
			return fmt.Sprintf("%x", sum)
		}

		// Alpha holds only the base keys plus a one-entry blocked_by list, so no
		// unrelated issue can appear in any of these assertions.
		alphaWithList := `---
status: in_progress
page_type: task
priority: 1
task_identifier: 22222222-2222-4222-8222-222222222222
blocked_by:
  - "[[Blocker A]]"
---
`

		It("AC5: task set refuses a non-empty blocked_by, names the field, points at add, and writes nothing", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": alphaWithList,
			})
			taskFile := filepath.Join(vaultPath, "Tasks", "Alpha.md")
			before := sha256OfFile(taskFile)

			session := runEntityCommand("task", "set", "Alpha", "blocked_by", "[[Blocker A]]")
			Eventually(session).Should(gexec.Exit(1))

			stderr := string(session.Err.Contents())
			Expect(stderr).To(ContainSubstring("blocked_by"))
			Expect(stderr).To(ContainSubstring("add"))

			Expect(sha256OfFile(taskFile)).To(Equal(before))
		})

		It("AC5b: task set keeps the tags and goals comma-split coercion", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": alphaWithList,
			})

			Eventually(runEntityCommand("task", "set", "Alpha", "tags", "a,b")).Should(gexec.Exit(0))
			Eventually(runEntityCommand("task", "set", "Alpha", "goals", "g1,g2")).
				Should(gexec.Exit(0))

			items := runTaskListJSON()
			Expect(items).To(HaveLen(1))
			Expect(items[0]).To(HaveKeyWithValue("goals", []any{"g1", "g2"}))

			getSession := runEntityCommand("task", "get", "Alpha", "tags")
			Eventually(getSession).Should(gexec.Exit(0))
			Expect(string(getSession.Out.Contents())).To(ContainSubstring("a,b"))

			content, err := os.ReadFile(filepath.Join(vaultPath, "Tasks", "Alpha.md")) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchRegexp(`(?m)^[[:space:]]*- a$`))
		})

		It("AC6: task set blocked_by with the empty value stays legal", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": alphaWithList,
			})

			Eventually(runEntityCommand("task", "set", "Alpha", "blocked_by", "")).
				Should(gexec.Exit(0))

			items := runTaskListJSON()
			Expect(items).To(HaveLen(1))
			Expect(items[0]).NotTo(HaveKey("blocked_by"))
			Expect(items[0]).NotTo(HaveKey("blocked"))
		})

		It("AC6b: task clear blocked_by removes the key", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": alphaWithList,
			})

			Eventually(runEntityCommand("task", "clear", "Alpha", "blocked_by")).
				Should(gexec.Exit(0))

			content, err := os.ReadFile(filepath.Join(vaultPath, "Tasks", "Alpha.md")) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).NotTo(ContainSubstring("blocked_by"))
		})

		It("AC3b: goal set refuses a non-empty blocked_by", func() {
			_, configPath, cleanup = createTempVaultWithGoals(nil, map[string]string{
				"Beta": `---
status: next
page_type: goal
priority: 1
blocked_by:
  - "[[Blocker E]]"
---
`,
			})

			session := runEntityCommand("goal", "set", "Beta", "blocked_by", "[[Blocker D]]")
			Eventually(session).Should(gexec.Exit(1))

			stderr := string(session.Err.Contents())
			Expect(stderr).To(ContainSubstring("blocked_by"))
			Expect(stderr).To(ContainSubstring("add"))
		})

		It(`AC6c: goal set blocked_by "" and goal clear blocked_by stay legal`, func() {
			_, configPath, cleanup = createTempVaultWithGoals(nil, map[string]string{
				"Beta": `---
status: next
page_type: goal
priority: 1
blocked_by:
  - "[[Blocker E]]"
---
`,
			})

			Eventually(runEntityCommand("goal", "set", "Beta", "blocked_by", "")).
				Should(gexec.Exit(0))
			Eventually(runEntityCommand("goal", "clear", "Beta", "blocked_by")).
				Should(gexec.Exit(0))
		})
	})

	Describe("task append-metrics-session", func() {
		const (
			s1 = "11111111-1111-4111-8111-111111111111"
			s2 = "22222222-2222-4222-8222-222222222222"
			s3 = "33333333-3333-4333-8333-333333333333"
		)

		var vaultPath, configPath string
		var cleanup func()

		AfterEach(func() {
			cleanup()
		})

		runEntityCommand := func(args ...string) *gexec.Session {
			fullArgs := append(
				[]string{"--config", configPath, "--vault", "test"},
				args...,
			)
			session, err := gexec.Start(exec.Command(binPath, fullArgs...), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			return session
		}

		sha256OfFile := func(path string) string {
			data, err := os.ReadFile(path) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			sum := sha256.Sum256(data)
			return fmt.Sprintf("%x", sum)
		}

		readFile := func(path string) string {
			content, err := os.ReadFile(path) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			return string(content)
		}

		// countMetricsEntries counts the entries under the `metrics_sessions:`
		// frontmatter key as written on disk: every block-sequence item opens with a
		// `- session_id:` line.
		countMetricsEntries := func(content string) int {
			count := 0
			for _, line := range strings.Split(content, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "- session_id:") {
					count++
				}
			}
			return count
		}

		// frontmatterOf unmarshals only the frontmatter block of a page file.
		// Unmarshalling the whole file would fail on the markdown body.
		frontmatterOf := func(path string) map[string]any {
			text := readFile(path)
			Expect(strings.HasPrefix(text, "---\n")).To(BeTrue(), "no frontmatter opening")
			rest := strings.TrimPrefix(text, "---\n")
			end := strings.Index(rest, "\n---\n")
			Expect(end).To(BeNumerically(">=", 0), "no frontmatter closing")
			var parsed map[string]any
			Expect(yaml.Unmarshal([]byte(rest[:end]), &parsed)).To(Succeed())
			return parsed
		}

		sortedKeys := func(m map[string]any) []string {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return keys
		}

		// yaml.v3 resolves a timestamp-shaped plain scalar to !!timestamp and
		// therefore quotes it, so the quote is present by construction.
		quotedStartedAtPattern := regexp.MustCompile(`started_at: "\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)
		startedAtTimePattern := regexp.MustCompile(`started_at: "?\d{4}-\d{2}-\d{2}T(\d{2}:\d{2}:\d{2})`)

		// Every fixture's frontmatter keys are authored in alphabetical order, and
		// its metrics_sessions block in the writer's own shape. The storage writer
		// re-serializes the whole frontmatter in alphabetical key order on every
		// write, so a hand-authored non-alphabetical fixture would be reordered and
		// every byte comparison below would be meaningless.
		baseFrontmatter := `---
page_type: task
priority: 1
status: in_progress
task_identifier: 11111111-1111-4111-8111-111111111111
---
body line
`

		twoEntryFrontmatter := `---
metrics_sessions:
    - session_id: 11111111-1111-4111-8111-111111111111
      started_at: "2026-09-01T08:00:00Z"
    - session_id: 22222222-2222-4222-8222-222222222222
      started_at: "2026-09-02T08:00:00Z"
page_type: task
priority: 1
status: in_progress
task_identifier: 11111111-1111-4111-8111-111111111111
---
body line
`

		betaWithSessionID := `---
claude_session_id: 11111111-1111-4111-8111-111111111111
page_type: task
priority: 1
status: in_progress
task_identifier: 11111111-1111-4111-8111-111111111111
---
body line
`

		alphaWithDuplicateSession := `---
claude_session_id: 11111111-1111-4111-8111-111111111111
metrics_sessions:
    - session_id: 11111111-1111-4111-8111-111111111111
      started_at: "2026-09-01T08:00:00Z"
page_type: task
priority: 1
status: in_progress
task_identifier: 11111111-1111-4111-8111-111111111111
---
body line
`

		It("AC1: task append-metrics-session appends exactly one entry and preserves every base key", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": baseFrontmatter,
			})
			taskFile := filepath.Join(vaultPath, "Tasks", "Alpha.md")
			before := readFile(taskFile)
			beforeTail := before[strings.Index(before, "page_type:"):]

			session := runEntityCommand("task", "append-metrics-session", "Alpha", s3)
			Eventually(session).Should(gexec.Exit(0))

			after := readFile(taskFile)
			// Every base key and the body are byte-identical, and they are contiguous
			// because metrics_sessions sorts before page_type.
			Expect(strings.Contains(after, beforeTail)).To(BeTrue(),
				"a base key or the body changed:\n%s", after)
			Expect(countMetricsEntries(after)).To(Equal(1))
			Expect(after).To(ContainSubstring("session_id: " + s3))
			Expect(after).To(MatchRegexp(quotedStartedAtPattern.String()))
			match := startedAtTimePattern.FindStringSubmatch(after)
			Expect(match).NotTo(BeNil(), "no started_at line in:\n%s", after)
			Expect(match[1]).NotTo(Equal("00:00:00"))
		})

		It("AC2: task append-metrics-session accumulates with the prior entry blocks byte-identical", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": twoEntryFrontmatter,
			})
			taskFile := filepath.Join(vaultPath, "Tasks", "Alpha.md")
			before := readFile(taskFile)
			beforeBlock := before[strings.Index(before, "metrics_sessions:"):strings.Index(before, "page_type:")]

			Eventually(runEntityCommand("task", "append-metrics-session", "Alpha", s3)).
				Should(gexec.Exit(0))

			after := readFile(taskFile)
			Expect(countMetricsEntries(after)).To(Equal(3))
			Expect(strings.Index(after, "session_id: "+s1)).
				To(BeNumerically("<", strings.Index(after, "session_id: "+s2)))
			Expect(strings.Index(after, "session_id: "+s2)).
				To(BeNumerically("<", strings.Index(after, "session_id: "+s3)))
			Expect(strings.Contains(after, beforeBlock)).To(BeTrue(),
				"the pre-existing entries changed:\n%s", after)
			Expect(strings.Contains(after, before[strings.Index(before, "page_type:"):])).To(BeTrue(),
				"a base key or the body changed:\n%s", after)
		})

		It("AC3: the new verb's entry is schema-identical to the work-on path's entry", func() {
			vaultPath, configPath, cleanup = createTempVaultWithCurrentUser(map[string]string{
				"Alpha": baseFrontmatter,
				"Beta":  betaWithSessionID,
			})

			Eventually(runEntityCommand("task", "append-metrics-session", "Alpha", s2)).
				Should(gexec.Exit(0))

			// The cached-session branch appends through the existing work-on path and
			// spawns nothing: claude_script is not installed, so the starter is nil.
			session := runEntityCommand("task", "work-on", "Beta", "--mode", "headless")
			Eventually(session, 30*time.Second).Should(gexec.Exit(0))

			alphaSessions, ok := frontmatterOf(
				filepath.Join(vaultPath, "Tasks", "Alpha.md"),
			)["metrics_sessions"].([]any)
			Expect(ok).To(BeTrue(), "Alpha metrics_sessions is not a list")
			Expect(alphaSessions).To(HaveLen(1))

			betaSessions, ok := frontmatterOf(
				filepath.Join(vaultPath, "Tasks", "Beta.md"),
			)["metrics_sessions"].([]any)
			Expect(ok).To(BeTrue(), "Beta metrics_sessions is not a list")
			Expect(betaSessions).To(HaveLen(1))

			alphaEntry, ok := alphaSessions[0].(map[string]any)
			Expect(ok).To(BeTrue(), "Alpha entry is not a map")
			betaEntry, ok := betaSessions[0].(map[string]any)
			Expect(ok).To(BeTrue(), "Beta entry is not a map")

			Expect(sortedKeys(alphaEntry)).To(Equal(sortedKeys(betaEntry)))
			Expect(sortedKeys(alphaEntry)).To(Equal([]string{"session_id", "started_at"}))

			// Values differ by design — assert shapes, not equality.
			for _, entry := range []map[string]any{alphaEntry, betaEntry} {
				sessionID, ok := entry["session_id"].(string)
				Expect(ok).To(BeTrue(), "session_id is not a string")
				Expect(sessionID).NotTo(BeEmpty())
				startedAt, ok := entry["started_at"].(string)
				Expect(ok).To(BeTrue(), "started_at is not a string")
				Expect(startedAt).To(MatchRegexp(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`))
				Expect(startedAt).NotTo(ContainSubstring("T00:00:00"))
			}
		})

		It("AC4: task set, add and remove each refuse metrics_sessions and leave the file byte-identical", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": baseFrontmatter,
			})
			taskFile := filepath.Join(vaultPath, "Tasks", "Alpha.md")
			before := sha256OfFile(taskFile)

			for _, args := range [][]string{
				{"task", "set", "Alpha", "metrics_sessions", `{"session_id":"x"}`},
				{"task", "add", "Alpha", "metrics_sessions", "x"},
				{"task", "remove", "Alpha", "metrics_sessions", "x"},
			} {
				session := runEntityCommand(args...)
				Eventually(session).Should(gexec.Exit(1))
				stderr := string(session.Err.Contents())
				Expect(stderr).To(ContainSubstring("metrics_sessions"))
				Expect(stderr).To(ContainSubstring("append-metrics-session"))
			}

			forceSession := runEntityCommand(
				"task", "set", "Alpha", "metrics_sessions", "x", "--force",
			)
			Eventually(forceSession).Should(gexec.Exit(1))
			forceStderr := string(forceSession.Err.Contents())
			Expect(forceStderr).To(ContainSubstring("metrics_sessions"))
			Expect(forceStderr).To(ContainSubstring("append-metrics-session"))

			Expect(sha256OfFile(taskFile)).To(Equal(before))
		})

		It("AC5: an empty, non-UUID or path-bearing session id is refused with nothing written", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"Alpha": baseFrontmatter,
			})
			taskFile := filepath.Join(vaultPath, "Tasks", "Alpha.md")
			before := sha256OfFile(taskFile)

			for _, sessionID := range []string{"", "not-a-uuid", "../escape"} {
				session := runEntityCommand("task", "append-metrics-session", "Alpha", sessionID)
				Eventually(session).Should(gexec.Exit(1))
				Expect(string(session.Err.Contents())).
					To(ContainSubstring("expected a well-formed UUID"))
			}

			Expect(sha256OfFile(taskFile)).To(Equal(before))
		})

		It("AC8: task work-on appends a second entry for an already-recorded session id", func() {
			vaultPath, configPath, cleanup = createTempVaultWithCurrentUser(map[string]string{
				"Alpha": alphaWithDuplicateSession,
			})

			session := runEntityCommand("task", "work-on", "Alpha", "--mode", "headless")
			Eventually(session, 30*time.Second).Should(gexec.Exit(0))

			after := readFile(filepath.Join(vaultPath, "Tasks", "Alpha.md"))
			Expect(countMetricsEntries(after)).To(Equal(2))
			// The entry-line prefix is load-bearing: `claude_session_id: <s1>` also
			// contains the substring `session_id: <s1>`, so a bare Count over the
			// file would report 3 and prove nothing about the accumulator.
			Expect(strings.Count(after, "- session_id: "+s1)).To(Equal(2))
		})
	})

	Describe("vault-cli defer", func() {
		var vaultPath, configPath string
		var cleanup func()

		Context("when task exists", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"my-task": `---
status: todo
priority: 2
---
# My Task
This is my task.
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 0 and adds defer_date", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"defer",
					"my-task",
					"+7d",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				// Verify file was updated with defer_date
				taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("defer_date:"))
			})
		})

		It("defaults to +1d when no date argument provided", func() {
			vaultPath, configPath, cleanup = createTempVault(map[string]string{
				"my-task": `---
status: todo
priority: 2
---
# My Task
This is my task.
`,
			})
			defer cleanup()
			cmd := exec.Command(
				binPath,
				"--config",
				configPath,
				"--vault",
				"test",
				"task",
				"defer",
				"my-task",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			// Verify file was updated with defer_date
			taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("defer_date:"))
		})

		Context("with invalid date format", func() {
			BeforeEach(func() {
				vaultPath, configPath, cleanup = createTempVault(map[string]string{
					"my-task": `---
status: todo
priority: 2
---
# My Task
`,
				})
			})

			AfterEach(func() {
				cleanup()
			})

			It("exits 1", func() {
				cmd := exec.Command(
					binPath,
					"--config",
					configPath,
					"--vault",
					"test",
					"task",
					"defer",
					"my-task",
					"invalid-date",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
			})
		})

	})

	Describe("vault-cli resolve", func() {
		var configPath string
		var cleanup func()

		Context("with task and goal present", func() {
			BeforeEach(func() {
				_, configPath, cleanup = createTempVaultWithGoals(
					map[string]string{
						"my-task": `---
status: todo
priority: 2
---
# My Task
`,
					},
					map[string]string{
						"my-goal": `---
status: todo
page_type: goal
---
# My Goal
`,
					},
				)
			})

			AfterEach(func() {
				cleanup()
			})

			It("returns task match as JSON", func() {
				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"resolve", "my-task",
					"--output", "json",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				var result map[string]any
				Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
				Expect(result).To(HaveKeyWithValue("type", "task"))
				Expect(result).To(HaveKeyWithValue("name", "my-task"))
				Expect(result).To(HaveKeyWithValue("found", true))
			})

			It("returns goal match as JSON", func() {
				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"resolve", "my-goal",
					"--output", "json",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				var result map[string]any
				Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
				Expect(result).To(HaveKeyWithValue("type", "goal"))
				Expect(result).To(HaveKeyWithValue("name", "my-goal"))
				Expect(result).To(HaveKeyWithValue("found", true))
			})

			It("returns not found for unknown name", func() {
				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"--vault", "test",
					"resolve", "nonexistent",
					"--output", "json",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				var result map[string]any
				Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
				Expect(result).To(HaveKeyWithValue("type", ""))
				Expect(result).To(HaveKeyWithValue("name", "nonexistent"))
				Expect(result).To(HaveKeyWithValue("found", false))
				// Verify empty-string type and false are serialized (not omitempty)
				raw := string(session.Out.Contents())
				Expect(raw).To(ContainSubstring(`"type": ""`))
				Expect(raw).To(ContainSubstring(`"found": false`))
			})

			It("task-first priority when name matches both", func() {
				_, configPath2, cleanup2 := createTempVaultWithGoals(
					map[string]string{
						"collision": `---
status: todo
priority: 1
---
# Collision Task
`,
					},
					map[string]string{
						"collision": `---
status: todo
page_type: goal
---
# Collision Goal
`,
					},
				)
				defer cleanup2()

				cmd := exec.Command(
					binPath,
					"--config", configPath2,
					"--vault", "test",
					"resolve", "collision",
					"--output", "json",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				var result map[string]any
				Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
				Expect(result).To(HaveKeyWithValue("type", "task"))
				Expect(result).To(HaveKeyWithValue("found", true))
			})
		})

		Context("multi-vault fall-through", func() {
			It("resolves a task found in one vault even when the other misses", func() {
				_, _, configPath, cleanup := createTwoTempVaults(
					map[string]string{
						"my-task": `---
status: todo
priority: 2
---
# My Task
`,
					},
					nil,
					nil,
					nil,
				)
				defer cleanup()

				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"resolve", "my-task",
					"--output", "json",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				var result map[string]any
				Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
				Expect(result).To(HaveKeyWithValue("type", "task"))
				Expect(result).To(HaveKeyWithValue("name", "my-task"))
				Expect(result).To(HaveKeyWithValue("found", true))
				Expect(strings.Count(string(session.Out.Contents()), `"found"`)).To(Equal(1))
			})

			It("resolves a goal found in one vault even when the other misses", func() {
				_, _, configPath, cleanup := createTwoTempVaults(
					nil,
					nil,
					nil,
					map[string]string{
						"my-goal": `---
status: todo
page_type: goal
---
# My Goal
`,
					},
				)
				defer cleanup()

				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"resolve", "my-goal",
					"--output", "json",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				var result map[string]any
				Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
				Expect(result).To(HaveKeyWithValue("type", "goal"))
				Expect(result).To(HaveKeyWithValue("name", "my-goal"))
				Expect(result).To(HaveKeyWithValue("found", true))
				Expect(strings.Count(string(session.Out.Contents()), `"found"`)).To(Equal(1))
			})

			It("returns a single found:false JSON after both vaults miss", func() {
				_, _, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)
				defer cleanup()

				cmd := exec.Command(
					binPath,
					"--config", configPath,
					"resolve", "nonexistent",
					"--output", "json",
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				var result map[string]any
				Expect(json.Unmarshal(session.Out.Contents(), &result)).To(Succeed())
				Expect(result).To(HaveKeyWithValue("type", ""))
				Expect(result).To(HaveKeyWithValue("name", "nonexistent"))
				Expect(result).To(HaveKeyWithValue("found", false))
				raw := string(session.Out.Contents())
				Expect(raw).To(ContainSubstring(`"type": ""`))
				Expect(raw).To(ContainSubstring(`"found": false`))
				Expect(strings.Count(raw, `"found"`)).To(Equal(1))
				Expect(raw).NotTo(ContainSubstring("not found in any vault"))
				Expect(string(session.Err.Contents())).NotTo(ContainSubstring("not found in any vault"))
			})
		})
	})

	Describe("vault-cli task assignee clear escalation", func() {
		It("task set assignee empty exits 0 and reports the failure when the broker is unreachable", func() {
			vaultPath, configPath, cleanup := createTempVaultWithBrokers("127.0.0.1:1", "master")
			defer cleanup()

			taskPath := filepath.Join(vaultPath, "Tasks", "Park Me.md")
			Expect(os.WriteFile(taskPath, []byte(
				"---\nstatus: in_progress\nassignee: alice\ntask_identifier: 0f6a3a0e-0000-4000-8000-000000000001\n---\n",
			), 0600)).To(Succeed())

			cmd := exec.Command(binPath, "--config", configPath, "task", "set", "Park Me", "assignee", "")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 30*time.Second).Should(gexec.Exit(0))

			Expect(string(session.Out.Contents())).To(Equal("✅ Set assignee= on: Park Me\n"))
			Expect(string(session.Err.Contents())).
				To(ContainSubstring("publish agent-escalation notification for task"))
			Expect(string(session.Err.Contents())).
				To(ContainSubstring("escalated by alice failed:"))

			content, err := os.ReadFile(taskPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).NotTo(ContainSubstring("alice"))
			Expect(string(content)).To(ContainSubstring("status: in_progress"))
		})

		It("task clear assignee exits 0 with unchanged output when no broker is configured", func() {
			vaultPath, configPath, cleanup := createTempVault(map[string]string{
				"Park Me": "---\nstatus: in_progress\nassignee: alice\n---\n",
			})
			defer cleanup()

			taskPath := filepath.Join(vaultPath, "Tasks", "Park Me.md")

			cmd := exec.Command(binPath, "--config", configPath, "task", "clear", "Park Me", "assignee")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 10*time.Second).Should(gexec.Exit(0))

			Expect(string(session.Out.Contents())).To(Equal("✅ Cleared assignee on: Park Me\n"))
			Expect(session.Err.Contents()).To(BeEmpty())

			content, err := os.ReadFile(taskPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).NotTo(ContainSubstring("assignee"))
			Expect(string(content)).To(ContainSubstring("status: in_progress"))
		})
	})

	Describe("vault-cli topic command family", func() {
		It("topic --help lists exactly the twelve leaves", func() {
			cmd := exec.Command(binPath, "topic", "--help")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			leaves := helpLeafNames(string(session.Out.Contents()))
			Expect(leaves).NotTo(BeEmpty())
			Expect(leaves).To(HaveLen(12))
			Expect(leaves).To(ConsistOf(
				"add", "clear", "complete", "defer", "get", "lint", "list",
				"remove", "search", "set", "show", "work-on",
			))
		})

		It("topic --help leaf set equals the goal --help leaf set", func() {
			topicCmd := exec.Command(binPath, "topic", "--help")
			topicSession, err := gexec.Start(topicCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(topicSession).Should(gexec.Exit(0))

			goalCmd := exec.Command(binPath, "goal", "--help")
			goalSession, err := gexec.Start(goalCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(goalSession).Should(gexec.Exit(0))

			topicLeaves := helpLeafNames(string(topicSession.Out.Contents()))
			goalLeaves := helpLeafNames(string(goalSession.Out.Contents()))

			// Non-empty on both sides first: two empty sets would otherwise satisfy
			// an equality check if the help format ever changed.
			Expect(topicLeaves).NotTo(BeEmpty())
			Expect(goalLeaves).NotTo(BeEmpty())

			topicSorted := append([]string(nil), topicLeaves...)
			goalSorted := append([]string(nil), goalLeaves...)
			sort.Strings(topicSorted)
			sort.Strings(goalSorted)
			Expect(topicSorted).To(Equal(goalSorted))
		})

		It("topic --help contains none of the eleven goal slash-command names", func() {
			cmd := exec.Command(binPath, "topic", "--help")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			out := string(session.Out.Contents())
			for _, name := range []string{
				"plan-goal", "execute-goal", "verify-goal", "audit-goal",
				"create-goal", "update-goal", "goal-status", "launch-goal",
				"work-on-goal", "complete-goal", "defer-goal",
			} {
				Expect(out).NotTo(ContainSubstring(name), "topic help must not mention %s", name)
			}
		})

		It("topic show emits phase at .fields.phase, equal to what goal show emits", func() {
			_, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"With Phase": "---\nstatus: in_progress\nphase: planning\n---\n# With Phase\n",
				},
				map[string]string{
					"With Phase": "---\nstatus: in_progress\nphase: planning\n---\n# With Phase\n",
				},
			)
			defer cleanup()

			topicPhase := showPhaseField(configPath, "topic", "With Phase")
			goalPhase := showPhaseField(configPath, "goal", "With Phase")

			Expect(topicPhase).To(Equal("planning"))
			Expect(topicPhase).To(Equal(goalPhase))
		})

		It("topic show omits the phase key entirely when the page carries none", func() {
			_, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Without Phase": "---\nstatus: in_progress\n---\n# Without Phase\n",
				},
				nil,
			)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "show", "Without Phase", "--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			var parsed map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &parsed)).To(Succeed())
			fields, ok := parsed["fields"].(map[string]any)
			Expect(ok).To(BeTrue())
			// Anchor on a key that IS present so the negative check cannot pass on
			// empty output.
			Expect(fields).To(HaveKey("status"))
			_, present := fields["phase"]
			Expect(present).To(BeFalse())
		})

		It("topic get prints an empty line and exits 0 for an absent phase", func() {
			_, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Without Phase": "---\nstatus: in_progress\n---\n",
					"With Phase":    "---\nstatus: in_progress\nphase: planning\n---\n",
				},
				nil,
			)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "get", "Without Phase", "phase",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(Equal("\n"))
		})

		It("topic get prints the on-disk phase value and exits 0 when the page carries one", func() {
			_, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"With Phase": "---\nstatus: in_progress\nphase: planning\n---\n",
				},
				nil,
			)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "get", "With Phase", "phase",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(Equal("planning\n"))
		})

		It("topic set writes a canonical phase to the page on disk and topic show surfaces it", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Phase Target": "---\nstatus: in_progress\n---\n# Phase Target\n",
				},
				nil,
			)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "set", "Phase Target", "phase", "execution",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			content, err := os.ReadFile(filepath.Join(vaultPath, "Topics", "Phase Target.md"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("phase: execution"))

			Expect(showPhaseField(configPath, "topic", "Phase Target")).To(Equal("execution"))
		})

		It("topic set refuses a non-canonical phase with the validator's wording and leaves the page byte-identical", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Phase Target": "---\nstatus: in_progress\n---\n# Phase Target\n",
				},
				nil,
			)
			defer cleanup()

			sha256OfFile := func(path string) string {
				data, err := os.ReadFile(path)
				Expect(err).NotTo(HaveOccurred())
				sum := sha256.Sum256(data)
				return fmt.Sprintf("%x", sum)
			}

			topicPath := filepath.Join(vaultPath, "Topics", "Phase Target.md")
			before := sha256OfFile(topicPath)

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "set", "Phase Target", "phase", "bogus",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))

			Expect(string(session.Err.Contents())).To(ContainSubstring("unknown topic phase 'bogus'"))
			Expect(sha256OfFile(topicPath)).To(Equal(before))
		})

		It("topic set with an empty phase value removes the phase line from the page", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Phase Target": "---\nstatus: in_progress\nphase: execution\n---\n# Phase Target\n",
				},
				nil,
			)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "set", "Phase Target", "phase", "",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			content, err := os.ReadFile(filepath.Join(vaultPath, "Topics", "Phase Target.md"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).NotTo(ContainSubstring("phase:"))

			showCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "show", "Phase Target", "--output", "json",
			)
			showSession, err := gexec.Start(showCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(showSession).Should(gexec.Exit(0))

			var parsed map[string]any
			Expect(json.Unmarshal(showSession.Out.Contents(), &parsed)).To(Succeed())
			fields, ok := parsed["fields"].(map[string]any)
			Expect(ok).To(BeTrue())
			// Anchor on a key that IS present so the negative check cannot pass on
			// empty output.
			Expect(fields).To(HaveKey("status"))
			_, present := fields["phase"]
			Expect(present).To(BeFalse())
		})

		It("topic set on an unrelated key leaves a page with no phase line without inventing a phase", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"No Phase": "---\nstatus: backlog\n---\n# No Phase\n",
				},
				nil,
			)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "set", "No Phase", "assignee", "alice",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			content, err := os.ReadFile(filepath.Join(vaultPath, "Topics", "No Phase.md"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).NotTo(ContainSubstring("phase:"))
			Expect(string(content)).To(ContainSubstring("assignee: alice"))
		})

		It("topic lint reports no phase mismatch for a consistent pair and one for an inconsistent pair", func() {
			_, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Phase Target": "---\nstatus: in_progress\nphase: execution\n---\n# Phase Target\n",
				},
				nil,
			)
			defer cleanup()

			cleanCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test", "topic", "lint",
			)
			cleanSession, err := gexec.Start(cleanCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(cleanSession).Should(gexec.Exit(0))
			Expect(string(cleanSession.Out.Contents())).NotTo(ContainSubstring("STATUS_PHASE_MISMATCH"))

			setCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "set", "Phase Target", "phase", "done",
			)
			setSession, err := gexec.Start(setCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(setSession).Should(gexec.Exit(0))

			badCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test", "topic", "lint",
			)
			badSession, err := gexec.Start(badCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(badSession).Should(gexec.Exit(1))

			out := string(badSession.Out.Contents())
			Expect(out).To(ContainSubstring("STATUS_PHASE_MISMATCH"))
			Expect(out).To(ContainSubstring("Phase Target.md"))
		})

		It("topic set, get and clear round-trip a frontmatter key on disk", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Round Trip": "---\nstatus: in_progress\n---\n# Round Trip\n",
				},
				nil,
			)
			defer cleanup()

			topicPath := filepath.Join(vaultPath, "Topics", "Round Trip.md")

			setCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "set", "Round Trip", "owner", "alice",
			)
			setSession, err := gexec.Start(setCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(setSession).Should(gexec.Exit(0))

			content, err := os.ReadFile(topicPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("owner: alice"))

			getCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "get", "Round Trip", "owner",
			)
			getSession, err := gexec.Start(getCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(getSession).Should(gexec.Exit(0))
			Expect(string(getSession.Out.Contents())).To(Equal("alice\n"))

			clearCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "clear", "Round Trip", "owner",
			)
			clearSession, err := gexec.Start(clearCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(clearSession).Should(gexec.Exit(0))

			cleared, err := os.ReadFile(topicPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(cleared)).NotTo(ContainSubstring("owner"))
			Expect(string(cleared)).To(ContainSubstring("status: in_progress"))
		})

		It("topic add and topic remove round-trip a list field on disk", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Round Trip": "---\nstatus: in_progress\n---\n# Round Trip\n",
				},
				nil,
			)
			defer cleanup()

			topicPath := filepath.Join(vaultPath, "Topics", "Round Trip.md")

			addAlpha := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "add", "Round Trip", "tags", "alpha",
			)
			addAlphaSession, err := gexec.Start(addAlpha, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(addAlphaSession).Should(gexec.Exit(0))

			afterFirst := countTagEntries(topicPath)

			addBeta := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "add", "Round Trip", "tags", "beta",
			)
			addBetaSession, err := gexec.Start(addBeta, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(addBetaSession).Should(gexec.Exit(0))

			afterSecond := countTagEntries(topicPath)
			Expect(afterSecond).To(Equal(afterFirst + 1))

			removeCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "remove", "Round Trip", "tags", "alpha",
			)
			removeSession, err := gexec.Start(removeCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(removeSession).Should(gexec.Exit(0))

			afterRemove := countTagEntries(topicPath)
			Expect(afterRemove).To(Equal(afterSecond - 1))
		})

		It("topic complete moves the status to completed and refuses a second complete", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Round Trip": "---\nstatus: in_progress\n---\n# Round Trip\n",
				},
				nil,
			)
			defer cleanup()

			topicPath := filepath.Join(vaultPath, "Topics", "Round Trip.md")

			completeCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "complete", "Round Trip",
			)
			completeSession, err := gexec.Start(completeCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(completeSession).Should(gexec.Exit(0))

			completed, err := os.ReadFile(topicPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(completed)).To(ContainSubstring("status: completed"))

			secondCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "complete", "Round Trip",
			)
			secondSession, err := gexec.Start(secondCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(secondSession).Should(gexec.Exit(1))

			afterSecond, err := os.ReadFile(topicPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(afterSecond).To(Equal(completed))
		})

		It("topic defer writes defer_date for a relative and an absolute date", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Round Trip": "---\nstatus: in_progress\n---\n# Round Trip\n",
				},
				nil,
			)
			defer cleanup()

			topicPath := filepath.Join(vaultPath, "Topics", "Round Trip.md")

			relativeCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "defer", "Round Trip", "+7d",
			)
			relativeSession, err := gexec.Start(relativeCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(relativeSession).Should(gexec.Exit(0))

			// The run date is not pinnable in a subprocess, so the expected date is
			// computed here rather than asserted as a literal. The value is stored as
			// a quoted YAML string, hence the quotes in the substring.
			expectedRelative := time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")
			relative, err := os.ReadFile(topicPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(relative)).To(ContainSubstring(`defer_date: "` + expectedRelative + `"`))

			absoluteCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "defer", "Round Trip", "2027-03-19",
			)
			absoluteSession, err := gexec.Start(absoluteCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(absoluteSession).Should(gexec.Exit(0))

			absolute, err := os.ReadFile(topicPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(absolute)).To(ContainSubstring("2027-03-19"))
		})

		It("topic defer refuses a past date and leaves the page byte-identical", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Round Trip": "---\nstatus: in_progress\n---\n# Round Trip\n",
				},
				nil,
			)
			defer cleanup()

			topicPath := filepath.Join(vaultPath, "Topics", "Round Trip.md")

			before, err := os.ReadFile(topicPath)
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "defer", "Round Trip", "2000-01-01",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))

			after, err := os.ReadFile(topicPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(before))
		})

		It("topic lint reports a seeded duplicate key and passes a clean page", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Dupe":  "---\nstatus: in_progress\nstatus: in_progress\n---\n# Dupe\n",
					"Clean": "---\nstatus: in_progress\n---\n# Clean\n",
				},
				nil,
			)
			defer cleanup()

			cmd := exec.Command(binPath, "--config", configPath, "--vault", "test", "topic", "lint")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))

			out := string(session.Out.Contents())
			Expect(out).To(ContainSubstring("Dupe.md"))
			Expect(out).To(ContainSubstring("DUPLICATE_KEY"))
			Expect(out).NotTo(ContainSubstring("Clean.md"))

			Expect(os.Remove(filepath.Join(vaultPath, "Topics", "Dupe.md"))).To(Succeed())

			cleanCmd := exec.Command(binPath, "--config", configPath, "--vault", "test", "topic", "lint")
			cleanSession, err := gexec.Start(cleanCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(cleanSession).Should(gexec.Exit(0))
			Expect(string(cleanSession.Out.Contents())).To(ContainSubstring("No lint issues found"))
		})

		It("topic search reaches the semantic search operation under a PATH that cannot resolve semantic-search-mcp", func() {
			_, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Attention Routing": "---\nstatus: in_progress\n---\nattention routing marker\n",
				},
				nil,
			)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "search", "attention routing",
			)
			// Run the command with an environment whose PATH cannot resolve
			// semantic-search-mcp. The inherited PATH is removed rather than
			// shadowed, so exactly one PATH reaches the child and the lookup fails
			// deterministically on every host — the container and a developer
			// machine alike.
			env := make([]string, 0, len(os.Environ())+1)
			for _, entry := range os.Environ() {
				if strings.HasPrefix(entry, "PATH=") {
					continue
				}
				env = append(env, entry)
			}
			cmd.Env = append(env, "PATH=/usr/bin:/bin")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// The child runs with a PATH that cannot resolve semantic-search-mcp, so
			// exec.LookPath fails on every host — the container and a developer
			// machine alike — and this spec no longer depends on where the binary
			// happens to be installed. The failure names the missing binary, which
			// is what proves the invocation reached the search operation rather than
			// stopping at the CLI layer.
			Eventually(session).Should(gexec.Exit(1))

			combined := string(session.Out.Contents()) + string(session.Err.Contents())
			Expect(combined).To(ContainSubstring("semantic-search-mcp not found on PATH"))

			Expect(string(session.Out.Contents())).NotTo(ContainSubstring("unknown command"))
			Expect(string(session.Err.Contents())).NotTo(ContainSubstring("unknown command"))
		})

		It("topic list returns exactly the pages on disk in the configured directory", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Attention Routing": "---\nstatus: in_progress\n---\n",
					"Second Topic":      "---\nstatus: backlog\n---\n",
				},
				nil,
			)
			defer cleanup()

			topicsDirPath := filepath.Join(vaultPath, "Topics")
			Expect(os.WriteFile(
				filepath.Join(topicsDirPath, "notes.txt"), []byte("not a page"), 0600,
			)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(topicsDirPath, "Nested"), 0755)).To(Succeed())
			Expect(os.WriteFile(
				filepath.Join(topicsDirPath, "Nested", "Hidden.md"), []byte("---\n---\n"), 0600,
			)).To(Succeed())

			// --all so the backlog page is not dropped by the default
			// next/todo/in_progress status filter the generic list shares with goals.
			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "list", "--all", "--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			var items []map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &items)).To(Succeed())
			Expect(items).To(HaveLen(2))
		})

		It("topic list uses a configured-but-absent directory verbatim without falling back", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopics("Absent Topics")
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "list", "--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			var items []map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &items)).To(Succeed())
			Expect(items).To(BeEmpty())

			// The default topics directory must not have been created as a fallback.
			_, statErr := os.Stat(filepath.Join(vaultPath, "23 Topics"))
			Expect(os.IsNotExist(statErr)).To(BeTrue())
		})

		It("topic show refuses a nonexistent topic with a non-zero exit", func() {
			_, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Real Topic": "---\nstatus: in_progress\n---\n",
				},
				nil,
			)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "show", "Nonexistent",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
		})

		It("topic show surfaces a non-canonical phase value without rejecting the page", func() {
			_, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Odd Phase": "---\nstatus: in_progress\nphase: whatever-the-vault-holds\n---\n",
				},
				nil,
			)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "show", "Odd Phase", "--output", "json",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			var parsed map[string]any
			Expect(json.Unmarshal(session.Out.Contents(), &parsed)).To(Succeed())
			fields, ok := parsed["fields"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(fields["phase"]).To(Equal("whatever-the-vault-holds"))
		})

		It("topic show and topic set refuse a traversal name and touch nothing outside the topics directory", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Real Topic": "---\nstatus: in_progress\n---\n",
				},
				nil,
			)
			defer cleanup()

			outsidePath := filepath.Join(vaultPath, "outside-file.md")
			outsideContent := []byte("---\nstatus: in_progress\nowner: untouched\n---\n# Outside\n")
			Expect(os.WriteFile(outsidePath, outsideContent, 0600)).To(Succeed())

			showCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "show", "../outside-file",
			)
			showSession, err := gexec.Start(showCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(showSession).Should(gexec.Exit(1))

			setCmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "set", "../outside-file", "owner", "alice",
			)
			setSession, err := gexec.Start(setCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(setSession).Should(gexec.Exit(1))

			after, err := os.ReadFile(outsidePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(outsideContent))
		})

		It("topic work-on moves the page into its in-progress state and reports a session outcome", func() {
			vaultPath, configPath, cleanup := createTempVaultWithTopicPages(
				"Topics",
				map[string]string{
					"Round Trip": "---\nstatus: backlog\n---\n# Round Trip\n",
				},
				nil,
			)
			defer cleanup()

			cmd := exec.Command(
				binPath, "--config", configPath, "--vault", "test",
				"topic", "work-on", "Round Trip", "--mode", "headless",
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 30*time.Second).Should(gexec.Exit(0))

			out := string(session.Out.Contents())
			Expect(out).NotTo(ContainSubstring("Usage:"))
			Expect(out).To(ContainSubstring("Now working on:"))

			content, err := os.ReadFile(filepath.Join(vaultPath, "Topics", "Round Trip.md"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("status: in_progress"))
			Expect(string(content)).To(ContainSubstring("assignee: tester@example.com"))
		})
	})

})
