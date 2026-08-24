// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"fmt"
	"strconv"
	"time"

	libtime "github.com/bborbe/time"
)

// MetricsSessions reads the "metrics_sessions" field as a slice of MetricsSession.
// Returns nil when the key is absent, the value is not a list, or every element is malformed.
func (f TaskFrontmatter) MetricsSessions() []MetricsSession {
	v := f.Get("metrics_sessions")
	switch s := v.(type) {
	case []MetricsSession:
		return s
	case []any:
		result := make([]MetricsSession, 0, len(s))
		for _, item := range s {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			entry, ok := coerceMetricsSession(m)
			if !ok {
				continue
			}
			result = append(result, entry)
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return nil
	}
}

// AppendMetricsSession appends one entry to "metrics_sessions", preserving every prior entry.
// A nil or empty SessionID is ignored (not appended).
func (f *TaskFrontmatter) AppendMetricsSession(entry MetricsSession) {
	if entry.SessionID == "" {
		return
	}
	f.Set("metrics_sessions", append(f.MetricsSessions(), entry))
}

// ClearMetricsSessions removes the "metrics_sessions" key entirely.
func (f *TaskFrontmatter) ClearMetricsSessions() { f.Delete("metrics_sessions") }

// MetricsCompletedAt reads "metrics_completed_at" as *libtime.DateOrDateTime.
// Returns nil when the key is absent or the value is unparseable.
func (f TaskFrontmatter) MetricsCompletedAt() *libtime.DateOrDateTime {
	t := f.GetTime("metrics_completed_at")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// SetMetricsCompletedAt stores "metrics_completed_at". A nil argument deletes the key.
func (f *TaskFrontmatter) SetMetricsCompletedAt(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("metrics_completed_at")
		return
	}
	f.Set("metrics_completed_at", *d)
}

// ClearMetricsCompletedAt removes the "metrics_completed_at" key entirely.
func (f *TaskFrontmatter) ClearMetricsCompletedAt() { f.Delete("metrics_completed_at") }

// MetricsInteractionCount reads "metrics_interaction_count" as *int.
// Returns nil when the key is absent or the value is not a number — "unknown"
// must never be forged as 0.
func (f TaskFrontmatter) MetricsInteractionCount() *int {
	n, ok := coerceMetricsInt(f.Get("metrics_interaction_count"))
	if !ok {
		return nil
	}
	return &n
}

// SetMetricsInteractionCount stores "metrics_interaction_count".
func (f *TaskFrontmatter) SetMetricsInteractionCount(count int) {
	f.Set("metrics_interaction_count", count)
}

// ClearMetricsInteractionCount removes the "metrics_interaction_count" key entirely.
func (f *TaskFrontmatter) ClearMetricsInteractionCount() { f.Delete("metrics_interaction_count") }

// MetricsCycles reads "metrics_cycles" as a slice of MetricsCycle.
// Returns nil when the key is absent, the value is not a list, or every element is malformed.
func (f TaskFrontmatter) MetricsCycles() []MetricsCycle {
	v := f.Get("metrics_cycles")
	switch s := v.(type) {
	case []MetricsCycle:
		return s
	case []any:
		result := make([]MetricsCycle, 0, len(s))
		for _, item := range s {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, coerceMetricsCycle(m))
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return nil
	}
}

// AppendMetricsCycle appends one archived cycle to "metrics_cycles", preserving every prior entry.
func (f *TaskFrontmatter) AppendMetricsCycle(cycle MetricsCycle) {
	f.Set("metrics_cycles", append(f.MetricsCycles(), cycle))
}

// coerceMetricsSession coerces one on-disk session element (a map[string]any
// produced by yaml.Unmarshal) into a typed MetricsSession. Returns false when the
// entry must be skipped: its session_id is absent or empty.
func coerceMetricsSession(m map[string]any) (MetricsSession, bool) {
	sid := metricsStringify(m["session_id"])
	if sid == "" {
		return MetricsSession{}, false
	}
	return MetricsSession{
		SessionID: sid,
		StartedAt: coerceMetricsTime(m["started_at"]),
	}, true
}

// coerceMetricsCycle coerces one on-disk cycle element into a typed MetricsCycle.
// Every field degrades leniently: an unparseable timestamp becomes the zero time
// and an unparseable interaction count reads as 0 — data-quality matters, never a
// dropped entry.
func coerceMetricsCycle(m map[string]any) MetricsCycle {
	count, _ := coerceMetricsInt(m["interaction_count"])
	return MetricsCycle{
		StartedAt:        coerceMetricsTime(m["started_at"]),
		CompletedAt:      coerceMetricsTime(m["completed_at"]),
		InteractionCount: count,
	}
}

// coerceMetricsTime coerces a stored time value into a DateOrDateTime, mirroring
// GetTime's multi-shape handling: time.Time and libtime.DateOrDateTime pass
// through, a string is parsed with libtime.ParseTime, and anything else or a parse
// failure becomes the zero DateOrDateTime — the entry is preserved, a zero time is
// a data-quality matter, not malformed.
func coerceMetricsTime(v any) libtime.DateOrDateTime {
	switch t := v.(type) {
	case time.Time:
		return libtime.DateOrDateTime(t)
	case libtime.DateOrDateTime:
		return t
	case string:
		if t == "" {
			return libtime.DateOrDateTime{}
		}
		parsed, err := libtime.ParseTime(context.Background(), t)
		if err != nil {
			return libtime.DateOrDateTime{}
		}
		return libtime.DateOrDateTime(*parsed)
	default:
		return libtime.DateOrDateTime{}
	}
}

// coerceMetricsInt coerces a stored count into an int. Accepts int, int64, float64,
// and numeric strings; reports ok=false for nil, an unparseable string, or an
// unsupported type so callers can distinguish "unknown/absent" from a real zero.
func coerceMetricsInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		nn, err := strconv.Atoi(n)
		if err != nil {
			return 0, false
		}
		return nn, true
	default:
		return 0, false
	}
}

// metricsStringify returns the string representation of v, mirroring GetString's
// stringification: strings pass through, nil becomes "", everything else is
// fmt.Sprintf.
func metricsStringify(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
