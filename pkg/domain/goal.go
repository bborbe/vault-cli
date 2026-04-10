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
type GoalStatus string

const (
	GoalStatusActive    GoalStatus = "active"
	GoalStatusCompleted GoalStatus = "completed"
	GoalStatusOnHold    GoalStatus = "on_hold"
)

// AvailableGoalStatuses lists all valid canonical goal status values.
var AvailableGoalStatuses = GoalStatuses{
	GoalStatusActive,
	GoalStatusCompleted,
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
