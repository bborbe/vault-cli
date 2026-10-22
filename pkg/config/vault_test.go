// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/config"
)

var _ = Describe("Vault", func() {
	Describe("GetTasksDir", func() {
		It("returns custom tasks dir when set", func() {
			vault := &config.Vault{TasksDir: "Custom Tasks"}
			Expect(vault.GetTasksDir()).To(Equal("Custom Tasks"))
		})

		It("returns default Tasks when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetTasksDir()).To(Equal("Tasks"))
		})
	})

	Describe("GetGoalsDir", func() {
		It("returns custom goals dir when set", func() {
			vault := &config.Vault{GoalsDir: "Custom Goals"}
			Expect(vault.GetGoalsDir()).To(Equal("Custom Goals"))
		})

		It("returns default Goals when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetGoalsDir()).To(Equal("Goals"))
		})
	})

	Describe("GetThemesDir", func() {
		It("returns custom themes dir when set", func() {
			vault := &config.Vault{ThemesDir: "Custom Themes"}
			Expect(vault.GetThemesDir()).To(Equal("Custom Themes"))
		})

		It("returns default 21 Themes when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetThemesDir()).To(Equal("21 Themes"))
		})
	})

	Describe("GetObjectivesDir", func() {
		It("returns custom objectives dir when set", func() {
			vault := &config.Vault{ObjectivesDir: "Custom Objectives"}
			Expect(vault.GetObjectivesDir()).To(Equal("Custom Objectives"))
		})

		It("returns default 22 Objectives when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetObjectivesDir()).To(Equal("22 Objectives"))
		})
	})

	Describe("GetVisionDir", func() {
		It("returns custom vision dir when set", func() {
			vault := &config.Vault{VisionDir: "Custom Vision"}
			Expect(vault.GetVisionDir()).To(Equal("Custom Vision"))
		})

		It("returns default 20 Vision when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetVisionDir()).To(Equal("20 Vision"))
		})
	})

	Describe("GetDailyDir", func() {
		It("returns custom daily dir when set", func() {
			vault := &config.Vault{DailyDir: "Custom Daily"}
			Expect(vault.GetDailyDir()).To(Equal("Custom Daily"))
		})

		It("returns default Daily Notes when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetDailyDir()).To(Equal("Daily Notes"))
		})
	})

	Describe("GetKnowledgeDir", func() {
		It("returns custom knowledge dir when set", func() {
			vault := &config.Vault{KnowledgeDir: "Custom Knowledge"}
			Expect(vault.GetKnowledgeDir()).To(Equal("Custom Knowledge"))
		})

		It("returns default 50 Knowledge Base when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetKnowledgeDir()).To(Equal("50 Knowledge Base"))
		})
	})

	Describe("GetTopicsDir", func() {
		It("returns custom topics dir when set", func() {
			vault := &config.Vault{TopicsDir: "Custom Topics"}
			Expect(vault.GetTopicsDir()).To(Equal("Custom Topics"))
		})

		It("returns default 23 Topics when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetTopicsDir()).To(Equal("23 Topics"))
		})
	})

	Describe("GetClaudeScript", func() {
		It("returns custom claude script when set", func() {
			vault := &config.Vault{ClaudeScript: "/usr/local/bin/my-claude"}
			Expect(vault.GetClaudeScript()).To(Equal("/usr/local/bin/my-claude"))
		})

		It("returns default claude when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetClaudeScript()).To(Equal("claude"))
		})
	})

	Describe("GetWorkerLauncher and GetManagerLauncher", func() {
		var dir string

		BeforeEach(func() {
			dir = GinkgoT().TempDir()
		})

		writeSiblings := func(names ...string) {
			for _, name := range names {
				Expect(os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0600)).
					To(Succeed())
			}
		}

		// want values are relative to the fixture directory unless absolute or empty.
		resolveWant := func(name string) string {
			if name == "" || filepath.IsAbs(name) {
				return name
			}
			return filepath.Join(dir, name)
		}

		DescribeTable(
			"resolves a role launcher",
			func(
				script string,
				siblings []string,
				workerExplicit, managerExplicit, wantWorker, wantManager string,
			) {
				writeSiblings(siblings...)
				vault := &config.Vault{
					WorkerLauncher:  workerExplicit,
					ManagerLauncher: managerExplicit,
				}
				if script != "" {
					vault.ClaudeScript = filepath.Join(dir, script)
				}

				Expect(vault.GetWorkerLauncher()).To(Equal(resolveWant(wantWorker)))
				Expect(vault.GetManagerLauncher()).To(Equal(resolveWant(wantManager)))
			},
			Entry(
				"derives both siblings when neither key is set",
				"cc-private",
				[]string{"cc-private-deepseek", "cc-private-claude"},
				"", "", "cc-private-deepseek", "cc-private-claude",
			),
			Entry(
				"returns an explicit bare worker value beside the script directory",
				"cc-private",
				[]string{"cc-private-deepseek", "cc-private-claude"},
				"cc-private-deepseek", "", "cc-private-deepseek", "cc-private-claude",
			),
			Entry(
				"returns an explicit manager value as given, even when it does not exist",
				"cc-private",
				[]string{"cc-private-deepseek", "cc-private-claude"},
				"", "/opt/manager-claude", "cc-private-deepseek", "/opt/manager-claude",
			),
			Entry(
				"derives the worker sibling across an already-suffixed -claude script",
				"cc-private-claude",
				[]string{"cc-private-deepseek", "cc-private-claude"},
				"", "", "cc-private-deepseek", "cc-private-claude",
			),
			Entry(
				"derives the manager sibling across an already-suffixed -deepseek script",
				"cc-private-deepseek",
				[]string{"cc-private-deepseek", "cc-private-claude"},
				"", "", "cc-private-deepseek", "cc-private-claude",
			),
			Entry(
				"returns empty when the derived candidate does not exist",
				"cc-private",
				nil,
				"", "", "", "",
			),
			Entry(
				"returns empty for a non-conforming script rather than inventing a name",
				"claude.sh",
				nil,
				"", "", "", "",
			),
			Entry(
				"returns empty when claude_script is empty",
				"",
				nil,
				"", "", "", "",
			),
		)
	})

	Describe("GetSessionProjectDir", func() {
		It("returns custom session project dir when set", func() {
			vault := &config.Vault{SessionProjectDir: "/custom/project/dir"}
			Expect(vault.GetSessionProjectDir()).To(Equal("/custom/project/dir"))
		})

		It("returns empty string when not set", func() {
			vault := &config.Vault{}
			Expect(vault.GetSessionProjectDir()).To(Equal(""))
		})
	})

	Describe("GetWorkOnGoalCommand", func() {
		It("returns custom work on goal command when set", func() {
			vault := &config.Vault{WorkOnGoalCommand: "/my-goal:work-on"}
			Expect(vault.GetWorkOnGoalCommand()).To(Equal("/my-goal:work-on"))
		})

		It("returns default /vault-cli:work-on-goal when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetWorkOnGoalCommand()).To(Equal("/vault-cli:work-on-goal"))
		})
	})

	Describe("GetWorkOnCommand", func() {
		It("returns custom work on command when set", func() {
			vault := &config.Vault{WorkOnCommand: "/my-custom:work-on"}
			Expect(vault.GetWorkOnCommand()).To(Equal("/my-custom:work-on"))
		})

		It("returns default /vault-cli:work-on-task when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetWorkOnCommand()).To(Equal("/vault-cli:work-on-task"))
		})
	})

	Describe("JSON marshalling", func() {
		It("includes knowledge_dir in JSON when set", func() {
			vault := config.Vault{Name: "main", Path: "/vault", KnowledgeDir: "50 Knowledge"}
			data, err := json.Marshal(vault)
			Expect(err).To(BeNil())
			Expect(string(data)).To(ContainSubstring(`"knowledge_dir":"50 Knowledge"`))
		})

		It("omits knowledge_dir from JSON when empty", func() {
			vault := config.Vault{Name: "main", Path: "/vault"}
			data, err := json.Marshal(vault)
			Expect(err).To(BeNil())
			Expect(string(data)).NotTo(ContainSubstring("knowledge_dir"))
		})

		It("includes work_on_command in JSON when set", func() {
			vault := config.Vault{Name: "main", Path: "/vault", WorkOnCommand: "/cmd"}
			data, err := json.Marshal(vault)
			Expect(err).To(BeNil())
			Expect(string(data)).To(ContainSubstring(`"work_on_command":"/cmd"`))
		})

		It("omits work_on_command from JSON when empty", func() {
			vault := config.Vault{Name: "main", Path: "/vault"}
			data, err := json.Marshal(vault)
			Expect(err).To(BeNil())
			Expect(string(data)).NotTo(ContainSubstring("work_on_command"))
		})

		It("includes work_on_goal_command in JSON when set", func() {
			vault := config.Vault{Name: "main", Path: "/vault", WorkOnGoalCommand: "/cmd"}
			data, err := json.Marshal(vault)
			Expect(err).To(BeNil())
			Expect(string(data)).To(ContainSubstring(`"work_on_goal_command":"/cmd"`))
		})

		It("omits work_on_goal_command from JSON when empty", func() {
			vault := config.Vault{Name: "main", Path: "/vault"}
			data, err := json.Marshal(vault)
			Expect(err).To(BeNil())
			Expect(string(data)).NotTo(ContainSubstring("work_on_goal_command"))
		})
	})

	Describe("YAML marshalling", func() {
		It("includes work_on_goal_command in YAML when set", func() {
			vault := config.Vault{Name: "main", Path: "/vault", WorkOnGoalCommand: "/cmd"}
			data, err := yaml.Marshal(vault)
			Expect(err).To(BeNil())
			Expect(string(data)).To(ContainSubstring("work_on_goal_command: /cmd"))
		})

		It("omits work_on_goal_command from YAML when empty", func() {
			vault := config.Vault{Name: "main", Path: "/vault"}
			data, err := yaml.Marshal(vault)
			Expect(err).To(BeNil())
			Expect(string(data)).NotTo(ContainSubstring("work_on_goal_command"))
		})
	})
})
