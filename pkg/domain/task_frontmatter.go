// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	"github.com/bborbe/validation"
)

// TaskFrontmatter holds the YAML frontmatter for a Task.
// It uses FrontmatterMap as its backing store so unknown fields survive round-trips.
type TaskFrontmatter struct {
	FrontmatterMap
}

// NewTaskFrontmatter constructs a TaskFrontmatter from a raw map.
func NewTaskFrontmatter(data map[string]any) TaskFrontmatter {
	return TaskFrontmatter{FrontmatterMap: NewFrontmatterMap(data)}
}

// Status reads "status" key and applies NormalizeTaskStatus.
// Returns "" (empty) if value is absent or unrecognized.
func (f TaskFrontmatter) Status() TaskStatus {
	raw := f.GetString("status")
	normalized, ok := NormalizeTaskStatus(raw)
	if !ok {
		return ""
	}
	return normalized
}

// PageType reads "page_type" key, returns string.
func (f TaskFrontmatter) PageType() string { return f.GetString("page_type") }

// Goals reads "goals" key via GetStringSlice.
func (f TaskFrontmatter) Goals() []string { return f.GetStringSlice("goals") }

// Priority reads "priority" key as int. Returns 0 on missing or parse failure.
func (f TaskFrontmatter) Priority() Priority {
	v := f.Get("priority")
	if v == nil {
		return 0
	}
	switch p := v.(type) {
	case int:
		return Priority(p)
	case int64:
		return Priority(p)
	case float64:
		return Priority(int(p))
	case string:
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0
		}
		return Priority(n)
	default:
		return 0
	}
}

// Assignee reads "assignee" key as string.
func (f TaskFrontmatter) Assignee() string { return f.GetString("assignee") }

