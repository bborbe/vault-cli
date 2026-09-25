// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// vaultDir is the vault root handed to every storage call as vaultPath.
// topicsDir is the default topics directory, matching DefaultConfig()'s
// TopicsDir literal, which storage.NewStorage(nil) resolves through.
var (
	vaultDir  string
	topicsDir string
)

var _ = Describe("TopicStorage", func() {
	var (
		ctx   context.Context
		store storage.Storage
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = storage.NewStorage(nil)

		var err error
		vaultDir, err = os.MkdirTemp("", "vault-topic-test-*")
		Expect(err).To(BeNil())

		topicsDir = filepath.Join(vaultDir, "23 Topics")
		Expect(os.MkdirAll(topicsDir, 0750)).To(Succeed())
	})

	AfterEach(func() {
		if vaultDir != "" {
			_ = os.RemoveAll(vaultDir)
		}
	})

	topicContent := func() string {
		return `---
status: in_progress
phase: planning
tags:
  - attention
unknown_key: kept
---
# Attention Routing

Body text.
`
	}

	Describe("FindTopicByName", func() {
		BeforeEach(func() {
			topicPath := filepath.Join(topicsDir, "Attention Routing.md")
			Expect(os.WriteFile(topicPath, []byte(topicContent()), 0600)).To(Succeed())
		})

		It("finds a topic by bare name", func() {
			topic, err := store.FindTopicByName(ctx, vaultDir, "Attention Routing")
			Expect(err).To(BeNil())
			Expect(topic).NotTo(BeNil())
			Expect(topic.Name).To(Equal("Attention Routing"))
			Expect(topic.GetField("phase")).To(Equal("planning"))
		})

		It("finds a topic by bracket-wrapped name", func() {
			topic, err := store.FindTopicByName(ctx, vaultDir, "[[Attention Routing]]")
			Expect(err).To(BeNil())
			Expect(topic).NotTo(BeNil())
			Expect(topic.Name).To(Equal("Attention Routing"))
		})

		It("returns an error for a nonexistent name", func() {
			_, err := store.FindTopicByName(ctx, vaultDir, "Nonexistent")
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())
		})

		It("returns an error whose message names the topic and the searched directory", func() {
			_, err := store.FindTopicByName(ctx, vaultDir, "Nonexistent")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Nonexistent"))
			Expect(err.Error()).To(ContainSubstring(filepath.Join(vaultDir, "23 Topics")))
		})

		It("returns an error when the page carries no frontmatter", func() {
			Expect(os.WriteFile(
				filepath.Join(topicsDir, "NoFrontmatter.md"),
				[]byte("# No frontmatter here\n"),
				0600,
			)).To(Succeed())

			_, err := store.FindTopicByName(ctx, vaultDir, "NoFrontmatter")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("parse frontmatter"))
		})
	})

	Describe("NewTopicStorage", func() {
		It("reads and writes through the narrow topic interface", func() {
			narrow := storage.NewTopicStorage(nil)
			Expect(narrow).NotTo(BeNil())

			topic := domain.NewTopic(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{
					Name:     "Narrow",
					FilePath: filepath.Join(topicsDir, "Narrow.md"),
				},
				domain.Content("---\nstatus: in_progress\n---\n"),
			)
			Expect(narrow.WriteTopic(ctx, topic)).To(Succeed())

			found, err := narrow.FindTopicByName(ctx, vaultDir, "Narrow")
			Expect(err).To(BeNil())
			Expect(found.Name).To(Equal("Narrow"))
		})

		It("uses the supplied config rather than the default", func() {
			narrow := storage.NewTopicStorage(&storage.Config{TopicsDir: "Custom Topics"})
			topic := domain.NewTopic(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{
					Name:     "Narrow Scoped",
					FilePath: filepath.Join(vaultDir, "Custom Topics", "Narrow Scoped.md"),
				},
				domain.Content("---\nstatus: in_progress\n---\n"),
			)
			Expect(narrow.WriteTopic(ctx, topic)).To(Succeed())

			found, err := narrow.FindTopicByName(ctx, vaultDir, "Narrow Scoped")
			Expect(err).To(BeNil())
			Expect(found.Name).To(Equal("Narrow Scoped"))
		})
	})

	Describe("FindTopicByName traversal refusal", func() {
		BeforeEach(func() {
			outsidePath := filepath.Join(vaultDir, "outside-file.md")
			Expect(
				os.WriteFile(outsidePath, []byte("---\nstatus: in_progress\n---\noutside\n"), 0600),
			).To(Succeed())
		})

		It("refuses a parent-directory name and leaves the outside file byte-identical", func() {
			before, err := os.ReadFile(filepath.Join(vaultDir, "outside-file.md"))
			Expect(err).To(BeNil())

			_, err = store.FindTopicByName(ctx, vaultDir, "../outside-file")
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())

			after, err := os.ReadFile(filepath.Join(vaultDir, "outside-file.md"))
			Expect(err).To(BeNil())
			Expect(after).To(Equal(before))
		})

		It("refuses a name that climbs out of the vault entirely", func() {
			_, err := store.FindTopicByName(ctx, vaultDir, "../../outside-file")
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())
		})

		It("still resolves a name nested inside the topics directory", func() {
			nestedPath := filepath.Join(topicsDir, "sub", "nested.md")
			Expect(os.MkdirAll(filepath.Dir(nestedPath), 0750)).To(Succeed())
			Expect(os.WriteFile(nestedPath, []byte(topicContent()), 0600)).To(Succeed())

			topic, err := store.FindTopicByName(ctx, vaultDir, "sub/nested")
			Expect(err).To(BeNil())
			Expect(topic.Name).To(Equal("sub/nested"))
			Expect(topic.FilePath).To(Equal(filepath.Join(topicsDir, "sub", "nested.md")))
		})
	})

	Describe("WriteTopic", func() {
		It("writes a topic and reads it back with every key preserved", func() {
			topic := domain.NewTopic(
				map[string]any{
					"status":      "in_progress",
					"phase":       "planning",
					"unknown_key": "kept",
				},
				domain.FileMetadata{
					Name:     "Attention Routing",
					FilePath: filepath.Join(topicsDir, "Attention Routing.md"),
				},
				domain.Content("---\nstatus: in_progress\n---\n# Attention Routing\n"),
			)
			Expect(store.WriteTopic(ctx, topic)).To(Succeed())

			reread, err := store.FindTopicByName(ctx, vaultDir, "Attention Routing")
			Expect(err).To(BeNil())
			Expect(reread.GetField("phase")).To(Equal("planning"))
			Expect(reread.GetField("unknown_key")).To(Equal("kept"))
			Expect(reread.GetField("status")).To(Equal("in_progress"))
		})

		It("creates a missing parent directory", func() {
			nestedDir := filepath.Join(vaultDir, "Custom Topics", "deeper")
			topic := domain.NewTopic(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{
					Name:     "Fresh",
					FilePath: filepath.Join(nestedDir, "Fresh.md"),
				},
				domain.Content("---\nstatus: in_progress\n---\n"),
			)
			Expect(store.WriteTopic(ctx, topic)).To(Succeed())

			_, err := os.Stat(filepath.Join(nestedDir, "Fresh.md"))
			Expect(err).To(BeNil())
		})

		It("writes the file with 0600 permissions", func() {
			topic := domain.NewTopic(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{
					Name:     "Mode",
					FilePath: filepath.Join(topicsDir, "Mode.md"),
				},
				domain.Content("---\nstatus: in_progress\n---\n"),
			)
			Expect(store.WriteTopic(ctx, topic)).To(Succeed())

			info, err := os.Stat(filepath.Join(topicsDir, "Mode.md"))
			Expect(err).To(BeNil())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
		})

		It("refuses to write through a symlink", func() {
			target := filepath.Join(topicsDir, "Target.md")
			Expect(os.WriteFile(target, []byte(topicContent()), 0600)).To(Succeed())

			link := filepath.Join(topicsDir, "Link.md")
			Expect(os.Symlink(target, link)).To(Succeed())

			topic := domain.NewTopic(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: "Link", FilePath: link},
				domain.Content("---\nstatus: in_progress\n---\n"),
			)
			err := store.WriteTopic(ctx, topic)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("refusing to write through symlink"))
		})

		It("returns an error when the parent directory cannot be created", func() {
			// A regular file where the parent directory should be makes MkdirAll
			// fail with ENOTDIR.
			blocker := filepath.Join(topicsDir, "blocker")
			Expect(os.WriteFile(blocker, []byte("not a directory\n"), 0600)).To(Succeed())

			topic := domain.NewTopic(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{
					Name:     "Blocked",
					FilePath: filepath.Join(blocker, "Blocked.md"),
				},
				domain.Content("---\nstatus: in_progress\n---\n"),
			)
			err := store.WriteTopic(ctx, topic)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("create directory"))
		})

		It("returns an error when the target path is a directory", func() {
			target := filepath.Join(topicsDir, "IsADir.md")
			Expect(os.MkdirAll(target, 0750)).To(Succeed())

			topic := domain.NewTopic(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{Name: "IsADir", FilePath: target},
				domain.Content("---\nstatus: in_progress\n---\n"),
			)
			err := store.WriteTopic(ctx, topic)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("write file"))
		})

		It("round-trips a bare wikilink value without destroying it", func() {
			wikilinkPath := filepath.Join(topicsDir, "Wikilink.md")
			Expect(os.WriteFile(wikilinkPath, []byte(
				"---\nrelated: [[A Topic]]\nstatus: in_progress\n---\n# Wikilink\n",
			), 0600)).To(Succeed())

			topic, err := store.FindTopicByName(ctx, vaultDir, "Wikilink")
			Expect(err).To(BeNil())
			Expect(topic.Get("related")).To(Equal("[[A Topic]]"))

			Expect(store.WriteTopic(ctx, topic)).To(Succeed())

			content, err := os.ReadFile(wikilinkPath)
			Expect(err).To(BeNil())
			Expect(string(content)).To(ContainSubstring("[[A Topic]]"))
			Expect(strings.Count(string(content), "[[A Topic]]")).To(Equal(1))
			Expect(string(content)).NotTo(ContainSubstring("- - A Topic"))
		})
	})

	Describe("topic storage configuration", func() {
		It("carries the vault's configured topics directory", func() {
			cfg := storage.NewConfigFromVault(&config.Vault{TopicsDir: "Custom Topics"})
			Expect(cfg.TopicsDir).To(Equal("Custom Topics"))
		})

		It("falls back to the 23 Topics default when the vault declares none", func() {
			cfg := storage.NewConfigFromVault(&config.Vault{})
			Expect(cfg.TopicsDir).To(Equal("23 Topics"))
		})

		It("carries 23 Topics in DefaultConfig", func() {
			Expect(storage.DefaultConfig().TopicsDir).To(Equal("23 Topics"))
		})

		It("writes under the configured directory and not under the default", func() {
			customStore := storage.NewStorage(&storage.Config{TopicsDir: "Custom Topics"})
			topic := domain.NewTopic(
				map[string]any{"status": "in_progress"},
				domain.FileMetadata{
					Name:     "Scoped",
					FilePath: filepath.Join(vaultDir, "Custom Topics", "Scoped.md"),
				},
				domain.Content("---\nstatus: in_progress\n---\n"),
			)
			Expect(customStore.WriteTopic(ctx, topic)).To(Succeed())

			_, err := os.Stat(filepath.Join(vaultDir, "Custom Topics", "Scoped.md"))
			Expect(err).To(BeNil())
			_, err = os.Stat(filepath.Join(vaultDir, "23 Topics", "Scoped.md"))
			Expect(err).NotTo(BeNil())
		})

		It("finds a page under the configured directory and not under the default", func() {
			customStore := storage.NewStorage(&storage.Config{TopicsDir: "Custom Topics"})
			scopedPath := filepath.Join(vaultDir, "Custom Topics", "Scoped.md")
			Expect(os.MkdirAll(filepath.Dir(scopedPath), 0750)).To(Succeed())
			Expect(os.WriteFile(scopedPath, []byte(topicContent()), 0600)).To(Succeed())

			topic, err := customStore.FindTopicByName(ctx, vaultDir, "Scoped")
			Expect(err).To(BeNil())
			Expect(topic.Name).To(Equal("Scoped"))

			_, err = store.FindTopicByName(ctx, vaultDir, "Scoped")
			Expect(err).NotTo(BeNil())
		})
	})

	Describe("absent directories", func() {
		It("returns an ErrNotFound-class error when the configured directory is absent", func() {
			customStore := storage.NewStorage(&storage.Config{TopicsDir: "Custom Topics"})
			_, err := customStore.FindTopicByName(ctx, vaultDir, "Anything")
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())

			_, statErr := os.Stat(filepath.Join(vaultDir, "Custom Topics"))
			Expect(os.IsNotExist(statErr)).To(BeTrue())
		})

		It("lists an absent configured directory as an empty result with no error", func() {
			pageStore := storage.NewPageStorage(&storage.Config{TopicsDir: "Custom Topics"})
			pages, err := pageStore.ListPages(ctx, vaultDir, "Custom Topics")
			Expect(err).To(BeNil())
			Expect(pages).To(BeEmpty())
		})
	})
})
