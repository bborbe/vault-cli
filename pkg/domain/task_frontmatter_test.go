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
	})

	Describe("SetField", func() {
		It("sets status", func() {
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
