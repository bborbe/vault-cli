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

	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("PullRequest parsing", func() {
	DescribeTable("reads the state token that follows the dash",
		func(line string, expected ops.PullRequestState) {
			content := "# Pull Requests\n\n" + line + "\n"
			entries := ops.ParsePullRequests(content)
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].State).To(Equal(expected))
		},
		Entry("open", "- [bborbe/nuke#1591](https://github.com/bborbe/nuke/pull/1591) — open, approved, conflicting", ops.PullRequestStateOpen),
		Entry("merged with sha", "- [bborbe/nuke#975](https://github.com/bborbe/nuke/pull/975) — merged `0791e7de`", ops.PullRequestStateMerged),
		Entry("closed", "- [bborbe/nuke#7](https://github.com/bborbe/nuke/pull/7) — closed unmerged", ops.PullRequestStateClosed),
		Entry("free-form, no state token", "- [bborbe/nuke#7](https://github.com/bborbe/nuke/pull/7) — needs work", ops.PullRequestStateUnknown),
	)

	It("keys an entry as owner/repo#number", func() {
		entries := ops.ParsePullRequests(
			"# Pull Requests\n\n- [Seibert-Data/lib-kafka#9](https://github.com/Seibert-Data/lib-kafka/pull/9) — open\n",
		)
		Expect(entries).To(HaveLen(1))
		Expect(entries[0].Key).To(Equal("Seibert-Data/lib-kafka#9"))
	})

	It("stops at the next heading", func() {
		entries := ops.ParsePullRequests(
			"# Pull Requests\n\n- [bborbe/a#1](https://github.com/bborbe/a/pull/1) — open\n" +
				"\n# Progress\n\n- [bborbe/a#2](https://github.com/bborbe/a/pull/2) — open\n",
		)
		Expect(entries).To(HaveLen(1))
		Expect(entries[0].Key).To(Equal("bborbe/a#1"))
	})

	It("returns nothing when the section is absent", func() {
		Expect(ops.ParsePullRequests("# Impact\n\nno pull requests here\n")).To(BeEmpty())
	})
})

var _ = Describe("LintOperation pull request binding rule", func() {
	var (
		ctx       context.Context
		lintOp    ops.LintOperation
		vaultPath string
	)

	BeforeEach(func() {
		ctx = context.Background()
		lintOp = ops.NewLintOperation(libtime.NewCurrentDateTime())
		var err error
		vaultPath, err = os.MkdirTemp("", "vault-pr-binding-lint-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	writeFixture := func(content string) {
		dirPath := filepath.Join(vaultPath, "Tasks")
		Expect(os.MkdirAll(dirPath, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dirPath, "Alpha.md"), []byte(content), 0600)).To(Succeed())
	}

	// taskFixture builds a page whose only unusual property is the status plus
	// the pull request section, so any issue reported is the one under test.
	taskFixture := func(status string, prSection string) string {
		// phase must agree with status, or STATUS_PHASE_MISMATCH fires and the
		// fixture would report two issues instead of the one under test.
		phase := "done"
		completedDate := "completed_date: \"2026-11-24T08:00:00Z\"\n"
		if status == "in_progress" {
			phase = "execution"
			completedDate = ""
		}
		return "---\n" +
			"status: " + status + "\n" +
			"page_type: task\n" +
			"priority: 1\n" +
			"phase: " + phase + "\n" +
			"task_identifier: 44444444-4444-4444-8444-444444444444\n" +
			completedDate +
			"---\n" +
			"# Impact\n\nAlpha.\n" +
			prSection
	}

	DescribeTable("reports an open pull request on a finished task",
		func(status string) {
			writeFixture(taskFixture(status,
				"# Pull Requests\n\n- [bborbe/nuke#1591](https://github.com/bborbe/nuke/pull/1591) — open\n"))
			issues, err := lintOp.Execute(ctx, ops.PageTypeTask, vaultPath, "Tasks", "Goals", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(issues).To(HaveLen(1))
			Expect(issues[0].IssueType).To(Equal(ops.IssueTypePROpenOnFinishedTask))
			Expect(issues[0].Description).To(ContainSubstring("bborbe/nuke#1591"))
			Expect(issues[0].Fixable).To(BeFalse())
		},
		Entry("completed", "completed"),
		Entry("aborted", "aborted"),
	)

	DescribeTable("reports nothing",
		func(status string, prSection string) {
			writeFixture(taskFixture(status, prSection))
			issues, err := lintOp.Execute(ctx, ops.PageTypeTask, vaultPath, "Tasks", "Goals", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(issues).To(BeEmpty())
		},
		Entry("merged pull request on a completed task", "completed",
			"# Pull Requests\n\n- [bborbe/nuke#975](https://github.com/bborbe/nuke/pull/975) — merged `0791e7de`\n"),
		Entry("closed pull request on an aborted task", "aborted",
			"# Pull Requests\n\n- [bborbe/nuke#7](https://github.com/bborbe/nuke/pull/7) — closed\n"),
		Entry("unclassifiable entry on a completed task", "completed",
			"# Pull Requests\n\n- [bborbe/nuke#7](https://github.com/bborbe/nuke/pull/7) — needs work\n"),
		Entry("open pull request on an in-progress task", "in_progress",
			"# Pull Requests\n\n- [bborbe/nuke#1591](https://github.com/bborbe/nuke/pull/1591) — open\n"),
		Entry("no pull request section on a completed task", "completed", ""),
	)
})
