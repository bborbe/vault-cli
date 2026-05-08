// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import libtime "github.com/bborbe/time"

// Decision represents a markdown file in the vault that has needs_review frontmatter.
type Decision struct {
	// Frontmatter fields
	NeedsReview  bool                    `yaml:"needs_review"`
	Reviewed     bool                    `yaml:"reviewed,omitempty"`
	ReviewedDate *libtime.DateOrDateTime `yaml:"-"` // managed by storage layer
	Status       string                  `yaml:"status,omitempty"`
	Type         string                  `yaml:"type,omitempty"`
	PageType     string                  `yaml:"page_type,omitempty"`

	// Metadata — excluded from YAML serialization
	Name     string `yaml:"-"` // Relative path from vault root without .md extension
	Content  string `yaml:"-"` // Full markdown content including frontmatter
	FilePath string `yaml:"-"` // Absolute path to file
}

// DecisionID represents a decision identifier (relative vault path without .md extension).
type DecisionID string

func (d DecisionID) String() string {
	return string(d)
}
