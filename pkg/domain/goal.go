// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"fmt"

	"github.com/bborbe/collection"
	"github.com/bborbe/validation"
)

// Goal represents a goal in the Obsidian vault.
// Frontmatter is stored in GoalFrontmatter (a typed map wrapper that preserves
// unknown fields). Filesystem metadata is in the embedded FileMetadata.
type Goal struct {
	GoalFrontmatter
	FileMetadata
	// Content is the full markdown content including the frontmatter block.
	Content Content

	// Tasks holds checkbox items parsed from content.
	// It is populated by the storage layer and is NOT stored in frontmatter.
	Tasks []CheckboxItem
}

// NewGoal creates a Goal from a parsed frontmatter map and metadata.
func NewGoal(data map[string]any, meta FileMetadata, content Content) *Goal {
	return &Goal{
		GoalFrontmatter: NewGoalFrontmatter(data),
		FileMetadata:    meta,
		Content:         content,
	}
}

// GoalStatus represents the status of a goal.
//
// The enum is aligned with the task status taxonomy so a single mental model
// and a single UI control set (e.g. task-orchestrator's Kanban columns) can
// act on both tasks and goals. The original 3 values (active, completed,
// on_hold) are retained as accepted aliases for backward compatibility with
// existing goal files: active ↔ in_progress semantically, on_hold ↔ hold
// semantically. New writes from agents are expected to use the new canonical
// values; reads tolerate both.
type GoalStatus string

const (
	GoalStatusNext       GoalStatus = "next"
	GoalStatusInProgress GoalStatus = "in_progress"
	GoalStatusBacklog    GoalStatus = "backlog"
	GoalStatusCompleted  GoalStatus = "completed"
	GoalStatusHold       GoalStatus = "hold"
	GoalStatusAborted    GoalStatus = "aborted"

	// Legacy values — accepted on read + write for backward compatibility.
	// New code should prefer GoalStatusInProgress / GoalStatusHold.
	GoalStatusActive GoalStatus = "active"
	GoalStatusOnHold GoalStatus = "on_hold"
)

// AvailableGoalStatuses lists all valid canonical goal status values.
// Includes both the new task-aligned set and the legacy 3-value set so
// existing vault files (which may use either) continue to validate.
var AvailableGoalStatuses = GoalStatuses{
	GoalStatusNext,
	GoalStatusInProgress,
	GoalStatusBacklog,
	GoalStatusCompleted,
	GoalStatusHold,
	GoalStatusAborted,
	GoalStatusActive,
	GoalStatusOnHold,
}

// GoalStatuses is a collection of GoalStatus values.
type GoalStatuses []GoalStatus

// Contains returns true if the collection contains the given status.
func (g GoalStatuses) Contains(status GoalStatus) bool {
	return collection.Contains(g, status)
}

// Validate returns an error if the status is not a valid canonical value.
func (s GoalStatus) Validate(ctx context.Context) error {
	if !AvailableGoalStatuses.Contains(s) {
		return fmt.Errorf("%w: unknown goal status '%s'", validation.Error, s)
	}
	return nil
}

// GoalID represents a goal identifier (filename without .md extension).
type GoalID string

func (g GoalID) String() string {
	return string(g)
}

// CheckboxItem represents a checkbox item in markdown content.
type CheckboxItem struct {
	Line       int
	Checked    bool
	InProgress bool
	Text       string
	RawLine    string
}
