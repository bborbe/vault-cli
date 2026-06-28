// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"strconv"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
)

// Page represents a page in the Obsidian vault with shared frontmatter fields.
// It is used by the storage layer to list pages from any directory (tasks, goals,
// themes, objectives, visions) without a type contract violation. Entity-specific
// types (Task, Goal, Theme, etc.) embed their own XxxFrontmatter; Page uses the
// generic FrontmatterMap so any entity can be read through it without field loss.
type Page struct {
	FrontmatterMap
	FileMetadata
	Content
}

// NewPage creates a Page from a parsed frontmatter map and metadata.
func NewPage(data map[string]any, meta FileMetadata, content Content) *Page {
	return &Page{
		FrontmatterMap: NewFrontmatterMap(data),
		FileMetadata:   meta,
		Content:        content,
	}
}

// Status reads "status" key and applies NormalizeTaskStatus.
// Returns "" (empty) if value is absent or unrecognized.
func (p Page) Status() TaskStatus {
	raw := p.GetString("status")
	normalized, ok := NormalizeTaskStatus(raw)
	if !ok {
		return ""
	}
	return normalized
}

// PageType reads "page_type" key, returns string.
func (p Page) PageType() string { return p.GetString("page_type") }

// Goals reads "goals" key via GetStringSlice.
func (p Page) Goals() []string { return p.GetStringSlice("goals") }

// Priority reads "priority" key as int. Returns 0 on missing or parse failure.
func (p Page) Priority() Priority {
	v := p.Get("priority")
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
func (p Page) Assignee() string { return p.GetString("assignee") }

// DeferDate reads "defer_date" key as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (p Page) DeferDate() *libtime.DateOrDateTime {
	t := p.GetTime("defer_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// Phase reads "phase" key as string, returns *TaskPhase.
func (p Page) Phase() *TaskPhase {
	raw := p.GetString("phase")
	if raw == "" {
		return nil
	}
	phase := TaskPhase(raw)
	return &phase
}

// ClaudeSessionID reads "claude_session_id" key as string.
func (p Page) ClaudeSessionID() string { return p.GetString("claude_session_id") }

// Recurring reads "recurring" key as string.
func (p Page) Recurring() string { return p.GetString("recurring") }

// CompletedDate reads "completed_date" as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (p Page) CompletedDate() *libtime.DateOrDateTime {
	t := p.GetTime("completed_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// PlannedDate reads "planned_date" key as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (p Page) PlannedDate() *libtime.DateOrDateTime {
	t := p.GetTime("planned_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// DueDate reads "due_date" key as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (p Page) DueDate() *libtime.DateOrDateTime {
	t := p.GetTime("due_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// ModifiedDateFromPage is a convenience accessor for the embedded FileMetadata.ModifiedDate.
// Deprecated: use page.ModifiedDate directly (accessible via struct embedding).
func (p Page) ModifiedDateFromPage() *time.Time {
	return p.ModifiedDate
}

// SetGoals stores goals in the map. Deletes the key if v is nil or empty.
func (p *Page) SetGoals(v []string) {
	if len(v) == 0 {
		p.Delete("goals")
		return
	}
	p.Set("goals", stringSliceToAny(v))
}

// SetDeferDate stores the defer_date in the map. Deletes the key if d is nil.
func (p *Page) SetDeferDate(d *libtime.DateOrDateTime) {
	if d == nil {
		p.Delete("defer_date")
		return
	}
	p.Set("defer_date", *d)
}

// SetPlannedDate stores the planned_date in the map. Deletes the key if d is nil.
func (p *Page) SetPlannedDate(d *libtime.DateOrDateTime) {
	if d == nil {
		p.Delete("planned_date")
		return
	}
	p.Set("planned_date", *d)
}

// SetDueDate stores the due_date in the map. Deletes the key if d is nil.
func (p *Page) SetDueDate(d *libtime.DateOrDateTime) {
	if d == nil {
		p.Delete("due_date")
		return
	}
	p.Set("due_date", *d)
}

// SetPriority validates the priority and stores it in the map.
func (p *Page) SetPriority(ctx context.Context, pr Priority) error {
	if err := pr.Validate(ctx); err != nil {
		return errors.Wrap(ctx, err, "invalid priority")
	}
	p.Set("priority", int(pr))
	return nil
}
