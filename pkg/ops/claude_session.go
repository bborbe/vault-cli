// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
	"fmt"
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
	// On the interactive branch it blocks until the headless turn completes (bounded
	// by a 5m timeout) and validates the JSON result. On the non-interactive branch
	// it spawns the child detached from the request context and returns within the
	// liveness window once the child has proven it survives startup. See
	// docs/work-on-session-lifecycle.md.
	StartSession(ctx context.Context, sessionID string, prompt string, cwd string, name string, isInteractive bool) error
}

// livenessWindow is how long the non-interactive branch waits for the detached child
// to prove it survived startup (auth failure, bad flag). Tunable constant; no config
// field unless a second caller needs one.
const livenessWindow = 10 * libtime.Second

// NewClaudeSessionStarter creates a ClaudeSessionStarter using the given claude script.
// Returns nil if the binary is not found.
func NewClaudeSessionStarter(claudeScript string) ClaudeSessionStarter {
	claudePath, err := exec.LookPath(claudeScript)
	if err != nil {
		return nil
	}
	return &claudeSessionStarter{
		claudePath:     claudePath,
		maxTurns:       -1,
		runCmd:         defaultCommandRunner,
		detachRun:      defaultDetachedRunner,
		waiter:         libtime.NewWaiterDuration(),
		livenessWindow: livenessWindow,
	}
}

// NewClaudeSessionStarterWithRunner creates a ClaudeSessionStarter with an injectable command runner.
// Intended for testing.
func NewClaudeSessionStarterWithRunner(
	claudePath string,
	runCmd func(ctx context.Context, args []string, dir string) ([]byte, error),
	detachRun func(args []string, dir string) (<-chan error, error),
	waiter libtime.WaiterDuration,
) ClaudeSessionStarter {
	return &claudeSessionStarter{
		claudePath:     claudePath,
		maxTurns:       -1,
		runCmd:         runCmd,
		detachRun:      detachRun,
		waiter:         waiter,
		livenessWindow: livenessWindow,
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
	return cmd.Output()
}

// defaultDetachedRunner is the non-interactive runner. It spawns the child detached
// from the request context: exec.Command (not CommandContext), stdout/stderr
// redirected to os.DevNull so the child never dies on EPIPE when the parent exits,
// and Setpgid so it lives in its own process group and survives the parent. It
// returns a buffered channel that receives cmd.Wait()'s error, plus a spawn error
// when Start fails. The child may outlive this process by minutes; that is the
// point of the detachment, not an accident.
func defaultDetachedRunner(args []string, dir string) (<-chan error, error) {
	cmd := exec.Command(args[0], args[1:]...) //#nosec G204 -- args[0] is the claude binary path from LookPath
	cmd.Dir = dir
	// os.DevNull is a string constant ("/dev/null"), NOT an io.Writer — assigning it
	// directly to cmd.Stdout does not compile. Open it as a file. Leaving Stdout/Stderr
	// nil would also route to /dev/null, but AC1 requires an explicit os.DevNull
	// reference so the detachment is deliberate rather than accidental.
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return nil, err // caller wraps with ctx; never context.Background() here
	}
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = devNull.Close()
		return nil, err
	}
	done := make(chan error, 1)
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
	runCmd         func(ctx context.Context, args []string, dir string) ([]byte, error)
	detachRun      func(args []string, dir string) (<-chan error, error)
	waiter         libtime.WaiterDuration
	livenessWindow libtime.Duration
}

func (c *claudeSessionStarter) StartSession(
	ctx context.Context,
	sessionID string,
	prompt string,
	cwd string,
	name string,
	isInteractive bool,
) error {
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
		// Non-interactive branch: spawn detached and return once the child has
		// outlived the liveness window. The child keeps running after this process
		// exits (the Vault UI Start button gets its session id back in ~10s).
		done, err := c.detachRun(args, cwd)
		if err != nil {
			return errors.Wrap(ctx, err, "start detached claude session")
		}
		waitCh := make(chan error, 1)
		go func() {
			waitCh <- c.waiter.Wait(ctx, c.livenessWindow)
		}()
		select {
		case exitErr := <-done:
			return errors.Errorf(ctx, "claude session exited during startup: %v", exitErr)
		case err := <-waitCh:
			if err != nil {
				// The request context was cancelled mid-window. The child is detached
				// and survives on its own; the parent is exiting anyway.
				return nil
			}
			return nil
		}
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
