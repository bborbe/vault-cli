// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"strconv"
	"strings"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
)

// ThemeFrontmatter holds the YAML frontmatter for a Theme.
// It uses FrontmatterMap as its backing store so unknown fields survive round-trips.
type ThemeFrontmatter struct {
	FrontmatterMap
}

// NewThemeFrontmatter constructs a ThemeFrontmatter from a raw map.
func NewThemeFrontmatter(data map[string]any) ThemeFrontmatter {
	return ThemeFrontmatter{FrontmatterMap: NewFrontmatterMap(data)}
}

// Status reads "status" key.
func (f ThemeFrontmatter) Status() ThemeStatus {
	return ThemeStatus(f.GetString("status"))
}

// PageType reads "page_type" key.
func (f ThemeFrontmatter) PageType() string { return f.GetString("page_type") }

// Priority reads "priority" key as int.
func (f ThemeFrontmatter) Priority() Priority {
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

// Assignee reads "assignee" key.
func (f ThemeFrontmatter) Assignee() string { return f.GetString("assignee") }

// StartDate reads "start_date" as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (f ThemeFrontmatter) StartDate() *libtime.DateOrDateTime {
	t := f.GetTime("start_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// TargetDate reads "target_date" as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (f ThemeFrontmatter) TargetDate() *libtime.DateOrDateTime {
	t := f.GetTime("target_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// Tags reads "tags" key via GetStringSlice.
func (f ThemeFrontmatter) Tags() []string { return f.GetStringSlice("tags") }

// SetStatus validates and stores the status in the map.
func (f *ThemeFrontmatter) SetStatus(s ThemeStatus) error {
	if err := s.Validate(context.Background()); err != nil {
		return err
	}
	f.Set("status", string(s))
	return nil
}

// SetPageType stores the page_type in the map.
func (f *ThemeFrontmatter) SetPageType(v string) { f.Set("page_type", v) }

// SetPriority validates the priority and stores it in the map.
func (f *ThemeFrontmatter) SetPriority(ctx context.Context, p Priority) error {
	if err := p.Validate(ctx); err != nil {
		return errors.Wrap(ctx, err, "invalid priority")
	}
	f.Set("priority", int(p))
	return nil
}

// SetAssignee stores the assignee in the map.
func (f *ThemeFrontmatter) SetAssignee(v string) { f.Set("assignee", v) }

// SetStartDate stores the start_date in the map. Deletes key if d is nil.
func (f *ThemeFrontmatter) SetStartDate(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("start_date")
		return
	}
	f.Set("start_date", formatDateOrDateTime(d))
}

// SetTargetDate stores the target_date in the map. Deletes key if d is nil.
func (f *ThemeFrontmatter) SetTargetDate(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("target_date")
		return
	}
	f.Set("target_date", formatDateOrDateTime(d))
}

// SetTags stores tags in the map. Deletes key if v is nil or empty.
func (f *ThemeFrontmatter) SetTags(v []string) {
	if len(v) == 0 {
		f.Delete("tags")
		return
	}
	f.Set("tags", stringSliceToAny(v))
}

// GetField returns the string representation of any frontmatter field by key.
func (f ThemeFrontmatter) GetField(key string) string {
	switch key {
	case "status":
		return string(f.Status())
	case "page_type":
		return f.PageType()
	case "priority":
		p := f.Priority()
		if p == 0 {
			return ""
		}
		return strconv.Itoa(int(p))
	case "assignee":
		return f.Assignee()
	case "start_date":
		return formatDateOrDateTime(f.StartDate())
	case "target_date":
		return formatDateOrDateTime(f.TargetDate())
	case "tags":
		return strings.Join(f.Tags(), ",")
	default:
		return f.GetString(key)
	}
}

// SetField sets a frontmatter field by key from a string value.
func (f *ThemeFrontmatter) SetField(ctx context.Context, key, value string) error {
	switch key {
	case "status":
		return f.SetStatus(ThemeStatus(value))
	case "page_type":
		f.SetPageType(value)
	case "priority":
		if value == "" {
			f.Delete("priority")
			return nil
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return errors.Wrap(ctx, err, "priority must be an integer")
		}
		return f.SetPriority(ctx, Priority(n))
	case "assignee":
		f.SetAssignee(value)
	case "start_date":
		return setDateField(ctx, f.SetStartDate, value)
	case "target_date":
		return setDateField(ctx, f.SetTargetDate, value)
	case "tags":
		if value == "" {
			f.SetTags(nil)
		} else {
			f.SetTags(strings.Split(value, ","))
		}
	default:
		f.Set(key, value)
	}
	return nil
}

// ClearField removes a frontmatter field by key.
func (f *ThemeFrontmatter) ClearField(key string) {
	f.Delete(key)
}
