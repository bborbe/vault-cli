// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/config"
)

var _ = Describe("Loader", func() {
	var (
		ctx        context.Context
		loader     config.Loader
		tempDir    string
		configPath string
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		tempDir, err = os.MkdirTemp("", "vault-cli-config-test-*")
		Expect(err).To(BeNil())
		configPath = filepath.Join(tempDir, "config.yaml")
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	Describe("Load", func() {
		Context("valid config file", func() {
			BeforeEach(func() {
				configData := `current_user: user@example.com
default_vault: main
vaults:
  main:
    name: main
    path: ~/Documents/vault
  work:
    name: work
    path: /work/vault
    tasks_dir: Custom Tasks
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("loads config successfully", func() {
				cfg, err := loader.Load(ctx)
				Expect(err).To(BeNil())
				Expect(cfg).NotTo(BeNil())
				Expect(cfg.CurrentUser).To(Equal("user@example.com"))
				Expect(cfg.DefaultVault).To(Equal("main"))
				Expect(cfg.Vaults).To(HaveLen(2))
			})

			It("parses vault configurations correctly", func() {
				cfg, err := loader.Load(ctx)
				Expect(err).To(BeNil())
				Expect(cfg.Vaults["main"].Name).To(Equal("main"))
				Expect(cfg.Vaults["main"].Path).To(Equal("~/Documents/vault"))
				Expect(cfg.Vaults["work"].Name).To(Equal("work"))
				Expect(cfg.Vaults["work"].Path).To(Equal("/work/vault"))
				Expect(cfg.Vaults["work"].TasksDir).To(Equal("Custom Tasks"))
			})
		})

		Context("missing config file", func() {
			BeforeEach(func() {
				loader = config.NewLoader(configPath) // File doesn't exist
			})

			It("returns default config", func() {
				cfg, err := loader.Load(ctx)
				Expect(err).To(BeNil())
				Expect(cfg).NotTo(BeNil())
			})

			It("has default vault named main", func() {
				cfg, err := loader.Load(ctx)
				Expect(err).To(BeNil())
				Expect(cfg.DefaultVault).To(Equal("main"))
				Expect(cfg.Vaults).To(HaveKey("main"))
			})
		})

		Context("mixed-case keys", func() {
			BeforeEach(func() {
				configData := `default_vault: Personal
vaults:
  Personal:
    name: Personal
    path: /path/personal
  WORK:
    name: WORK
    path: /path/work
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("normalizes vault map keys to lowercase", func() {
				cfg, err := loader.Load(ctx)
				Expect(err).To(BeNil())
				Expect(cfg.Vaults).To(HaveKey("personal"))
				Expect(cfg.Vaults).To(HaveKey("work"))
				Expect(cfg.Vaults).NotTo(HaveKey("Personal"))
				Expect(cfg.Vaults).NotTo(HaveKey("WORK"))
			})

			It("normalizes Vault.Name to lowercase", func() {
				cfg, err := loader.Load(ctx)
				Expect(err).To(BeNil())
				Expect(cfg.Vaults["personal"].Name).To(Equal("personal"))
				Expect(cfg.Vaults["work"].Name).To(Equal("work"))
			})

			It("normalizes DefaultVault to lowercase", func() {
				cfg, err := loader.Load(ctx)
				Expect(err).To(BeNil())
				Expect(cfg.DefaultVault).To(Equal("personal"))
			})
		})

		Context("malformed YAML", func() {
			BeforeEach(func() {
				configData := `current_user: user@example.com
vaults:
  main: [invalid: yaml: structure
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("returns error", func() {
				_, err := loader.Load(ctx)
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Describe("GetVault", func() {
		Context("existing vault", func() {
			BeforeEach(func() {
				configData := `default_vault: main
vaults:
  main:
    name: main
    path: /path/to/vault
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("returns vault configuration", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault).NotTo(BeNil())
				Expect(vault.Name).To(Equal("main"))
				Expect(vault.Path).To(Equal("/path/to/vault"))
			})
		})

		Context("vault with tilde path", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: ~/Documents/vault
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("expands tilde to home directory", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault.Path).NotTo(ContainSubstring("~"))
			})
		})

		Context("unknown vault", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: /path/to/vault
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("returns error", func() {
				_, err := loader.GetVault(ctx, "nonexistent")
				Expect(err).NotTo(BeNil())
			})
		})

		Context("case-insensitive lookup", func() {
			BeforeEach(func() {
				configData := `vaults:
  personal:
    name: personal
    path: /path/personal
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("resolves mixed-case vault name", func() {
				vault, err := loader.GetVault(ctx, "Personal")
				Expect(err).To(BeNil())
				Expect(vault.Name).To(Equal("personal"))
				Expect(vault.Path).To(Equal("/path/personal"))
			})

			It("resolves upper-case vault name", func() {
				vault, err := loader.GetVault(ctx, "PERSONAL")
				Expect(err).To(BeNil())
				Expect(vault.Name).To(Equal("personal"))
			})

			It("still returns error for unknown vault regardless of case", func() {
				_, err := loader.GetVault(ctx, "Nonexistent")
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("vault not found"))
			})
		})

		Context("mixed-case default vault", func() {
			BeforeEach(func() {
				configData := `default_vault: Personal
vaults:
  personal:
    name: personal
    path: /path/personal
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("resolves default vault when called with empty name", func() {
				vault, err := loader.GetVault(ctx, "")
				Expect(err).To(BeNil())
				Expect(vault.Name).To(Equal("personal"))
			})
		})

		Context("empty vault name uses default", func() {
			BeforeEach(func() {
				configData := `default_vault: work
vaults:
  work:
    name: work
    path: /work/vault
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("uses default vault", func() {
				vault, err := loader.GetVault(ctx, "")
				Expect(err).To(BeNil())
				Expect(vault.Name).To(Equal("work"))
			})
		})
	})

	Describe("GetAllVaults", func() {
		Context("multiple vaults", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: ~/Documents/vault
  work:
    name: work
    path: /work/vault
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("returns all vaults", func() {
				vaults, err := loader.GetAllVaults(ctx)
				Expect(err).To(BeNil())
				Expect(vaults).To(HaveLen(2))
			})

			It("expands tilde paths", func() {
				vaults, err := loader.GetAllVaults(ctx)
				Expect(err).To(BeNil())
				for _, v := range vaults {
					Expect(v.Path).NotTo(ContainSubstring("~"))
				}
			})
		})
	})

	Describe("GetVaultPath", func() {
		BeforeEach(func() {
			configData := `default_vault: main
vaults:
  main:
    name: main
    path: /vault/main
  work:
    name: work
    path: /vault/work
`
			err := os.WriteFile(configPath, []byte(configData), 0600)
			Expect(err).To(BeNil())
			loader = config.NewLoader(configPath)
		})

		It("returns path for specified vault", func() {
			path, err := loader.GetVaultPath(ctx, "work")
			Expect(err).To(BeNil())
			Expect(path).To(Equal("/vault/work"))
		})

		It("returns path for default vault when name is empty", func() {
			path, err := loader.GetVaultPath(ctx, "")
			Expect(err).To(BeNil())
			Expect(path).To(Equal("/vault/main"))
		})

		It("returns error for unknown vault", func() {
			_, err := loader.GetVaultPath(ctx, "nonexistent")
			Expect(err).NotTo(BeNil())
		})
	})

	Describe("Vault template accessors", func() {
		Context("when no template fields are set", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: /vault/main
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("GetTaskTemplate returns empty string", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault.GetTaskTemplate()).To(Equal(""))
			})

			It("GetGoalTemplate returns empty string", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault.GetGoalTemplate()).To(Equal(""))
			})

			It("GetThemeTemplate returns empty string", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault.GetThemeTemplate()).To(Equal(""))
			})

			It("GetObjectiveTemplate returns empty string", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault.GetObjectiveTemplate()).To(Equal(""))
			})

			It("GetVisionTemplate returns empty string", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault.GetVisionTemplate()).To(Equal(""))
			})
		})

		Context("when task_template is an absolute path", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: /vault/main
    task_template: /absolute/path/task.md
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("returns the absolute path unchanged", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault.GetTaskTemplate()).To(Equal("/absolute/path/task.md"))
			})
		})

		Context("when task_template is a relative path", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: /vault/main
    task_template: 90 Templates/Task Template.md
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("resolves relative path against vault root", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(
					vault.GetTaskTemplate(),
				).To(Equal("/vault/main/90 Templates/Task Template.md"))
			})
		})

		Context("when task_template is a tilde-prefixed path", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: /vault/main
    task_template: ~/Templates/task.md
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("expands tilde to home directory", func() {
				homeDir, err := os.UserHomeDir()
				Expect(err).To(BeNil())
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault.GetTaskTemplate()).NotTo(ContainSubstring("~"))
				Expect(vault.GetTaskTemplate()).To(HavePrefix(homeDir))
			})
		})

		Context("when all five template fields are set to absolute paths", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: /vault/main
    task_template: /tmpl/task.md
    goal_template: /tmpl/goal.md
    theme_template: /tmpl/theme.md
    objective_template: /tmpl/objective.md
    vision_template: /tmpl/vision.md
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("returns all five template paths correctly", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault.GetTaskTemplate()).To(Equal("/tmpl/task.md"))
				Expect(vault.GetGoalTemplate()).To(Equal("/tmpl/goal.md"))
				Expect(vault.GetThemeTemplate()).To(Equal("/tmpl/theme.md"))
				Expect(vault.GetObjectiveTemplate()).To(Equal("/tmpl/objective.md"))
				Expect(vault.GetVisionTemplate()).To(Equal("/tmpl/vision.md"))
			})
		})

		Context("round-trip YAML serialization", func() {
			It("includes task_template in YAML when set", func() {
				v := config.Vault{
					Name:         "main",
					Path:         "/vault/main",
					TaskTemplate: "/tmpl/task.md",
				}
				data, err := yaml.Marshal(v)
				Expect(err).To(BeNil())
				Expect(string(data)).To(ContainSubstring("task_template"))
			})

			It("omits task_template from YAML when not set", func() {
				v := config.Vault{
					Name: "main",
					Path: "/vault/main",
				}
				data, err := yaml.Marshal(v)
				Expect(err).To(BeNil())
				Expect(string(data)).NotTo(ContainSubstring("task_template"))
			})
		})

		Context("GetAllVaults resolves template paths", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: /vault/main
    task_template: 90 Templates/Task Template.md
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("resolves relative task_template against vault root", func() {
				vaults, err := loader.GetAllVaults(ctx)
				Expect(err).To(BeNil())
				Expect(vaults).To(HaveLen(1))
				Expect(
					vaults[0].GetTaskTemplate(),
				).To(Equal("/vault/main/90 Templates/Task Template.md"))
			})
		})
	})

	Describe("GetCurrentUser", func() {
		Context("configured user", func() {
			BeforeEach(func() {
				configData := `current_user: user@example.com
vaults:
  main:
    name: main
    path: /vault
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("returns current user", func() {
				user, err := loader.GetCurrentUser(ctx)
				Expect(err).To(BeNil())
				Expect(user).To(Equal("user@example.com"))
			})
		})

		Context("missing current_user", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: /vault
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("returns error", func() {
				_, err := loader.GetCurrentUser(ctx)
				Expect(err).NotTo(BeNil())
			})
		})
	})
})
