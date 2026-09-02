// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"context"
	"os"
	"time"

	errors "github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	"github.com/bborbe/validation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("TaskFrontmatter", func() {
	var (
		ctx context.Context
		fm  domain.TaskFrontmatter
	)

	BeforeEach(func() {
		ctx = context.Background()
		fm = domain.NewTaskFrontmatter(nil)
	})

	Describe("Status", func() {
		It("returns empty for missing key", func() {
			Expect(fm.Status()).To(Equal(domain.TaskStatus("")))
		})

		It("returns canonical status for known value", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"status": "todo"})
			Expect(fm.Status()).To(Equal(domain.TaskStatusNext))
		})

		It("normalizes alias 'done' to completed", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"status": "done"})
			Expect(fm.Status()).To(Equal(domain.TaskStatusCompleted))
		})

		It("normalizes alias 'current' to in_progress", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"status": "current"})
			Expect(fm.Status()).To(Equal(domain.TaskStatusInProgress))
		})

		It("returns canonical 'next' unchanged", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"status": "next"})
			Expect(fm.Status()).To(Equal(domain.TaskStatusNext))
		})

		It("normalizes alias 'deferred' to hold", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"status": "deferred"})
			Expect(fm.Status()).To(Equal(domain.TaskStatusHold))
		})

		It("returns empty for unknown value", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"status": "invalid"})
			Expect(fm.Status()).To(Equal(domain.TaskStatus("")))
		})
	})

	Describe("SetStatus", func() {
		It("stores valid status", func() {
			Expect(fm.SetStatus(domain.TaskStatusInProgress)).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.TaskStatusInProgress))
		})

		It("returns error for invalid status", func() {
			Expect(fm.SetStatus(domain.TaskStatus("garbage"))).NotTo(BeNil())
		})
	})

	Describe("SetStatus close-out guard", func() {
		It("rejects aborted without aborted_reason and gate_successor and leaves the frontmatter unchanged", func() {
			err := fm.SetStatus(domain.TaskStatusAborted)
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, validation.Error)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("missing close-out field(s) aborted_reason, gate_successor"))
			Expect(fm.Status()).To(Equal(domain.TaskStatus("")))
		})

		It("accepts completed without close-out fields", func() {
			Expect(fm.SetStatus(domain.TaskStatusCompleted)).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.TaskStatusCompleted))
			Expect(fm.GetString("aborted_reason")).To(Equal(""))
			Expect(fm.GetString("gate_successor")).To(Equal(""))
		})

		It("accepts aborted when aborted_reason and gate_successor are both present", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"aborted_reason": "no longer needed", "gate_successor": "none"})
			Expect(fm.SetStatus(domain.TaskStatusAborted)).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.TaskStatusAborted))
		})

		It("accepts completed when aborted_reason and gate_successor are both present", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"aborted_reason": "all criteria met", "gate_successor": "none"})
			Expect(fm.SetStatus(domain.TaskStatusCompleted)).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.TaskStatusCompleted))
		})

		It("rejects aborted when only aborted_reason is present", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"aborted_reason": "no longer needed"})
			err := fm.SetStatus(domain.TaskStatusAborted)
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, validation.Error)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("missing close-out field(s) gate_successor"))
			Expect(err.Error()).NotTo(ContainSubstring("missing close-out field(s) aborted_reason, gate_successor"))
		})

		It("rejects aborted when only gate_successor is present", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"gate_successor": "none"})
			err := fm.SetStatus(domain.TaskStatusAborted)
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, validation.Error)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("missing close-out field(s) aborted_reason"))
		})

		It("treats whitespace-only close-out fields as missing", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"aborted_reason": "   ", "gate_successor": "none"})
			err := fm.SetStatus(domain.TaskStatusAborted)
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, validation.Error)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("aborted_reason"))
		})

		It("does not require close-out fields for non-close-out statuses", func() {
			for _, status := range []domain.TaskStatus{
				domain.TaskStatusNext,
				domain.TaskStatusInProgress,
				domain.TaskStatusBacklog,
				domain.TaskStatusHold,
			} {
				fm = domain.NewTaskFrontmatter(nil)
				Expect(fm.SetStatus(status)).To(Succeed())
				Expect(fm.Status()).To(Equal(status))
			}
		})
	})

	Describe("Priority", func() {
		It("returns 0 for missing key", func() {
			Expect(fm.Priority()).To(Equal(domain.Priority(0)))
		})

		It("returns priority for int value", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"priority": 3})
			Expect(fm.Priority()).To(Equal(domain.Priority(3)))
		})

		It("returns priority for string int value", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"priority": "5"})
			Expect(fm.Priority()).To(Equal(domain.Priority(5)))
		})

		It("returns 0 for non-numeric string", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"priority": "medium"})
			Expect(fm.Priority()).To(Equal(domain.Priority(0)))
		})
	})

	Describe("SetPriority", func() {
		It("stores valid priority", func() {
			Expect(fm.SetPriority(ctx, domain.Priority(2))).To(Succeed())
			Expect(fm.Priority()).To(Equal(domain.Priority(2)))
		})

		It("accepts zero priority", func() {
			Expect(fm.SetPriority(ctx, domain.Priority(0))).To(Succeed())
			Expect(fm.Priority()).To(Equal(domain.Priority(0)))
		})

		It("returns error for negative priority", func() {
			Expect(fm.SetPriority(ctx, domain.Priority(-1))).NotTo(BeNil())
		})
	})

	Describe("Flag", func() {
		DescribeTable("coerces the stored value",
			func(stored any, expected bool) {
				fm = domain.NewTaskFrontmatter(map[string]any{"flag": stored})
				Expect(fm.Flag()).To(Equal(expected))
			},
			Entry("bool true", true, true),
			Entry("bool false", false, false),
			Entry("string true", "true", true),
			Entry("string yes", "yes", true),
			Entry("string TRUE uppercase", "TRUE", true),
			Entry("string no", "no", false),
			Entry("string FALSE uppercase", "FALSE", false),
			Entry("string with surrounding whitespace", "  yes  ", true),
			Entry("unparseable string", "banana", false),
			Entry("nil value", nil, false),
		)

		It("returns false for a missing key", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"status": "todo"})
			Expect(fm.Flag()).To(BeFalse())
		})
	})

	Describe("SetFlag", func() {
		It("stores true", func() {
			Expect(fm.SetFlag(ctx, true)).To(Succeed())
			Expect(fm.Flag()).To(BeTrue())
		})

		It("stores false", func() {
			Expect(fm.SetFlag(ctx, false)).To(Succeed())
			Expect(fm.Flag()).To(BeFalse())
		})

		It("stores a real YAML bool, not the string", func() {
			Expect(fm.SetFlag(ctx, true)).To(Succeed())
			Expect(fm.Get("flag")).To(Equal(true))
		})
	})

	Describe("ClearFlag", func() {
		It("removes the key entirely", func() {
			Expect(fm.SetFlag(ctx, true)).To(Succeed())
			fm.ClearFlag()
			Expect(fm.Flag()).To(BeFalse())
			Expect(fm.Get("flag")).To(BeNil())
		})

		It("is a no-op when the key is absent", func() {
			fm.ClearFlag()
			Expect(fm.Get("flag")).To(BeNil())
		})
	})

	Describe("Goals", func() {
		It("returns nil for missing key", func() {
			Expect(fm.Goals()).To(BeNil())
		})

		It("returns goals for list value", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"goals": []any{"goal-a", "goal-b"}})
			Expect(fm.Goals()).To(Equal([]string{"goal-a", "goal-b"}))
		})
	})

	Describe("SetGoals", func() {
		It("stores goals", func() {
			fm.SetGoals([]string{"g1", "g2"})
			Expect(fm.Goals()).To(Equal([]string{"g1", "g2"}))
		})

		It("clears goals when nil", func() {
			fm.SetGoals([]string{"g1"})
			fm.SetGoals(nil)
			Expect(fm.Goals()).To(BeNil())
		})

		It("clears goals when empty slice", func() {
			fm.SetGoals([]string{"g1"})
			fm.SetGoals([]string{})
			Expect(fm.Goals()).To(BeNil())
		})
	})

	Describe("Tags", func() {
		It("returns nil for missing key", func() {
			Expect(fm.Tags()).To(BeNil())
		})

		It("returns tags for list value", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"tags": []any{"urgent", "backend"}})
			Expect(fm.Tags()).To(Equal([]string{"urgent", "backend"}))
		})
	})

	Describe("SetTags", func() {
		It("clears tags when nil", func() {
			fm.SetTags([]string{"t1"})
			fm.SetTags(nil)
			Expect(fm.Tags()).To(BeNil())
		})
	})

	Describe("Phase", func() {
		It("returns nil for missing key", func() {
			Expect(fm.Phase()).To(BeNil())
		})

		It("returns phase for known value", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"phase": "planning"})
			Expect(fm.Phase()).NotTo(BeNil())
			Expect(*fm.Phase()).To(Equal(domain.TaskPhasePlanning))
		})
	})

	Describe("SetPhase", func() {
		It("stores phase", func() {
			phase := domain.TaskPhaseInProgress
			fm.SetPhase(&phase)
			Expect(fm.Phase()).NotTo(BeNil())
			Expect(*fm.Phase()).To(Equal(domain.TaskPhaseInProgress))
		})

		It("clears phase when nil", func() {
			phase := domain.TaskPhasePlanning
			fm.SetPhase(&phase)
			fm.SetPhase(nil)
			Expect(fm.Phase()).To(BeNil())
		})
	})

	Describe("ClearClaudeSessionID", func() {
		It("removes the key when set", func() {
			fm.SetClaudeSessionID("session-uuid")
			fm.ClearClaudeSessionID()
			// Get returns nil only for absent keys; "" would mean key present with empty value.
			Expect(fm.Get("claude_session_id")).To(BeNil())
		})

		It("is a no-op when the key is absent", func() {
			Expect(func() { fm.ClearClaudeSessionID() }).NotTo(Panic())
			Expect(fm.Get("claude_session_id")).To(BeNil())
		})
	})

	Describe("DeferDate", func() {
		It("returns nil for missing key", func() {
			Expect(fm.DeferDate()).To(BeNil())
		})

		It("parses date-only string value", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"defer_date": "2026-03-01"})
			d := fm.DeferDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-03-01"))
		})

		It("handles time.Time value (YAML-parsed path)", func() {
			fm = domain.NewTaskFrontmatter(
				map[string]any{"defer_date": time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)},
			)
			d := fm.DeferDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-04-13"))
		})
	})

	Describe("PlannedDate", func() {
		It("returns nil for missing key", func() {
			Expect(fm.PlannedDate()).To(BeNil())
		})

		It("parses date-only string value", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"planned_date": "2026-05-01"})
			d := fm.PlannedDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-05-01"))
		})

		It("handles time.Time value (YAML-parsed path)", func() {
			fm = domain.NewTaskFrontmatter(
				map[string]any{"planned_date": time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
			)
			d := fm.PlannedDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-05-01"))
		})
	})

	Describe("DueDate", func() {
		It("returns nil for missing key", func() {
			Expect(fm.DueDate()).To(BeNil())
		})

		It("parses date-only string value", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"due_date": "2026-06-15"})
			d := fm.DueDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-06-15"))
		})

		It("handles time.Time value (YAML-parsed path)", func() {
			fm = domain.NewTaskFrontmatter(
				map[string]any{"due_date": time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)},
			)
			d := fm.DueDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-06-15"))
		})
	})

	Describe("LastCompleted", func() {
		It("returns empty string for missing key", func() {
			Expect(fm.LastCompleted()).To(Equal(""))
		})

		It("formats time.Time midnight-UTC as YYYY-MM-DD (regression guard)", func() {
			fm = domain.NewTaskFrontmatter(
				map[string]any{"last_completed": time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)},
			)
			Expect(fm.LastCompleted()).To(Equal("2026-03-08"))
			Expect(fm.LastCompleted()).NotTo(ContainSubstring("00:00:00 +0000 UTC"))
		})

		It("parses string date value", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"last_completed": "2026-03-08"})
			Expect(fm.LastCompleted()).To(Equal("2026-03-08"))
		})

		It("formats datetime with non-zero time as RFC3339", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"last_completed": "2026-03-08T12:30:00Z"})
			Expect(fm.LastCompleted()).To(Equal("2026-03-08T12:30:00Z"))
		})

		It("reads from last_completed_date when both keys present (prefers canonical)", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{
				"last_completed_date": "2026-04-01",
				"last_completed":      "2026-03-08",
			})
			Expect(fm.LastCompleted()).To(Equal("2026-04-01"))
		})
	})

	Describe("LastCompletedDate", func() {
		It("returns nil when both keys absent", func() {
			Expect(fm.LastCompletedDate()).To(BeNil())
		})

		It("reads last_completed_date when present", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"last_completed_date": "2026-04-01"})
			d := fm.LastCompletedDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-04-01"))
		})

		It("falls back to last_completed when only legacy key present", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"last_completed": "2026-03-08"})
			d := fm.LastCompletedDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-03-08"))
		})

		It("prefers last_completed_date when both keys present", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{
				"last_completed_date": "2026-04-01",
				"last_completed":      "2026-03-08",
			})
			d := fm.LastCompletedDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-04-01"))
		})
	})

	Describe("SetLastCompletedDate", func() {
		It("writes both last_completed_date and last_completed (dual-write)", func() {
			t := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
			d := libtime.DateOrDateTime(t)
			fm.SetLastCompletedDate(&d)
			Expect(fm.GetField("last_completed_date")).To(Equal("2026-05-01"))
			Expect(fm.GetField("last_completed")).To(Equal("2026-05-01"))
		})

		It("deletes both keys when nil", func() {
			t := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
			d := libtime.DateOrDateTime(t)
			fm.SetLastCompletedDate(&d)
			fm.SetLastCompletedDate(nil)
			Expect(fm.LastCompletedDate()).To(BeNil())
			Expect(fm.LastCompleted()).To(Equal(""))
		})
	})

	Describe("SetLastCompleted (compat)", func() {
		It("dual-writes both keys via compat setter", func() {
			fm.SetLastCompleted("2026-01-15")
			Expect(fm.GetField("last_completed")).To(Equal("2026-01-15"))
			Expect(fm.GetField("last_completed_date")).To(Equal("2026-01-15"))
		})

		It("clears both keys on empty string", func() {
			fm.SetLastCompleted("2026-01-15")
			fm.SetLastCompleted("")
			Expect(fm.LastCompletedDate()).To(BeNil())
			Expect(fm.LastCompleted()).To(Equal(""))
		})

		It("stores raw string for unparseable value (fallback path)", func() {
			fm.SetLastCompleted("not-a-date!!!")
			// GetField routes through LastCompleted() which parses as date; use GetString for raw access
			Expect(fm.GetString("last_completed")).To(Equal("not-a-date!!!"))
			Expect(fm.GetString("last_completed_date")).To(Equal("not-a-date!!!"))
		})
	})

	Describe("CompletedDate", func() {
		It("returns nil for missing key", func() {
			Expect(fm.CompletedDate()).To(BeNil())
		})

		It("returns non-nil *DateOrDateTime for time.Time midnight-UTC value", func() {
			fm = domain.NewTaskFrontmatter(
				map[string]any{"completed_date": time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)},
			)
			d := fm.CompletedDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-03-09"))
		})

		It("returns non-nil *DateOrDateTime for date-only string", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"completed_date": "2026-03-09"})
			d := fm.CompletedDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2026-03-09"))
		})

		It("returns non-nil *DateOrDateTime for RFC3339 string", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"completed_date": "2026-03-09T12:30:00Z"})
			d := fm.CompletedDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format(time.RFC3339)).To(Equal("2026-03-09T12:30:00Z"))
		})
	})

	Describe("SetCompletedDate", func() {
		It("deletes key when nil", func() {
			t := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
			d := libtime.DateOrDateTime(t)
			fm.SetCompletedDate(&d)
			fm.SetCompletedDate(nil)
			Expect(fm.CompletedDate()).To(BeNil())
		})

		It("stores value and retrieves it", func() {
			t := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
			d := libtime.DateOrDateTime(t)
			fm.SetCompletedDate(&d)
			result := fm.CompletedDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2026-03-09"))
		})

		It("round-trips date-only value as YYYY-MM-DD", func() {
			t := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
			d := libtime.DateOrDateTime(t)
			fm.SetCompletedDate(&d)
			Expect(fm.GetField("completed_date")).To(Equal("2026-03-09"))
		})

		It("round-trips RFC3339 value preserving timezone", func() {
			t := time.Date(2026, 3, 9, 12, 30, 0, 0, time.UTC)
			d := libtime.DateOrDateTime(t)
			fm.SetCompletedDate(&d)
			Expect(fm.GetField("completed_date")).To(Equal("2026-03-09T12:30:00Z"))
		})
	})

	Describe("CreatedDate", func() {
		It("returns nil when key absent", func() {
			Expect(fm.CreatedDate()).To(BeNil())
		})

		It("round-trips date-only value", func() {
			t := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
			d := libtime.DateOrDateTime(t)
			fm.SetCreatedDate(&d)
			result := fm.CreatedDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2026-01-15"))
		})

		It("round-trips RFC3339 value", func() {
			t := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
			d := libtime.DateOrDateTime(t)
			fm.SetCreatedDate(&d)
			Expect(fm.GetField("created_date")).To(Equal("2026-01-15T09:00:00Z"))
		})
	})

	Describe("SetCreatedDate", func() {
		It("deletes key when nil", func() {
			t := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
			d := libtime.DateOrDateTime(t)
			fm.SetCreatedDate(&d)
			fm.SetCreatedDate(nil)
			Expect(fm.CreatedDate()).To(BeNil())
		})
	})

	Describe("SetDeferDate", func() {
		It("stores a date", func() {
			t := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
			d := libtime.DateOrDateTime(t)
			fm.SetDeferDate(&d)
			Expect(fm.DeferDate()).NotTo(BeNil())
		})

		It("clears date when nil", func() {
			t := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
			d := libtime.DateOrDateTime(t)
			fm.SetDeferDate(&d)
			fm.SetDeferDate(nil)
			Expect(fm.DeferDate()).To(BeNil())
		})
	})

	Describe("GetField", func() {
		It("returns status", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"status": "todo"})
			Expect(fm.GetField("status")).To(Equal("next"))
		})

		It("returns goals as comma-separated", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"goals": []any{"g1", "g2"}})
			Expect(fm.GetField("goals")).To(Equal("g1,g2"))
		})

		It("returns empty for missing key", func() {
			Expect(fm.GetField("status")).To(Equal(""))
		})

		It("returns raw value for unknown key", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"custom_field": "custom_value"})
			Expect(fm.GetField("custom_field")).To(Equal("custom_value"))
		})

		It("returns empty for an absent flag", func() {
			Expect(fm.GetField("flag")).To(Equal(""))
		})

		It("returns true for flag true", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"flag": true})
			Expect(fm.GetField("flag")).To(Equal("true"))
		})

		It("returns false for flag false", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"flag": false})
			Expect(fm.GetField("flag")).To(Equal("false"))
		})
	})

	Describe("SetField", func() {
		It("sets status", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"aborted_reason": "test reason", "gate_successor": "none"})
			Expect(fm.SetField(ctx, "status", "completed")).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.TaskStatusCompleted))
		})

		It("returns error for invalid status", func() {
			Expect(fm.SetField(ctx, "status", "garbage")).NotTo(BeNil())
		})

		It("sets goals from comma-separated string", func() {
			Expect(fm.SetField(ctx, "goals", "g1,g2")).To(Succeed())
			Expect(fm.Goals()).To(Equal([]string{"g1", "g2"}))
		})

		It("clears goals on empty string", func() {
			fm.SetGoals([]string{"old"})
			Expect(fm.SetField(ctx, "goals", "")).To(Succeed())
			Expect(fm.Goals()).To(BeNil())
		})

		It("sets phase", func() {
			Expect(fm.SetField(ctx, "phase", "planning")).To(Succeed())
			Expect(fm.Phase()).NotTo(BeNil())
			Expect(*fm.Phase()).To(Equal(domain.TaskPhasePlanning))
		})

		It("returns error for invalid phase", func() {
			Expect(fm.SetField(ctx, "phase", "invalid_phase_value")).NotTo(BeNil())
		})

		It("stores unknown field without error", func() {
			Expect(fm.SetField(ctx, "custom_field", "custom_value")).To(Succeed())
			Expect(fm.GetField("custom_field")).To(Equal("custom_value"))
		})

		It("sets flag true from the string 'yes'", func() {
			Expect(fm.SetField(ctx, "flag", "yes")).To(Succeed())
			Expect(fm.Flag()).To(BeTrue())
		})

		It("sets flag false from the string 'FALSE'", func() {
			Expect(fm.SetField(ctx, "flag", "FALSE")).To(Succeed())
			Expect(fm.Flag()).To(BeFalse())
		})

		It("returns a validation error for an invalid flag value", func() {
			err := fm.SetField(ctx, "flag", "banana")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, validation.Error)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("banana"))
			Expect(err.Error()).To(ContainSubstring("true"))
			Expect(err.Error()).To(ContainSubstring("false"))
		})

		It("clears flag on empty string", func() {
			Expect(fm.SetField(ctx, "flag", "true")).To(Succeed())
			Expect(fm.SetField(ctx, "flag", "")).To(Succeed())
			Expect(fm.Get("flag")).To(BeNil())
		})
	})

	Describe("ClearField", func() {
		It("clears a known field", func() {
			Expect(fm.SetField(ctx, "assignee", "alice")).To(Succeed())
			fm.ClearField("assignee")
			Expect(fm.Assignee()).To(Equal(""))
		})

		It("clears an unknown field", func() {
			Expect(fm.SetField(ctx, "custom_field", "value")).To(Succeed())
			fm.ClearField("custom_field")
			Expect(fm.GetField("custom_field")).To(Equal(""))
		})
	})

	Describe("unknown field round-trip", func() {
		It("preserves unknown fields through SetField/GetField", func() {
			Expect(fm.SetField(ctx, "my_custom_tag", "special_value")).To(Succeed())
			Expect(fm.GetField("my_custom_tag")).To(Equal("special_value"))
		})

		It("preserves unknown fields stored in constructor map", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"unknown_field": "preserved"})
			Expect(fm.GetField("unknown_field")).To(Equal("preserved"))
		})
	})
})

