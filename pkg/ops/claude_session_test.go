// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("ClaudeSessionStarter", func() {
	var (
		ctx     context.Context
		starter ops.ClaudeSessionStarter
		runErr  error
		output  []byte
	)

	BeforeEach(func() {
		ctx = context.Background()
		runErr = nil
		output = nil
	})

	JustBeforeEach(func() {
		capturedOutput := output
		capturedErr := runErr
		starter = ops.NewClaudeSessionStarterWithRunner(
			"/usr/local/bin/claude",
			func(_ context.Context, _ []string, _ string) ([]byte, error) {
				return capturedOutput, capturedErr
			},
		)
	})

	Context("successful session start", func() {
		BeforeEach(func() {
			output = []byte(`{"session_id":"abc-123","result":"ok"}`)
		})

		It("returns the session_id", func() {
			sessionID, err := starter.StartSession(ctx, "/work-on-task \"my-task\"", "/vault")
			Expect(err).To(BeNil())
			Expect(sessionID).To(Equal("abc-123"))
		})
	})

	Context("command fails", func() {
		BeforeEach(func() {
			runErr = errors.New("exit status 1")
		})

		It("returns error", func() {
			_, err := starter.StartSession(ctx, "prompt", "/vault")
			Expect(err).NotTo(BeNil())
		})
	})

	Context("invalid JSON output", func() {
		BeforeEach(func() {
			output = []byte(`not valid json`)
		})

		It("returns parse error", func() {
			_, err := starter.StartSession(ctx, "prompt", "/vault")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("parse claude output"))
		})
	})

	Context("empty session_id in response", func() {
		BeforeEach(func() {
			output = []byte(`{"session_id":"","result":"ok"}`)
		})

		It("returns error about empty session_id", func() {
			_, err := starter.StartSession(ctx, "prompt", "/vault")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("empty session_id"))
		})
	})

	Context("missing session_id field", func() {
		BeforeEach(func() {
			output = []byte(`{"result":"ok"}`)
		})

		It("returns error about empty session_id", func() {
			_, err := starter.StartSession(ctx, "prompt", "/vault")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("empty session_id"))
		})
	})

	Context("args passed to command runner", func() {
		var capturedArgs []string
		var capturedDir string

		JustBeforeEach(func() {
			output = []byte(`{"session_id":"sid-1"}`)
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/bin/claude",
				func(_ context.Context, args []string, dir string) ([]byte, error) {
					capturedArgs = args
					capturedDir = dir
					return []byte(`{"session_id":"sid-1"}`), nil
				},
			)
		})

		It("passes correct args and cwd", func() {
			_, err := starter.StartSession(ctx, "my prompt", "/my/vault")
			Expect(err).To(BeNil())
			Expect(capturedArgs).To(Equal([]string{
				"/bin/claude", "--print", "-p", "my prompt",
				"--output-format", "json", "--max-turns", "1",
			}))
			Expect(capturedDir).To(Equal("/my/vault"))
		})
	})
})
