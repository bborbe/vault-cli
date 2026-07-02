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

var _ = Describe("FindConfigDir", func() {
	var (
		ctx      context.Context
		tempHome string
		origHome string
	)

	BeforeEach(func() {
		ctx = context.Background()
		origHome = os.Getenv("HOME")
		var err error
		tempHome, err = os.MkdirTemp("", "vault-cli-findconfig-*")
		Expect(err).To(BeNil())
		Expect(os.Setenv("HOME", tempHome)).To(BeNil())
	})

	AfterEach(func() {
		Expect(os.Setenv("HOME", origHome)).To(BeNil())
		_ = os.RemoveAll(tempHome)
	})

	Describe("XDG-first lookup", func() {
		It("returns XDG dir when it exists", func() {
			xdgDir := filepath.Join(tempHome, ".config", "vault-cli")
			err := os.MkdirAll(xdgDir, 0700)
			Expect(err).To(BeNil())

			dir, err := config.FindConfigDir(ctx, "vault-cli")
			Expect(err).To(BeNil())
			Expect(dir).To(Equal(xdgDir))
		})

		It("returns legacy dir when only legacy exists", func() {
			legacyDir := filepath.Join(tempHome, ".vault-cli")
			err := os.MkdirAll(legacyDir, 0700)
			Expect(err).To(BeNil())

			dir, err := config.FindConfigDir(ctx, "vault-cli")
			Expect(err).To(BeNil())
			Expect(dir).To(Equal(legacyDir))
		})

		It("returns XDG default when neither dir exists", func() {
			xdgDir := filepath.Join(tempHome, ".config", "vault-cli")

			dir, err := config.FindConfigDir(ctx, "vault-cli")
			Expect(err).To(BeNil())
			Expect(dir).To(Equal(xdgDir))
		})

		It("prefers XDG over legacy when both exist", func() {
			xdgDir := filepath.Join(tempHome, ".config", "vault-cli")
			err := os.MkdirAll(xdgDir, 0700)
			Expect(err).To(BeNil())

			legacyDir := filepath.Join(tempHome, ".vault-cli")
			err = os.MkdirAll(legacyDir, 0700)
			Expect(err).To(BeNil())

			dir, err := config.FindConfigDir(ctx, "vault-cli")
			Expect(err).To(BeNil())
			Expect(dir).To(Equal(xdgDir))
		})

		It("falls through to legacy when XDG path is a file", func() {
			xdgDir := filepath.Join(tempHome, ".config")
			err := os.MkdirAll(xdgDir, 0700)
			Expect(err).To(BeNil())

			xdgFile := filepath.Join(tempHome, ".config", "vault-cli")
			err = os.WriteFile(xdgFile, []byte("x"), 0600)
			Expect(err).To(BeNil())

			legacyDir := filepath.Join(tempHome, ".vault-cli")
			err = os.MkdirAll(legacyDir, 0700)
			Expect(err).To(BeNil())

			dir, err := config.FindConfigDir(ctx, "vault-cli")
			Expect(err).To(BeNil())
			Expect(dir).To(Equal(legacyDir))
		})
	})

	Describe("Load via FindConfigDir", func() {
		It("reads config from XDG directory when path is empty", func() {
			xdgDir := filepath.Join(tempHome, ".config", "vault-cli")
			err := os.MkdirAll(xdgDir, 0700)
			Expect(err).To(BeNil())

			configData := `current_user: xdg@example.com
default_vault: main
vaults:
  main:
    name: main
    path: /vault/main
`
			configPath := filepath.Join(xdgDir, "config.yaml")
			err = os.WriteFile(configPath, []byte(configData), 0600)
			Expect(err).To(BeNil())

			loader := config.NewLoader("")
			cfg, err := loader.Load(ctx)
			Expect(err).To(BeNil())
			Expect(cfg.CurrentUser).To(Equal("xdg@example.com"))
			Expect(cfg.Vaults).To(HaveKey("main"))
		})
	})
})