var _ = Describe("TaskFrontmatter SetField alias normalization", func() {
	var ctx context.Context
	var fm domain.TaskFrontmatter

	BeforeEach(func() {
		ctx = context.Background()
		fm = domain.NewTaskFrontmatter(nil)
	})

	Context("status field", func() {
		It("normalises alias 'todo' to canonical 'next' on disk", func() {
			Expect(fm.SetField(ctx, "status", "todo")).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.TaskStatusNext))
		})

		It("accepts canonical 'next' verbatim", func() {
			Expect(fm.SetField(ctx, "status", "next")).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.TaskStatusNext))
		})

		It("rejects an unknown status value with validation.Error", func() {
			err := fm.SetField(ctx, "status", "banana")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, validation.Error)).To(BeTrue())
		})
	})

	Context("phase field", func() {
		It("normalises alias 'in_progress' to canonical 'execution' on disk", func() {
			Expect(fm.SetField(ctx, "phase", "in_progress")).To(Succeed())
			Expect(fm.Phase()).NotTo(BeNil())
			Expect(*fm.Phase()).To(Equal(domain.TaskPhaseExecution))
		})

		It("accepts canonical 'execution' verbatim", func() {
			Expect(fm.SetField(ctx, "phase", "execution")).To(Succeed())
			Expect(fm.Phase()).NotTo(BeNil())
			Expect(*fm.Phase()).To(Equal(domain.TaskPhaseExecution))
		})

		It("clears the phase on empty value", func() {
			Expect(fm.SetField(ctx, "phase", "execution")).To(Succeed())
			Expect(fm.SetField(ctx, "phase", "")).To(Succeed())
			Expect(fm.Phase()).To(BeNil())
		})

		It("rejects an unknown phase value with validation.Error", func() {
			err := fm.SetField(ctx, "phase", "banana")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, validation.Error)).To(BeTrue())
		})
	})
})

