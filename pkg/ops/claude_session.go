// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
)

//counterfeiter:generate -o ../../mocks/claude-session-starter.go --fake-name ClaudeSessionStarter . ClaudeSessionStarter

// ClaudeSessionStarter starts a new headless Claude session.
type ClaudeSessionStarter interface {
	// StartSession runs claude in headless mode to create a session with the given
	// session id and returns only an error. The session id is minted by the caller
	// (not by claude) so it can be persisted before the child process exists.
	// When name is non-empty, the session is created with -n <name> so its
	// custom-title and agent-name are set from turn 1.
	// Both branches block until the headless turn completes and validate its JSON
	// result; they differ in how. The interactive branch runs the child under the
	// request context (bounded by a 5m timeout). The non-interactive branch spawns
	// the child detached from the request context and waits for its exit, bounded by
	// sessionTurnTimeout — a wait bound, not a kill. Any outcome other than a clean,
	// validated turn returns an error, so the caller persists no session id and the
	// UI never offers Resume against a live or failed transcript. See
	// docs/work-on-session-lifecycle.md.
	StartSession(ctx context.Context, sessionID string, prompt string, cwd string, name string, isInteractive bool) error
}

// sessionTurnTimeout bounds how long the non-interactive branch waits for the detached
// child's headless turn to finish. It is a wait bound, never a kill: the child is
// detached (own process group) and survives expiry — the parent simply stops waiting
// and reports an error so the caller persists no session id. --max-turns is inert
// (maxTurns is -1), so a legitimate agentic chain can run for minutes; 30m is ~6-10x
// the observed turn length. Tunable constant; no config field unless a second caller
// needs one.
const sessionTurnTimeout = 30 * libtime.Minute

// NewClaudeSessionStarter creates a ClaudeSessionStarter using the given claude script.
// Returns nil if the binary is not found.
func NewClaudeSessionStarter(claudeScript string, locker SessionLocker) ClaudeSessionStarter {
	claudePath, err := exec.LookPath(claudeScript)
	if err != nil {
		return nil
	}
	return &claudeSessionStarter{
		claudePath:         claudePath,
		maxTurns:           -1,
		runCmd:             defaultCommandRunner,
		detachRun:          defaultDetachedRunner,
		waiter:             libtime.NewWaiterDuration(),
		sessionTurnTimeout: sessionTurnTimeout,
		locker:             locker,
	}
}

// NewClaudeSessionStarterWithRunner creates a ClaudeSessionStarter with an injectable command runner.
// Intended for testing.
func NewClaudeSessionStarterWithRunner(
	claudePath string,
	runCmd func(ctx context.Context, args []string, dir string) ([]byte, error),
	detachRun func(args []string, dir string, stdout *os.File) (<-chan error, error),
	waiter libtime.WaiterDuration,
	locker SessionLocker,
) ClaudeSessionStarter {
	return &claudeSessionStarter{
		claudePath:         claudePath,
		maxTurns:           -1,
		runCmd:             runCmd,
		detachRun:          detachRun,
		waiter:             waiter,
		sessionTurnTimeout: sessionTurnTimeout,
		locker:             locker,
	}
}

// defaultCommandRunner is the interactive (blocking) runner. It runs the command
// under the request context and returns combined stdout, so StartSession blocks
// through the whole headless turn before any output is available.
func defaultCommandRunner(ctx context.Context, args []string, dir string) ([]byte, error) {
	cmd := exec.CommandContext(
		ctx,
		args[0],
		args[1:]...) //#nosec G204 -- args[0] is the claude binary path from LookPath
	cmd.Dir = dir
	started := time.Now()
	output, err := cmd.Output()
	slog.Info(
		"claude interactive spawn completed",
		"args", args,
		"cwd", dir,
		"latency", time.Since(started),
		"err", err,
	)
	return output, err
}

// defaultDetachedRunner is the non-interactive runner. It spawns the child detached
// from the request context: exec.Command (not CommandContext), stderr redirected to
// os.DevNull so the child never dies on EPIPE when the parent exits, and Setpgid so
// it lives in its own process group and survives the parent. It returns a buffered
// channel that receives cmd.Wait()'s error, plus a spawn error when Start fails. The
// child may outlive this process by minutes; that is the point of the detachment,
// not an accident.
//
// stdout is the caller-owned temp file the turn's --output-format json blob lands in.
// A file (not a pipe) is deliberate: the child writes to an inherited fd with no
// reader, so there is no pipe-buffer deadlock and no EPIPE, and the file is complete
// once cmd.Wait() returns. The caller owns its lifecycle — this function never closes it.
func defaultDetachedRunner(args []string, dir string, stdout *os.File) (<-chan error, error) {
	cmd := exec.Command(args[0], args[1:]...) //#nosec G204 -- args[0] is the claude binary path from LookPath
	cmd.Dir = dir
	// os.DevNull is a string constant ("/dev/null"), NOT an io.Writer — assigning it
	// directly to cmd.Stderr does not compile. Open it as a file. Leaving Stderr nil
	// would also route to /dev/null, but the explicit os.DevNull reference keeps the
	// detachment deliberate rather than accidental.
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return nil, err // caller wraps with ctx; never context.Background() here
	}
	cmd.Stdout = stdout
	cmd.Stderr = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = devNull.Close()
		return nil, err
	}
	// Audit line for the detached spawn. This child writes to os.DevNull and outlives
	// this process, so its pid recorded here is the only trace it leaves — without it a
	// detached session is unobservable from the parent side.
	slog.Info("claude detached spawn started", "args", args, "cwd", dir, "pid", cmd.Process.Pid)
	done := make(chan error, 1)
	// Raw go func is deliberate here (go-concurrency/no-raw-go-func): this is a reaper
	// for a child we have intentionally detached, not orchestrated concurrent work, so
	// run.CancelOnFirstErrorWait does not apply. It is unbounded in time by design — the
	// child outlives this process — and cannot leak or block: the channel is buffered
	// with capacity 1 and has exactly one send.
	go func() {
		err := cmd.Wait()
		_ = devNull.Close() // close only after the child exits — closing earlier would
		// hand the still-running child a dead fd
		done <- err
	}()
	return done, nil
}

