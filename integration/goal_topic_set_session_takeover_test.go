// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("goal/topic set session take-over", func() {
	const (
		sessionA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		sessionB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)

	var vaultPath, configPath string
	var cleanup func()

	// createVault writes one goal page and one topic page into a vault whose
	// config points at both directories.
	createVault := func(goalContent, topicContent string) {
		var err error
		vaultPath, err = os.MkdirTemp("", "vault-*")
		Expect(err).NotTo(HaveOccurred())

		goalsDir := filepath.Join(vaultPath, "Goals")
		topicsDir := filepath.Join(vaultPath, "Topics")
		Expect(os.MkdirAll(goalsDir, 0755)).To(Succeed())
		Expect(os.MkdirAll(topicsDir, 0755)).To(Succeed())
		Expect(
			os.WriteFile(filepath.Join(goalsDir, "Alpha.md"), []byte(goalContent), 0600),
		).To(Succeed())
		Expect(
			os.WriteFile(filepath.Join(topicsDir, "Alpha.md"), []byte(topicContent), 0600),
		).To(Succeed())

		configContent := fmt.Sprintf(`default_vault: test
vaults:
  test:
    name: test
    path: %s
    tasks_dir: Tasks
    goals_dir: Goals
    topics_dir: Topics
`, vaultPath)
		configFile, err := os.CreateTemp("", "vault-config-*.yaml")
		Expect(err).NotTo(HaveOccurred())
		_, err = configFile.WriteString(configContent)
		Expect(err).NotTo(HaveOccurred())
		Expect(configFile.Close()).To(Succeed())

		configPath = configFile.Name()
		cleanup = func() {
			_ = os.RemoveAll(vaultPath)
			_ = os.Remove(configFile.Name())
		}
	}

	BeforeEach(func() {
		// Reset per spec: a declaration-time value would be evaluated once for
		// the whole suite and leave the previous spec's closure in place.
		cleanup = func() {}
	})

	AfterEach(func() {
		cleanup()
	})

	runCommand := func(args ...string) *gexec.Session {
		fullArgs := append([]string{"--config", configPath, "--vault", "test"}, args...)
		session, err := gexec.Start(
			exec.Command(binPath, fullArgs...),
			GinkgoWriter,
			GinkgoWriter,
		)
		Expect(err).NotTo(HaveOccurred())
		return session
	}

	setSession := func(command, value string) {
		session := runCommand(command, "set", "Alpha", "claude_session_id", value)
		Eventually(session).Should(gexec.Exit(0))
	}

	readFile := func(dir string) string {
		content, err := os.ReadFile(filepath.Join(vaultPath, dir, "Alpha.md")) //#nosec G304 -- test file
		Expect(err).NotTo(HaveOccurred())
		return string(content)
	}

	ownedFixture := func(owner string) string {
		return fmt.Sprintf(`---
claude_session_id: %s
status: active
---
body line
`, owner)
	}

	DescribeTable("records the displaced owner for both page types",
		func(command, dir string) {
			createVault(ownedFixture(sessionA), ownedFixture(sessionA))

			setSession(command, sessionB)

			content := readFile(dir)
			// The `- ` list-item prefix is load-bearing: a bare `session_id: A`
			// also matches the `claude_session_id: A` line.
			Expect(strings.Count(content, "- session_id: "+sessionA)).To(Equal(1))
			Expect(content).To(ContainSubstring("claude_session_id: " + sessionB))
		},
		Entry("goal", "goal", "Goals"),
		Entry("topic", "topic", "Topics"),
	)

	DescribeTable("records nothing when a display name is rewritten to its uuid",
		func(command, dir string) {
			displayName := "vault-cli " + command + " set fix"
			createVault(ownedFixture(displayName), ownedFixture(displayName))

			setSession(command, sessionA)

			content := readFile(dir)
			Expect(strings.Count(content, "- session_id:")).To(Equal(0))
			Expect(content).To(ContainSubstring("claude_session_id: " + sessionA))
		},
		Entry("goal", "goal", "Goals"),
		Entry("topic", "topic", "Topics"),
	)
})
