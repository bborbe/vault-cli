// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/bborbe/errors"
)

//counterfeiter:generate -o ../../mocks/claude-session-starter.go --fake-name ClaudeSessionStarter . ClaudeSessionStarter

// ClaudeSessionStarter starts a new headless Claude session.
type ClaudeSessionStarter interface {
	// StartSession runs claude in headless mode to create a session, returns session_id.
	StartSession(ctx context.Context, prompt string, cwd string) (string, error)
}

// NewClaudeSessionStarter creates a ClaudeSessionStarter using the given claude script.
// Returns nil if the binary is not found.
func NewClaudeSessionStarter(claudeScript string) ClaudeSessionStarter {
	claudePath, err := exec.LookPath(claudeScript)
	if err != nil {
		return nil
	}
	return &claudeSessionStarter{
		claudePath: claudePath,
		maxTurns:   -1,
		runCmd:     defaultCommandRunner,
	}
}

// NewClaudeSessionStarterWithRunner creates a ClaudeSessionStarter with an injectable command runner.
// Intended for testing.
func NewClaudeSessionStarterWithRunner(
	claudePath string,
	runCmd func(ctx context.Context, args []string, dir string) ([]byte, error),
) ClaudeSessionStarter {
	return &claudeSessionStarter{
		claudePath: claudePath,
		maxTurns:   -1,
		runCmd:     runCmd,
	}
}

func defaultCommandRunner(ctx context.Context, args []string, dir string) ([]byte, error) {
	cmd := exec.CommandContext(
		ctx,
		args[0],
		args[1:]...) //#nosec G204 -- args[0] is the claude binary path from LookPath
	cmd.Dir = dir
	return cmd.Output()
}

type claudeSessionStarter struct {
	claudePath string
	maxTurns   int // -1 = no limit, >0 = limit
	runCmd     func(ctx context.Context, args []string, dir string) ([]byte, error)
}

func (c *claudeSessionStarter) StartSession(
	ctx context.Context,
	prompt string,
	cwd string,
) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	args := []string{
		c.claudePath,
		"--print",
		"-p",
		prompt,
		"--output-format",
		"json",
	}
	if c.maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", c.maxTurns))
	}
	output, err := c.runCmd(timeoutCtx, args, cwd)
	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("claude session start timed out after 5m")
		}
		return "", errors.Wrap(ctx, err, "run claude")
	}

	var result struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", errors.Wrap(ctx, err, "parse claude output")
	}

	if result.SessionID == "" {
		return "", fmt.Errorf("claude returned empty session_id")
	}

	return result.SessionID, nil
}
