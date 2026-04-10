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

// Vision represents a vision in the Obsidian vault.
// Frontmatter is stored in VisionFrontmatter (a typed map wrapper that preserves
// unknown fields). Filesystem metadata is in the embedded FileMetadata.
type Vision struct {
	VisionFrontmatter
	FileMetadata
	// Content is the full markdown content including the frontmatter block.
	Content Content
}

// NewVision creates a Vision from a parsed frontmatter map and metadata.
func NewVision(data map[string]any, meta FileMetadata, content Content) *Vision {
	return &Vision{
		VisionFrontmatter: NewVisionFrontmatter(data),
		FileMetadata:      meta,
		Content:           content,
	}
}

// VisionStatus represents the status of a vision.
type VisionStatus string

const (
	VisionStatusActive    VisionStatus = "active"
	VisionStatusCompleted VisionStatus = "completed"
	VisionStatusArchived  VisionStatus = "archived"
)

// AvailableVisionStatuses lists all valid canonical vision status values.
var AvailableVisionStatuses = VisionStatuses{
	VisionStatusActive,
	VisionStatusCompleted,
	VisionStatusArchived,
}

// VisionStatuses is a collection of VisionStatus values.
type VisionStatuses []VisionStatus

// Contains returns true if the collection contains the given status.
func (v VisionStatuses) Contains(status VisionStatus) bool {
	return collection.Contains(v, status)
}

// Validate returns an error if the status is not a valid canonical value.
func (s VisionStatus) Validate(ctx context.Context) error {
	if !AvailableVisionStatuses.Contains(s) {
		return fmt.Errorf("%w: unknown vision status '%s'", validation.Error, s)
	}
	return nil
}

// VisionID represents a vision identifier (filename without .md extension).
type VisionID string

func (v VisionID) String() string {
	return string(v)
}