// DeferDate reads "defer_date" key as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (f TaskFrontmatter) DeferDate() *libtime.DateOrDateTime {
	t := f.GetTime("defer_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// Tags reads "tags" key via GetStringSlice.
func (f TaskFrontmatter) Tags() []string { return f.GetStringSlice("tags") }

// Phase reads "phase" key as string, returns *TaskPhase.
func (f TaskFrontmatter) Phase() *TaskPhase {
	raw := f.GetString("phase")
	if raw == "" {
		return nil
	}
	p := TaskPhase(raw)
	return &p
}

// ClaudeSessionID reads "claude_session_id" key as string.
func (f TaskFrontmatter) ClaudeSessionID() string { return f.GetString("claude_session_id") }

// Recurring reads "recurring" key as string.
func (f TaskFrontmatter) Recurring() string { return f.GetString("recurring") }

// LastCompletedDate reads "last_completed_date" as *libtime.DateOrDateTime.
// Falls back to the legacy "last_completed" key for backward compatibility.
// Returns nil on missing or unparseable value.
func (f TaskFrontmatter) LastCompletedDate() *libtime.DateOrDateTime {
	if t := f.GetTime("last_completed_date"); t != nil {
		d := libtime.DateOrDateTime(*t)
		return &d
	}
	if t := f.GetTime("last_completed"); t != nil {
		d := libtime.DateOrDateTime(*t)
		return &d
	}
	return nil
}

// LastCompleted reads "last_completed" (legacy) or "last_completed_date" (canonical)
// as a formatted date string. Kept for backward compatibility.
func (f TaskFrontmatter) LastCompleted() string {
	return formatDateOrDateTime(f.LastCompletedDate())
}

// CompletedDate reads "completed_date" as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (f TaskFrontmatter) CompletedDate() *libtime.DateOrDateTime {
	t := f.GetTime("completed_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// CreatedDate reads "created_date" as *libtime.DateOrDateTime.
// Returns nil on missing or unparseable value.
func (f TaskFrontmatter) CreatedDate() *libtime.DateOrDateTime {
	t := f.GetTime("created_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// PlannedDate reads "planned_date" key as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (f TaskFrontmatter) PlannedDate() *libtime.DateOrDateTime {
	t := f.GetTime("planned_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// DueDate reads "due_date" key as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (f TaskFrontmatter) DueDate() *libtime.DateOrDateTime {
	t := f.GetTime("due_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// TaskIdentifier reads "task_identifier" key as string.
func (f TaskFrontmatter) TaskIdentifier() string { return f.GetString("task_identifier") }

// SetStatus validates and stores the status in the map.
func (f *TaskFrontmatter) SetStatus(s TaskStatus) error {
	if err := s.Validate(context.Background()); err != nil {
		return err
	}
	f.Set("status", string(s))
	return nil
}

// SetPageType stores the page_type in the map.
func (f *TaskFrontmatter) SetPageType(v string) { f.Set("page_type", v) }

// SetGoals stores goals in the map. Deletes the key if v is nil or empty.
func (f *TaskFrontmatter) SetGoals(v []string) {
	if len(v) == 0 {
		f.Delete("goals")
		return
	}
	f.Set("goals", stringSliceToAny(v))
}

// SetAssignee stores the assignee in the map.
func (f *TaskFrontmatter) SetAssignee(v string) { f.Set("assignee", v) }

// SetClaudeSessionID stores the claude_session_id in the map.
func (f *TaskFrontmatter) SetClaudeSessionID(v string) { f.Set("claude_session_id", v) }

// SetRecurring stores the recurring value in the map.
func (f *TaskFrontmatter) SetRecurring(v string) { f.Set("recurring", v) }

// SetLastCompletedDate stores the last_completed_date in the map.
// Dual-writes to both "last_completed_date" (canonical) and "last_completed" (legacy)
// for one release cycle to allow external consumers to migrate.
// Deletes both keys if d is nil.
func (f *TaskFrontmatter) SetLastCompletedDate(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("last_completed_date")
		f.Delete("last_completed")
		return
	}
	formatted := formatDateOrDateTime(d)
	f.Set("last_completed_date", formatted)
	f.Set("last_completed", formatted) // dual-write window
}

// SetLastCompleted stores the last_completed value. Kept for backward compatibility.
// Delegates to SetLastCompletedDate for dual-write behavior.
func (f *TaskFrontmatter) SetLastCompleted(v string) {
	if v == "" {
		f.SetLastCompletedDate(nil)
		return
	}
	t, err := libtime.ParseTime(context.Background(), v)
	if err != nil {
		f.Set("last_completed", v)
		f.Set("last_completed_date", v)
		return
	}
	d := libtime.DateOrDateTime(*t)
	f.SetLastCompletedDate(&d)
}

// SetCompletedDate stores the completed_date in the map. Deletes the key if d is nil.
func (f *TaskFrontmatter) SetCompletedDate(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("completed_date")
		return
	}
	f.Set("completed_date", formatDateOrDateTime(d))
}

// SetCreatedDate stores the created_date in the map. Deletes the key if d is nil.
func (f *TaskFrontmatter) SetCreatedDate(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("created_date")
		return
	}
	f.Set("created_date", formatDateOrDateTime(d))
}

// SetTaskIdentifier stores the task_identifier value in the map.
func (f *TaskFrontmatter) SetTaskIdentifier(v string) { f.Set("task_identifier", v) }

// SetTags stores tags in the map. Deletes the key if v is nil or empty.
func (f *TaskFrontmatter) SetTags(v []string) {
	if len(v) == 0 {
		f.Delete("tags")
		return
	}
	f.Set("tags", stringSliceToAny(v))
}

// SetPriority validates the priority and stores it in the map.
// Returns an error when the value is negative, per spec AC #6.
func (f *TaskFrontmatter) SetPriority(ctx context.Context, p Priority) error {
	if err := p.Validate(ctx); err != nil {
		return errors.Wrap(ctx, err, "invalid priority")
	}
	f.Set("priority", int(p))
	return nil
}

// SetPhase stores the phase pointer in the map. Deletes the key if p is nil.
func (f *TaskFrontmatter) SetPhase(p *TaskPhase) {
	if p == nil {
		f.Delete("phase")
		return
	}
	f.Set("phase", string(*p))
}

// SetDeferDate stores the defer_date in the map. Deletes the key if d is nil.
func (f *TaskFrontmatter) SetDeferDate(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("defer_date")
		return
	}
	f.Set("defer_date", formatDateOrDateTime(d))
}

// SetPlannedDate stores the planned_date in the map. Deletes the key if d is nil.
func (f *TaskFrontmatter) SetPlannedDate(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("planned_date")
		return
	}
	f.Set("planned_date", formatDateOrDateTime(d))
}

// SetDueDate stores the due_date in the map. Deletes the key if d is nil.
func (f *TaskFrontmatter) SetDueDate(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("due_date")
		return
	}
	f.Set("due_date", formatDateOrDateTime(d))
}

// GetField returns the string representation of any frontmatter field by key.
// Known fields return formatted values. Unknown fields return the raw string.
// Returns "" if the key is absent.
func (f TaskFrontmatter) GetField(key string) string {
	switch key {
	case "status":
		return string(f.Status())
	case "page_type":
		return f.PageType()
	case "goals":
		return strings.Join(f.Goals(), ",")
	case "priority":
		p := f.Priority()
		if p == 0 {
			return ""
		}
		return strconv.Itoa(int(p))
	case "assignee":
		return f.Assignee()
	case "defer_date":
		return formatDateOrDateTime(f.DeferDate())
	case "tags":
		return strings.Join(f.Tags(), ",")
	case "phase":
		ph := f.Phase()
		if ph == nil {
			return ""
		}
		return string(*ph)
	case "claude_session_id":
		return f.ClaudeSessionID()
	case "recurring":
		return f.Recurring()
	case "last_completed":
		return f.LastCompleted()
	case "last_completed_date":
		return formatDateOrDateTime(f.LastCompletedDate())
	case "completed_date":
		return formatDateOrDateTime(f.CompletedDate())
	case "created_date":
		return formatDateOrDateTime(f.CreatedDate())
	case "planned_date":
		return formatDateOrDateTime(f.PlannedDate())
	case "due_date":
		return formatDateOrDateTime(f.DueDate())
	case "task_identifier":
		return f.TaskIdentifier()
	default:
		return f.GetString(key)
	}
}

