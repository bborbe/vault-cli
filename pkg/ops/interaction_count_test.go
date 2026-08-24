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
})
