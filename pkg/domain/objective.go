// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain //nolint:dupl

import (
	"context"
	"fmt"

	"github.com/bborbe/collection"
	"github.com/bborbe/validation"
)

// Objective represents an objective in the Obsidian vault.
// Frontmatter is stored in ObjectiveFrontmatter (a typed map wrapper that preserves
// unknown fields). Filesystem metadata is in the embedded FileMetadata.
type Objective struct {
	ObjectiveFrontmatter
	FileMetadata
	// Content is the full markdown content including the frontmatter block.
	Content Content
}

// NewObjective creates an Objective from a parsed frontmatter map and metadata.
func NewObjective(data map[string]any, meta FileMetadata, content Content) *Objective {
	return &Objective{
		ObjectiveFrontmatter: NewObjectiveFrontmatter(data),
		FileMetadata:         meta,
		Content:              content,
	}
}

// ObjectiveStatus represents the status of an objective.
type ObjectiveStatus string

const (
	ObjectiveStatusActive    ObjectiveStatus = "active"
	ObjectiveStatusCompleted ObjectiveStatus = "completed"
	ObjectiveStatusOnHold    ObjectiveStatus = "on_hold"
)

// AvailableObjectiveStatuses lists all valid canonical objective status values.
var AvailableObjectiveStatuses = ObjectiveStatuses{
	ObjectiveStatusActive,
	ObjectiveStatusCompleted,
	ObjectiveStatusOnHold,
}

// ObjectiveStatuses is a collection of ObjectiveStatus values.
type ObjectiveStatuses []ObjectiveStatus

// Contains returns true if the collection contains the given status.
func (o ObjectiveStatuses) Contains(status ObjectiveStatus) bool {
	return collection.Contains(o, status)
}

// Validate returns an error if the status is not a valid canonical value.
func (s ObjectiveStatus) Validate(ctx context.Context) error {
	if !AvailableObjectiveStatuses.Contains(s) {
		return fmt.Errorf("%w: unknown objective status '%s'", validation.Error, s)
	}
	return nil
}

// ObjectiveID represents an objective identifier (filename without .md extension).
type ObjectiveID string

func (o ObjectiveID) String() string {
	return string(o)
}
