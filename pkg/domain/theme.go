// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"time"
)

// Theme represents a theme in the Obsidian vault with YAML frontmatter.
type Theme struct {
	// Frontmatter fields
	Status     ThemeStatus `yaml:"status"`
	PageType   string      `yaml:"page_type"`
	Priority   Priority    `yaml:"priority,omitempty"`
	StartDate  *time.Time  `yaml:"start_date,omitempty"`
	TargetDate *time.Time  `yaml:"target_date,omitempty"`
	Tags       []string    `yaml:"tags,omitempty"`

	// Metadata
	Name     string // Filename without extension
	Content  string // Full markdown content including frontmatter
	FilePath string // Absolute path to file
}

// ThemeStatus represents the status of a theme.
type ThemeStatus string

const (
	ThemeStatusActive    ThemeStatus = "active"
	ThemeStatusCompleted ThemeStatus = "completed"
	ThemeStatusArchived  ThemeStatus = "archived"
)

// ThemeID represents a theme identifier (filename without .md extension).
type ThemeID string

func (t ThemeID) String() string {
	return string(t)
}
