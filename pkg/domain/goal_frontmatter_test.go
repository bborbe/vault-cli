// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"context"
	"time"

	libtime "github.com/bborbe/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("GoalFrontmatter", func() {
	var (
		ctx context.Context
		fm  domain.GoalFrontmatter
	)

	BeforeEach(func() {
		ctx = context.Background()
		fm = domain.NewGoalFrontmatter(nil)
	})

	Describe("Status", func() {
		It("returns empty for missing key", func() {
			Expect(fm.Status()).To(Equal(domain.GoalStatus("")))
		})

		It("returns canonical status for known value", func() {
			fm = domain.NewGoalFrontmatter(map[string]any{"status": "active"})
			Expect(fm.Status()).To(Equal(domain.GoalStatusActive))
		})
	})

	Describe("SetStatus", func() {
		It("stores valid status", func() {
			Expect(fm.SetStatus(domain.GoalStatusActive)).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.GoalStatusActive))
		})

		It("returns error for invalid status", func() {
			Expect(fm.SetStatus(domain.GoalStatus("garbage"))).NotTo(BeNil())
		})
	})

	Describe("SetField / GetField - unknown field round-trip", func() {
		It("stores and retrieves unknown fields without error", func() {
			Expect(fm.SetField(ctx, "custom_note", "hello")).To(Succeed())
			Expect(fm.GetField("custom_note")).To(Equal("hello"))
		})

		It("does not error on set of unknown field", func() {
			Expect(fm.SetField(ctx, "unknown_key", "value")).To(Succeed())
		})

		It("returns empty string for unset unknown field", func() {
			Expect(fm.GetField("unknown_key")).To(Equal(""))
		})
	})

	Describe("SetField status", func() {
		It("sets status to active via SetField", func() {
			Expect(fm.SetField(ctx, "status", "active")).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.GoalStatusActive))
		})

		It("returns error for invalid status via SetField", func() {
			Expect(fm.SetField(ctx, "status", "invalid")).NotTo(BeNil())
		})
	})

	Describe("Tags", func() {
		It("returns nil for missing tags", func() {
			Expect(fm.Tags()).To(BeNil())
		})

		It("returns tags as string slice", func() {
			fm = domain.NewGoalFrontmatter(map[string]any{"tags": []any{"urgent", "q1"}})
			Expect(fm.Tags()).To(Equal([]string{"urgent", "q1"}))
		})

		It("SetField joins tags with comma from SetField and splits on GetField", func() {
			Expect(fm.SetField(ctx, "tags", "urgent,q1")).To(Succeed())
			Expect(fm.GetField("tags")).To(Equal("urgent,q1"))
			Expect(fm.Tags()).To(Equal([]string{"urgent", "q1"}))
		})
	})

	Describe("StartDate", func() {
		It("returns nil for missing start_date", func() {
			Expect(fm.StartDate()).To(BeNil())
		})

		It("parses a YAML date literal", func() {
			fm = domain.NewGoalFrontmatter(
				map[string]any{"start_date": time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)},
			)
			result := fm.StartDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2025-01-15"))
		})

		It("parses a string value (hand-authored date)", func() {
			fm = domain.NewGoalFrontmatter(map[string]any{"start_date": "2025-01-15"})
			result := fm.StartDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2025-01-15"))
		})

		It("parses an RFC3339 string value", func() {
			fm = domain.NewGoalFrontmatter(
				map[string]any{"start_date": "2025-01-15T14:30:00+01:00"},
			)
			result := fm.StartDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Format(time.RFC3339)).To(Equal("2025-01-15T14:30:00+01:00"))
		})
	})

	Describe("SetStartDate", func() {
		It("returns nil after SetStartDate(nil)", func() {
			d := libtime.DateOrDateTime(time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
			fm.SetStartDate(&d)
			fm.SetStartDate(nil)
			Expect(fm.StartDate()).To(BeNil())
		})

		It("round-trips a date-only value as YYYY-MM-DD", func() {
			d := libtime.DateOrDateTime(time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
			fm.SetStartDate(&d)
			result := fm.StartDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2026-03-17"))
			Expect(fm.GetField("start_date")).To(Equal("2026-03-17"))
		})
	})

	Describe("SetField / GetField - start_date", func() {
		It("round-trips a date-only value via SetField/GetField", func() {
			Expect(fm.SetField(ctx, "start_date", "2025-01-15")).To(Succeed())
			Expect(fm.GetField("start_date")).To(Equal("2025-01-15"))
		})

		It("returns error for invalid date format", func() {
			Expect(fm.SetField(ctx, "start_date", "not-a-date")).NotTo(Succeed())
		})

		It("clears start_date on empty value", func() {
			Expect(fm.SetField(ctx, "start_date", "2025-01-15")).To(Succeed())
			Expect(fm.SetField(ctx, "start_date", "")).To(Succeed())
			Expect(fm.GetField("start_date")).To(Equal(""))
		})
	})

	Describe("TargetDate", func() {
		It("returns nil for missing target_date", func() {
			Expect(fm.TargetDate()).To(BeNil())
		})

		It("parses a YAML date literal", func() {
			fm = domain.NewGoalFrontmatter(
				map[string]any{"target_date": time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)},
			)
			result := fm.TargetDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2025-12-31"))
		})

		It("parses a string value (hand-authored date)", func() {
			fm = domain.NewGoalFrontmatter(map[string]any{"target_date": "2025-12-31"})
			result := fm.TargetDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2025-12-31"))
		})

		It("parses an RFC3339 string value", func() {
			fm = domain.NewGoalFrontmatter(
				map[string]any{"target_date": "2025-12-31T23:59:59+05:30"},
			)
			result := fm.TargetDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Format(time.RFC3339)).To(Equal("2025-12-31T23:59:59+05:30"))
		})
	})

	Describe("SetTargetDate", func() {
		It("returns nil after SetTargetDate(nil)", func() {
			d := libtime.DateOrDateTime(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))
			fm.SetTargetDate(&d)
			fm.SetTargetDate(nil)
			Expect(fm.TargetDate()).To(BeNil())
		})

		It("round-trips a date-only value as YYYY-MM-DD", func() {
			d := libtime.DateOrDateTime(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))
			fm.SetTargetDate(&d)
			result := fm.TargetDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2025-12-31"))
			Expect(fm.GetField("target_date")).To(Equal("2025-12-31"))
		})
	})

	Describe("SetField / GetField - target_date", func() {
		It("round-trips a date-only value via SetField/GetField", func() {
			Expect(fm.SetField(ctx, "target_date", "2025-12-31")).To(Succeed())
			Expect(fm.GetField("target_date")).To(Equal("2025-12-31"))
		})

		It("returns error for invalid date format", func() {
			Expect(fm.SetField(ctx, "target_date", "not-a-date")).NotTo(Succeed())
		})

		It("clears target_date on empty value", func() {
			Expect(fm.SetField(ctx, "target_date", "2025-12-31")).To(Succeed())
			Expect(fm.SetField(ctx, "target_date", "")).To(Succeed())
			Expect(fm.GetField("target_date")).To(Equal(""))
		})
	})

	Describe("Priority validation", func() {
		It("rejects negative priority via SetPriority", func() {
			Expect(fm.SetPriority(ctx, domain.Priority(-1))).NotTo(BeNil())
		})

		It("accepts zero priority", func() {
			Expect(fm.SetPriority(ctx, domain.Priority(0))).To(Succeed())
		})

		It("accepts positive priority", func() {
			Expect(fm.SetPriority(ctx, domain.Priority(3))).To(Succeed())
			Expect(fm.Priority()).To(Equal(domain.Priority(3)))
		})
	})

	Describe("ClearField", func() {
		It("clears an existing field", func() {
			Expect(fm.SetField(ctx, "status", "active")).To(Succeed())
			fm.ClearField("status")
			Expect(fm.GetField("status")).To(Equal(""))
		})

		It("does not error on clearing unknown field", func() {
			Expect(func() { fm.ClearField("nonexistent") }).NotTo(Panic())
			Expect(fm.GetField("nonexistent")).To(Equal(""))
		})
	})

	Describe("ClaudeSessionID", func() {
		It("returns empty for missing key", func() {
			Expect(fm.ClaudeSessionID()).To(Equal(""))
		})

		It("returns value when set", func() {
			fm = domain.NewGoalFrontmatter(map[string]any{"claude_session_id": "sess-abc"})
			Expect(fm.ClaudeSessionID()).To(Equal("sess-abc"))
		})
	})

	Describe("SetClaudeSessionID", func() {
		It("stores the value", func() {
			fm.SetClaudeSessionID("sess-xyz")
			Expect(fm.ClaudeSessionID()).To(Equal("sess-xyz"))
		})
	})

	Describe("ClearClaudeSessionID", func() {
		It("removes the key", func() {
			fm.SetClaudeSessionID("sess-abc")
			fm.ClearClaudeSessionID()
			Expect(fm.Get("claude_session_id")).To(BeNil())
		})
	})

	Describe("SetField / GetField - claude_session_id", func() {
		It("round-trips via SetField and GetField", func() {
			Expect(fm.SetField(ctx, "claude_session_id", "sess-abc")).To(Succeed())
			Expect(fm.GetField("claude_session_id")).To(Equal("sess-abc"))
			Expect(fm.ClaudeSessionID()).To(Equal("sess-abc"))
		})

		It("preserves unknown field alongside session id", func() {
			Expect(fm.SetField(ctx, "custom_note", "hello")).To(Succeed())
			Expect(fm.SetField(ctx, "claude_session_id", "sess-abc")).To(Succeed())
			Expect(fm.GetField("custom_note")).To(Equal("hello"))
			Expect(fm.GetField("claude_session_id")).To(Equal("sess-abc"))
		})
	})

	Describe("DeferDate", func() {
		It("returns nil for missing key", func() {
			Expect(fm.DeferDate()).To(BeNil())
		})

		It("parses string value", func() {
			fm = domain.NewGoalFrontmatter(map[string]any{"defer_date": "2026-04-13"})
			d := fm.DeferDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-04-13"))
		})

		It("handles time.Time value (YAML-parsed path)", func() {
			fm = domain.NewGoalFrontmatter(
				map[string]any{"defer_date": time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)},
			)
			d := fm.DeferDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-04-13"))
		})
	})
})
