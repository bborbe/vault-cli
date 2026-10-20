// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/config"
)

var _ = Describe("ValidateA2ADefaultCluster", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("accepts RFC 1123 label shapes", func() {
		for _, value := range []string{
			"nukedev",
			"nukeprod",
			"octopus-dev",
			"my-cluster-2",
			"a",
			"0",
			"cluster123",
			strings.Repeat("a", 63),
		} {
			Expect(config.ValidateA2ADefaultCluster(ctx, value)).To(BeNil(), "cluster: %s", value)
		}
	})

	It("rejects a target string rather than a bare name", func() {
		err := config.ValidateA2ADefaultCluster(ctx, "k8s:nukedev")
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("k8s:nukedev"))
	})

	It("rejects every other malformed shape", func() {
		for _, value := range []string{
			"Nukedev",
			"nuke_dev",
			"-nukedev",
			"nukedev-",
			"nuke dev",
			"nuke.dev",
			"nuke/dev",
			"nük edev",
			strings.Repeat("a", 64),
		} {
			Expect(config.ValidateA2ADefaultCluster(ctx, value)).NotTo(BeNil(), "cluster: %s", value)
		}
	})

	It("rejects an empty cluster name", func() {
		Expect(config.ValidateA2ADefaultCluster(ctx, "")).NotTo(BeNil())
	})
})

var _ = Describe("A2ADefaultClusterWriter", func() {
	var (
		ctx        context.Context
		tempDir    string
		configPath string
		writer     config.A2ADefaultClusterWriter
	)

	readBytes := func(path string) []byte {
		b, err := os.ReadFile(path)
		Expect(err).To(BeNil())
		return b
	}

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		tempDir, err = os.MkdirTemp("", "vault-cli-a2a-default-cluster-test-*")
		Expect(err).To(BeNil())
		configPath = filepath.Join(tempDir, "config.yaml")
		writer = config.NewA2ADefaultClusterWriter(configPath)
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

		It("writes the cluster name onto the named vault", func() {
			Expect(writer.SetA2ADefaultCluster(ctx, "personal", "nukedev")).To(Succeed())

			vault, err := config.NewLoader(configPath).GetVault(ctx, "personal")
			Expect(err).To(BeNil())
			Expect(vault.A2ADefaultCluster).To(Equal("nukedev"))
		})

		It("writes the literal key the on-disk contract names", func() {
			Expect(writer.SetA2ADefaultCluster(ctx, "personal", "nukedev")).To(Succeed())

			Expect(string(readBytes(configPath))).
				To(ContainSubstring("a2a_default_cluster: nukedev"))
		})

		It("preserves the vault's other keys", func() {
			Expect(writer.SetA2ADefaultCluster(ctx, "personal", "nukedev")).To(Succeed())

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

		It("rejects a malformed cluster name and leaves the file byte-identical", func() {
			before := readBytes(configPath)

			err := writer.SetA2ADefaultCluster(ctx, "personal", "k8s:nukedev")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("k8s:nukedev"))
			Expect(readBytes(configPath)).To(Equal(before))
		})

		It("leaves no temp file behind on a successful write", func() {
			Expect(writer.SetA2ADefaultCluster(ctx, "personal", "nukedev")).To(Succeed())

			entries, err := os.ReadDir(tempDir)
			Expect(err).To(BeNil())
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].Name()).To(Equal("config.yaml"))
		})
	})

	Context("with two vaults, only one of which is set", func() {
		BeforeEach(func() {
			configData := `current_user: user@example.com
vaults:
  test:
    name: test
    path: /vault/test
    tasks_dir: Tasks
  other:
    name: other
    path: /vault/other
    tasks_dir: "25 Tasks"
`
			Expect(os.WriteFile(configPath, []byte(configData), 0600)).To(Succeed())
		})

		It("writes the key only where it was set", func() {
			Expect(writer.SetA2ADefaultCluster(ctx, "test", "nukedev")).To(Succeed())

			content := string(readBytes(configPath))
			Expect(strings.Count(content, "a2a_default_cluster")).To(Equal(1))

			loader := config.NewLoader(configPath)
			other, err := loader.GetVault(ctx, "other")
			Expect(err).To(BeNil())
			Expect(other.A2ADefaultCluster).To(BeEmpty())
			Expect(other.Path).To(Equal("/vault/other"))
			Expect(other.TasksDir).To(Equal("25 Tasks"))
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

		It("matches case-insensitively and preserves the file's key spelling", func() {
			Expect(writer.SetA2ADefaultCluster(ctx, "PERSONAL", "nukedev")).To(Succeed())

			content := string(readBytes(configPath))
			Expect(content).To(ContainSubstring("Personal:"))
			Expect(content).NotTo(ContainSubstring("personal:"))
			Expect(content).To(ContainSubstring("a2a_default_cluster: nukedev"))
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

		It("rejects the vault and leaves the file byte-identical", func() {
			before := readBytes(configPath)

			err := writer.SetA2ADefaultCluster(ctx, "nosuchvault", "nukedev")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("nosuchvault"))
			Expect(readBytes(configPath)).To(Equal(before))
		})
	})

	Context("with a malformed config", func() {
		BeforeEach(func() {
			Expect(os.WriteFile(configPath, []byte("vaults: [\n"), 0600)).To(Succeed())
		})

		It("rejects the config and leaves the file byte-identical", func() {
			before := readBytes(configPath)

			err := writer.SetA2ADefaultCluster(ctx, "personal", "nukedev")
			Expect(err).NotTo(BeNil())
			Expect(readBytes(configPath)).To(Equal(before))
		})
	})
})
