// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

func TestValidateExecuteFileWithInvalidStatus(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	lintOp := ops.NewLintOperation()

	f, err := os.CreateTemp("", "task-*.md")
	g.Expect(err).NotTo(HaveOccurred())
	defer func() { _ = os.Remove(f.Name()) }()

	content := "---\nstatus: invalid_status\npriority: 1\n---\n# Task\n"
	_, err = f.WriteString(content)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(f.Close()).To(Succeed())

	issues, err := lintOp.ExecuteFile(ctx, f.Name(), "Test Task", "test")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(issues).NotTo(BeEmpty())
	g.Expect(issues[0].IssueType).To(Equal(ops.IssueTypeInvalidStatus))
}

func TestValidateExecuteFileWithNoIssues(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	lintOp := ops.NewLintOperation()

	f, err := os.CreateTemp("", "task-*.md")
	g.Expect(err).NotTo(HaveOccurred())
	defer func() { _ = os.Remove(f.Name()) }()

	content := "---\nstatus: todo\npriority: 1\ntask_identifier: test-uuid\n---\n# Task\n"
	_, err = f.WriteString(content)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(f.Close()).To(Succeed())

	issues, err := lintOp.ExecuteFile(ctx, f.Name(), "Test Task", "test")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(issues).To(BeEmpty())
}

func TestValidateExecuteFileWithMissingFrontmatter(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	lintOp := ops.NewLintOperation()

	f, err := os.CreateTemp("", "task-*.md")
	g.Expect(err).NotTo(HaveOccurred())
	defer func() { _ = os.Remove(f.Name()) }()

	content := "# Task Without Frontmatter\n\nThis task is missing frontmatter.\n"
	_, err = f.WriteString(content)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(f.Close()).To(Succeed())

	issues, err := lintOp.ExecuteFile(ctx, f.Name(), "Missing Frontmatter", "test")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(issues).NotTo(BeEmpty())
	g.Expect(issues[0].IssueType).To(Equal(ops.IssueTypeMissingFrontmatter))
}

func TestValidateExecuteFileWithFixableIssues(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	lintOp := ops.NewLintOperation()

	f, err := os.CreateTemp("", "task-*.md")
	g.Expect(err).NotTo(HaveOccurred())
	defer func() { _ = os.Remove(f.Name()) }()

	content := "---\nstatus: next\npriority: high\nassignee: alice\nassignee: bob\n---\n# Task\n"
	_, err = f.WriteString(content)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(f.Close()).To(Succeed())

	issues, err := lintOp.ExecuteFile(ctx, f.Name(), "Test Task", "test")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(issues).NotTo(BeEmpty())
}
