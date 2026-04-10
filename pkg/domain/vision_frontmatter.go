// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"strconv"
	"strings"

	"github.com/bborbe/errors"
)

// VisionFrontmatter holds the YAML frontmatter for a Vision.
// It uses FrontmatterMap as its backing store so unknown fields survive round-trips.
type VisionFrontmatter struct {
	FrontmatterMap
}

// NewVisionFrontmatter constructs a VisionFrontmatter from a raw map.
func NewVisionFrontmatter(data map[string]any) VisionFrontmatter {
	return VisionFrontmatter{FrontmatterMap: NewFrontmatterMap(data)}
}

// Status reads "status" key.
func (f VisionFrontmatter) Status() VisionStatus {
	return VisionStatus(f.GetString("status"))
}

// PageType reads "page_type" key.
func (f VisionFrontmatter) PageType() string { return f.GetString("page_type") }

// Priority reads "priority" key as int.
func (f VisionFrontmatter) Priority() Priority {
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
func (f VisionFrontmatter) Assignee() string { return f.GetString("assignee") }

// Tags reads "tags" key via GetStringSlice.
func (f VisionFrontmatter) Tags() []string { return f.GetStringSlice("tags") }

// SetStatus validates and stores the status in the map.
func (f *VisionFrontmatter) SetStatus(s VisionStatus) error {
	if err := s.Validate(context.Background()); err != nil {
		return err
	}
	f.Set("status", string(s))
	return nil
}

// SetPageType stores the page_type in the map.
func (f *VisionFrontmatter) SetPageType(v string) { f.Set("page_type", v) }

// SetPriority validates the priority and stores it in the map.
func (f *VisionFrontmatter) SetPriority(ctx context.Context, p Priority) error {
	if err := p.Validate(ctx); err != nil {
		return errors.Wrap(ctx, err, "invalid priority")
	}
	f.Set("priority", int(p))
	return nil
}

// SetAssignee stores the assignee in the map.
func (f *VisionFrontmatter) SetAssignee(v string) { f.Set("assignee", v) }

// SetTags stores tags in the map. Deletes key if v is nil or empty.
func (f *VisionFrontmatter) SetTags(v []string) {
	if len(v) == 0 {
		f.Delete("tags")
		return
	}
	f.Set("tags", stringSliceToAny(v))
}

// GetField returns the string representation of any frontmatter field by key.
func (f VisionFrontmatter) GetField(key string) string {
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
	case "tags":
		return strings.Join(f.Tags(), ",")
	default:
		return f.GetString(key)
	}
}

// SetField sets a frontmatter field by key from a string value.
func (f *VisionFrontmatter) SetField(ctx context.Context, key, value string) error {
	switch key {
	case "status":
		return f.SetStatus(VisionStatus(value))
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
func (f *VisionFrontmatter) ClearField(key string) {
	f.Delete(key)
}
