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
)

// GoalFrontmatter holds the YAML frontmatter for a Goal.
// It uses FrontmatterMap as its backing store so unknown fields survive round-trips.
type GoalFrontmatter struct {
	FrontmatterMap
}

// NewGoalFrontmatter constructs a GoalFrontmatter from a raw map.
func NewGoalFrontmatter(data map[string]any) GoalFrontmatter {
	return GoalFrontmatter{FrontmatterMap: NewFrontmatterMap(data)}
}

// Status reads "status" key.
func (f GoalFrontmatter) Status() GoalStatus {
	return GoalStatus(f.GetString("status"))
}

// PageType reads "page_type" key.
func (f GoalFrontmatter) PageType() string { return f.GetString("page_type") }

// Theme reads "theme" key.
func (f GoalFrontmatter) Theme() string { return f.GetString("theme") }

// Priority reads "priority" key as int.
func (f GoalFrontmatter) Priority() Priority {
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
func (f GoalFrontmatter) Assignee() string { return f.GetString("assignee") }

// StartDate reads "start_date" key as *time.Time.
func (f GoalFrontmatter) StartDate() *time.Time {
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
func (f GoalFrontmatter) TargetDate() *time.Time {
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
func (f GoalFrontmatter) Tags() []string { return f.GetStringSlice("tags") }

// Completed reads "completed" key as *libtime.Date.
func (f GoalFrontmatter) Completed() *libtime.Date {
	v := f.Get("completed")
	if v == nil {
		return nil
	}
	if t, ok := v.(time.Time); ok {
		d := libtime.ToDate(t)
		return d.Ptr()
	}
	raw, ok := v.(string)
	if !ok || raw == "" {
		return nil
	}
	d, err := libtime.ParseDate(context.Background(), raw)
	if err != nil {
		return nil
	}
	return d
}

// DeferDate reads "defer_date" key as *DateOrDateTime.
func (f GoalFrontmatter) DeferDate() *DateOrDateTime {
	t := f.GetTime("defer_date")
	if t == nil {
		return nil
	}
	d := DateOrDateTime(*t)
	return &d
}

// SetStatus validates and stores the status in the map.
func (f *GoalFrontmatter) SetStatus(s GoalStatus) error {
	if err := s.Validate(context.Background()); err != nil {
		return err
	}
	f.Set("status", string(s))
	return nil
}

// SetPageType stores the page_type in the map.
func (f *GoalFrontmatter) SetPageType(v string) { f.Set("page_type", v) }

// SetTheme stores the theme in the map.
func (f *GoalFrontmatter) SetTheme(v string) { f.Set("theme", v) }

// SetPriority validates the priority and stores it in the map.
func (f *GoalFrontmatter) SetPriority(ctx context.Context, p Priority) error {
	if err := p.Validate(ctx); err != nil {
		return errors.Wrap(ctx, err, "invalid priority")
	}
	f.Set("priority", int(p))
	return nil
}

// SetAssignee stores the assignee in the map.
func (f *GoalFrontmatter) SetAssignee(v string) { f.Set("assignee", v) }

// SetStartDate stores the start_date in the map. Deletes key if t is nil.
func (f *GoalFrontmatter) SetStartDate(t *time.Time) {
	if t == nil {
		f.Delete("start_date")
		return
	}
	f.Set("start_date", t.UTC().Format(time.DateOnly))
}

// SetTargetDate stores the target_date in the map. Deletes key if t is nil.
func (f *GoalFrontmatter) SetTargetDate(t *time.Time) {
	if t == nil {
		f.Delete("target_date")
		return
	}
	f.Set("target_date", t.UTC().Format(time.DateOnly))
}

// SetTags stores tags in the map. Deletes key if v is nil or empty.
func (f *GoalFrontmatter) SetTags(v []string) {
	if len(v) == 0 {
		f.Delete("tags")
		return
	}
	f.Set("tags", stringSliceToAny(v))
}

// SetCompleted stores the completed date in the map. Deletes key if d is nil.
func (f *GoalFrontmatter) SetCompleted(d *libtime.Date) {
	if d == nil {
		f.Delete("completed")
		return
	}
	f.Set("completed", d.String())
}

// SetDeferDate stores the defer_date in the map. Deletes key if d is nil.
func (f *GoalFrontmatter) SetDeferDate(d *DateOrDateTime) {
	if d == nil {
		f.Delete("defer_date")
		return
	}
	f.Set("defer_date", formatDateOrDateTime(d))
}

// GetField returns the string representation of any frontmatter field by key.
func (f GoalFrontmatter) GetField(key string) string {
	switch key {
	case "status":
		return string(f.Status())
	case "page_type":
		return f.PageType()
	case "theme":
		return f.Theme()
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
	case "completed":
		d := f.Completed()
		if d == nil {
			return ""
		}
		return d.String()
	case "defer_date":
		return formatDateOrDateTime(f.DeferDate())
	default:
		return f.GetString(key)
	}
}

// SetField sets a frontmatter field by key from a string value.
func (f *GoalFrontmatter) SetField(ctx context.Context, key, value string) error {
	switch key {
	case "status":
		return f.SetStatus(GoalStatus(value))
	case "page_type":
		f.SetPageType(value)
	case "theme":
		f.SetTheme(value)
	case "priority":
		return f.setPriorityFromString(ctx, value)
	case "assignee":
		f.SetAssignee(value)
	case "start_date":
		return f.setDateFromString(ctx, value, f.SetStartDate)
	case "target_date":
		return f.setDateFromString(ctx, value, f.SetTargetDate)
	case "tags":
		f.setTagsFromString(value)
	case "completed":
		return f.setCompletedFromString(ctx, value)
	case "defer_date":
		return f.setDeferDateFromString(ctx, value)
	default:
		f.Set(key, value)
	}
	return nil
}

func (f *GoalFrontmatter) setPriorityFromString(ctx context.Context, value string) error {
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

func (f *GoalFrontmatter) setDateFromString(
	ctx context.Context,
	value string,
	setter func(*time.Time),
) error {
	if value == "" {
		setter(nil)
		return nil
	}
	t, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return errors.Wrap(ctx, err, "invalid date format (expected YYYY-MM-DD)")
	}
	setter(&t)
	return nil
}

func (f *GoalFrontmatter) setTagsFromString(value string) {
	if value == "" {
		f.SetTags(nil)
	} else {
		f.SetTags(strings.Split(value, ","))
	}
}

func (f *GoalFrontmatter) setCompletedFromString(ctx context.Context, value string) error {
	if value == "" {
		f.SetCompleted(nil)
		return nil
	}
	d, err := libtime.ParseDate(ctx, value)
	if err != nil {
		return errors.Wrap(ctx, err, "invalid date format (expected YYYY-MM-DD)")
	}
	f.SetCompleted(d)
	return nil
}

func (f *GoalFrontmatter) setDeferDateFromString(ctx context.Context, value string) error {
	if value == "" {
		f.SetDeferDate(nil)
		return nil
	}
	t, err := libtime.ParseTime(ctx, value)
	if err != nil {
		return errors.Wrap(ctx, err, "invalid date format")
	}
	d := DateOrDateTime(*t)
	f.SetDeferDate(&d)
	return nil
}

// ClearField removes a frontmatter field by key.
func (f *GoalFrontmatter) ClearField(key string) {
	f.Delete(key)
}
