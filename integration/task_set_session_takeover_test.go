// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v3"
)

var _ = Describe("task set session take-over", func() {
	const (
		sessionA = "AAAA"
		sessionB = "BBBB"
	)

	var vaultPath, configPath string
	var cleanup func()

	// The fixture's frontmatter keys are alphabetical, because the storage writer
	// re-serializes the whole block in key order on every write.
	alphaFrontmatter := `---
page_type: task
status: in_progress
task_identifier: 11111111-1111-4111-8111-111111111111
---
body line
`

	BeforeEach(func() {
		vaultPath, configPath, cleanup = createTempVault(map[string]string{
			"Alpha": alphaFrontmatter,
		})
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

	bind := func(value string) {
		session := runCommand("task", "set", "Alpha", "claude_session_id", value)
		Eventually(session).Should(gexec.Exit(0))
	}

	readFile := func(path string) string {
		content, err := os.ReadFile(path) //#nosec G304 -- test file
		Expect(err).NotTo(HaveOccurred())
		return string(content)
	}

	alphaPath := func() string {
		return filepath.Join(vaultPath, "Tasks", "Alpha.md")
	}

	// getValue reads one field through the CLI and returns its rendered value.
	getValue := func(key string) string {
		session := runCommand("task", "get", "Alpha", key, "--output", "json")
		Eventually(session).Should(gexec.Exit(0))
		var parsed struct {
			Value string `json:"value"`
		}
		Expect(json.Unmarshal(session.Out.Contents(), &parsed)).To(Succeed())
		return parsed.Value
	}

	// frontmatterOf unmarshals only the frontmatter block; the markdown body
	// would fail the whole-file unmarshal.
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

	// firstSessionEntry returns the session_id and started_at of the first
	// metrics_sessions entry, as the YAML decoder resolved them.
	firstSessionEntry := func(frontmatter map[string]any) (string, any) {
		entries, ok := frontmatter["metrics_sessions"].([]any)
		Expect(ok).To(BeTrue(), "metrics_sessions is not a list")
		Expect(entries).NotTo(BeEmpty())
		entry, ok := entries[0].(map[string]any)
		Expect(ok).To(BeTrue(), "metrics_sessions entry is not a map")
		sessionID, ok := entry["session_id"].(string)
		Expect(ok).To(BeTrue(), "session_id is not a string")
		return sessionID, entry["started_at"]
	}

	It("records the displaced owner and not the incoming one", func() {
		bind(sessionA)

		start := time.Now()
		bind(sessionB)
		end := time.Now()

		Expect(getValue("claude_session_id")).To(Equal(sessionB))

		rendered := getValue("metrics_sessions")
		Expect(rendered).To(ContainSubstring("session_id:" + sessionA))
		Expect(rendered).NotTo(ContainSubstring("session_id:" + sessionB))

		content := readFile(alphaPath())
		// The `- ` list-item prefix is load-bearing: a bare `session_id: BBBB`
		// also matches the `claude_session_id: BBBB` line.
		Expect(strings.Count(content, "- session_id: AAAA")).To(Equal(1))
		Expect(strings.Count(content, "- session_id: BBBB")).To(Equal(0))

		sessionID, startedAt := firstSessionEntry(frontmatterOf(alphaPath()))
		Expect(sessionID).To(Equal(sessionA))
		Expect(startedAt).NotTo(BeNil(), "started_at missing")

		switch value := startedAt.(type) {
		case string:
			// The shared time type renders a midnight-UTC instant as a date only.
			if len(value) == len("2006-01-02") {
				Expect(value).To(Equal(start.UTC().Format("2006-01-02")))
				return
			}
			parsed, err := time.Parse(time.RFC3339, value)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed).NotTo(BeZero())
			Expect(parsed).To(BeTemporally(">=", start.Add(-time.Minute)))
			Expect(parsed).To(BeTemporally("<=", end.Add(time.Minute)))
		case time.Time:
			Expect(value).NotTo(BeZero())
			Expect(value).To(BeTemporally(">=", start.Add(-time.Minute)))
			Expect(value).To(BeTemporally("<=", end.Add(time.Minute)))
		default:
			Fail("started_at has an unexpected type")
		}
	})

	It("records nothing on a first bind", func() {
		bind(sessionB)

		Expect(getValue("claude_session_id")).To(Equal(sessionB))
		Expect(getValue("metrics_sessions")).To(BeEmpty())
		Expect(readFile(alphaPath())).NotTo(ContainSubstring("metrics_sessions:"))
	})

	It("records nothing when re-binding the same value", func() {
		bind(sessionA)
		bind(sessionB)
		before := getValue("metrics_sessions")

		bind(sessionB)

		Expect(getValue("metrics_sessions")).To(Equal(before))
		Expect(strings.Count(readFile(alphaPath()), "- session_id:")).To(Equal(1))
	})
})
