// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/bborbe/vault-cli/pkg/ops"
)

// TestValidateOutputWithIssuesPlain tests outputValidatePlain with issues using subprocess
func TestValidateOutputWithIssuesPlain(t *testing.T) {
	if os.Getenv("TEST_VALIDATE_PLAIN_EXIT") == "1" {
		// This code runs in the subprocess and will call os.Exit(1)
		ctx := context.Background()
		lintOp := ops.NewLintOperation()

		f, err := os.CreateTemp("", "task-*.md")
		if err != nil {
			t.Fatal(err)
		}
		tmpFile := f.Name()
		defer func() { _ = os.Remove(tmpFile) }()

		content := `---
status: invalid_status
priority: 1
---
# Task with Invalid Status
`
		if _, err := f.WriteString(content); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		// This will call os.Exit(1) through outputValidatePlain
		_ = lintOp.ExecuteFile(ctx, tmpFile, "Test Task", "test", "plain")
		return
	}

	// Run the test in a subprocess
	//#nosec G204,G702 -- test binary path from os.Args, subprocess for exit testing
	cmd := exec.Command(
		os.Args[0],
		"-test.run=TestValidateOutputWithIssuesPlain",
	)
	cmd.Env = append(os.Environ(), "TEST_VALIDATE_PLAIN_EXIT=1")
	err := cmd.Run()

	// We expect exit status 1
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() == 1 {
			// Success - the function called os.Exit(1) as expected
			return
		}
	}

	t.Fatalf("expected exit status 1, got: %v", err)
}

// TestValidateOutputWithIssuesJSON tests outputValidateJSON with issues using subprocess
func TestValidateOutputWithIssuesJSON(t *testing.T) {
	if os.Getenv("TEST_VALIDATE_JSON_EXIT") == "1" {
		// This code runs in the subprocess and will call os.Exit(1)
		ctx := context.Background()
		lintOp := ops.NewLintOperation()

		f, err := os.CreateTemp("", "task-*.md")
		if err != nil {
			t.Fatal(err)
		}
		tmpFile := f.Name()
		defer func() { _ = os.Remove(tmpFile) }()

		content := `---
status: invalid_status
priority: 1
---
# Task with Invalid Status
`
		if _, err := f.WriteString(content); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		// This will call os.Exit(1) through outputValidateJSON
		_ = lintOp.ExecuteFile(ctx, tmpFile, "Test Task", "test", "json")
		return
	}

	// Run the test in a subprocess
	//#nosec G204,G702 -- test binary path from os.Args, subprocess for exit testing
	cmd := exec.Command(
		os.Args[0],
		"-test.run=TestValidateOutputWithIssuesJSON",
	)
	cmd.Env = append(os.Environ(), "TEST_VALIDATE_JSON_EXIT=1")
	err := cmd.Run()

	// We expect exit status 1
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() == 1 {
			// Success - the function called os.Exit(1) as expected
			return
		}
	}

	t.Fatalf("expected exit status 1, got: %v", err)
}

// TestValidateOutputWithIssuesFixable tests outputValidatePlain with fixable issues
func TestValidateOutputWithIssuesFixable(t *testing.T) {
	if os.Getenv("TEST_VALIDATE_FIXABLE_EXIT") == "1" {
		// This code runs in the subprocess and will call os.Exit(1)
		ctx := context.Background()
		lintOp := ops.NewLintOperation()

		f, err := os.CreateTemp("", "task-*.md")
		if err != nil {
			t.Fatal(err)
		}
		tmpFile := f.Name()
		defer func() { _ = os.Remove(tmpFile) }()

		content := `---
status: next
priority: high
assignee: alice
assignee: bob
---
# Task with Fixable Issues
`
		if _, err := f.WriteString(content); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		// This will call os.Exit(1) through outputValidatePlain (multiple fixable issues)
		_ = lintOp.ExecuteFile(ctx, tmpFile, "Test Task", "test", "plain")
		return
	}

	// Run the test in a subprocess
	//#nosec G204,G702 -- test binary path from os.Args, subprocess for exit testing
	cmd := exec.Command(
		os.Args[0],
		"-test.run=TestValidateOutputWithIssuesFixable",
	)
	cmd.Env = append(os.Environ(), "TEST_VALIDATE_FIXABLE_EXIT=1")
	err := cmd.Run()

	// We expect exit status 1
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() == 1 {
			// Success - the function called os.Exit(1) as expected
			return
		}
	}

	t.Fatalf("expected exit status 1, got: %v", err)
}

// TestValidateOutputJSONWithMultipleIssues tests outputValidateJSON with multiple issue types
func TestValidateOutputJSONWithMultipleIssues(t *testing.T) {
	if os.Getenv("TEST_VALIDATE_JSON_MULTI_EXIT") == "1" {
		ctx := context.Background()
		lintOp := ops.NewLintOperation()

		f, err := os.CreateTemp("", "task-*.md")
		if err != nil {
			t.Fatal(err)
		}
		tmpFile := f.Name()
		defer func() { _ = os.Remove(tmpFile) }()

		content := `---
status: invalid_status
priority: high
assignee: alice
assignee: bob
---
# Task with Multiple Issues
`
		if _, err := f.WriteString(content); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		_ = lintOp.ExecuteFile(ctx, tmpFile, "Multi Issue Task", "test", "json")
		return
	}

	//#nosec G204,G702 -- test binary path from os.Args, subprocess for exit testing
	cmd := exec.Command(
		os.Args[0],
		"-test.run=TestValidateOutputJSONWithMultipleIssues",
	)
	cmd.Env = append(os.Environ(), "TEST_VALIDATE_JSON_MULTI_EXIT=1")
	err := cmd.Run()

	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() == 1 {
			return
		}
	}

	t.Fatalf("expected exit status 1, got: %v", err)
}

// TestValidateOutputPlainWithMissingFrontmatter tests missing frontmatter detection
func TestValidateOutputPlainWithMissingFrontmatter(t *testing.T) {
	if os.Getenv("TEST_VALIDATE_PLAIN_MISSING_EXIT") == "1" {
		ctx := context.Background()
		lintOp := ops.NewLintOperation()

		f, err := os.CreateTemp("", "task-*.md")
		if err != nil {
			t.Fatal(err)
		}
		tmpFile := f.Name()
		defer func() { _ = os.Remove(tmpFile) }()

		content := `# Task Without Frontmatter

This task is missing frontmatter entirely.
`
		if _, err := f.WriteString(content); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		_ = lintOp.ExecuteFile(ctx, tmpFile, "Missing Frontmatter", "test", "plain")
		return
	}

	//#nosec G204,G702 -- test binary path from os.Args, subprocess for exit testing
	cmd := exec.Command(
		os.Args[0],
		"-test.run=TestValidateOutputPlainWithMissingFrontmatter",
	)
	cmd.Env = append(os.Environ(), "TEST_VALIDATE_PLAIN_MISSING_EXIT=1")
	err := cmd.Run()

	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() == 1 {
			return
		}
	}

	t.Fatalf("expected exit status 1, got: %v", err)
}
