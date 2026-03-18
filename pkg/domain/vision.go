// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import "time"

// Vision represents a vision in the Obsidian vault with YAML frontmatter.
type Vision struct {
	// Frontmatter fields
	Status   VisionStatus `yaml:"status"`
	PageType string       `yaml:"page_type"`
	Priority Priority     `yaml:"priority,omitempty"`
	Assignee string       `yaml:"assignee,omitempty"`
	Tags     []string     `yaml:"tags,omitempty"`

	// Metadata
	Name         string     `yaml:"-"` // Filename without extension
	Content      string     `yaml:"-"` // Full markdown content including frontmatter
	FilePath     string     `yaml:"-"` // Absolute path to file
	ModifiedDate *time.Time `yaml:"-"` // File modification time, populated by storage layer
}

// VisionStatus represents the status of a vision.
type VisionStatus string

const (
	VisionStatusActive    VisionStatus = "active"
	VisionStatusCompleted VisionStatus = "completed"
	VisionStatusArchived  VisionStatus = "archived"
)

// VisionID represents a vision identifier (filename without .md extension).
type VisionID string

func (v VisionID) String() string {
	return string(v)
}
