// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("LintOperation blocked_by scalar detector", func() {
	var (
		ctx       context.Context
		lintOp    ops.LintOperation
		vaultPath string
	)

	BeforeEach(func() {
		ctx = context.Background()
		lintOp = ops.NewLintOperation()

		var err error
		vaultPath, err = os.MkdirTemp("", "vault-blocked-by-lint-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	// writeFixture writes content to <vaultPath>/<dir>/<name>.md and returns the path.
	writeFixture := func(dir string, name string, content string) string {
		dirPath := filepath.Join(vaultPath, dir)
		Expect(os.MkdirAll(dirPath, 0755)).To(Succeed())
		path := filepath.Join(dirPath, name+".md")
		Expect(os.WriteFile(path, []byte(content), 0600)).To(Succeed())
		return path
	}

	// taskFixture builds a task page carrying only base keys plus the blocked_by
	// block under test, so the only issue any run can report is the one under test.
	taskFixture := func(blockedByBlock string) string {
		return "---\n" +
			"status: in_progress\n" +
			"page_type: task\n" +
			"priority: 1\n" +
			"task_identifier: 33333333-3333-4333-8333-333333333333\n" +
			blockedByBlock +
			"---\n# Alpha\n"
	}

	DescribeTable("reports a scalar-shaped blocked_by exactly once",
		func(blockedByBlock string) {
			writeFixture("Tasks", "Alpha", taskFixture(blockedByBlock))
			issues, err := lintOp.Execute(ctx, ops.PageTypeTask, vaultPath, "Tasks", "Goals", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(issues).To(HaveLen(1))
			Expect(issues[0].IssueType).To(Equal(ops.IssueTypeBlockedByScalar))
			Expect(issues[0].Description).To(ContainSubstring("blocked_by"))
			Expect(issues[0].Description).To(ContainSubstring("YAML list"))
			Expect(issues[0].Fixable).To(BeFalse())
		},
		Entry("plain scalar", "blocked_by: Blocker A\n"),
		Entry("quoted wikilink scalar", "blocked_by: '[[Blocker A]]'\n"),
		Entry("comma-joined scalar", "blocked_by: A,B\n"),
		Entry("non-string scalar", "blocked_by: 42\n"),
	)

	DescribeTable("does not report a legal shape",
		func(blockedByBlock string) {
			writeFixture("Tasks", "Alpha", taskFixture(blockedByBlock))
			issues, err := lintOp.Execute(ctx, ops.PageTypeTask, vaultPath, "Tasks", "Goals", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(issues).To(BeEmpty())
		},
		Entry("block-indented list", "blocked_by:\n  - \"[[Blocker A]]\"\n"),
		Entry("inline list", "blocked_by: [\"[[Blocker A]]\"]\n"),
		Entry("empty scalar — the documented clear", "blocked_by: \"\"\n"),
		Entry("null value", "blocked_by:\n"),
		Entry("absent key", ""),
	)

	It("reports the issue through ExecuteFile — the task validate path", func() {
		path := writeFixture("Tasks", "Alpha", taskFixture("blocked_by: Blocker A\n"))

		issues, err := lintOp.ExecuteFile(ctx, ops.PageTypeTask, path, "Alpha", "test")
		Expect(err).NotTo(HaveOccurred())
		Expect(issues).To(HaveLen(1))
		Expect(issues[0].IssueType).To(Equal(ops.IssueTypeBlockedByScalar))
	})

	It("reports the issue for a goal through the goals-directory walk — the goal lint path", func() {
		writeFixture(
			"Goals",
			"Beta",
			"---\nstatus: next\npage_type: goal\npriority: 1\nblocked_by: Blocker C\n---\n# Beta\n",
		)

		issues, err := lintOp.Execute(ctx, "goal", vaultPath, "Goals", "Goals", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(issues).To(HaveLen(1))
		Expect(issues[0].IssueType).To(Equal(ops.IssueTypeBlockedByScalar))
	})

	It("reports the issue and leaves the file byte-identical under fix", func() {
		path := writeFixture("Tasks", "Alpha", taskFixture("blocked_by: Blocker A\n"))

		before, err := os.ReadFile(path) //#nosec G304 -- user-controlled test vault path
		Expect(err).NotTo(HaveOccurred())

		issues, err := lintOp.Execute(ctx, ops.PageTypeTask, vaultPath, "Tasks", "Goals", true)
		Expect(err).NotTo(HaveOccurred())
		Expect(issues).To(HaveLen(1))
		Expect(issues[0].IssueType).To(Equal(ops.IssueTypeBlockedByScalar))
		Expect(issues[0].Fixed).To(BeFalse())

		after, err := os.ReadFile(path) //#nosec G304 -- user-controlled test vault path
		Expect(err).NotTo(HaveOccurred())
		Expect(after).To(Equal(before))
	})
})
