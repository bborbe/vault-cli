// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
})
