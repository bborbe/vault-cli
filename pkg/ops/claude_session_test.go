// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"
	"os"
	"time"

	libtime "github.com/bborbe/time"
	"github.com/google/uuid"
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
			nil,
			libtime.NewWaiterDuration(),
		)
	})

	Context("successful session start", func() {
		BeforeEach(func() {
			output = []byte(`{"session_id":"abc-123","result":"ok","num_turns":1,"is_error":false}`)
		})

		It("returns no error", func() {
			err := starter.StartSession(ctx, "session-abc", "/work-on-task \"my-task\"", "/vault", "", true)
			Expect(err).To(BeNil())
		})
	})

	Context("command fails", func() {
		BeforeEach(func() {
			runErr = errors.New("exit status 1")
		})

		It("returns error", func() {
			err := starter.StartSession(ctx, "session-abc", "prompt", "/vault", "", true)
			Expect(err).NotTo(BeNil())
		})
	})

	Context("invalid JSON output", func() {
		BeforeEach(func() {
			output = []byte(`not valid json`)
		})

		It("returns parse error", func() {
			err := starter.StartSession(ctx, "session-abc", "prompt", "/vault", "", true)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("parse claude output"))
		})
	})

	Context("empty session_id in response", func() {
		BeforeEach(func() {
			output = []byte(`{"session_id":"","result":"ok","num_turns":1,"is_error":false}`)
		})

		It("returns error about empty session_id", func() {
			err := starter.StartSession(ctx, "session-abc", "prompt", "/vault", "", true)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("empty session_id"))
		})
	})

	Context("missing session_id field", func() {
		BeforeEach(func() {
			output = []byte(`{"result":"ok"}`)
		})

		It("returns error about empty session_id", func() {
			err := starter.StartSession(ctx, "session-abc", "prompt", "/vault", "", true)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("empty session_id"))
		})
	})

	Context("returns no error when num_turns >= 1 and is_error is false", func() {
		BeforeEach(func() {
			output = []byte(
				`{"session_id":"happy-path-sid","result":"done","num_turns":3,"is_error":false}`,
			)
		})

		It("returns nil error", func() {
			err := starter.StartSession(ctx, "session-abc", "prompt", "/vault", "", true)
			Expect(err).To(BeNil())
		})
	})

	Context("num_turns is zero", func() {
		BeforeEach(func() {
			output = []byte(
				`{"session_id":"sid-123","result":"Unknown command: /x","num_turns":0,"is_error":false}`,
			)
		})

		It("returns error containing 0 turns and result", func() {
			err := starter.StartSession(ctx, "session-abc", "prompt", "/vault", "", true)
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
			err := starter.StartSession(ctx, "session-abc", "prompt", "/vault", "", true)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("error"))
			Expect(err.Error()).To(ContainSubstring("something failed"))
		})
	})

	Context("interactive branch deadline", func() {
		It("reports bootstrap turn timed out when the parent deadline has already passed", func() {
			parentCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Minute))
			defer cancel()
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				func(_ context.Context, _ []string, _ string) ([]byte, error) {
					return nil, errors.New("exit status 1")
				},
				nil,
				libtime.NewWaiterDuration(),
			)
			err := starter.StartSession(parentCtx, "id", "prompt", "/vault", "", true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bootstrap turn timed out"))
		})
	})

	Context("interactive branch blocks until the child exits", func() {
		It("does not return until the command completes", func() {
			release := make(chan struct{})
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				func(_ context.Context, _ []string, _ string) ([]byte, error) {
					<-release
					return []byte(`{"session_id":"sid-1","num_turns":1,"is_error":false}`), nil
				},
				nil,
				libtime.NewWaiterDuration(),
			)
			done := make(chan struct{})
			go func() {
				_ = starter.StartSession(context.Background(), "id", "prompt", "/vault", "", true)
				close(done)
			}()
			Consistently(done, "100ms").ShouldNot(BeClosed())
			close(release)
			Eventually(done, "2s").Should(BeClosed())
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
				nil,
				libtime.NewWaiterDuration(),
			)
		})

		It("passes correct args and cwd", func() {
			err := starter.StartSession(ctx, "session-abc", "my prompt", "/my/vault", "", true)
			Expect(err).To(BeNil())
			Expect(capturedArgs).To(Equal([]string{
				"/bin/claude", "--print", "-p", "my prompt",
				"--output-format", "json", "--session-id", "session-abc",
			}))
			Expect(capturedDir).To(Equal("/my/vault"))
		})

		It("inserts -n <name> after --print when name is non-empty", func() {
			err := starter.StartSession(ctx, "session-abc", "my prompt", "/my/vault", "My Task Title", true)
			Expect(err).To(BeNil())
			Expect(capturedArgs).To(Equal([]string{
				"/bin/claude", "--print", "-n", "My Task Title", "-p", "my prompt",
				"--output-format", "json", "--session-id", "session-abc",
			}))
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
				nil,
				libtime.NewWaiterDuration(),
			)
			err := customStarter.StartSession(ctx, "session-abc", "prompt", "/vault", "", true)
			Expect(err).To(BeNil())
			Expect(capturedArgs[0]).To(Equal("/opt/custom-claude"))
		})
	})

	Context("non-interactive branch", func() {
		var (
			detachArgs     []string
			detachDir      string
			doneCh         chan error
			detachErr      error
			capturedWindow libtime.Duration
			blockWaiter    chan struct{}
		)

		// validTurnJSON is what a clean headless turn writes to the captured stdout
		// file. Every success-path fake must write it, or StartSession fails
		// validation with "parse claude output".
		const validTurnJSON = `{"session_id":"session-abc","num_turns":3,"is_error":false,"result":"done"}`

		BeforeEach(func() {
			detachArgs = nil
			detachDir = ""
			doneCh = make(chan error, 1)
			detachErr = nil
			capturedWindow = 0
			// The waiter must BLOCK on the success paths. StartSession selects on the
			// child-exit channel and the waiter channel; a waiter that returns
			// immediately makes both ready and the select picks nondeterministically,
			// flipping between success and a spurious turn-timeout error.
			blockWaiter = make(chan struct{})
			DeferCleanup(func() { close(blockWaiter) })
		})

		JustBeforeEach(func() {
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(args []string, dir string, stdout *os.File) (<-chan error, error) {
					detachArgs = args
					detachDir = dir
					if stdout != nil {
						_, _ = stdout.WriteString(validTurnJSON)
					}
					return doneCh, detachErr
				},
				libtime.WaiterDurationFunc(func(_ context.Context, d libtime.Duration) error {
					capturedWindow = d
					<-blockWaiter
					return nil
				}),
			)
		})

		It("passes the session id and name to the detached runner", func() {
			sessionID := "123e4567-e89b-12d3-a456-426614174000"
			doneCh <- nil
			err := starter.StartSession(ctx, sessionID, "prompt", "/my/vault", "My Task", false)
			Expect(err).To(BeNil())
			Expect(detachArgs).To(ContainElement("--session-id"))
			Expect(detachArgs).To(ContainElement(sessionID))
			Expect(detachArgs).To(ContainElement("--print"))
			Expect(detachArgs).To(ContainElement("-n"))
			Expect(detachArgs).To(ContainElement("My Task"))
			Expect(detachDir).To(Equal("/my/vault"))
			_, err = uuid.Parse(sessionID)
			Expect(err).To(BeNil())
		})

		It("blocks until the detached child exits", func() {
			returned := make(chan error, 1)
			go func() {
				returned <- starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)
			}()
			// The child has not exited, so StartSession must still be waiting. This is
			// the whole point of the fix: returning here would hand the caller an id
			// whose transcript is still being written.
			Consistently(returned, "100ms").ShouldNot(Receive())
			doneCh <- nil
			Eventually(returned).Should(Receive(BeNil()))
			// Locks the wiring: StartSession hands the constant, not a stray literal.
			Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))
			// Locks the value: SessionTurnTimeout is an alias, so the line above moves
			// with the constant and would survive any retune. This line is the one that
			// fails when the bound is changed.
			Expect(capturedWindow).To(Equal(30 * libtime.Minute))
		})

		It("validates the turn and rejects a zero-turn result", func() {
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(_ []string, _ string, stdout *os.File) (<-chan error, error) {
					_, _ = stdout.WriteString(`{"session_id":"session-abc","num_turns":0,"is_error":false,"result":"Unknown command"}`)
					done := make(chan error, 1)
					done <- nil
					return done, nil
				},
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
					<-blockWaiter
					return nil
				}),
			)
			err := starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("0 turns"))
		})

		It("validates the turn and rejects an is_error result", func() {
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(_ []string, _ string, stdout *os.File) (<-chan error, error) {
					_, _ = stdout.WriteString(`{"session_id":"session-abc","num_turns":2,"is_error":true,"result":"boom"}`)
					done := make(chan error, 1)
					done <- nil
					return done, nil
				},
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
					<-blockWaiter
					return nil
				}),
			)
			err := starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("claude reported error"))
		})

		It("rejects an unparseable turn result", func() {
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(_ []string, _ string, _ *os.File) (<-chan error, error) {
					done := make(chan error, 1)
					done <- nil
					return done, nil
				},
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
					<-blockWaiter
					return nil
				}),
			)
			err := starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parse claude output"))
		})

		It("treats a child exit error as an error", func() {
			earlyDone := make(chan error, 1)
			earlyDone <- errors.New("exit status 1")
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(_ []string, _ string, _ *os.File) (<-chan error, error) {
					return earlyDone, nil
				},
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
					<-blockWaiter
					return nil
				}),
			)
			err := starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exit status 1"))
			Expect(err.Error()).To(ContainSubstring("exited with error"))
		})

		It("treats the turn timeout as an error so no id is persisted", func() {
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(_ []string, _ string, _ *os.File) (<-chan error, error) {
					// Child never exits within the bound.
					return make(chan error), nil
				},
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error { return nil }),
			)
			err := starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("did not complete within"))
		})

		It("treats context cancellation as an error so no id is persisted", func() {
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(_ []string, _ string, _ *os.File) (<-chan error, error) {
					return make(chan error), nil
				},
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
					return context.Canceled
				}),
			)
			err := starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("wait cancelled"))
		})

		It("wraps a spawn failure", func() {
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(_ []string, _ string, _ *os.File) (<-chan error, error) {
					return nil, ErrTest
				},
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
					<-blockWaiter
					return nil
				}),
			)
			err := starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start detached claude session"))
		})
	})
})
