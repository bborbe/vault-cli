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

var _ = Describe("TaskFrontmatter metrics", func() {
	var fm domain.TaskFrontmatter

	BeforeEach(func() {
		fm = domain.NewTaskFrontmatter(nil)
	})

	Describe("MetricsSessions", func() {
		It("preserves prior entries when appending", func() {
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
			fm = domain.NewTaskFrontmatter(map[string]any{
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

		DescribeTable("coerces each on-disk session element",
			func(sessionID any, startedAt any, wantSessionID string, wantLen int, wantZero bool) {
				fm = domain.NewTaskFrontmatter(map[string]any{
					"metrics_sessions": []any{
						map[string]any{"session_id": sessionID, "started_at": startedAt},
					},
				})
				sessions := fm.MetricsSessions()
				if wantLen == 0 {
					Expect(sessions).To(BeNil())
					return
				}
				Expect(sessions).To(HaveLen(wantLen))
				Expect(sessions[0].SessionID).To(Equal(wantSessionID))
				Expect(sessions[0].StartedAt.IsZero()).To(Equal(wantZero))
			},
			Entry("string session id with time.Time started_at",
				"s1", time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC), "s1", 1, false),
			Entry(
				"string session id with DateOrDateTime started_at",
				"s1",
				libtime.DateOrDateTime(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)),
				"s1",
				1,
				false,
			),
			Entry("string session id with RFC3339 string started_at",
				"s1", "2026-08-24T09:00:00Z", "s1", 1, false),
			Entry("non-string session id is stringified",
				123, "2026-08-24T09:00:00Z", "123", 1, false),
			Entry("empty session id is skipped",
				"", "2026-08-24T09:00:00Z", "", 0, false),
			Entry("missing session id is skipped",
				nil, "2026-08-24T09:00:00Z", "", 0, false),
			Entry("unparseable started_at is retained as zero time",
				"s1", "not-a-time", "s1", 1, true),
			Entry("empty string started_at is retained as zero time",
				"s1", "", "s1", 1, true),
			Entry("non-time started_at is retained as zero time",
				"s1", 42, "s1", 1, true),
		)

		It("treats malformed metrics values as absent", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"metrics_sessions": "not-a-list"})
			Expect(fm.MetricsSessions()).To(BeNil())

			fm = domain.NewTaskFrontmatter(map[string]any{
				"metrics_sessions": map[string]any{"session_id": "s1"},
			})
			Expect(fm.MetricsSessions()).To(BeNil())

			fm = domain.NewTaskFrontmatter(map[string]any{"metrics_sessions": []any{"not-a-map"}})
			Expect(fm.MetricsSessions()).To(BeNil())

			fm = domain.NewTaskFrontmatter(map[string]any{"metrics_interaction_count": "abc"})
			Expect(fm.MetricsInteractionCount()).To(BeNil())

			fm = domain.NewTaskFrontmatter(map[string]any{"metrics_interaction_count": true})
			Expect(fm.MetricsInteractionCount()).To(BeNil())
		})

		It("ignores an append with an empty session id", func() {
			fm.AppendMetricsSession(domain.MetricsSession{SessionID: ""})
			Expect(fm.MetricsSessions()).To(BeNil())
			Expect(fm.Get("metrics_sessions")).To(BeNil())
		})
	})

	Describe("MetricsInteractionCount", func() {
		It("unknown is never forged as zero", func() {
			Expect(fm.MetricsInteractionCount()).To(BeNil())

			fm.SetMetricsInteractionCount(0)
			count := fm.MetricsInteractionCount()
			Expect(count).NotTo(BeNil())
			Expect(*count).To(Equal(0))
		})

		DescribeTable("coerces stored count shapes",
			func(stored any, want int, wantNil bool) {
				fm = domain.NewTaskFrontmatter(map[string]any{"metrics_interaction_count": stored})
				got := fm.MetricsInteractionCount()
				if wantNil {
					Expect(got).To(BeNil())
					return
				}
				Expect(got).NotTo(BeNil())
				Expect(*got).To(Equal(want))
			},
			Entry("int", 7, 7, false),
			Entry("int64", int64(8), 8, false),
			Entry("float64", 9.0, 9, false),
			Entry("numeric string", "10", 10, false),
			Entry("non-numeric string", "abc", 0, true),
			Entry("bool", true, 0, true),
			Entry("nil", nil, 0, true),
		)
	})

	Describe("MetricsCycles", func() {
		It("preserves prior cycles when appending", func() {
			fm.AppendMetricsCycle(domain.MetricsCycle{
				StartedAt: libtime.DateOrDateTime(
					time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
				),
				CompletedAt: libtime.DateOrDateTime(
					time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC),
				),
				InteractionCount: 12,
			})
			fm.AppendMetricsCycle(domain.MetricsCycle{
				StartedAt: libtime.DateOrDateTime(
					time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC),
				),
				CompletedAt: libtime.DateOrDateTime(
					time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC),
				),
				InteractionCount: 3,
			})

			cycles := fm.MetricsCycles()
			Expect(cycles).To(HaveLen(2))
			Expect(cycles[0].InteractionCount).To(Equal(12))
			Expect(cycles[1].InteractionCount).To(Equal(3))
		})

		It("reads the on-disk generic shape and degrades leniently", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{
				"metrics_cycles": []any{
					map[string]any{
						"started_at":        "2026-08-01T00:00:00Z",
						"completed_at":      "2026-08-07T00:00:00Z",
						"interaction_count": "12",
					},
					map[string]any{
						"started_at":        "not-a-time",
						"completed_at":      42,
						"interaction_count": "junk",
					},
				},
			})

			cycles := fm.MetricsCycles()
			Expect(cycles).To(HaveLen(2))
			Expect(cycles[0].StartedAt.IsZero()).To(BeFalse())
			Expect(cycles[0].InteractionCount).To(Equal(12))
			Expect(cycles[1].StartedAt.IsZero()).To(BeTrue())
			Expect(cycles[1].CompletedAt.IsZero()).To(BeTrue())
			Expect(cycles[1].InteractionCount).To(Equal(0))
		})

		It("treats a non-list metrics_cycles value as absent", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"metrics_cycles": "not-a-list"})
			Expect(fm.MetricsCycles()).To(BeNil())

			fm = domain.NewTaskFrontmatter(map[string]any{"metrics_cycles": []any{"not-a-map"}})
			Expect(fm.MetricsCycles()).To(BeNil())
		})
	})

	Describe("YAML round-trip", func() {
		It("preserves all metrics values through the real serializer shape", func() {
			start1 := libtime.DateOrDateTime(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC))
			start2 := libtime.DateOrDateTime(time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC))
			fm.AppendMetricsSession(domain.MetricsSession{SessionID: "s1", StartedAt: start1})
			fm.AppendMetricsSession(domain.MetricsSession{SessionID: "s2", StartedAt: start2})

			completed := libtime.DateOrDateTime(time.Date(2026, 8, 24, 18, 14, 35, 0, time.UTC))
			fm.SetMetricsCompletedAt(&completed)
			fm.SetMetricsInteractionCount(7)

			cycleStart := libtime.DateOrDateTime(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
			cycleEnd := libtime.DateOrDateTime(time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC))
			fm.AppendMetricsCycle(domain.MetricsCycle{
				StartedAt:        cycleStart,
				CompletedAt:      cycleEnd,
				InteractionCount: 12,
			})

			data, err := yaml.Marshal(fm.RawMap())
			Expect(err).NotTo(HaveOccurred())

			// The serialized YAML carries every metrics key.
			yamlText := string(data)
			for _, key := range []string{
				"metrics_sessions:",
				"session_id:",
				"started_at:",
				"metrics_completed_at:",
				"metrics_interaction_count:",
				"metrics_cycles:",
				"completed_at:",
				"interaction_count:",
			} {
				Expect(yamlText).To(ContainSubstring(key))
			}

			// Unmarshal back through the same generic shape storage uses on read.
			var raw map[string]any
			Expect(yaml.Unmarshal(data, &raw)).To(Succeed())
			re := domain.NewTaskFrontmatter(raw)

			sessions := re.MetricsSessions()
			Expect(sessions).To(HaveLen(2))
			Expect(sessions[0].SessionID).To(Equal("s1"))
			Expect(sessions[0].StartedAt.Time().Equal(start1.Time())).To(BeTrue())
			Expect(sessions[1].SessionID).To(Equal("s2"))
			Expect(sessions[1].StartedAt.Time().Equal(start2.Time())).To(BeTrue())

			gotCompleted := re.MetricsCompletedAt()
			Expect(gotCompleted).NotTo(BeNil())
			Expect(gotCompleted.Time().Equal(completed.Time())).To(BeTrue())

			gotCount := re.MetricsInteractionCount()
			Expect(gotCount).NotTo(BeNil())
			Expect(*gotCount).To(Equal(7))

			cycles := re.MetricsCycles()
			Expect(cycles).To(HaveLen(1))
			Expect(cycles[0].StartedAt.Time().Equal(cycleStart.Time())).To(BeTrue())
			Expect(cycles[0].CompletedAt.Time().Equal(cycleEnd.Time())).To(BeTrue())
			Expect(cycles[0].InteractionCount).To(Equal(12))
		})
	})

	Describe("Clear metrics", func() {
		It("clearers delete the keys entirely", func() {
			d := libtime.DateOrDateTime(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC))

			fm.SetMetricsCompletedAt(&d)
			fm.ClearMetricsCompletedAt()
			Expect(fm.MetricsCompletedAt()).To(BeNil())
			Expect(fm.Get("metrics_completed_at")).To(BeNil())

			fm.AppendMetricsSession(domain.MetricsSession{SessionID: "s1", StartedAt: d})
			fm.ClearMetricsSessions()
			Expect(fm.MetricsSessions()).To(BeNil())
			Expect(fm.Get("metrics_sessions")).To(BeNil())

			fm.SetMetricsInteractionCount(3)
			fm.ClearMetricsInteractionCount()
			Expect(fm.MetricsInteractionCount()).To(BeNil())
			Expect(fm.Get("metrics_interaction_count")).To(BeNil())
		})

		It("SetMetricsCompletedAt(nil) deletes the key", func() {
			d := libtime.DateOrDateTime(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC))
			fm.SetMetricsCompletedAt(&d)
			Expect(fm.MetricsCompletedAt()).NotTo(BeNil())

			fm.SetMetricsCompletedAt(nil)
			Expect(fm.MetricsCompletedAt()).To(BeNil())
			Expect(fm.Get("metrics_completed_at")).To(BeNil())
		})
	})
})
