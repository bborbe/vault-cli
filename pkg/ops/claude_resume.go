// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/bborbe/errors"
)

//counterfeiter:generate -o ../../mocks/claude-resumer.go --fake-name ClaudeResumer . ClaudeResumer

// ClaudeResumer resumes an existing Claude session.
type ClaudeResumer interface {
	// ResumeSession replaces the current process with an interactive
	// claude --resume session.
	//
	// A non-empty prompt is appended to argv as the resumed session's first
	// interactive turn. An empty or whitespace-only prompt is omitted
	// entirely, so argv is identical to a plain `claude --resume <id>` —
	// callers that have nothing to say must not produce a trailing empty
	// positional argument.
	ResumeSession(ctx context.Context, sessionID string, cwd string, prompt string) error
}

// NewClaudeResumer creates a ClaudeResumer using the given claude script.
// Returns nil if the binary is not found.
func NewClaudeResumer(claudeScript string) ClaudeResumer {
	claudePath, err := exec.LookPath(claudeScript)
	if err != nil {
		return nil
	}
	return &claudeResumer{
		claudePath: claudePath,
		chdir:      os.Chdir,
		execFn:     syscall.Exec,
	}
}

// NewClaudeResumerForTesting creates a ClaudeResumer with injectable dependencies.
// Intended for testing.
func NewClaudeResumerForTesting(
	claudePath string,
	chdir func(string) error,
	execFn func(string, []string, []string) error,
) ClaudeResumer {
	return &claudeResumer{
		claudePath: claudePath,
		chdir:      chdir,
		execFn:     execFn,
	}
}

type claudeResumer struct {
	claudePath string
	chdir      func(dir string) error
	execFn     func(argv0 string, argv []string, envv []string) error
}

func (c *claudeResumer) ResumeSession(
	ctx context.Context,
	sessionID string,
	cwd string,
	prompt string,
) error {
	if err := c.chdir(cwd); err != nil {
		return errors.Wrapf(ctx, err, "change directory to %s", cwd)
	}
	args := []string{"claude", "--resume", sessionID}
	if strings.TrimSpace(prompt) != "" {
		args = append(args, prompt)
	}
	return c.execFn(c.claudePath, args, os.Environ())
}
