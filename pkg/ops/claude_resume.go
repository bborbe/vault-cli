// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

//counterfeiter:generate -o ../../mocks/claude-resumer.go --fake-name ClaudeResumer . ClaudeResumer

// ClaudeResumer resumes an existing Claude session.
type ClaudeResumer interface {
	// ResumeSession replaces the current process with an interactive claude --resume session.
	ResumeSession(sessionID string, cwd string) error
}

// NewClaudeResumer creates a ClaudeResumer using the system claude binary.
// Returns nil if the claude binary is not found.
func NewClaudeResumer() ClaudeResumer {
	claudePath, err := exec.LookPath("claude")
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

func (c *claudeResumer) ResumeSession(sessionID string, cwd string) error {
	if err := c.chdir(cwd); err != nil {
		return fmt.Errorf("change directory to %s: %w", cwd, err)
	}
	args := []string{"claude", "--resume", sessionID}
	return c.execFn(c.claudePath, args, os.Environ())
}
