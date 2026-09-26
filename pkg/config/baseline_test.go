// Copyright (c) 2026 Benjamin Borbe All rights reserved.
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

var _ = Describe("ValidateBaselinePath", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("accepts a vault-relative path", func() {
		for _, value := range []string{
			"baseline.md",
			"notes/baseline.md",
			"./notes/baseline.md",
			"notes/../baseline.md",
			"60 Baseline/2026-09-12.md",
		} {
			Expect(config.ValidateBaselinePath(ctx, value)).To(BeNil(), "path: %s", value)
		}
	})

	It("rejects an empty path", func() {
		Expect(config.ValidateBaselinePath(ctx, "")).NotTo(BeNil())
	})

	It("rejects an absolute path and names it", func() {
		err := config.ValidateBaselinePath(ctx, "/etc/passwd")
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("/etc/passwd"))
	})

	It("rejects a path escaping the vault root and names it", func() {
		for _, value := range []string{"../../outside.md", "a/../../b.md", ".."} {
			err := config.ValidateBaselinePath(ctx, value)
			Expect(err).NotTo(BeNil(), "path: %s", value)
			Expect(err.Error()).To(ContainSubstring(value))
		}
	})
})

var _ = Describe("BaselineWriter", func() {
	var (
		ctx        context.Context
		tempDir    string
		configPath string
		writer     config.BaselineWriter
	)

	readBytes := func(path string) []byte {
		b, err := os.ReadFile(path)
		Expect(err).To(BeNil())
		return b
	}

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		tempDir, err = os.MkdirTemp("", "vault-cli-baseline-test-*")
		Expect(err).To(BeNil())
		configPath = filepath.Join(tempDir, "config.yaml")
		writer = config.NewBaselineWriter(configPath)
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	Context("with a personal vault", func() {
		BeforeEach(func() {
			configData := `current_user: user@example.com
default_vault: personal
vaults:
  personal:
    name: personal
    path: /vault/personal
    tasks_dir: "25 Tasks"
`
			Expect(os.WriteFile(configPath, []byte(configData), 0600)).To(Succeed())
		})

		It("writes the baseline key onto the named vault", func() {
			Expect(
				writer.SetBaseline(ctx, "personal", "60 Baseline/2026-09-12.md"),
			).To(Succeed())

			vault, err := config.NewLoader(configPath).GetVault(ctx, "personal")
			Expect(err).To(BeNil())
			Expect(vault.Baseline).To(Equal("60 Baseline/2026-09-12.md"))
		})

		It("preserves the vault's other keys", func() {
			Expect(writer.SetBaseline(ctx, "personal", "b.md")).To(Succeed())

			loader := config.NewLoader(configPath)
			vault, err := loader.GetVault(ctx, "personal")
			Expect(err).To(BeNil())
			Expect(vault.TasksDir).To(Equal("25 Tasks"))
			Expect(vault.Path).To(Equal("/vault/personal"))

			cfg, err := loader.Load(ctx)
			Expect(err).To(BeNil())
			Expect(cfg.CurrentUser).To(Equal("user@example.com"))
			Expect(cfg.DefaultVault).To(Equal("personal"))
		})

		It("writes the path verbatim without normalising it", func() {
			Expect(writer.SetBaseline(ctx, "personal", "./notes/baseline.md")).To(Succeed())

			vault, err := config.NewLoader(configPath).GetVault(ctx, "personal")
			Expect(err).To(BeNil())
			Expect(vault.Baseline).To(Equal("./notes/baseline.md"))
		})

		It("rejects an absolute path and leaves the file unchanged", func() {
			before := readBytes(configPath)

			err := writer.SetBaseline(ctx, "personal", "/etc/passwd")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("/etc/passwd"))
			Expect(readBytes(configPath)).To(Equal(before))
		})

		It("rejects a path escaping the vault root and leaves the file unchanged", func() {
			before := readBytes(configPath)

			err := writer.SetBaseline(ctx, "personal", "../../outside.md")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("../../outside.md"))
			Expect(readBytes(configPath)).To(Equal(before))
		})

		It("leaves no temp file behind on a successful write", func() {
			Expect(writer.SetBaseline(ctx, "personal", "b.md")).To(Succeed())

			entries, err := os.ReadDir(tempDir)
			Expect(err).To(BeNil())
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].Name()).To(Equal("config.yaml"))
		})
	})

	Context("with a mixed-case vault key", func() {
		BeforeEach(func() {
			configData := `vaults:
  Personal:
    name: Personal
    path: /vault/personal
`
			Expect(os.WriteFile(configPath, []byte(configData), 0600)).To(Succeed())
		})

		It("matches the vault name case-insensitively and preserves the file's key spelling", func() {
			Expect(writer.SetBaseline(ctx, "personal", "b.md")).To(Succeed())

			content := string(readBytes(configPath))
			Expect(content).To(ContainSubstring("Personal:"))
			Expect(content).NotTo(ContainSubstring("personal:"))
		})
	})

	Context("with a vault absent from the config", func() {
		BeforeEach(func() {
			configData := `vaults:
  personal:
    name: personal
    path: /vault/personal
`
			Expect(os.WriteFile(configPath, []byte(configData), 0600)).To(Succeed())
		})

		It("rejects a vault absent from the config and leaves the file unchanged", func() {
			before := readBytes(configPath)

			err := writer.SetBaseline(ctx, "nope", "b.md")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("nope"))
			Expect(readBytes(configPath)).To(Equal(before))
		})
	})

	Context("with a malformed config", func() {
		BeforeEach(func() {
			Expect(os.WriteFile(configPath, []byte("vaults: [\n"), 0600)).To(Succeed())
		})

		It("rejects a malformed config and leaves the file unchanged", func() {
			before := readBytes(configPath)

			err := writer.SetBaseline(ctx, "personal", "b.md")
			Expect(err).NotTo(BeNil())
			Expect(readBytes(configPath)).To(Equal(before))
		})
	})
})
