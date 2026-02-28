// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"time"
)

// Goal represents a goal in the Obsidian vault with YAML frontmatter.
type Goal struct {
	// Frontmatter fields
	Status     GoalStatus `yaml:"status"`
	PageType   string     `yaml:"page_type"`
	Theme      string     `yaml:"theme,omitempty"`
	Priority   Priority   `yaml:"priority,omitempty"`
	StartDate  *time.Time `yaml:"start_date,omitempty"`
	TargetDate *time.Time `yaml:"target_date,omitempty"`
	Tags       []string   `yaml:"tags,omitempty"`

	// Metadata
	Name     string         // Filename without extension
	Content  string         // Full markdown content including frontmatter
	FilePath string         // Absolute path to file
	Tasks    []CheckboxItem // Parsed checkbox tasks from content
}

// GoalStatus represents the status of a goal.
type GoalStatus string

const (
	GoalStatusActive    GoalStatus = "active"
	GoalStatusCompleted GoalStatus = "completed"
	GoalStatusOnHold    GoalStatus = "on_hold"
)

// GoalID represents a goal identifier (filename without .md extension).
type GoalID string

func (g GoalID) String() string {
	return string(g)
}

// CheckboxItem represents a checkbox item in markdown content.
type CheckboxItem struct {
	Line    int
	Checked bool
	Text    string
	RawLine string
}
