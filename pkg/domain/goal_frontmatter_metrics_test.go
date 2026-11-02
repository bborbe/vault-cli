// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"time"

	libtime "github.com/bborbe/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("GoalFrontmatter metrics_sessions", func() {
	var fm domain.GoalFrontmatter

	BeforeEach(func() {
		fm = domain.NewGoalFrontmatter(nil)
	})

	It("returns nil on a fresh frontmatter", func() {
		Expect(fm.MetricsSessions()).To(BeNil())
	})

	It("preserves prior entries in order when appending", func() {
		start1 := libtime.DateOrDateTime(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC))
		fm.AppendMetricsSession(domain.MetricsSession{SessionID: "s1", StartedAt: start1})
		fm.AppendMetricsSession(domain.MetricsSession{
			SessionID: "s2",
			StartedAt: libtime.DateOrDateTime(time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC)),
		})

		sessions := fm.MetricsSessions()
		Expect(sessions).To(HaveLen(2))
		Expect(sessions[0].SessionID).To(Equal("s1"))
		Expect(sessions[0].StartedAt).To(Equal(start1))
		Expect(sessions[1].SessionID).To(Equal("s2"))
	})

	It("reads the on-disk generic shape into typed sessions", func() {
		fm = domain.NewGoalFrontmatter(map[string]any{
			"metrics_sessions": []any{
				map[string]any{
					"session_id": "s1",
					"started_at": time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC),
				},
				map[string]any{"session_id": "s2", "started_at": "2026-08-24T18:14:35Z"},
			},
		})

		sessions := fm.MetricsSessions()
		Expect(sessions).To(HaveLen(2))
		Expect(sessions[0].SessionID).To(Equal("s1"))
		Expect(
			sessions[0].StartedAt.Time().Equal(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)),
		).To(BeTrue())
		Expect(sessions[1].SessionID).To(Equal("s2"))
		Expect(sessions[1].StartedAt.String()).To(Equal("2026-08-24T18:14:35Z"))
	})

	It("drops malformed entries and keeps the well-formed ones in order", func() {
		fm = domain.NewGoalFrontmatter(map[string]any{
			"metrics_sessions": []any{
				map[string]any{"session_id": "s1"},
				"not-a-map",
				map[string]any{"session_id": ""},
				map[string]any{"started_at": "2026-08-24T09:00:00Z"},
				map[string]any{"session_id": "s2"},
			},
		})

		sessions := fm.MetricsSessions()
		Expect(sessions).To(HaveLen(2))
		Expect(sessions[0].SessionID).To(Equal("s1"))
		Expect(sessions[1].SessionID).To(Equal("s2"))

		fm.AppendMetricsSession(domain.MetricsSession{SessionID: "s3"})
		sessions = fm.MetricsSessions()
		Expect(sessions).To(HaveLen(3))
		Expect(sessions[0].SessionID).To(Equal("s1"))
		Expect(sessions[1].SessionID).To(Equal("s2"))
		Expect(sessions[2].SessionID).To(Equal("s3"))
	})

	It("treats a non-list metrics_sessions value as absent and appends onto nothing", func() {
		fm = domain.NewGoalFrontmatter(map[string]any{"metrics_sessions": "not-a-list"})
		Expect(fm.MetricsSessions()).To(BeNil())

		fm.AppendMetricsSession(domain.MetricsSession{SessionID: "s1"})
		sessions := fm.MetricsSessions()
		Expect(sessions).To(HaveLen(1))
		Expect(sessions[0].SessionID).To(Equal("s1"))
	})

	It("ignores an append with an empty session id", func() {
		fm.AppendMetricsSession(domain.MetricsSession{SessionID: ""})
		Expect(fm.Get("metrics_sessions")).To(BeNil())
	})

	It("preserves sessions through a YAML round-trip", func() {
		start1 := libtime.DateOrDateTime(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC))
		start2 := libtime.DateOrDateTime(time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC))
		fm.AppendMetricsSession(domain.MetricsSession{SessionID: "s1", StartedAt: start1})
		fm.AppendMetricsSession(domain.MetricsSession{SessionID: "s2", StartedAt: start2})

		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("- session_id: s1"))

		var raw map[string]any
		Expect(yaml.Unmarshal(data, &raw)).To(Succeed())
		re := domain.NewGoalFrontmatter(raw)

		sessions := re.MetricsSessions()
		Expect(sessions).To(HaveLen(2))
		Expect(sessions[0].SessionID).To(Equal("s1"))
		Expect(sessions[0].StartedAt.Time().Equal(start1.Time())).To(BeTrue())
		Expect(sessions[1].SessionID).To(Equal("s2"))
		Expect(sessions[1].StartedAt.Time().Equal(start2.Time())).To(BeTrue())
	})

	It("coexists with claude_session_id", func() {
		fm = domain.NewGoalFrontmatter(map[string]any{"claude_session_id": "owner"})
		fm.AppendMetricsSession(domain.MetricsSession{SessionID: "binder"})

		Expect(fm.ClaudeSessionID()).To(Equal("owner"))
		sessions := fm.MetricsSessions()
		Expect(sessions).To(HaveLen(1))
		Expect(sessions[0].SessionID).To(Equal("binder"))
	})
})
