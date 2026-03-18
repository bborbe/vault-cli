// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"time"

	libtime "github.com/bborbe/time"
)

// Objective represents an objective in the Obsidian vault with YAML frontmatter.
type Objective struct {
	// Frontmatter fields
	Status     ObjectiveStatus `yaml:"status"`
	PageType   string          `yaml:"page_type"`
	Priority   Priority        `yaml:"priority,omitempty"`
	Assignee   string          `yaml:"assignee,omitempty"`
	StartDate  *time.Time      `yaml:"start_date,omitempty"`
	TargetDate *time.Time      `yaml:"target_date,omitempty"`
	Tags       []string        `yaml:"tags,omitempty"`
	Completed  *libtime.Date   `yaml:"completed,omitempty"`

	// Metadata
	Name         string     `yaml:"-"` // Filename without extension
	Content      string     `yaml:"-"` // Full markdown content including frontmatter
	FilePath     string     `yaml:"-"` // Absolute path to file
	ModifiedDate *time.Time `yaml:"-"` // File modification time, populated by storage layer
}

// ObjectiveStatus represents the status of an objective.
type ObjectiveStatus string

const (
	ObjectiveStatusActive    ObjectiveStatus = "active"
	ObjectiveStatusCompleted ObjectiveStatus = "completed"
	ObjectiveStatusOnHold    ObjectiveStatus = "on_hold"
)

// ObjectiveID represents an objective identifier (filename without .md extension).
type ObjectiveID string

func (o ObjectiveID) String() string {
	return string(o)
}
