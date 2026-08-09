// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	libtime "github.com/bborbe/time"
)

// FrontmatterMap is a typed wrapper around map[string]any that stores YAML frontmatter
// fields. It preserves all fields, including unknown ones, through read-write cycles.
// Entity-specific types embed FrontmatterMap and layer typed accessors on top.
type FrontmatterMap struct {
	data map[string]any
}

// NewFrontmatterMap constructs a FrontmatterMap from a raw map.
// If data is nil, an empty map is used.
func NewFrontmatterMap(data map[string]any) FrontmatterMap {
	if data == nil {
		data = make(map[string]any)
	}
	return FrontmatterMap{data: data}
}

// Get returns the raw value stored for key, or nil if absent.
func (f FrontmatterMap) Get(key string) any {
	return f.data[key]
}

// GetString returns the string representation of the value stored for key.
// Returns "" if the key is absent or the value cannot be stringified.
func (f FrontmatterMap) GetString(key string) string {
	v := f.data[key]
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprintf("%v", v)
	}
}

// GetBool returns the bool value stored for key.
//
// A bool value passes through unchanged. A string value is matched
// case-insensitively after trimming surrounding whitespace: "true" and "yes"
// yield true, everything else yields false. A missing key, a nil value, or any
// other type also yields false.
//
// Coercion rather than a bare type assertion is deliberate: YAML 1.2 leaves a
// hand-quoted reviewed: "true" as a string, and a .(bool) assertion would read
// that as false — "not reviewed" — silently resurfacing an already-acknowledged
// page.
func (f FrontmatterMap) GetBool(key string) bool {
	v := f.data[key]
	if v == nil {
		return false
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		switch strings.ToLower(strings.TrimSpace(b)) {
		case "true", "yes":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// GetTime returns the time.Time value stored for key.
// Handles three shapes:
//   - time.Time (YAML parses date/datetime literals into this automatically)
//   - libtime.DateOrDateTime (in-memory typed value set by domain setters)
//   - string (falls back to libtime.ParseTime for manually-authored values)
//   - anything else → nil
//
// Returns nil on missing key, empty string, parse failure, or unsupported type.
func (f FrontmatterMap) GetTime(key string) *time.Time {
	v := f.data[key]
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case time.Time:
		tc := t
		return &tc
	case libtime.DateOrDateTime:
		tc := t.Time()
		return &tc
	case string:
		if t == "" {
			return nil
		}
		// context.Background() here is pre-existing and deliberately left alone by this
		// change. Threading a ctx into GetTime means adding a parameter to an accessor
		// with 19 non-test call sites across all six entity types — a repo-wide API
		// refactor, not part of a decision-frontmatter bug fix. ParseTime does no I/O
		// and cannot block, so the missing ctx carries no cancellation risk today.
		// Tracked for a follow-up that changes the accessor family as a unit.
		parsed, err := libtime.ParseTime(context.Background(), t)
		if err != nil {
			return nil
		}
		return parsed
	default:
		return nil
	}
}

// GetStringSlice returns a []string for the value stored under key.
// Handles: nil (returns nil), []any (coerces each element to string),
// []string (returned directly), and string (splits on comma).
func (f FrontmatterMap) GetStringSlice(key string) []string {
	v := f.data[key]
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		result := make([]string, 0, len(s))
		for _, item := range s {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	case string:
		if s == "" {
			return nil
		}
		return strings.Split(s, ",")
	default:
		return nil
	}
}

// Set stores value under key. A nil value is equivalent to Delete.
func (f *FrontmatterMap) Set(key string, value any) {
	if f.data == nil {
		f.data = make(map[string]any)
	}
	if value == nil {
		delete(f.data, key)
		return
	}
	f.data[key] = value
}

// Delete removes key from the map. No-op if key is absent.
func (f *FrontmatterMap) Delete(key string) {
	delete(f.data, key)
}

// Keys returns all keys present in the map, in no guaranteed order.
func (f FrontmatterMap) Keys() []string {
	if len(f.data) == 0 {
		return nil
	}
	keys := make([]string, 0, len(f.data))
	for k := range f.data {
		keys = append(keys, k)
	}
	return keys
}

// RawMap returns the underlying map. Callers must not mutate the returned map.
// This method is intended for serialization (yaml.Marshal).
func (f FrontmatterMap) RawMap() map[string]any {
	return f.data
}