type claudeSessionStarter struct {
	claudePath string
	maxTurns   int // -1 = no limit, >0 = limit. Keep the field and keep the
	// `if c.maxTurns > 0 { args = append(args, "--max-turns", …) }`
	// branch in the rewritten StartSession. It is already inert
	// (both constructors hardcode -1, no test sets it positive),
	// but dropping it silently changes the struct's contract —
	// out of scope for this bug fix.
	runCmd             func(ctx context.Context, args []string, dir string) ([]byte, error)
	detachRun          func(args []string, dir string, stdout *os.File) (<-chan error, error)
	waiter             libtime.WaiterDuration
	sessionTurnTimeout libtime.Duration
	locker             SessionLocker
}

func (c *claudeSessionStarter) StartSession(
	ctx context.Context,
	sessionID string,
	prompt string,
	cwd string,
	name string,
	isInteractive bool,
) error {
	// Acquire the per-session lock first: a second work-on against a live session
	// must be refused before any child is spawned. The deferred release covers
	// every return path — clean turn, child exit error, ctx cancel, the 30m turn
	// bound expiry, the interactive 5m timeout, and validation errors — so a live
	// session can never leave the lock held.
	lock, err := c.locker.Acquire(ctx, sessionID)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()

	args := []string{
		c.claudePath,
		"--print",
	}
	if name != "" {
		args = append(args, "-n", name)
	}
	args = append(args, "-p", prompt, "--output-format", "json", "--session-id", sessionID)
	if c.maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", c.maxTurns))
	}

	if !isInteractive {
		return c.runDetachedTurn(ctx, args, cwd)
	}

	// Interactive branch, unchanged behaviour: block through the headless turn so
	// turn 2's `claude --resume` reads turn 1's completed on-disk result.
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	output, err := c.runCmd(timeoutCtx, args, cwd)
	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return errors.Errorf(ctx, "claude bootstrap turn timed out after 5m")
		}
		return errors.Wrap(ctx, err, "run claude")
	}

	return validateSessionTurn(ctx, output)
}

// runDetachedTurn spawns the child detached and blocks until its headless turn
// finishes. Returning early would hand the caller a session id whose transcript is
// still being written — the Vault UI would offer Resume against a live,
// single-writer-assumed jsonl and `claude --resume` would fail. So every exit path
// except a clean, validated turn returns an error, and the caller persists nothing.
func (c *claudeSessionStarter) runDetachedTurn(
	ctx context.Context,
	args []string,
	cwd string,
) error {
	outFile, err := os.CreateTemp("", "vault-claude-session-*.json")
	if err != nil {
		return errors.Wrap(ctx, err, "create claude output file")
	}
	// Unlink eagerly: on POSIX the child keeps its inherited fd, so the data stays
	// readable to us until we close, and nothing is left behind on any return path
	// (including the cancel/timeout ones, where the child still holds the fd).
	defer func() {
		_ = os.Remove(outFile.Name())
		_ = outFile.Close()
	}()

	done, err := c.detachRun(args, cwd, outFile)
	if err != nil {
		return errors.Wrap(ctx, err, "start detached claude session")
	}
	waitCh := make(chan error, 1)
	// Raw go func is deliberate here (go-concurrency/no-raw-go-func): this adapts the
	// injectable waiter into a channel so the select below can race it against the
	// child's exit. Bounded by sessionTurnTimeout, buffered with capacity 1 and
	// exactly one send, so it neither leaks nor blocks when the child wins the race.
	go func() {
		waitCh <- c.waiter.Wait(ctx, c.sessionTurnTimeout)
	}()
	select {
	case exitErr := <-done:
		if exitErr != nil {
			return errors.Errorf(ctx, "claude session exited with error: %v", exitErr)
		}
	case err := <-waitCh:
		// Both outcomes are errors so the caller persists no session id. The child
		// is detached and keeps running in either case; we only stop waiting on it.
		if err != nil {
			return errors.Wrap(ctx, err, "claude session wait cancelled")
		}
		return errors.Errorf(
			ctx,
			"claude session turn did not complete within %v",
			c.sessionTurnTimeout,
		)
	}

	// The child has exited, so its fd is closed and the file is complete.
	output, err := os.ReadFile(outFile.Name())
	if err != nil {
		return errors.Wrap(ctx, err, "read claude output")
	}
	return validateSessionTurn(ctx, output)
}

// validateSessionTurn checks the --output-format json blob a finished headless turn
// emits. Shared by both branches: a session id alone proves nothing, because claude
// reports one even for a turn that did no work or failed, so an unvalidated id would
// be handed to the operator as resumable when it is not.
func validateSessionTurn(ctx context.Context, output []byte) error {
	var result struct {
		SessionID string `json:"session_id"`
		NumTurns  int    `json:"num_turns"`
		IsError   bool   `json:"is_error"`
		Result    string `json:"result"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return errors.Wrap(ctx, err, "parse claude output")
	}

	if result.SessionID == "" {
		return errors.Errorf(ctx, "claude returned empty session_id")
	}

	if result.NumTurns == 0 {
		return errors.Errorf(ctx, "claude returned 0 turns: %s", result.Result)
	}

	if result.IsError {
		return errors.Errorf(ctx, "claude reported error: %s", result.Result)
	}

	return nil
}
