// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("InteractionCounter", func() {
	var (
		ctx         context.Context
		projectsDir string
		sessionDir  string
		encodedDir  string
		counter     ops.InteractionCounter
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		projectsDir, err = os.MkdirTemp("", "interaction-counter-*")
		Expect(err).To(BeNil())
		sessionDir = "/home/node/vault-cli"
		encodedDir = "-home-node-vault-cli"
		counter = ops.NewInteractionCounter(projectsDir, sessionDir)
	})

	AfterEach(func() {
		if projectsDir != "" {
			_ = os.RemoveAll(projectsDir)
		}
	})

	// writeFixture writes a session log named <id>.jsonl under the encoded project
	// dir. The dir is created lazily so the missing-file case has no fixtures at all.
	writeFixture := func(id string, lines ...string) {
		dir := filepath.Join(projectsDir, encodedDir)
		Expect(os.MkdirAll(dir, 0750)).To(Succeed())
		content := strings.Join(lines, "\n")
		if content != "" {
			content += "\n"
		}
		Expect(os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte(content), 0600)).To(Succeed())
	}

	userLine := `{"type":"user","message":{"role":"user","content":"hi"}}`

	It("Sum across sessions", func() {
		writeFixture("s1", userLine, userLine, userLine)
		writeFixture("s2", userLine, userLine)
		Expect(counter.Count(ctx, []string{"s1", "s2"})).To(Equal(5))
	})

	It("Missing file contributes 0", func() {
		Expect(counter.Count(ctx, []string{"missing"})).To(Equal(0))
	})

	It("Malformed lines skipped", func() {
		writeFixture("s1", userLine, "not json at all {", "garbage!!!")
		Expect(counter.Count(ctx, []string{"s1"})).To(Equal(1))
	})

	It("Non-user types ignored", func() {
		writeFixture("s1",
			`{"type":"assistant","message":{"role":"assistant","content":"x"}}`,
			`{"type":"system","message":{"role":"system","content":"x"}}`,
			`{"type":"summary","summary":"x"}`,
			userLine,
		)
		Expect(counter.Count(ctx, []string{"s1"})).To(Equal(1))
	})

	It("Unsafe session ids rejected", func() {
		for _, id := range []string{"../x", "a/b", `a\b`, "..", ""} {
			Expect(counter.Count(ctx, []string{id})).To(Equal(0))
		}
		// The guard never builds a path for an unsafe id, so nothing is ever read
		// outside the encoded project directory.
		Expect(counter.Count(ctx, []string{"s1"})).To(Equal(0))
	})

	It("Large line", func() {
		big := `{"type":"user","message":{"content":"` + strings.Repeat("a", 200*1024) + `"}}`
		writeFixture("s1", big)
		Expect(counter.Count(ctx, []string{"s1"})).To(Equal(1))
	})

	It("Duplicate session ids counted once", func() {
		writeFixture("s1", userLine, userLine, userLine)
		// Adjacent duplicate (AC1): the shared session's 3 user turns count once.
		Expect(counter.Count(ctx, []string{"s1", "s1"})).To(Equal(3))
		Expect(counter.Count(ctx, []string{"s1"})).To(Equal(3))
		// Repeated N times (Desired Behavior 2): never more than once.
		Expect(counter.Count(ctx, []string{"s1", "s1", "s1"})).To(Equal(3))
	})

	It("Interleaved duplicate session ids counted once", func() {
		writeFixture("s1", userLine, userLine, userLine)
		writeFixture("s2", userLine, userLine)
		// AC2: each distinct session once, order-independent.
		Expect(counter.Count(ctx, []string{"s1", "s2", "s1"})).To(Equal(5))
		Expect(counter.Count(ctx, []string{"s2", "s1", "s2"})).To(Equal(5))
	})

	It("Duplicated missing file contributes 0", func() {
		// Failure Mode 4: a missing file contributes 0 whether duplicated or not (AC3).
		Expect(counter.Count(ctx, []string{"missing", "missing"})).To(Equal(0))
		// Failure Mode 3: every unsafe id still contributes 0 when duplicated, and the
		// dedup never panics on a malformed/empty id (AC3, unchanged guard table).
		for _, id := range []string{"../x", "a/b", `a\b`, "..", ""} {
			Expect(counter.Count(ctx, []string{id, id})).To(Equal(0))
		}
	})

	It("Counts only operator prompts while the interaction count counts every user turn", func() {
		operatorPromptFixture := []string{
			// counted — 7 lines
			`{"type":"user","message":{"role":"user","content":"fix the flaky test"},"promptSource":"typed"}`,
			`{"type":"user","message":{"role":"user","content":"also update the docs"},"promptSource":"queued"}`,
			`{"type":"user","message":{"role":"user","content":"run make test"},"promptSource":"suggestion_accepted"}`,
			`{"type":"user","message":{"role":"user","content":"check the log"},"turnOrigin":"human"}`,
			`{"type":"user","message":{"role":"user","content":"legacy typed prompt"}}`,
			`{"type":"user","message":{"role":"user","content":"<command-name>/compact</command-name>"}}`,
			`{"type":"user","message":{"role":"user","content":"<command-message>vault-cli:work-on-task</command-message>"}}`,
			// excluded — 14 type:"user" lines
			`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"output"}]}}`,
			`{"type":"user","toolUseResult":{"stdout":"x"},"message":{"role":"user","content":[{"type":"text","text":"tool output"}]}}`,
			`{"type":"user","isMeta":true,"message":{"role":"user","content":"injected system prompt"}}`,
			`{"type":"user","turnOrigin":"scheduled","message":{"role":"user","content":"scheduled wakeup"}}`,
			`{"type":"user","turnOrigin":"task_notification","message":{"role":"user","content":"background task finished"}}`,
			`{"type":"user","turnOrigin":"peer","message":{"role":"user","content":"peer message"}}`,
			`{"type":"user","promptSource":"system","message":{"role":"user","content":"<task-notification>done</task-notification>"}}`,
			`{"type":"user","message":{"role":"user","content":"<local-command-stdout>Compacted</local-command-stdout>"}}`,
			`{"type":"user","message":{"role":"user","content":"<local-command-caveat>caveat</local-command-caveat>"}}`,
			`{"type":"user","message":{"role":"user","content":"Caveat: The messages below were generated by the user"}}`,
			`{"type":"user","message":{"role":"user","content":"This session is being continued from a previous conversation"}}`,
			`{"type":"user","message":{"role":"user","content":"[Request interrupted by user]"}}`,
			`{"type":"user","message":{"role":"user","content":"<task-notification>done</task-notification>"}}`,
			`{"type":"user","message":{"role":"user","content":"Another Claude session sent a message: hi"}}`,
			// excluded — not a user turn
			`{"type":"assistant","message":{"role":"assistant","content":"x"}}`,
			`not json at all {`,
		}
		writeFixture("s1", operatorPromptFixture...)

		// Only the seven operator-typed prompts count.
		Expect(counter.CountOperatorPrompts(ctx, []string{"s1"})).To(Equal(7))
		// The interaction count is unchanged: every type:"user" line counts (7 + 14).
		Expect(counter.Count(ctx, []string{"s1"})).To(Equal(21))

		// The same guards apply: a missing file and an unsafe id contribute 0.
		Expect(counter.CountOperatorPrompts(ctx, []string{"missing"})).To(Equal(0))
		Expect(counter.CountOperatorPrompts(ctx, []string{"../x"})).To(Equal(0))
	})
})
