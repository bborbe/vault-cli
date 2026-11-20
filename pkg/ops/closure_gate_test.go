// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("Closure gate", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("verdict",
		func(
			kind ops.ClosureKind,
			content string,
			members []ops.ClosureMember,
			expectErr bool,
			expectedOutcome ops.ClosureOutcome,
			expectedItemCount int,
			contains string,
		) {
			verdict, err := ops.CheckClosureGate(ctx, kind, content, members)
			if expectErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(verdict.Outcome).To(Equal(expectedOutcome))
			Expect(verdict.OpenItems).To(HaveLen(expectedItemCount))
			if contains != "" {
				Expect(strings.Join(verdict.OpenItems, "\n")).To(ContainSubstring(contains))
			}
		},
		Entry(
			"SC box open",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [ ] an open criterion\n"+
				"# Definition of Done\n- [x] done item\n",
			nil,
			false,
			ops.ClosureOutcomeOpen,
			1,
			"an open criterion",
		),
		Entry(
			"DoD section present but empty",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n# Definition of Done\n",
			nil,
			false,
			ops.ClosureOutcomeUnmeasurable,
			1,
			"# Definition of Done",
		),
		Entry(
			"DoD absent before cutoff is grandfathered",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\ncreated: \"2026-06-01\"\n---\n# Success Criteria\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeClose,
			0,
			"",
		),
		Entry(
			"DoD absent with no created is grandfathered",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeClose,
			0,
			"",
		),
		Entry(
			"DoD heading in a non-# spelling is unmeasurable",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\ncreated: \"2026-06-01\"\n---\n# Success Criteria\n- [x] done\n"+
				"## Definition of Done\n",
			nil,
			false,
			ops.ClosureOutcomeUnmeasurable,
			1,
			"# Definition of Done",
		),
		Entry(
			"DoD heading as a bullet is unmeasurable",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\ncreated: \"2026-06-01\"\n---\n# Success Criteria\n- [x] done\n"+
				"- Definition of Done\n",
			nil,
			false,
			ops.ClosureOutcomeUnmeasurable,
			1,
			"# Definition of Done",
		),
		Entry(
			"DoD heading as setext is unmeasurable",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\ncreated: \"2026-06-01\"\n---\n# Success Criteria\n- [x] done\n"+
				"Definition of Done\n===\n",
			nil,
			false,
			ops.ClosureOutcomeUnmeasurable,
			1,
			"# Definition of Done",
		),
		Entry(
			"DoD absent on or after cutoff",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\ncreated: \"2026-06-26\"\n---\n# Success Criteria\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeUnmeasurable,
			1,
			"# Definition of Done",
		),
		Entry(
			"DoD absent with an unquoted post-cutoff date is not grandfathered",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\ncreated: 2026-07-01\n---\n# Success Criteria\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeUnmeasurable,
			1,
			"# Definition of Done",
		),
		Entry(
			"DoD absent with an unquoted RFC3339 post-cutoff created is not grandfathered",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\ncreated: 2026-07-01T00:00:00Z\n---\n# Success Criteria\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeUnmeasurable,
			1,
			"# Definition of Done",
		),
		Entry(
			"DoD absent with an unquoted pre-cutoff date is grandfathered",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\ncreated: 2026-01-05\n---\n# Success Criteria\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeClose,
			0,
			"",
		),
		Entry(
			"DoD absent with a quoted RFC3339 pre-cutoff created is grandfathered",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\ncreated: \"2026-01-05T00:00:00Z\"\n---\n# Success Criteria\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeClose,
			0,
			"",
		),
		Entry(
			"DoD absent with non-ISO created",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\ncreated: \"15.01.2026\"\n---\n# Success Criteria\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeUnmeasurable,
			1,
			"# Definition of Done",
		),
		Entry(
			"star bullet open box",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n* [ ] open\n# Definition of Done\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeOpen,
			1,
			"open",
		),
		Entry(
			"unrecognised dash marker",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [-] open\n# Definition of Done\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeOpen,
			1,
			"open",
		),
		Entry(
			"unrecognised question marker",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [?] open\n# Definition of Done\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeOpen,
			1,
			"open",
		),
		Entry(
			"box inside fenced block is not a box",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n```\n- [ ] example\n```\n"+
				"# Definition of Done\n- [x] done\n",
			nil,
			false,
			ops.ClosureOutcomeUnmeasurable,
			1,
			"# Success Criteria",
		),
		Entry(
			"unterminated fence",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n```\n- [ ] example\n",
			nil,
			false,
			ops.ClosureOutcomeUnmeasurable,
			1,
			"unterminated code fence",
		),
		Entry(
			"BOM prefixed CRLF page",
			ops.ClosureKindGoal,
			"\ufeff---\r\nstatus: in_progress\r\ncreated: \"2026-06-01\"\r\n---\r\n"+
				"# Success Criteria\r\n- [x] done\r\n",
			nil,
			false,
			ops.ClosureOutcomeClose,
			0,
			"",
		),
		Entry(
			"Completion Gate box open",
			ops.ClosureKindTopic,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n# Completion Gate\n- [ ] gate\n",
			nil,
			false,
			ops.ClosureOutcomeOpen,
			1,
			"gate",
		),
		Entry(
			"Completion Gate mixes a checkbox with an ordered item",
			ops.ClosureKindTopic,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n"+
				"# Completion Gate\n- [x] done\n1. an ordered item\n",
			nil,
			false,
			ops.ClosureOutcomeUnmeasurable,
			1,
			"an ordered item",
		),
		Entry(
			"member goal not terminal",
			ops.ClosureKindTopic,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n"+
				"# Completion Gate\n- [x] gated\n",
			[]ops.ClosureMember{{Name: "member", Status: "in_progress"}},
			false,
			ops.ClosureOutcomeOpen,
			1,
			"member",
		),
		Entry(
			"member goal terminal",
			ops.ClosureKindTopic,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n"+
				"# Completion Gate\n- [x] gated\n",
			[]ops.ClosureMember{{Name: "member", Status: "completed"}},
			false,
			ops.ClosureOutcomeClose,
			0,
			"",
		),
		Entry(
			"exempt open member does not block",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n"+
				"# Definition of Done\n- [x] done\n",
			[]ops.ClosureMember{{Name: "repair-row", Status: "next", Exempt: true}},
			false,
			ops.ClosureOutcomeClose,
			0,
			"",
		),
		Entry(
			"exempt open member does not mask a genuine open member",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n"+
				"# Definition of Done\n- [x] done\n",
			[]ops.ClosureMember{
				{Name: "repair-row", Status: "next", Exempt: true},
				{Name: "re-check", Status: "in_progress"},
			},
			false,
			ops.ClosureOutcomeOpen,
			1,
			"re-check",
		),
		Entry(
			"exempt member does not mask an open Success Criteria box",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [ ] an open criterion\n"+
				"# Definition of Done\n- [x] done\n",
			[]ops.ClosureMember{{Name: "repair-row", Status: "next", Exempt: true}},
			false,
			ops.ClosureOutcomeOpen,
			1,
			"an open criterion",
		),
		Entry(
			"unmarked open member still blocks",
			ops.ClosureKindGoal,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n"+
				"# Definition of Done\n- [x] done\n",
			[]ops.ClosureMember{{Name: "ordinary-task", Status: "todo"}},
			false,
			ops.ClosureOutcomeOpen,
			1,
			"ordinary-task",
		),
		Entry(
			"topic with no Goals section",
			ops.ClosureKindTopic,
			"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n"+
				"# Completion Gate\n- [x] gated\n",
			nil,
			false,
			ops.ClosureOutcomeClose,
			0,
			"",
		),
		Entry(
			"already completed page",
			ops.ClosureKindGoal,
			"---\nstatus: completed\n---\n",
			nil,
			false,
			ops.ClosureOutcomeTerminal,
			0,
			"",
		),
		Entry(
			"already aborted page",
			ops.ClosureKindGoal,
			"---\nstatus: aborted\n---\n",
			nil,
			false,
			ops.ClosureOutcomeTerminal,
			0,
			"",
		),
		Entry(
			"missing frontmatter",
			ops.ClosureKindGoal,
			"# Success Criteria\n- [x] done\n",
			nil,
			true,
			ops.ClosureOutcomeOpen,
			0,
			"",
		),
	)
})

var _ = Describe("Closure gate Goals section", func() {
	DescribeTable("members",
		func(content string, expected []string) {
			Expect(ops.ParseGoalSectionMembers(content)).To(Equal(expected))
		},
		Entry(
			"reads a ## Goals list",
			"## Goals\n- [[A]]\n- [[B]]\n# Next\n",
			[]string{"A", "B"},
		),
		Entry(
			"runs past a ### sub-heading inside ## Goals",
			"## Goals\n- [[A]]\n- [[B]]\n### Lever A\n- [[C]]\n# Next\n",
			[]string{"A", "B", "C"},
		),
		Entry(
			"runs past ## inside # Goals",
			"# Goals\n- [[A]]\n## Sub\n- [[D]]\n# Other\n",
			[]string{"A", "D"},
		),
		Entry(
			"stops at the next # heading after # Goals",
			"# Goals\n- [[A]]\n# Other\n- [[E]]\n",
			[]string{"A"},
		),
		Entry(
			"ignores non-declaration lines",
			"## Goals\n1. [[X]]\n**[[Y]]**\nsome [[Z]] here\n- [[Real]]\n",
			[]string{"Real"},
		),
		Entry(
			"strips an alias",
			"## Goals\n- [[Name|label]]\n",
			[]string{"Name"},
		),
		Entry(
			"returns empty without a Goals heading",
			"# Other\n- [[A]]\n",
			[]string(nil),
		),
	)
})
