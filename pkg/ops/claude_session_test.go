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
			output = []byte(`{"session_id":"abc-123","result":"ok","num_turns":1,"is_error":false}`)
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
			output = []byte(`{"session_id":"","result":"ok","num_turns":1,"is_error":false}`)
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

	Context("returns session_id when num_turns >= 1 and is_error is false", func() {
		BeforeEach(func() {
			output = []byte(
				`{"session_id":"happy-path-sid","result":"done","num_turns":3,"is_error":false}`,
			)
		})

		It("returns session_id and nil error", func() {
			sessionID, err := starter.StartSession(ctx, "prompt", "/vault")
			Expect(err).To(BeNil())
			Expect(sessionID).To(Equal("happy-path-sid"))
		})
	})

	Context("num_turns is zero", func() {
		BeforeEach(func() {
			output = []byte(
				`{"session_id":"sid-123","result":"Unknown command: /x","num_turns":0,"is_error":false}`,
			)
		})

		It("returns error containing 0 turns and result", func() {
			_, err := starter.StartSession(ctx, "prompt", "/vault")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("0 turns"))
			Expect(err.Error()).To(ContainSubstring("Unknown command: /x"))
		})
	})

	Context("is_error is true", func() {
		BeforeEach(func() {
			output = []byte(
				`{"session_id":"sid-456","result":"something failed","num_turns":1,"is_error":true}`,
			)
		})

		It("returns error containing error and result", func() {
			_, err := starter.StartSession(ctx, "prompt", "/vault")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("error"))
			Expect(err.Error()).To(ContainSubstring("something failed"))
		})
	})

	Context("args passed to command runner", func() {
		var capturedArgs []string
		var capturedDir string

		JustBeforeEach(func() {
			output = []byte(`{"session_id":"sid-1","num_turns":1,"is_error":false}`)
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/bin/claude",
				func(_ context.Context, args []string, dir string) ([]byte, error) {
					capturedArgs = args
					capturedDir = dir
					return []byte(`{"session_id":"sid-1","num_turns":1,"is_error":false}`), nil
				},
			)
		})

		It("passes correct args and cwd", func() {
			_, err := starter.StartSession(ctx, "my prompt", "/my/vault")
			Expect(err).To(BeNil())
			Expect(capturedArgs).To(Equal([]string{
				"/bin/claude", "--print", "-p", "my prompt",
				"--output-format", "json",
			}))
			Expect(capturedDir).To(Equal("/my/vault"))
		})
	})

	Context("custom claude path via NewClaudeSessionStarterWithRunner", func() {
		It("uses the given claude path", func() {
			var capturedArgs []string
			customStarter := ops.NewClaudeSessionStarterWithRunner(
				"/opt/custom-claude",
				func(_ context.Context, args []string, _ string) ([]byte, error) {
					capturedArgs = args
					return []byte(`{"session_id":"sid-2","num_turns":1,"is_error":false}`), nil
				},
			)
			_, err := customStarter.StartSession(ctx, "prompt", "/vault")
			Expect(err).To(BeNil())
			Expect(capturedArgs[0]).To(Equal("/opt/custom-claude"))
		})
	})
})
