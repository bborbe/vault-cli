// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import libtime "github.com/bborbe/time"

// MetricsSession records one work-on run: the Claude session it started and when.
type MetricsSession struct {
	SessionID string                 `yaml:"session_id" json:"session_id"`
	StartedAt libtime.DateOrDateTime `yaml:"started_at" json:"started_at"`
}

// MetricsCycle archives one finished cycle of a recurring task: when it ran,
// when it ended, and how many user interactions it had.
type MetricsCycle struct {
	StartedAt        libtime.DateOrDateTime `yaml:"started_at"        json:"started_at"`
	CompletedAt      libtime.DateOrDateTime `yaml:"completed_at"      json:"completed_at"`
	InteractionCount int                    `yaml:"interaction_count" json:"interaction_count"`
}