var _ = Describe("TypedDateRoundTrip", func() {
	var (
		fixedTime time.Time
		d         *libtime.DateOrDateTime
		fm        domain.TaskFrontmatter
	)

	BeforeEach(func() {
		fixedTime = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
		dv := libtime.DateOrDateTime(fixedTime)
		d = dv.Ptr()
		fm = domain.NewTaskFrontmatter(nil)
	})

	It("SetDeferDate then DeferDate returns equal value", func() {
		fm.SetDeferDate(d)
		got := fm.DeferDate()
		Expect(got).NotTo(BeNil())
		Expect(got.Time()).To(Equal(fixedTime))
	})

	It("SetPlannedDate then PlannedDate returns equal value", func() {
		fm.SetPlannedDate(d)
		got := fm.PlannedDate()
		Expect(got).NotTo(BeNil())
		Expect(got.Time()).To(Equal(fixedTime))
	})

	It("SetDueDate then DueDate returns equal value", func() {
		fm.SetDueDate(d)
		got := fm.DueDate()
		Expect(got).NotTo(BeNil())
		Expect(got.Time()).To(Equal(fixedTime))
	})

	It("SetCompletedDate then CompletedDate returns equal value", func() {
		fm.SetCompletedDate(d)
		got := fm.CompletedDate()
		Expect(got).NotTo(BeNil())
		Expect(got.Time()).To(Equal(fixedTime))
	})

	It("SetCreatedDate then CreatedDate returns equal value", func() {
		fm.SetCreatedDate(d)
		got := fm.CreatedDate()
		Expect(got).NotTo(BeNil())
		Expect(got.Time()).To(Equal(fixedTime))
	})

	It(
		"SetLastCompletedDate then LastCompletedDate returns equal value and dual-writes last_completed",
		func() {
			fm.SetLastCompletedDate(d)
			got := fm.LastCompletedDate()
			Expect(got).NotTo(BeNil())
			Expect(got.Time()).To(Equal(fixedTime))
			raw := fm.RawMap()
			Expect(raw["last_completed"]).NotTo(BeNil())
		},
	)
})

