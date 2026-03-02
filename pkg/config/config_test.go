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
