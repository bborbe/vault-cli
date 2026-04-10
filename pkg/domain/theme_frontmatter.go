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

// StartDate reads "start_date" key as *time.Time.
func (f ThemeFrontmatter) StartDate() *time.Time {
	v := f.Get("start_date")
	if v == nil {
		return nil
	}
	if t, ok := v.(time.Time); ok {
		utc := t.UTC()
		return &utc
	}
	raw, ok := v.(string)
	if !ok || raw == "" {
		return nil
	}
	t, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return nil
	}
	return &t
}

// TargetDate reads "target_date" key as *time.Time.
func (f ThemeFrontmatter) TargetDate() *time.Time {
	v := f.Get("target_date")
	if v == nil {
		return nil
	}
	if t, ok := v.(time.Time); ok {
		utc := t.UTC()
		return &utc
	}
	raw, ok := v.(string)
	if !ok || raw == "" {
		return nil
	}
	t, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return nil
	}
	return &t
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

// SetStartDate stores the start_date in the map. Deletes key if t is nil.
func (f *ThemeFrontmatter) SetStartDate(t *time.Time) {
	if t == nil {
		f.Delete("start_date")
		return
	}
	f.Set("start_date", t.UTC().Format(time.DateOnly))
}

// SetTargetDate stores the target_date in the map. Deletes key if t is nil.
func (f *ThemeFrontmatter) SetTargetDate(t *time.Time) {
	if t == nil {
		f.Delete("target_date")
		return
	}
	f.Set("target_date", t.UTC().Format(time.DateOnly))
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
		t := f.StartDate()
		if t == nil {
			return ""
		}
		return t.UTC().Format(time.DateOnly)
	case "target_date":
		t := f.TargetDate()
		if t == nil {
			return ""
		}
		return t.UTC().Format(time.DateOnly)
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
		if value == "" {
			f.SetStartDate(nil)
			return nil
		}
		t, err := time.Parse(time.DateOnly, value)
		if err != nil {
			return errors.Wrap(ctx, err, "invalid date format (expected YYYY-MM-DD)")
		}
		f.SetStartDate(&t)
	case "target_date":
		if value == "" {
			f.SetTargetDate(nil)
			return nil
		}
		t, err := time.Parse(time.DateOnly, value)
		if err != nil {
			return errors.Wrap(ctx, err, "invalid date format (expected YYYY-MM-DD)")
		}
		f.SetTargetDate(&t)
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