// setStringSliceField parses a comma-separated string and calls setter, or clears on empty.
func setStringSliceField(setter func([]string), value string) {
	if value == "" {
		setter(nil)
	} else {
		setter(strings.Split(value, ","))
	}
}

// setDateField parses a date string and calls setter, or clears on empty.
func setDateField(ctx context.Context, setter func(*libtime.DateOrDateTime), value string) error {
	if value == "" {
		setter(nil)
		return nil
	}
	t, err := libtime.ParseTime(ctx, value)
	if err != nil {
		return errors.Wrap(ctx, err, "invalid date format")
	}
	d := libtime.DateOrDateTime(*t)
	setter(&d)
	return nil
}

// setPriorityField parses an integer string and stores the priority, or deletes on empty.
func (f *TaskFrontmatter) setPriorityField(ctx context.Context, value string) error {
	if value == "" {
		f.Delete("priority")
		return nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return errors.Wrap(ctx, err, "priority must be an integer")
	}
	return f.SetPriority(ctx, Priority(n))
}

// setPhaseField normalises the value (accepting aliases) and stores the phase, or clears on empty.
func (f *TaskFrontmatter) setPhaseField(ctx context.Context, value string) error {
	if value == "" {
		f.SetPhase(nil)
		return nil
	}
	canonical, ok := NormalizeTaskPhase(value)
	if !ok {
		return errors.Wrapf(ctx, validation.Error, "unknown task phase '%s'", value)
	}
	f.SetPhase(&canonical)
	return nil
}

// SetField sets a frontmatter field by key from a string value.
// Known fields apply type coercion and validation; unknown fields are stored as-is.
func (f *TaskFrontmatter) SetField(ctx context.Context, key, value string) error {
	switch key {
	case "status":
		canonical, ok := NormalizeTaskStatus(value)
		if !ok {
			return errors.Wrapf(ctx, validation.Error, "unknown task status '%s'", value)
		}
		return f.SetStatus(canonical)
	case "page_type":
		f.SetPageType(value)
	case "goals":
		setStringSliceField(f.SetGoals, value)
	case "priority":
		return f.setPriorityField(ctx, value)
	case "assignee":
		f.SetAssignee(value)
	case "defer_date":
		return setDateField(ctx, f.SetDeferDate, value)
	case "tags":
		setStringSliceField(f.SetTags, value)
	case "phase":
		return f.setPhaseField(ctx, value)
	case "claude_session_id":
		f.SetClaudeSessionID(value)
	case "recurring":
		f.SetRecurring(value)
	case "last_completed":
		f.SetLastCompleted(value)
	case "last_completed_date":
		return setDateField(ctx, f.SetLastCompletedDate, value)
	case "completed_date":
		return setDateField(ctx, f.SetCompletedDate, value)
	case "created_date":
		return setDateField(ctx, f.SetCreatedDate, value)
	case "planned_date":
		return setDateField(ctx, f.SetPlannedDate, value)
	case "due_date":
		return setDateField(ctx, f.SetDueDate, value)
	case "task_identifier":
		f.SetTaskIdentifier(value)
	default:
		// Unknown field — store as string without validation
		f.Set(key, value)
	}
	return nil
}

// ClearField removes a frontmatter field by key.
// Works for both known and unknown fields.
func (f *TaskFrontmatter) ClearField(key string) {
	f.Delete(key)
}

// formatTimeAsDate serializes a time.Time using the same rule as formatDateOrDateTime:
// YYYY-MM-DD for midnight-UTC values, RFC3339 preserving timezone otherwise.
func formatTimeAsDate(t time.Time) string {
	tUTC := t.UTC()
	if tUTC.Hour() == 0 && tUTC.Minute() == 0 && tUTC.Second() == 0 && tUTC.Nanosecond() == 0 {
		return tUTC.Format(time.DateOnly)
	}
	return t.Format(time.RFC3339)
}

// formatDateOrDateTime serializes a libtime.DateOrDateTime to YYYY-MM-DD for date-only values
// (midnight UTC) and RFC3339 preserving the original timezone for values with a time component.
func formatDateOrDateTime(d *libtime.DateOrDateTime) string {
	if d == nil {
		return ""
	}
	return formatTimeAsDate(d.Time())
}

// stringSliceToAny converts []string to []any for map storage.
func stringSliceToAny(ss []string) []any {
	if ss == nil {
		return nil
	}
	result := make([]any, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}
