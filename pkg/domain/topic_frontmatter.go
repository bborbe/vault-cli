// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"strings"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
)

// TopicFrontmatter holds the YAML frontmatter for a Topic.
// It uses FrontmatterMap as its backing store so unknown fields survive round-trips.
type TopicFrontmatter struct {
	FrontmatterMap
}

// NewTopicFrontmatter constructs a TopicFrontmatter from a raw map.
func NewTopicFrontmatter(data map[string]any) TopicFrontmatter {
	return TopicFrontmatter{FrontmatterMap: NewFrontmatterMap(data)}
}

// Tags reads "tags" key via GetStringSlice.
func (f TopicFrontmatter) Tags() []string { return f.GetStringSlice("tags") }

// SetTags stores tags in the map. Deletes key if v is nil or empty.
func (f *TopicFrontmatter) SetTags(v []string) {
	if len(v) == 0 {
		f.Delete("tags")
		return
	}
	f.Set("tags", stringSliceToAny(v))
}

// DeferDate reads "defer_date" key as *libtime.DateOrDateTime.
func (f TopicFrontmatter) DeferDate() *libtime.DateOrDateTime {
	t := f.GetTime("defer_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// SetDeferDate stores the defer_date in the map. Deletes key if d is nil.
func (f *TopicFrontmatter) SetDeferDate(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("defer_date")
		return
	}
	f.Set("defer_date", *d)
}

func (f *TopicFrontmatter) setDeferDateFromString(ctx context.Context, value string) error {
	if value == "" {
		f.SetDeferDate(nil)
		return nil
	}
	t, err := libtime.ParseTime(ctx, value)
	if err != nil {
		return errors.Wrap(ctx, err, "invalid date format")
	}
	d := libtime.DateOrDateTime(*t)
	f.SetDeferDate(&d)
	return nil
}

// GetField returns the string representation of any frontmatter field by key.
func (f TopicFrontmatter) GetField(key string) string {
	switch key {
	case "phase":
		// Raw on-disk read: no validation, no default, no normalisation.
		// A topic page with no `phase` line has no key in the map, so this
		// returns "" and the key stays absent from Keys().
		return f.GetString("phase")
	case "tags":
		return strings.Join(f.Tags(), ",")
	case "defer_date":
		return dateFieldString(f.DeferDate())
	default:
		return f.GetString(key)
	}
}

// SetField sets a frontmatter field by key from a string value.
func (f *TopicFrontmatter) SetField(ctx context.Context, key, value string) error {
	switch key {
	case "defer_date":
		return f.setDeferDateFromString(ctx, value)
	default:
		f.Set(key, value)
	}
	return nil
}

// ClearField removes a frontmatter field by key.
func (f *TopicFrontmatter) ClearField(key string) {
	f.Delete(key)
}