var _ = Describe("TaskFrontmatterGoldenYAML", func() {
	It("serializes all date fields as YYYY-MM-DD", func() {
		fixedTime := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
		dv := libtime.DateOrDateTime(fixedTime)
		d := dv.Ptr()

		fm := domain.NewTaskFrontmatter(nil)
		Expect(fm.SetStatus(domain.TaskStatusNext)).To(Succeed())
		fm.SetTaskIdentifier("TASK-001")
		fm.SetDeferDate(d)
		fm.SetPlannedDate(d)
		fm.SetDueDate(d)
		fm.SetCompletedDate(d)
		fm.SetCreatedDate(d)
		fm.SetLastCompletedDate(d)

		got, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())

		want, err := os.ReadFile("testdata/task_frontmatter_golden.yaml")
		Expect(err).NotTo(HaveOccurred())

		Expect(string(got)).To(Equal(string(want)))
	})
})

var _ = Describe("TaskFrontmatter flag YAML round-trip", func() {
	var ctx context.Context
	var fm domain.TaskFrontmatter

	BeforeEach(func() {
		ctx = context.Background()
		fm = domain.NewTaskFrontmatter(nil)
	})

	It("writes flag true alongside pre-existing keys without touching them", func() {
		fm = domain.NewTaskFrontmatter(map[string]any{
			"status":       "in_progress",
			"priority":     3,
			"planned_date": "2025-03-15",
			"themes":       []any{"t1"},
			"custom_key":   "custom-value",
		})

		Expect(fm.SetField(ctx, "flag", "true")).To(Succeed())

		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		yamlText := string(data)

		Expect(yamlText).To(ContainSubstring("flag: true"))
		for _, preexisting := range []string{"status:", "priority:", "planned_date:", "themes:", "custom_key:"} {
			Expect(yamlText).To(ContainSubstring(preexisting))
		}
	})

	It("never emits a flag key for a task that was never flagged", func() {
		fm = domain.NewTaskFrontmatter(map[string]any{"status": "todo"})
		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("flag:"))
	})

	It("emits no flag key after clear", func() {
		Expect(fm.SetField(ctx, "flag", "true")).To(Succeed())
		fm.ClearFlag()
		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("flag:"))
	})

	It("round-trips flag true through unmarshal", func() {
		Expect(fm.SetField(ctx, "flag", "true")).To(Succeed())
		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())

		var raw map[string]any
		Expect(yaml.Unmarshal(data, &raw)).To(Succeed())
		re := domain.NewTaskFrontmatter(raw)
		Expect(re.Flag()).To(BeTrue())
	})

	It("round-trips explicit false as flag: false", func() {
		Expect(fm.SetFlag(ctx, false)).To(Succeed())
		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("flag: false"))

		var raw map[string]any
		Expect(yaml.Unmarshal(data, &raw)).To(Succeed())
		Expect(domain.NewTaskFrontmatter(raw).Flag()).To(BeFalse())
	})

	It("preserves an unparseable flag value on write until explicitly changed", func() {
		fm = domain.NewTaskFrontmatter(map[string]any{"flag": "banana", "status": "todo"})
		Expect(fm.Flag()).To(BeFalse())

		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("flag: banana"))
	})
})
